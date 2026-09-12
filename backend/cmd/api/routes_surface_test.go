package main

import (
	"sort"
	"strings"
	"testing"
)

// apiSurface is the complete set of endpoints this API serves.
//
// TestEveryRouteIsGuarded proves that whatever is registered is protected. It
// cannot prove that what is registered is what was meant: renaming every client
// route to /klients left the whole suite green while the frontend got 404s.
// This inventory closes that gap. Adding, removing or renaming an endpoint is
// now a deliberate edit here, visible in the diff, rather than a silent change
// in a module's Routes method.
//
// Regenerate after an intentional change by running the audit and copying the
// reported differences.
var apiSurface = map[string]struct{}{
	"DELETE /api/v1/auth/me":                                           {},
	"DELETE /api/v1/complexes/{id}":                                    {},
	"DELETE /api/v1/complexes/{id}/blocked-slots/{slotID}":             {},
	"DELETE /api/v1/complexes/{id}/courts/{courtID}":                   {},
	"DELETE /api/v1/complexes/{id}/mp/connect":                         {},
	"DELETE /api/v1/complexes/{id}/uploads":                            {},
	"GET /api/sitemap.xml":                                             {},
	"GET /api/v1/sitemap.xml":                                          {},
	"GET /api/v1/admin/audit-log":                                      {},
	"GET /api/v1/admin/complexes":                                      {},
	"GET /api/v1/admin/complexes/{id}":                                 {},
	"GET /api/v1/admin/healthcheck":                                    {},
	"GET /api/v1/admin/stats":                                          {},
	"GET /api/v1/admin/users":                                          {},
	"GET /api/v1/admin/users/{id}":                                     {},
	"GET /api/v1/auth/me":                                              {},
	"GET /api/v1/book/cancel-info":                                     {},
	"GET /api/v1/book/status":                                          {},
	"GET /api/v1/slug-available":                                       {},
	"GET /api/v1/complexes":                                            {},
	"GET /api/v1/complexes/{id}":                                       {},
	"GET /api/v1/complexes/{id}/audit-log":                             {},
	"GET /api/v1/complexes/{id}/blocked-slots":                         {},
	"GET /api/v1/complexes/{id}/bookings":                              {},
	"GET /api/v1/complexes/{id}/bookings/{bookingID}":                  {},
	"GET /api/v1/complexes/{id}/clients":                               {},
	"GET /api/v1/complexes/{id}/clients/{clientID}":                    {},
	"GET /api/v1/complexes/{id}/courts":                                {},
	"GET /api/v1/complexes/{id}/events":                                {},
	"GET /api/v1/complexes/{id}/mp/status":                             {},
	"GET /api/v1/complexes/{id}/reports/export":                        {},
	"GET /api/v1/complexes/{id}/reports/monthly":                       {},
	"GET /api/v1/complexes/{id}/stats":                                 {},
	"GET /api/v1/complexes/{id}/stats/clients":                         {},
	"GET /api/v1/complexes/{id}/stats/occupancy":                       {},
	"GET /api/v1/complexes/{id}/stats/revenue":                         {},
	"GET /api/v1/docs":                                                 {},
	"GET /api/v1/healthcheck":                                          {},
	"GET /api/v1/livez":                                                {},
	"GET /api/v1/openapi.json":                                         {},
	"GET /api/v1/openapi.yaml":                                         {},
	"GET /api/v1/places/autocomplete":                                  {},
	"GET /api/v1/places/details":                                       {},
	"GET /api/v1/public/complexes/{slug}":                              {},
	"GET /api/v1/public/complexes/{slug}/availability":                 {},
	"GET /api/v1/public/prerender/{slug}":                              {},
	"GET /api/v1/webhooks/whatsapp":                                    {},
	"PATCH /api/v1/admin/users/{id}/toggle-active":                     {},
	"POST /api/v1/auth/forgot-password":                                {},
	"POST /api/v1/auth/google":                                         {},
	"POST /api/v1/auth/google/complete":                                {},
	"POST /api/v1/auth/login":                                          {},
	"POST /api/v1/auth/logout":                                         {},
	"POST /api/v1/auth/refresh":                                        {},
	"POST /api/v1/auth/register":                                       {},
	"POST /api/v1/auth/resend-verification":                            {},
	"POST /api/v1/auth/reset-password":                                 {},
	"POST /api/v1/auth/verify-email":                                   {},
	"POST /api/v1/book":                                                {},
	"POST /api/v1/book/cancel":                                         {},
	"POST /api/v1/complexes":                                           {},
	"POST /api/v1/complexes/{id}/bookings":                             {},
	"POST /api/v1/complexes/{id}/bookings/{bookingID}/cancel":          {},
	"POST /api/v1/complexes/{id}/bookings/{bookingID}/confirm-payment": {},
	"POST /api/v1/complexes/{id}/bookings/{bookingID}/manual-refund":   {},
	"POST /api/v1/complexes/{id}/courts":                               {},
	"POST /api/v1/complexes/{id}/courts/{courtID}/block":               {},
	"POST /api/v1/complexes/{id}/mp/connect":                           {},
	"POST /api/v1/complexes/{id}/uploads/presign":                      {},
	"POST /api/v1/public/leads/abandoned-registration":                 {},
	"POST /api/v1/webhooks/mercadopago":                                {},
	"POST /api/v1/webhooks/whatsapp":                                   {},
	"PUT /api/v1/auth/me":                                              {},
	"PUT /api/v1/complexes/{id}":                                       {},
	"PUT /api/v1/complexes/{id}/bookings/{bookingID}":                  {},
	"PUT /api/v1/complexes/{id}/clients/{clientID}":                    {},
	"PUT /api/v1/complexes/{id}/courts/{courtID}":                      {},
	"PUT /api/v1/complexes/{id}/courts/{courtID}/prices":               {},
	"PUT /api/v1/complexes/{id}/schedules":                             {},
}

// TestAPISurfaceMatchesTheInventory fails when the registered routes and the
// inventory above disagree in either direction.
func TestAPISurfaceMatchesTheInventory(t *testing.T) {
	app := newTestApplication(t)

	registered := make(map[string]struct{})
	for _, rt := range recordRoutes(t, app) {
		if strings.HasPrefix(rt.path, "/debug/") {
			continue // only registered when pprof is explicitly enabled
		}
		registered[rt.method+" "+rt.path] = struct{}{}
	}

	var missing, unexpected []string
	for want := range apiSurface {
		if _, ok := registered[want]; !ok {
			missing = append(missing, want)
		}
	}
	for got := range registered {
		if _, ok := apiSurface[got]; !ok {
			unexpected = append(unexpected, got)
		}
	}
	sort.Strings(missing)
	sort.Strings(unexpected)

	for _, m := range missing {
		t.Errorf("route %q is in the inventory but is no longer registered — was it renamed or dropped?", m)
	}
	for _, u := range unexpected {
		t.Errorf("route %q is registered but not in the inventory — add it there if it is intended", u)
	}
}
