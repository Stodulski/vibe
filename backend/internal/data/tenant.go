package data

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// This file is how the tenant a request is acting for reaches the SQL session,
// so that the row-level security policies in db/migrations/001_init.sql have something
// to compare against.
//
// THE SHAPE OF IT
//
// Two PostgreSQL settings decide what a session can see:
//
//	app.complex_id     the tenant, as a uuid in text form, or the empty string
//	app.bypass_tenant  'on' for the paths that legitimately span tenants
//
// Every tenant policy in db/migrations/001_init.sql is the same pair of permissive rules: the row's
// complex_id equals app.complex_id, or app.bypass_tenant is 'on'. Neither set
// means neither matches, so a session that arrives without a declared scope
// sees nothing and can write nothing. That is the point: a path somebody
// forgot to scope fails visibly rather than reading somebody else's rows.
//
// The scope travels on the context, because that is the only thing already
// threaded from the middleware through every handler into every store method.
// It is set in three places and nowhere else:
//
//   - httpx.ContextSetComplex, which the RequireComplexOwner middleware calls
//     once it has verified that the caller owns the complex named in the URL.
//     That single line puts every owner-scoped route — the whole surface
//     finding F03 is about — under the policies.
//   - middleware.CrossTenantRoutes, the declared list of routes that resolve
//     their own tenant (the storefront by slug, the public booking link by
//     token hash, the MercadoPago webhook by payment id) or that span tenants
//     by design (the superadmin console). Those run with the bypass.
//   - the cron wrapper in cmd/api/cron.go, because a sweep that expires
//     pending payments across the platform is cross-tenant by definition.
//
// HOW IT REACHES POSTGRESQL
//
// Two mechanisms, because there are two ways a statement gets to the server.
//
//  1. StampTenantScope is the pool's PrepareConn hook (cmd/api/main.go). It
//     runs on every checkout, before the borrower's first statement, and sets
//     both settings at session level from the context doing the borrowing. This
//     is what covers the pool queries that are not in a transaction, which is
//     most reads: a streaming pgx.Rows cannot be wrapped in an implicit
//     transaction, because the caller consumes it after the function that
//     opened it has returned.
//
//     A connection therefore goes back to the pool still carrying the last
//     borrower's tenant. That is harmless only because the hook always writes
//     both settings, including the empty string and 'off' for a borrower with
//     no scope; a hook that skipped the unscoped case would hand the next
//     caller the previous caller's tenant. It never skips.
//
//  2. DB.Begin issues the same pair as SET LOCAL, inside the transaction it
//     just opened. Every transaction in this package goes through that one
//     method, so this needs no call-site changes. It is deliberately
//     redundant with the hook: SET LOCAL reverts at COMMIT or ROLLBACK whatever
//     the session was carrying, so a transaction's scope is a property of the
//     transaction rather than of whichever connection it landed on.
//
// WHY NOT A CONNECTION PER TENANT, OR A ROLE PER TENANT
//
// Both were considered. A pool per tenant multiplies idle connections by the
// number of venues and moves the tenant decision from the request to the pool
// registry, which is a worse place to forget it. A database role per tenant
// needs a CREATE ROLE on signup and turns a tenant list into a cluster-level
// catalog. One pool, one setting per checkout, is what the guidelines' §10
// describes and what the policies were written against.
//
// WHAT THIS DOES NOT REPLACE
//
// The hand-written `if row.ComplexID != complex.ID` comparison in the handlers
// stays. It answers a different question: it produces a 404 with a sentence a
// human wrote, for the case the handler thought about. This produces an empty
// result for the case it did not.

// tenantSettingName and bypassSettingName are the two PostgreSQL settings
// the tenant policies read. They are named here once because a typo in
// either is invisible: current_setting(..., true) answers NULL for a setting
// that was never set, so a misspelt name reads exactly like an unscoped
// session — every query returns nothing, and nothing says why.
const (
	tenantSettingName = "app.complex_id"
	// The nolint is a false positive with a real cause: gosec's G101 pattern
	// looks for "pass" anywhere in a string constant, and finds it inside
	// "bypass". There is no credential here — this is the name of a
	// PostgreSQL setting, and the tenant policies read it by that exact
	// spelling.
	bypassSettingName = "app.bypass_tenant" //nolint:gosec // G101: "bypass" contains "pass"; this is a setting name, not a credential

	bypassOn  = "on"
	bypassOff = "off"
)

// tenantScopeKey is the context key the scope travels under. It is an
// unexported struct type so no other package can construct one, which is what
// makes ContextWithTenant and ContextWithTenantBypass the only two ways in.
type tenantScopeKey struct{}

