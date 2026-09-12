//go:build integration

package data_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/stores"
)

// These tests are the only ones in the repository that connect as the
// application role. Everything else in the suite connects as DATABASE_URL,
// which is a superuser today, and a superuser carries rolbypassrls: run as
// that, every assertion below would pass without a single policy existing.
//
// So they take two DSNs of their own:
//
//	DATABASE_APP_URL    vibe_app, the role the tenant policies exist for.
//	                    Unset, these tests skip — visibly, with the reason.
//	DATABASE_ADMIN_URL  a role that owns the schema (or a superuser), used
//	                    only to build and tear down fixtures. Falls back to
//	                    DATABASE_URL, which is what it is today.
//
// The split is not decoration. The fixtures below insert rows for two
// different tenants and read them back to check what a tenant could see; a
// fixture that had to satisfy the policies in order to write its own rows
// could not set up the cross-tenant case at all, and TRUNCATE and
// ALTER TABLE ... DISABLE TRIGGER, which the rest of this package's harness
// uses, need ownership that vibe_app deliberately does not have.
//
// The exact psql commands that create the two roles on a test cluster are in
// the Makefile's e2e-db-roles target.

// appPool opens a pool as the application role, wired exactly as
// cmd/api/main.go wires the real one: same exec mode, and the same PrepareConn
// hook that stamps the tenant scope from the context on every checkout. A test
// that opened a plain pool would be testing a mechanism the application does
// not use.
func appPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_APP_URL")
	if dsn == "" {
		t.Skip("DATABASE_APP_URL not set, skipping the row-level-security tests (they must not run as a superuser)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsing DATABASE_APP_URL: %v", err)
	}
	config.MaxConns = 4
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	config.PrepareConn = data.StampTenantScope

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connecting as the application role: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pinging as the application role: %v", err)
	}
	t.Cleanup(pool.Close)

	// A test that is silently running as a superuser proves nothing, and the
	// symptom would be a green suite. Refuse rather than skip: an operator who
	// pointed DATABASE_APP_URL at the wrong DSN wants to hear about it.
	var (
		user     string
		super    bool
		bypasses bool
	)
	err = pool.QueryRow(ctx, `
		SELECT r.rolname, r.rolsuper, r.rolbypassrls
		FROM pg_roles r WHERE r.rolname = current_user`).Scan(&user, &super, &bypasses)
	if err != nil {
		t.Fatalf("reading the application role's attributes: %v", err)
	}
	if super || bypasses {
		t.Fatalf("DATABASE_APP_URL connects as %q, which is rolsuper=%v rolbypassrls=%v: "+
			"row-level security does not apply to it and these tests would pass on nothing", user, super, bypasses)
	}

	return pool
}

// adminPool opens the pool the fixtures are built with.
func adminPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_ADMIN_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("neither DATABASE_ADMIN_URL nor DATABASE_URL is set, skipping")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsing the admin DSN: %v", err)
	}
	config.MaxConns = 4
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connecting with the admin DSN: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pinging with the admin DSN: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// tenant is one complex's worth of rows, owned by its own user.
type tenant struct {
	UserID    uuid.UUID
	ComplexID uuid.UUID
	CourtID   uuid.UUID
	ClientID  uuid.UUID
	BookingID uuid.UUID
}

// rlsFixture is two tenants that know nothing about each other, plus the two
// pools: one to build them with, one to run the code under test as.
type rlsFixture struct {
	Admin  *pgxpool.Pool
	App    *pgxpool.Pool
	Stores stores.Stores
	A      tenant
	B      tenant
}

func newRLSFixture(t *testing.T) *rlsFixture {
	t.Helper()

	f := &rlsFixture{Admin: adminPool(t), App: appPool(t)}
	f.Stores = stores.New(f.App, stores.Config{Keys: datatest.CredentialKeyring(t)})
	f.A = f.seedTenant(t, "a")
	f.B = f.seedTenant(t, "b")
	return f
}

