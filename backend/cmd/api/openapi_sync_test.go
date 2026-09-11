package main

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// pathParam matches an httprouter path parameter, e.g. ":id" or ":bookingID".
var pathParam = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

// canonicalPath rewrites an httprouter path (":name") to the OpenAPI template
// form ("{name}") so the two can be compared directly.
func canonicalPath(path string) string {
	return pathParam.ReplaceAllString(path, "{$1}")
}

// TestOpenAPISyncWithRouter proves internal/openapi/openapi.yaml documents
// exactly the routes this application registers, outside /debug/.
//
// It reuses recordRoutes, the same primitive TestAPISurfaceMatchesTheInventory
// and the authz/exemption/tenant tables build on, so this test asks its
// question against the one place that can enumerate every registered route
// without booting a real listener. Adding a route without documenting it, or
// documenting one that was never registered or was renamed, fails here and
// names which side is wrong.
func TestOpenAPISyncWithRouter(t *testing.T) {
	app := newTestApplication(t)

	registered := make(map[string]struct{})
	for _, rt := range recordRoutes(t, app) {
		if strings.HasPrefix(rt.path, "/debug/") {
			continue // dev-only, framework-owned; excluded from the document by design
		}
		registered[rt.method+" "+canonicalPath(rt.path)] = struct{}{}
	}

	documented := make(map[string]struct{})
	for path, item := range app.openapi.Document().Paths.Map() {
		for method := range item.Operations() {
			documented[method+" "+path] = struct{}{}
		}
	}

	var missingFromSpec, missingFromRouter []string
	for key := range registered {
		if _, ok := documented[key]; !ok {
			missingFromSpec = append(missingFromSpec, key)
		}
	}
	for key := range documented {
		if _, ok := registered[key]; !ok {
			missingFromRouter = append(missingFromRouter, key)
		}
	}
	sort.Strings(missingFromSpec)
	sort.Strings(missingFromRouter)

	for _, key := range missingFromSpec {
		t.Errorf("route %q is registered but not documented in internal/openapi/openapi.yaml.\n"+
			"Add a paths entry for it there, translating httprouter's \":name\" segments to \"{name}\".", key)
	}
	for _, key := range missingFromRouter {
		t.Errorf("internal/openapi/openapi.yaml documents %q, which is not a registered route.\n"+
			"Remove the stale paths entry, or the route was renamed/dropped and the router side "+
			"needs the update instead.", key)
	}
}

// TestOpenAPIOperationCount pins the total operation count so an accidental
// duplicate path key, or a path registered under the wrong method, shows up
// even if it happens not to change the missing/unexpected sets above.
func TestOpenAPIOperationCount(t *testing.T) {
	app := newTestApplication(t)

	n := 0
	for _, item := range app.openapi.Document().Paths.Map() {
		n += len(item.Operations())
	}

	registered := 0
	for _, rt := range recordRoutes(t, app) {
		if strings.HasPrefix(rt.path, "/debug/") {
			continue
		}
		registered++
	}

	if n != registered {
		t.Errorf("the document has %d operations, the router has %d non-debug routes; "+
			"they must match exactly", n, registered)
	}
}
