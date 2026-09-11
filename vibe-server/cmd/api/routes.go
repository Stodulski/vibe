package main

import (
	"expvar"
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/middleware"
)

// routes builds the full HTTP handler: the route table wrapped in the
// middleware chain, outermost first.
func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(app.respond.NotFound)
	router.MethodNotAllowed = http.HandlerFunc(app.respond.MethodNotAllowed)

	app.registerRoutes(router)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{app.config.frontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           86400,
	})

	return app.middleware.Wrap(router, func(next http.Handler) http.Handler {
		return normalizeCORSPreflightHeaders(c.Handler(next))
	})
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
//
// linear wiring sequence into arbitrarily-named helpers without clarifying it.
//
//nolint:funlen // flat sequential route-registration table; splitting would fragment a single
func (app *application) registerRoutes(router httpx.Router) {
	// Every route gets the tenant posture its entry in
	// middleware.CrossTenantRoutes declares, or none — which is the
	// fail-closed default that leaves the tenant policies answering
	// "no rows" for a route nobody scoped. The wrapping happens here, inside
	// registerRoutes rather than in routes(), so that it cannot be skipped by
	// a caller that builds the table another way.
	router = middleware.TenantAwareRouter(router)

	// Public routes.
	if app.config.env == "development" {
		router.Handler(http.MethodGet, "/debug/vars", expvar.Handler())
	}
	if app.config.pprof && app.config.env == "development" {
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

	// Webhook routes (public, verified via signature).

	// Extracted domain modules register their own routes.
	app.places.Routes(router, app.middleware.Guards())
	app.clients.Routes(router, app.middleware.Guards())
	app.realtime.Routes(router, app.middleware.Guards())
	app.publicsite.Routes(router, app.middleware.Guards())
	app.leads.Routes(router, app.middleware.Guards())
	app.reporting.Routes(router, app.middleware.Guards())
	app.admin.Routes(router, app.middleware.Guards())
	app.health.Routes(router, app.middleware.Guards())
	app.courts.Routes(router, app.middleware.Guards())
	app.complexes.Routes(router, app.middleware.Guards())
	app.auth.Routes(router, app.middleware.Guards())
	app.payments.Routes(router, app.middleware.Guards())
	app.bookings.Routes(router, app.middleware.Guards())
	app.auditTrail.Routes(router, app.middleware.Guards())
	app.openapi.Routes(router, app.middleware.Guards())

}