// seedTenant writes one complete tenant with the admin pool, and registers the
// cleanup that removes it.
func (f *rlsFixture) seedTenant(t *testing.T, label string) tenant {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	suffix := uuid.NewString()
	var tn tenant

	mustScan := func(query string, args ...any) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := f.Admin.QueryRow(ctx, query, args...).Scan(&id); err != nil {
			t.Fatalf("seeding tenant %s: %v", label, err)
		}
		return id
	}

	tn.UserID = mustScan(`
		INSERT INTO users (email, password_hash, first_name, last_name, phone, role, email_verified)
		VALUES ($1, $2, 'Owner', 'Rls', '+5491100000000', 'owner', true)
		RETURNING id`,
		"rls-"+label+"-"+suffix+"@example.test", []byte("not-a-real-hash"))

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		for _, statement := range []string{
			`DELETE FROM booking_link_tokens WHERE booking_id IN (SELECT id FROM bookings WHERE complex_id = $1)`,
			`DELETE FROM payments WHERE complex_id = $1`,
			`DELETE FROM bookings WHERE complex_id = $1`,
			`DELETE FROM blocked_slots WHERE court_id IN (SELECT id FROM courts WHERE complex_id = $1)`,
			`DELETE FROM court_prices WHERE court_id IN (SELECT id FROM courts WHERE complex_id = $1)`,
			`DELETE FROM clients WHERE complex_id = $1`,
			`DELETE FROM courts WHERE complex_id = $1`,
			`DELETE FROM audit_log WHERE complex_id = $1`,
			`DELETE FROM complexes WHERE id = $1`,
		} {
			if _, err := f.Admin.Exec(cleanupCtx, statement, tn.ComplexID); err != nil {
				t.Errorf("cleaning tenant %s (%s): %v", label, statement, err)
			}
		}
		if _, err := f.Admin.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, tn.UserID); err != nil {
			t.Errorf("cleaning tenant %s owner: %v", label, err)
		}
	})

	tn.ComplexID = mustScan(`
		INSERT INTO complexes (owner_id, name, slug, address, city, province, phone)
		VALUES ($1, $2, $3, 'Av. Siempreviva 742', 'Rosario', 'Santa Fe', '+5491100000001')
		RETURNING id`,
		tn.UserID, "RLS "+label, "rls-"+label+"-"+suffix)

	tn.CourtID = mustScan(`
		INSERT INTO courts (complex_id, name) VALUES ($1, $2) RETURNING id`,
		tn.ComplexID, "Court "+label)

	tn.ClientID = mustScan(`
		INSERT INTO clients (complex_id, first_name, last_name, phone)
		VALUES ($1, 'Ana', 'Diaz', $2) RETURNING id`,
		tn.ComplexID, "+54911"+suffix[:8])

	tn.BookingID = mustScan(insertBookingSQL, tn.ComplexID, tn.CourtID, tn.ClientID)

	return tn
}

// insertBookingSQL is the one place these tests spell the bookings table's
// required columns, so that a migration that adds or removes one is a single
// edit here. Everything not named has a default.
const insertBookingSQL = `
	INSERT INTO bookings (complex_id, court_id, client_id, date, start_time, duration_minutes, price)
	VALUES ($1, $2, $3, CURRENT_DATE + 7, '10:00', 90, 500000)
	RETURNING id`

// insertBookingAtAFreeHourSQL writes to an hour no fixture booking occupies.
//
// The cross-tenant insert test needs it: with the seeded hour, an insert
// naming another tenant's court is refused by the overlap exclusion constraint
// from bookings_no_overlapping_span before any policy is consulted, and the test would then
// pass whether or not WITH CHECK exists. Moving the hour leaves the policy as
// the only thing that can refuse it.
const insertBookingAtAFreeHourSQL = `
	INSERT INTO bookings (complex_id, court_id, client_id, date, start_time, duration_minutes, price)
	VALUES ($1, $2, $3, CURRENT_DATE + 7, '15:00', 90, 500000)
	RETURNING id`

