package main

import (
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/middleware"
)

// The city hub is read by people and crawlers without a session, and it reads
// switched-on complexes across tenants. That is a platform listing, with the same
// posture as the sitemap it is linked from, so it is declared as one. Without the
// declaration its session is scoped to no tenant and every hub comes back empty.
func TestCityHubIsDeclaredAsAPlatformListing(t *testing.T) {
	const route = "GET /api/v1/public/hubs/{city}"

	reason, ok := middleware.CrossTenantRoutes()[route]
	if !ok {
		t.Fatalf("%s reads complexes across tenants and is not named in crossTenantRoutes", route)
	}
	if !strings.HasPrefix(reason, "platform:") {
		t.Errorf("%s reason = %q; want a platform reason, like the sitemap's", route, reason)
	}
}
