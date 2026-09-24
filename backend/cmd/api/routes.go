package main

import (
	"expvar"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/rs/cors"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/middleware"
	gen "github.com/stodulski/vibe-server/internal/openapi/gen"
)

// routes builds the full HTTP handler: the route table wrapped in the
// middleware chain, outermost first.
func (app *application) routes() http.Handler {
	router := httpx.NewServeMux(
		http.HandlerFunc(app.respond.RouteNotFound),
		http.HandlerFunc(app.respond.MethodNotAllowed),
	)

	app.registerRoutes(router)

	// Spec validation wraps the router, so it runs before the per-route auth
	// guards (requireAuth, requireComplexOwner, requireRole) that live inside
	// each route's own handler chain: an unauthenticated or unauthorized
	// request is still checked against the document before those guards ever
	// see it.
	handler := router.Build()
	if app.specValidator != nil {
		handler = app.specValidator.ValidateRequests(handler)
	}

	c := cors.New(corsOptions(app.config.FrontendURL))

	return app.middleware.Wrap(handler, func(next http.Handler) http.Handler {
		return normalizeCORSPreflightHeaders(c.Handler(next))
	})
}

// corsOptions is the browser-facing CORS policy: one allowed origin, the
// methods the API answers, and the headers a request may carry.
//
// AllowedHeaders must name every header parameter openapi.yaml declares
// (Idempotency-Key and If-Match from components.parameters; X-Signature and
// X-Request-Id from the MercadoPago webhook operation), plus Authorization,
// Content-Type and X-CSRF-Token, which the document carries as security
// schemes rather than parameters and so are named here by hand. rs/cors
// applies one global policy to every route, so a header the webhook accepts
// is CORS-checked the same as one the frontend sends, even though a
// server-to-server webhook call is never actually preflighted by a browser.
// TestEveryDeclaredHeaderParameterIsAllowedByCORS (cors_test.go) walks the
// document and fails if a future header parameter is added here without a
// matching entry, instead of that only surfacing as a preflight rejection in
// production — Idempotency-Key did exactly that: see Sentry
// VIBE-FRONTEND-5, "Failed to fetch" on every cross-origin POST that carried
// it.
//
// ExposedHeaders is the one that is not about the request. Without it a browser
// hides every response header outside the CORS-safelist from the page's own
// JavaScript, X-Request-ID included — so the correlation id the server puts on
// every response, and asks support tickets to quote, was readable by curl and
// invisible to the app that would have to show it.
func corsOptions(frontendURL string) cors.Options {
	return cors.Options{
		AllowedOrigins: []string{frontendURL},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{
			"Authorization", "Content-Type", "X-CSRF-Token",
			"Idempotency-Key", "If-Match", "X-Signature", "X-Request-Id",
		},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           86400,
	}
}

// normalizeCORSPreflightHeaders lowercases an incoming
// Access-Control-Request-Headers value before rs/cors matches it against its
// (already-lowercased) AllowedHeaders set.
//
// A spec-compliant browser always sends that header pre-lowercased — the
// Fetch standard guarantees it — which is exactly what rs/cors relies on and
// documents in its own source. But nothing stops a non-browser client, an
// SDK, or a test tool from sending it mixed case (e.g. "X-CSRF-Token"
// instead of "x-csrf-token"), and rs/cors then fails the match and answers
// the preflight with no Access-Control-Allow-* headers at all, silently
// blocking that client. Normalizing here keeps the fix at the transport edge
// instead of touching the AllowedHeaders configuration, which was already
// correct.
func normalizeCORSPreflightHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			if reqHeaders, ok := r.Header["Access-Control-Request-Headers"]; ok {
				lowered := make([]string, len(reqHeaders))
				for i, h := range reqHeaders {
					lowered[i] = strings.ToLower(h)
				}
				r.Header["Access-Control-Request-Headers"] = lowered
			}
		}
		next.ServeHTTP(w, r)
	})
}