// TestABookingOfAnotherTenantIsInvisible is the finding this whole change
// exists for. The by-id read used to be `WHERE id = $1` with no tenant
// predicate at all, and twelve handlers compared the row's complex_id by hand
// afterwards; it carries one now (005_tenant_columns.sql, TEN-01), and this
// asks what the policies do when the query runs anyway. The predicate's own
// half is proved with the policies switched off, in
// tenant_columns_integration_test.go.
func TestABookingOfAnotherTenantIsInvisible(t *testing.T) {
	f := newRLSFixture(t)

	own, err := f.Stores.Bookings.GetByID(data.ContextWithTenant(context.Background(), f.A.ComplexID), f.A.BookingID)
	if err != nil {
		t.Fatalf("tenant A reading its own booking: %v", err)
	}
	if own.ID != f.A.BookingID {
		t.Fatalf("tenant A read booking %s, want its own %s", own.ID, f.A.BookingID)
	}

	stolen, err := f.Stores.Bookings.GetByID(data.ContextWithTenant(context.Background(), f.A.ComplexID), f.B.BookingID)
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("tenant A read tenant B's booking %s and got (%+v, %v), want ErrRecordNotFound: "+
			"a by-id query with no tenant predicate returned another tenant's row", f.B.BookingID, stolen, err)
	}
}

// TestInsertingAnotherTenantsRowIsRefused is the write half. USING decides what
// a SELECT can see; WITH CHECK decides what an INSERT is allowed to leave
// behind, and without it a handler could stamp another tenant's id onto a row
// it creates.
func TestInsertingAnotherTenantsRowIsRefused(t *testing.T) {
	f := newRLSFixture(t)

	ctx := data.ContextWithTenant(context.Background(), f.A.ComplexID)

	var id uuid.UUID
	err := f.App.QueryRow(ctx, insertBookingAtAFreeHourSQL, f.B.ComplexID, f.B.CourtID, f.B.ClientID).Scan(&id)
	if err == nil {
		t.Fatalf("an insert naming tenant B's complex succeeded while the session was scoped to tenant A "+
			"(booking %s); WITH CHECK did not refuse it", id)
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("insert failed with %v, want SQLSTATE 42501 (insufficient_privilege) from the row-security policy", err)
	}
}

// TestTheCrossTenantBypassSeesBothTenants is the other direction, and it has to
// be tested or the bypass is a claim rather than a mechanism: the fourteen cron
// sweeps and the superadmin console genuinely need to read every tenant, and if
// the bypass did not work they would report finding nothing to do — the worst
// possible way for this to fail, because it is silent.
func TestTheCrossTenantBypassSeesBothTenants(t *testing.T) {
	f := newRLSFixture(t)

	ctx := data.ContextWithTenantBypass(context.Background())

	for _, want := range []struct {
		name string
		id   uuid.UUID
	}{{"A", f.A.BookingID}, {"B", f.B.BookingID}} {
		got, err := f.Stores.Bookings.GetByID(ctx, want.id)
		if err != nil {
			t.Fatalf("a cross-tenant context could not read tenant %s's booking: %v", want.name, err)
		}
		if got.ID != want.id {
			t.Fatalf("read booking %s, want %s", got.ID, want.id)
		}
	}

	// And a context with neither a tenant nor the bypass sees nothing at all.
	// This is the fail-closed default, and it is what makes a path nobody
	// scoped break in a test rather than leak in production.
	if _, err := f.Stores.Bookings.GetByID(context.Background(), f.A.BookingID); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("an unscoped context read a booking (err = %v), want ErrRecordNotFound: "+
			"a session that declared no tenant must see nothing", err)
	}
}

