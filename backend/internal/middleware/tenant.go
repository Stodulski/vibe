package middleware

import (
	"maps"
	"net/http"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// crossTenantRoutes names the exact routes whose database session is allowed
// to cross tenants, each with the reason it is allowed to. The key is
// "METHOD /path", written exactly as the route is registered — including the
// :params, because the wrapping happens at registration time where the pattern
// is still visible rather than at request time where only the concrete URL is.
//
// This is the same shape, and the same discipline, as csrfExemptRoutes in
// chain.go: one route, one entry, no subtrees. A subtree grant is made to routes that do
// not exist yet, and here the consequence of inheriting it silently is one
// tenant reading another's bookings.
//
// EVERYTHING NOT LISTED HERE IS SCOPED OR BLIND.
//
// A route wrapped in RequireComplexOwner gets its tenant from
// httpx.ContextSetComplex. A route that is neither in this table nor
// owner-guarded gets neither, and the tenant policies then answer every
// query against a tenant table with no rows and refuse every insert. That is
// the intended failure: a new public endpoint that reads bookings breaks in
// the first test that exercises it, loudly, instead of shipping a leak.
//
// Three tests in cmd/api keep it honest:
// TestNoExemptionNamesAnUnregisteredRoute (no entry may name a route that is
// not registered, which is how an entry outlives the endpoint it was written
// for), TestNoOwnerScopedRouteCrossesTenants (no RequireComplexOwner route may
// appear here at all) and TestSuperAdminRoutesDeclareTheirCrossTenantPosture
// (every superadmin route must).
//
// The four reasons, since every entry is one of them:
//
//	resolves    the route discovers its own tenant from something that is not
//	            a verified owner — a slug in a public URL, a booking-link token
//	            hash, a MercadoPago payment id. The lookup that discovers it
//	            cannot be scoped to it.
//	bootstrap   there is no tenant yet, or the tenant is being created.
//	by-owner    the scope is the logged-in user's own account, across whichever
//	            complexes belong to it, so it is a user filter and not a
//	            complex one.
//	platform    the route exists to look across tenants (the superadmin
//	            console), or writes a platform-level row with no complex at all.
var crossTenantRoutes = map[string]string{
	// platform — the auth routes write audit_log rows whose complex_id is
	// deliberately NULL (internal/auth/trail.go), and none of them knows a
	// complex.
	"POST /api/v1/auth/register": "bootstrap: creates the account, before any complex exists",
	"POST /api/v1/auth/login":    "platform: the sign-in audit row belongs to no complex",
	"POST /api/v1/auth/google": "platform: the sign-in audit row belongs to no complex, " +
		"same as login",
	"POST /api/v1/auth/google/complete": "bootstrap: creates the account, before any complex exists",
	"POST /api/v1/auth/google/exchange": "platform: the sign-in audit row belongs to no complex, " +
		"same as /auth/google, which this is the redirect-mode path to",
	"POST /api/v1/auth/refresh":             "platform: the session audit row belongs to no complex",
	"POST /api/v1/auth/logout":              "platform: the sign-out audit row belongs to no complex",
	"POST /api/v1/auth/verify-email":        "platform: reached from an emailed link, before any complex is in play",
	"POST /api/v1/auth/resend-verification": "platform: reached before the account can log in",
	"POST /api/v1/auth/forgot-password":     "platform: the caller cannot log in, that is the point",
	"POST /api/v1/auth/reset-password":      "platform: authenticated by the emailed token, not by a complex",
	"POST /api/v1/auth/confirm-email-change": "platform: authenticated by the emailed token, not by a complex, " +
		"the same posture as /auth/reset-password",
	"PUT /api/v1/auth/me": "platform: edits the account, and its audit row belongs to no complex",
	"DELETE /api/v1/auth/me": "by-owner: reads every complex of the account and deletes the user, " +
		"whose cascade reaches every tenant table under it",

	// by-owner — the dashboard's own list, and the two steps before a complex
	// exists to be the owner of.
	"GET /api/v1/complexes":      "by-owner: lists the complexes of the logged-in account, filtered by owner_id",
	"POST /api/v1/complexes":     "bootstrap: creates the complex, so there is no complex to be scoped to yet",
	"GET /api/v1/slug-available": "platform: a slug is unique across every tenant, so the check has to see them all",

	// resolves — the storefront and the public booking flow. Each of these
	// starts from a slug or a token hash and works out which complex it is
	// for; scoping the session to a tenant it has not identified yet is
	// impossible by construction.
	"GET /api/v1/public/complexes/{slug}":              "resolves: the storefront finds its complex by slug",
	"GET /api/v1/public/complexes/{slug}/availability": "resolves: the availability grid finds its complex by slug",
	"GET /api/v1/public/prerender/{slug}":              "resolves: the prerendered storefront finds its complex by slug",
	"GET /api/v1/sitemap.xml":                          "platform: every active slug on the platform, by definition",
	"GET /api/sitemap.xml":                             "platform: the 301 to the line above; it reads nothing",
	"POST /api/v1/book":                                "resolves: the public booking flow is handed a complex id and validates it",
	"GET /api/v1/book/status":                          "resolves: the booking link authenticates by token hash, which yields the booking",
	"GET /api/v1/book/cancel-info":                     "resolves: same token hash, same booking",
	"POST /api/v1/book/cancel":                         "resolves: same token hash, same booking",

	// resolves — MercadoPago arrives with a payment id and nothing else.
	"POST /api/v1/webhooks/mercadopago": "resolves: the tenant is found from the payment's external reference",

	// platform — the superadmin console exists to see across tenants.
	"GET /api/v1/admin/stats":                      "platform: aggregates across every tenant",
	"GET /api/v1/admin/users":                      "platform: the operator's user list",
	"GET /api/v1/admin/users/{id}":                 "platform: the operator's user detail, with that user's complexes",
	"PATCH /api/v1/admin/users/{id}/toggle-active": "platform: the operator suspends an account",
	"GET /api/v1/admin/complexes":                  "platform: the operator's complex list",
	"GET /api/v1/admin/complexes/{id}":             "platform: the operator's complex detail, with no ownership check by design",
	"GET /api/v1/admin/audit-log":                  "platform: the operator's audit trail, optionally filtered to one complex",
	"GET /api/v1/admin/healthcheck":                "platform: queue depths counted across every tenant",
}

// CrossTenantRoutes returns the routes whose database session crosses tenants,
// keyed by "METHOD /path" with the reason as the value.
//
// Exported for the route audit in cmd/api, which is the only place that can
// see the registered route table and this list at the same time.
func CrossTenantRoutes() map[string]string { return maps.Clone(crossTenantRoutes) }

// TenantAwareRouter wraps a router so that every route it registers carries
// the tenant posture its entry in crossTenantRoutes declares — or none, which
// is the fail-closed default.
//
// The decision is made once, at registration, rather than per request. That is
// what lets the table be keyed by the route pattern: by the time a request is
// being served, the mux has already turned "/api/v1/complexes/{id}" into
// "/api/v1/complexes/3f2a…", and a runtime lookup would have to either match
// prefixes — the subtree grant this table exists to avoid — or reconstruct the
// pattern from the parameters.
func TenantAwareRouter(r httpx.Router) httpx.Router { return tenantAwareRouter{inner: r} }

type tenantAwareRouter struct{ inner httpx.Router }

func (t tenantAwareRouter) HandlerFunc(method, path string, handler http.HandlerFunc) {
	t.inner.HandlerFunc(method, path, withTenantPosture(method, path, handler))
}

func (t tenantAwareRouter) Handler(method, path string, handler http.Handler) {
	t.inner.Handler(method, path, withTenantPosture(method, path, handler.ServeHTTP))
}

// withTenantPosture returns handler unchanged unless the route is declared
// cross-tenant, in which case it runs with the bypass.
func withTenantPosture(method, path string, handler http.HandlerFunc) http.HandlerFunc {
	if _, cross := crossTenantRoutes[method+" "+path]; !cross {
		return handler
	}
	return func(w http.ResponseWriter, r *http.Request) {
		handler(w, r.WithContext(data.ContextWithTenantBypass(r.Context())))
	}
}