// tenantScope is what a context carries: either a tenant, or permission to
// cross tenants, or (the zero value, which is also the absence of the key)
// neither.
type tenantScope struct {
	// complexID is the tenant in text form, empty when there is none. It is
	// stored formatted because the pool hook runs on every checkout and the
	// alternative is formatting the same uuid thousands of times a second.
	complexID string
	bypass    bool
}

// ContextWithTenant returns a context that scopes every statement it reaches
// the database with to one complex.
//
// It clears any bypass the context was carrying. A request that has resolved
// its tenant no longer needs to cross tenants, and leaving the bypass in place
// would mean the narrower scope was the weaker one.
func ContextWithTenant(ctx context.Context, complexID uuid.UUID) context.Context {
	return context.WithValue(ctx, tenantScopeKey{}, tenantScope{complexID: complexID.String()})
}

// ContextWithTenantBypass returns a context whose statements see every tenant.
//
// It is for three kinds of work, and the call sites are listed in the file
// comment above so the set stays small enough to read: the sweeps that run on
// a schedule across the whole platform, the superadmin console, and the step
// that resolves which tenant a request is for — loading a complex by its id
// before the ownership check, or by its slug from a public URL, is itself a
// query, and it cannot be scoped to the tenant it is trying to discover.
func ContextWithTenantBypass(ctx context.Context) context.Context {
	return context.WithValue(ctx, tenantScopeKey{}, tenantScope{bypass: true})
}

// TenantFromContext returns the complex every statement on this context is
// scoped to. The boolean is false on a context that carries no tenant, which
// includes every context carrying the bypass.
func TenantFromContext(ctx context.Context) (uuid.UUID, bool) {
	scope, ok := ctx.Value(tenantScopeKey{}).(tenantScope)
	if !ok || scope.complexID == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(scope.complexID)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// TenantBypassed reports whether this context is allowed to cross tenants.
func TenantBypassed(ctx context.Context) bool {
	scope, ok := ctx.Value(tenantScopeKey{}).(tenantScope)
	return ok && scope.bypass
}

// scopeFrom returns the two values the settings take, for any context.
//
// The zero value — no tenant, bypass off — is what an unscoped context gets,
// and it is what makes the policies fail closed.
func scopeFrom(ctx context.Context) (complexID, bypass string) {
	scope, _ := ctx.Value(tenantScopeKey{}).(tenantScope)
	if scope.bypass {
		return scope.complexID, bypassOn
	}
	return scope.complexID, bypassOff
}

// stampSession is the statement that writes both settings. set_config is used
// rather than SET because SET takes no parameters: the tenant would have to be
// interpolated into the SQL text, and a tenant id that arrives as a string and
// is concatenated into a statement is the shape of every injection there has
// ever been. The third argument is is_local — false for the session, true for
// the current transaction only.
const stampSession = `SELECT set_config($1, $2, false), set_config($3, $4, false)`

const stampTransaction = `SELECT set_config($1, $2, true), set_config($3, $4, true)`

// StampTenantScope writes ctx's scope onto conn as session settings. It is the
// pool's PrepareConn hook: pgxpool calls it on every checkout, with the context
// of the caller doing the checking out, before that caller's first statement.
//
// The return values follow pgxpool's contract for PrepareConn. A connection
// that is already gone answers (false, nil): the pool destroys it and retries
// the query on another connection, which is the only correct thing to do with
// a dead socket. This happens in ordinary operation, not only in failures: the
// pool retires connections at MaxConnLifetime, and the server drops idle ones,
// and a checkout can race either. Answering (true, err) here instead, as this
// hook once did, turned every such race into a 500 for whoever was borrowing
// the connection.
//
// A statement failure on a live connection still answers (true, err): the
// connection is fine and goes back to the pool, and the query that asked for
// it fails, because it must never run carrying somebody else's tenant.
func StampTenantScope(ctx context.Context, conn *pgx.Conn) (bool, error) {
	if conn.IsClosed() {
		return false, nil
	}

	complexID, bypass := scopeFrom(ctx)

	_, err := conn.Exec(ctx, stampSession, tenantSettingName, complexID, bypassSettingName, bypass)
	if err != nil {
		if conn.IsClosed() {
			return false, nil
		}
		return true, fmt.Errorf("stamping the tenant scope on a pooled connection: %w", err)
	}
	return true, nil
}

// stampTx writes ctx's scope onto tx with SET LOCAL, so it reverts when the
// transaction ends whatever the connection was carrying before.
func stampTx(ctx context.Context, tx pgx.Tx) error {
	complexID, bypass := scopeFrom(ctx)

	_, err := tx.Exec(ctx, stampTransaction, tenantSettingName, complexID, bypassSettingName, bypass)
	if err != nil {
		return fmt.Errorf("stamping the tenant scope on a transaction: %w", err)
	}
	return nil
}
