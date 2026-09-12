package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"testing"
)

// TestRouteGuardsMatchesInventory proves routeGuards (apiserver_guards.go)
// has exactly one entry per route in apiSurface (routes_surface_test.go) —
// the same inventory TestAPISurfaceMatchesTheInventory checks the live
// router against. Every operation gen.HandlerWithOptions registers must have
// a guard entry, or muxAdapter.HandleFunc panics at boot; this test is the
// one that fails a "go test" instead of a boot, and it also catches the
// opposite mistake — a stale entry left behind by a renamed or removed
// route.
func TestRouteGuardsMatchesInventory(t *testing.T) {
	var missing, unexpected []string

	for route := range apiSurface {
		if _, ok := routeGuards[route]; !ok {
			missing = append(missing, route)
		}
	}
	for route := range routeGuards {
		if _, ok := apiSurface[route]; !ok {
			unexpected = append(unexpected, route)
		}
	}
	sort.Strings(missing)
	sort.Strings(unexpected)

	for _, route := range missing {
		t.Errorf("route %q is in apiSurface but has no entry in routeGuards; "+
			"gen.HandlerWithOptions will panic on it at boot", route)
	}
	for _, route := range unexpected {
		t.Errorf("routeGuards has %q, which is not in apiSurface — "+
			"the route was renamed or removed and this entry is stale", route)
	}
}

// TestApiServerParamErrorAnswersAMalformedQueryParamAs422 pins
// apiServerParamError's mapping of oapi-codegen's own parameter binding: a
// malformed "date" never reaches courtsPublicAvailability at all, so this is
// the generated wrapper's *InvalidParamFormatError, not the handler's own
// field validation. Before this change every one of these answered 400
// (bad-request); the fix maps it to 422 validation with the field name, the
// same shape a handler's own validator produces.
func TestApiServerParamErrorAnswersAMalformedQueryParamAs422(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		ts.URL+"/api/v1/public/complexes/some-slug/availability?date=not-a-date", nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("GET availability: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for a malformed date query parameter; got %d", resp.StatusCode)
	}

	var body struct {
		Errors []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding the response body: %v", err)
	}
	if len(body.Errors) != 1 || body.Errors[0].Field != "date" {
		t.Fatalf("want one field error naming \"date\"; got %+v", body.Errors)
	}
}
