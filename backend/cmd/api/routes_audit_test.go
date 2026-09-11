package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// publicRoutes is the allowlist of endpoints that are reachable without a
// session, each with the reason it is public. Every other registered route
// must reject an anonymous request.
//
// This map is the point of the whole test: adding an endpoint forces a choice
// between guarding it and writing down here, in review, why it is open. The
// previous arrangement relied on someone remembering to write a 401 test per
// handler, which is how the Places proxy briefly shipped unguarded.
var publicRoutes = map[string]string{
	"GET /api/v1/healthcheck":                         "liveness probe for the platform",
	"GET /api/sitemap.xml":                            "crawled by search engines",
	"GET /api/v1/public/prerender/:slug":              "server-rendered page for social and search crawlers",
	"GET /api/v1/public/complexes/:slug":              "the public booking page for a complex",
	"GET /api/v1/public/complexes/:slug/availability": "slot grid on the public booking page",
	"POST /api/v1/public/leads/abandoned-registration": "captures an email left on the register form " +
		"or the Google sign-up before an account exists, so there is no session yet",

	"POST /api/v1/auth/register": "creates the account, so there is no session yet",
	"POST /api/v1/auth/login":    "establishes the session",
	"POST /api/v1/auth/google": "verifies a Google Identity Services ID token and either establishes " +
		"the session or returns a profile token for /auth/google/complete — there is no session yet either way",
	"POST /api/v1/auth/google/complete":     "creates the account from a profile token; there is no session yet",
	"POST /api/v1/auth/refresh":             "runs on an expired access token by design",
	"POST /api/v1/auth/logout":              "must succeed even with an already-invalid session",
	"POST /api/v1/auth/verify-email":        "reached from an emailed link, before first login",
	"POST /api/v1/auth/resend-verification": "reached before the account can log in",
	"POST /api/v1/auth/forgot-password":     "the user cannot log in, that is the point",
	"POST /api/v1/auth/reset-password":      "authenticated by the emailed token, not a session",

	"POST /api/v1/book":            "public booking: clients book without an account",
	"GET /api/v1/book/status":      "clients check their booking by token, without an account",
	"GET /api/v1/book/cancel-info": "same, before cancelling",
	"POST /api/v1/book/cancel":     "clients cancel their own booking without an account",

	"POST /api/v1/webhooks/mercadopago": "authenticated by MercadoPago's signature, not a session",
	"GET /api/v1/webhooks/whatsapp":     "Meta's webhook verification handshake",
	"POST /api/v1/webhooks/whatsapp":    "authenticated by Meta's signature, not a session",

	"GET /api/v1/openapi.json": "this API's own machine-readable contract, read by tooling before any session exists",
	"GET /api/v1/openapi.yaml": "same document, raw bytes",
	"GET /api/v1/docs":         "the interactive reference built from the document above",
}

type recordedRoute struct {
	method string
	path   string
}

// routeRecorder captures the route table instead of serving it.
type routeRecorder struct {
	routes []recordedRoute
}

func (rr *routeRecorder) HandlerFunc(method, path string, _ http.HandlerFunc) {
	rr.routes = append(rr.routes, recordedRoute{method, path})
}

func (rr *routeRecorder) Handler(method, path string, _ http.Handler) {
	rr.routes = append(rr.routes, recordedRoute{method, path})
}

// recordRoutes returns every route the application registers.
func recordRoutes(t *testing.T, app *application) []recordedRoute {
	t.Helper()

	rr := &routeRecorder{}
	app.registerRoutes(rr)
	if len(rr.routes) == 0 {
		t.Fatal("no routes were registered; the audit would pass vacuously")
	}
	return rr.routes
}

// concretePath substitutes router parameters with values that parse, so the
// request reaches the guard rather than failing earlier on a malformed id.
func concretePath(path string) string {
	segments := strings.Split(path, "/")
	for i, s := range segments {
		if !strings.HasPrefix(s, ":") {
			continue
		}
		if s == ":slug" {
			segments[i] = "some-complex"
			continue
		}
		segments[i] = uuid.New().String()
	}
	return strings.Join(segments, "/")
}

// TestEveryRouteIsGuarded walks the registered route table and asserts that
// every endpoint outside publicRoutes rejects a request carrying no
// credentials.
//
// It replaces the per-handler 401 tests, which only covered the endpoints
// somebody remembered to write one for.
func TestEveryRouteIsGuarded(t *testing.T) {
	app := newTestApplication(t)
	routes := recordRoutes(t, app)
	ts := newTestServer(t, app)

	for _, rt := range routes {
		key := rt.method + " " + rt.path
		if _, public := publicRoutes[key]; public {
			continue
		}
		if strings.HasPrefix(rt.path, "/debug/") {
			continue // registered only when pprof is explicitly enabled
		}

		t.Run(key, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), rt.method, ts.URL+concretePath(rt.path), nil)
			if err != nil {
				t.Fatal(err)
			}

			resp, err := ts.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()

			// 401 is the expected rejection. A state-changing method may be
			// stopped one layer earlier by CSRF, which is also a rejection.
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				return
			}
			t.Errorf("anonymous request was not rejected: want 401 or 403; got %d\n"+
				"either guard this route, or add %q to publicRoutes with the reason it is open",
				resp.StatusCode, key)
		})
	}
}

// TestPublicRouteAllowlistIsCurrent fails when publicRoutes names an endpoint
// that no longer exists, so a deleted route cannot leave a stale exemption
// behind for a future route with the same path to inherit.
func TestPublicRouteAllowlistIsCurrent(t *testing.T) {
	app := newTestApplication(t)

	registered := make(map[string]bool)
	for _, rt := range recordRoutes(t, app) {
		registered[rt.method+" "+rt.path] = true
	}

	for key := range publicRoutes {
		if !registered[key] {
			t.Errorf("publicRoutes exempts %q, which is not a registered route; remove it", key)
		}
	}
}