// registerRoutes populates the route table. It is separate from routes() so
// that the table can be walked without standing up the middleware chain — see
// TestEveryRouteIsGuarded, which enumerates it to prove no endpoint ships
// unprotected by accident.
func (app *application) registerRoutes(router httpx.Router) {
	// Every route gets the tenant posture its entry in
	// middleware.CrossTenantRoutes declares, or none — which is the
	// fail-closed default that leaves the tenant policies answering
	// "no rows" for a route nobody scoped. The wrapping happens here, inside
	// registerRoutes rather than in routes(), so that it cannot be skipped by
	// a caller that builds the table another way.
	router = middleware.TenantAwareRouter(router)

	// Public routes.
	if app.config.Env == "development" {
		router.Handler(http.MethodGet, "/debug/vars", expvar.Handler())
	}
	if app.config.PProf && app.config.Env == "development" {
		router.HandlerFunc(http.MethodGet, "/debug/pprof/", pprof.Index)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/cmdline", pprof.Cmdline)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/profile", pprof.Profile)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/symbol", pprof.Symbol)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/trace", pprof.Trace)
		router.Handler(http.MethodGet, "/debug/pprof/heap", pprof.Handler("heap"))
		router.Handler(http.MethodGet, "/debug/pprof/goroutine", pprof.Handler("goroutine"))
		router.Handler(http.MethodGet, "/debug/pprof/allocs", pprof.Handler("allocs"))
		router.Handler(http.MethodGet, "/debug/pprof/block", pprof.Handler("block"))
		router.Handler(http.MethodGet, "/debug/pprof/mutex", pprof.Handler("mutex"))
	}

	// Every operation in internal/openapi/openapi.yaml is registered here,
	// through the generated ServerInterface: the document's own paths and
	// methods become the mux patterns, so a documented operation apiServer
	// does not implement fails the build, and a route this application does
	// not serve is simply absent from ServerInterface. apiserver_guards.go
	// carries the guard chain (auth, ownership, role, idempotency) each
	// domain's former Routes() method applied inline; openapi_sync_test.go
	// and routes_surface_test.go are the runtime guard that the registered
	// surface still matches the document and the inventory exactly.
	//
	// The returned http.Handler is discarded: HandleFunc registers directly
	// onto router (through muxAdapter), which is the same tenant-aware
	// httpx.Router the debug routes above use, so the handler this
	// application actually serves is router.Build() in routes(), once every
	// route — generated and hand-written — has registered.
	gen.HandlerWithOptions(newAPIServer(app), gen.StdHTTPServerOptions{
		BaseRouter:       muxAdapter{router: router, guards: app.middleware.Guards()},
		ErrorHandlerFunc: app.apiServerParamError,
	})
}

// muxAdapter satisfies gen.ServeMux (HandleFunc(pattern, handler) plus
// http.Handler) by delegating registration to the httpx.Router already in
// use — the same tenant-aware wrapper and the same httpx.ServeMux whose
// Build() assembles the JSON 404/405 fallbacks — and by applying that
// route's entry in routeGuards around the WHOLE generated per-route dispatch
// function (parameter binding included) before registering it.
//
// That ordering is the reason the guard is applied here rather than inside
// apiServer's own methods: oapi-codegen's std-http-server binds a route's
// path and required query parameters before ever calling into
// ServerInterface, so an unauthenticated or cross-tenant caller must be
// rejected before that binding runs — exactly where the guard ran when the
// domain handler still did both jobs itself. Wrapping inside apiServer would
// let a missing required query parameter answer 400 before the guard had a
// chance to answer 401/403, which is what routes_authz_test.go and
// routes_audit_test.go exist to catch.
type muxAdapter struct {
	router httpx.Router
	guards httpx.Guards
}

func (a muxAdapter) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		panic(fmt.Sprintf("apiserver: generated pattern %q is missing its method", pattern))
	}
	g, ok := routeGuards[pattern]
	if !ok {
		panic(fmt.Sprintf("apiserver: %q has no entry in routeGuards", pattern))
	}
	a.router.HandlerFunc(method, path, guard(a.guards, g, handler))
}

// ServeHTTP is never invoked: HandlerWithOptions returns its BaseRouter as an
// http.Handler, but routes() discards that return value — registration
// already happened as HandleFunc's side effect, and the handler this
// application actually serves is httpx.ServeMux.Build(), run once every
// route (generated and hand-written) has registered. ServeMux only requires
// the method to exist.
func (a muxAdapter) ServeHTTP(http.ResponseWriter, *http.Request) {
	panic("apiserver: muxAdapter.ServeHTTP is never invoked; see the comment on ServeHTTP")
}
