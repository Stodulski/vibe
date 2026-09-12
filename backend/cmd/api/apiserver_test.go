package main

import (
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
