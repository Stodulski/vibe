package httpx

import (
	"fmt"
	"net/http"
	"strings"
)

// Guard wraps a handler with a check that may reject the request before it
// runs — authentication, ownership, or role.
type Guard func(http.HandlerFunc) http.HandlerFunc

// Guards is the set of protections a domain module can ask for when it
// registers its routes. The module declares which of its routes need which
// guard; the application supplies the implementations.
//
// It exists so a domain package can own its route table without importing the
// middleware that enforces access, which would point the dependency the wrong
// way and make every domain depend on every other one's auth needs.
type Guards struct {
	// RequireAuth rejects requests without a valid session.
	RequireAuth Guard
	// RequireComplexOwner rejects requests from users who do not own the
	// complex named in the route, and puts that complex in the context.
	RequireComplexOwner Guard
	// RequireSuperAdmin rejects requests from users without the superadmin role.
	RequireSuperAdmin Guard
	// Idempotent makes a route replay its first answer to a repeat of the same
	// request under the same Idempotency-Key, rather than running it twice.
	// The scope separates one endpoint's keys from another's, so a caller
	// reusing "1" on two endpoints is not replayed the wrong answer.
	//
	// A request without the header is unaffected, which is what lets a caller
	// adopt it one endpoint at a time.
	Idempotent func(scope string) Guard
}

// Validate reports which guards are missing.
//
// A nil guard is a wiring mistake, and without this check it stays invisible
// until a request reaches the route it was meant to protect — at which point
// the endpoint is either panicking or, worse, open. Calling this while the
// route table is built turns that into a startup failure.
func (g Guards) Validate() error {
	var missing []string
	if g.RequireAuth == nil {
		missing = append(missing, "RequireAuth")
	}
	if g.RequireComplexOwner == nil {
		missing = append(missing, "RequireComplexOwner")
	}
	if g.RequireSuperAdmin == nil {
		missing = append(missing, "RequireSuperAdmin")
	}
	if g.Idempotent == nil {
		missing = append(missing, "Idempotent")
	}
	if len(missing) > 0 {
		return fmt.Errorf("httpx: unwired guards: %s", strings.Join(missing, ", "))
	}
	return nil
}
