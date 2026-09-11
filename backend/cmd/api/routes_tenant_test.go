package main

import (
	"sort"
	"testing"

	"github.com/stodulski/vibe-server/internal/middleware"
)

// TestNoOwnerScopedRouteCrossesTenants is the fail-closed half of the
// cross-tenant list.
//
// Row-level security puts a policy on every tenant-scoped table, and the only thing
// that turns it off for a request is the app.bypass_tenant setting that
// middleware.CrossTenantRoutes grants. A route that names a complex in its path
// and is guarded by RequireComplexOwner has its tenant already — it does not
// need the bypass, and granting it one would silently return every owner-scoped
// route to the state F03 describes, where the only thing between one tenant and
// another is a line of Go somebody remembered to write.
//
// The stale-entry direction is covered by TestNoExemptionNamesAnUnregisteredRoute,
// which this table is registered with.
func TestNoOwnerScopedRouteCrossesTenants(t *testing.T) {
	cross := middleware.CrossTenantRoutes()

	var offenders []string
	for route, p := range routePolicies {
		if p != complexOwner {
			continue
		}
		if reason, ok := cross[route]; ok {
			offenders = append(offenders, route+" ("+reason+")")
		}
	}
	sort.Strings(offenders)

	for _, offender := range offenders {
		t.Errorf("%s is guarded by RequireComplexOwner, so its session is already scoped to one complex; "+
			"naming it in crossTenantRoutes hands it every tenant's rows instead", offender)
	}
}

// TestSuperAdminRoutesDeclareTheirCrossTenantPosture is the other direction,
// for the one policy where the bypass is not optional.
//
// The superadmin console exists to read across tenants. Under the policies a route of
// that kind with no bypass does not leak anything — it returns nothing, which
// is safe — but it is broken, and broken quietly: an operator's dashboard full
// of zeroes reads as a quiet platform rather than as a missing line here.
func TestSuperAdminRoutesDeclareTheirCrossTenantPosture(t *testing.T) {
	cross := middleware.CrossTenantRoutes()

	var missing []string
	for route, p := range routePolicies {
		if p != superAdmin {
			continue
		}
		if _, ok := cross[route]; !ok {
			missing = append(missing, route)
		}
	}
	sort.Strings(missing)

	for _, route := range missing {
		t.Errorf("%s is a superadmin route and is not named in crossTenantRoutes, so its database session "+
			"is scoped to no tenant and every query it makes against a tenant table returns nothing", route)
	}
}
