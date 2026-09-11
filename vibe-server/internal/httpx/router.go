package httpx

import "net/http"

// Router is the slice of the HTTP router that route registration needs.
//
// Domain modules register against this rather than against a concrete router
// type, for two reasons: the routing library stays an application-level choice
// instead of a dependency of every domain module, and the route table can be
// walked by passing a recorder — which is how TestEveryRouteIsGuarded proves no
// endpoint ships without a guard.
type Router interface {
	// HandlerFunc registers handler for method and path.
	HandlerFunc(method, path string, handler http.HandlerFunc)
	// Handler registers a plain http.Handler, for endpoints supplied by
	// another package such as expvar or pprof.
	Handler(method, path string, handler http.Handler)
}