// TestTheApplicationRoleCannotReshapeTheSchema is the roles' half. RLS is
// only worth having if the role it applies to cannot simply turn it off, and
// the wider point is that a defect on the request path should be a bounded data
// error rather than a DROP TABLE.
func TestTheApplicationRoleCannotReshapeTheSchema(t *testing.T) {
	f := newRLSFixture(t)
	ctx := context.Background()

	for _, statement := range []string{
		`ALTER TABLE bookings ADD COLUMN rls_probe int`,
		`ALTER TABLE bookings DISABLE ROW LEVEL SECURITY`,
		`DROP POLICY tenant_isolation ON bookings`,
		`DROP TABLE booking_link_tokens`,
		`TRUNCATE bookings`,
		`CREATE TABLE rls_probe (id int)`,
	} {
		_, err := f.App.Exec(ctx, statement)
		if err == nil {
			t.Fatalf("the application role was allowed to run %q; it owns nothing and must not be able to", statement)
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%q failed with %v, want SQLSTATE 42501 (insufficient_privilege)", statement, err)
		}
	}

	// It can still do the DML the application actually issues. Without this
	// half, a role with no privileges at all would pass the loop above.
	var reachable int
	if err := f.App.QueryRow(data.ContextWithTenant(ctx, f.A.ComplexID),
		`SELECT count(*) FROM bookings`).Scan(&reachable); err != nil {
		t.Fatalf("the application role cannot read the table it is supposed to: %v", err)
	}
	if reachable != 1 {
		t.Fatalf("tenant A sees %d bookings, want exactly its own 1", reachable)
	}
}

// TestTheFilteredViewsReturnOnlyTheCallersTenant is the active_* views
// under the tenant policies. A view runs with the privileges of its owner unless it
// is security_invoker, and the schema creates these two as ordinary views owned by
// vibe_migrator. Since the soft-delete work moved every public and owner-facing
// read of complexes and courts onto them, an ordinary view here would have been
// a hole through the middle of the read surface rather than a corner case.
func TestTheFilteredViewsReturnOnlyTheCallersTenant(t *testing.T) {
	f := newRLSFixture(t)

	ctx := data.ContextWithTenant(context.Background(), f.A.ComplexID)

	// Named, not counted. Counting assumes this database holds nothing but
	// these two fixtures, which is a property of the harness rather than of
	// the thing under test, and a leftover row from an earlier failure would
	// turn a real result into a flake.
	var (
		sawOwn     bool
		sawForeign bool
	)
	rows, err := f.App.Query(ctx, `SELECT id FROM active_complexes`)
	if err != nil {
		t.Fatalf("reading active_complexes: %v", err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scanning active_complexes: %v", err)
		}
		switch id {
		case f.A.ComplexID:
			sawOwn = true
		case f.B.ComplexID:
			sawForeign = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("reading active_complexes: %v", err)
	}
	if sawForeign {
		t.Fatalf("active_complexes returned tenant B's complex (%s) to a session scoped to tenant A (%s); "+
			"the view is not security_invoker, so it is answering as its owner and the policies do not apply through it",
			f.B.ComplexID, f.A.ComplexID)
	}
	if !sawOwn {
		t.Fatalf("active_complexes did not return tenant A's own complex (%s) to a session scoped to it", f.A.ComplexID)
	}

	var foreignCourts int
	if err := f.App.QueryRow(ctx,
		`SELECT count(*) FROM active_courts WHERE complex_id <> $1`, f.A.ComplexID).Scan(&foreignCourts); err != nil {
		t.Fatalf("reading active_courts: %v", err)
	}
	if foreignCourts != 0 {
		t.Fatalf("active_courts returned %d courts of another tenant", foreignCourts)
	}
}

// TestRowLevelSecurityIsTheSecondWallWithTheHandlerComparisonRemoved is the
// point of the whole exercise, stated as a test.
//
// Every owner-scoped handler does two things: it takes the complex the
// RequireComplexOwner middleware put in the request context, and it compares
// the loaded row's complex_id against it by hand. This replays the first step
// with the same call the middleware ends up making — httpx.ContextSetComplex
// is one line over ContextWithTenant, and TestContextSetComplexScopesTheTenant
// in internal/httpx pins that it stays one line over it — and then
// deliberately omits the second step, which is what a new handler does by
// forgetting a line.
//
// Before row-level security the omission returns the other tenant's booking. After
// it, there is no row left to compare.
func TestRowLevelSecurityIsTheSecondWallWithTheHandlerComparisonRemoved(t *testing.T) {
	f := newRLSFixture(t)

	// What the middleware leaves behind on a request for tenant A, after it
	// has verified that the caller owns that complex.
	ctx := data.ContextWithTenant(context.Background(), f.A.ComplexID)

	// What a handler with the comparison removed does: load by id, use it.
	// The id is tenant B's, which is exactly the request an attacker sends.
	booking, err := f.Stores.Bookings.GetByID(ctx, f.B.BookingID)
	if err == nil {
		t.Fatalf("a handler with no tenant comparison read tenant B's booking %s while serving tenant A; "+
			"the database was supposed to be the second wall and it did not hold", booking.ID)
	}
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("got %v, want ErrRecordNotFound", err)
	}
}

