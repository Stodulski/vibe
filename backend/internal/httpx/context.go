package httpx

import (
	"context"
	"net/http"

	"github.com/stodulski/vibe-server/internal/data"
)

// Context keys are unexported struct types so that no package outside httpx can
// construct one, and so they cannot collide with keys set by other libraries.
type (
	userContextKey      struct{}
	complexContextKey   struct{}
	requestIDContextKey struct{}
)

// ContextSetUser returns a copy of r carrying the authenticated user.
func ContextSetUser(r *http.Request, user *data.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userContextKey{}, user))
}

// ContextGetAuthenticatedUser returns the user set by the authentication
// middleware. The boolean is false on unauthenticated requests.
func ContextGetAuthenticatedUser(r *http.Request) (*data.User, bool) {
	user, ok := r.Context().Value(userContextKey{}).(*data.User)
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
func ContextSetComplex(r *http.Request, c *data.Complex) *http.Request {
	ctx := context.WithValue(r.Context(), complexContextKey{}, c)
	return r.WithContext(data.ContextWithTenant(ctx, c.ID))
}

// ContextGetComplex returns the complex set by the ownership middleware. The
// boolean is false on routes that are not scoped to a complex.
func ContextGetComplex(r *http.Request) (*data.Complex, bool) {
	c, ok := r.Context().Value(complexContextKey{}).(*data.Complex)
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
