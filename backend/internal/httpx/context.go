package httpx

import (
	"context"
	"net/http"
	"sync"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// Context keys are unexported struct types so that no package outside httpx can
// construct one, and so they cannot collide with keys set by other libraries.
type (
	userContextKey      struct{}
	complexContextKey   struct{}
	requestIDContextKey struct{}
	actorContextKey     struct{}
)

// Actor is the slot the authentication middleware writes the authenticated
// actor's id into, so that middleware which ran before authentication can
// still name who the request turned out to be.
//
// It exists because the request log is written by a middleware that wraps the
// authenticator: its own *http.Request was copied and replaced downstream by
// the time authentication finished, so the id cannot be read back out of the
// context it holds. A mutable slot placed in the context before the chain runs
// is the one thing both halves can see.
//
// It carries the id and nothing else. A log line naming a user is a support
// ticket answered; a log line carrying their email address is personal data in
// a log aggregator nobody scoped for it.
type Actor struct {
	mu sync.RWMutex
	id string
}

// set records the id. It is called once per request, by ContextSetUser.
func (a *Actor) set(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.id = id
}

// ID is the authenticated actor's id, or "" on a request that never
// authenticated.
func (a *Actor) ID() string {
	if a == nil {
		return ""
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.id
}

// ContextWithActor returns a context carrying an empty actor slot, and the
// slot itself for the caller to read once the chain below it has run.
func ContextWithActor(ctx context.Context) (context.Context, *Actor) {
	actor := &Actor{}
	return context.WithValue(ctx, actorContextKey{}, actor), actor
}

// contextActor returns the slot placed by ContextWithActor, if there is one.
func contextActor(ctx context.Context) *Actor {
	actor, _ := ctx.Value(actorContextKey{}).(*Actor)
	return actor
}

// ContextSetUser returns a copy of r carrying the authenticated user, and
// records the actor's id in the slot any outer middleware left for it.
func ContextSetUser(r *http.Request, user *authstore.User) *http.Request {
	if user != nil {
		if actor := contextActor(r.Context()); actor != nil {
			actor.set(user.ID.String())
		}
	}
	return r.WithContext(context.WithValue(r.Context(), userContextKey{}, user))
}

// ContextGetAuthenticatedUser returns the user set by the authentication
// middleware. The boolean is false on unauthenticated requests.
func ContextGetAuthenticatedUser(r *http.Request) (*authstore.User, bool) {
	user, ok := r.Context().Value(userContextKey{}).(*authstore.User)
	if !ok {
		return nil, false
	}
	return user, true
}

// ContextSetComplex returns a copy of r carrying the complex whose ownership
// has already been verified, scoped to that complex for the database as well.
//
// The second half is the whole of the row-level-security wiring for every
// owner-guarded route. Row-level security gives each tenant table a policy that
// compares its complex_id against the app.complex_id setting, and
// data.ContextWithTenant is what puts a value there — the pool stamps it on
// the connection at checkout and DB.Begin repeats it as SET LOCAL inside each
// transaction. So a handler that forgets its own `if row.ComplexID !=
// complex.ID` comparison now gets an empty result from the database instead of
// another tenant's row.
//
// Ownership is verified before this is called (middleware.RequireComplexOwner
// is the only production caller), which is what makes it safe to widen the
// database session to that tenant here.
func ContextSetComplex(r *http.Request, c *complexstore.Complex) *http.Request {
	ctx := context.WithValue(r.Context(), complexContextKey{}, c)
	return r.WithContext(data.ContextWithTenant(ctx, c.ID))
}

// ContextGetComplex returns the complex set by the ownership middleware. The
// boolean is false on routes that are not scoped to a complex.
func ContextGetComplex(r *http.Request) (*complexstore.Complex, bool) {
	c, ok := r.Context().Value(complexContextKey{}).(*complexstore.Complex)
	return c, ok
}

// ContextSetRequestID returns a copy of r carrying the per-request correlation
// id that error logs and the X-Request-ID response header report.
func ContextSetRequestID(r *http.Request, id string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, id))
}

// ContextGetRequestID returns the correlation id, or "" when the request never
// passed through the request-id middleware.
func ContextGetRequestID(r *http.Request) string {
	if id, ok := r.Context().Value(requestIDContextKey{}).(string); ok {
		return id
	}
	return ""
}