// TestTheTenantScopeReachesTheSessionSettings is the mechanism itself, checked
// against the database rather than against the Go code that sets it. A typo in
// either setting name would make every test above pass for the wrong reason —
// an unscoped session sees nothing, which is also what a correctly scoped one
// sees when it looks at another tenant.
func TestTheTenantScopeReachesTheSessionSettings(t *testing.T) {
	f := newRLSFixture(t)

	for _, tc := range []struct {
		name       string
		ctx        context.Context
		wantTenant string
		wantBypass string
	}{
		{"scoped", data.ContextWithTenant(context.Background(), f.A.ComplexID), f.A.ComplexID.String(), "off"},
		{"bypass", data.ContextWithTenantBypass(context.Background()), "", "on"},
		{"neither", context.Background(), "", "off"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotTenant, gotBypass string
			err := f.App.QueryRow(tc.ctx,
				`SELECT current_setting('app.complex_id', true), current_setting('app.bypass_tenant', true)`).
				Scan(&gotTenant, &gotBypass)
			if err != nil {
				t.Fatalf("reading the settings back: %v", err)
			}
			if gotTenant != tc.wantTenant || gotBypass != tc.wantBypass {
				t.Fatalf("session carries app.complex_id=%q app.bypass_tenant=%q, want %q and %q",
					gotTenant, gotBypass, tc.wantTenant, tc.wantBypass)
			}
		})
	}

	// The same, inside a transaction, because DB.Begin repeats the stamp as
	// SET LOCAL and a transaction that inherited the wrong connection's
	// setting would be the failure nobody sees.
	db := data.NewDB(f.App)
	tx, err := db.Begin(data.ContextWithTenant(context.Background(), f.B.ComplexID))
	if err != nil {
		t.Fatalf("beginning a scoped transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var inTx string
	if err := tx.QueryRow(context.Background(), `SELECT current_setting('app.complex_id', true)`).Scan(&inTx); err != nil {
		t.Fatalf("reading the setting inside the transaction: %v", err)
	}
	if inTx != f.B.ComplexID.String() {
		t.Fatalf("transaction carries app.complex_id=%q, want %q", inTx, f.B.ComplexID)
	}

	var visible int
	if err := tx.QueryRow(context.Background(), `SELECT count(*) FROM bookings`).Scan(&visible); err != nil {
		t.Fatalf("counting bookings inside the transaction: %v", err)
	}
	if visible != 1 {
		t.Fatalf("the transaction sees %d bookings, want tenant B's 1", visible)
	}
}
