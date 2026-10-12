package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/middleware"
	"github.com/stodulski/vibe-server/internal/stores"
)

// The city hubs are read by people and crawlers without a session, and they read
// switched-on complexes across tenants. That is a platform listing, with the same
// posture as the sitemap they are linked from, so each route is declared as one.
// Without the declaration its session is scoped to no tenant and every hub comes
// back empty.
func TestCityHubRoutesAreDeclaredAsPlatformListings(t *testing.T) {
	routes := []string{
		"GET /api/v1/public/hubs",
		"GET /api/v1/public/hubs/{city}/data",
	}
	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			reason, ok := middleware.CrossTenantRoutes()[route]
			if !ok {
				t.Fatalf("%s reads complexes across tenants and is not named in crossTenantRoutes", route)
			}
			if !strings.HasPrefix(reason, "platform:") {
				t.Errorf("%s reason = %q; want a platform reason, like the sitemap's", route, reason)
			}
		})
	}
}

// The hub JSON routes answer with the switched-on complexes of every tenant. The
// store here stands in for a tenant-scoped session: it answers an unscoped read
// with nothing, as row-level security does, and answers only a read carrying the
// platform bypass. So a route that lost its declaration would return an empty
// list and fail this test, which is the failure the declaration exists to stop.
func TestHubJSONRoutesReturnTheRealHubsThroughTheTenantMiddleware(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	rows := []complexstore.HubComplex{
		{ID: uuid.New(), Name: "Club Norte", Slug: "club-norte", Address: "Av. Rivadavia 100", City: "Banfield", IsActive: true, CourtSports: []string{"padel"}, UpdatedAt: now},
		{ID: uuid.New(), Name: "Club Lopez", Slug: "club-lopez", Address: "Calle 1 200", City: "Vicente López", IsActive: true, CourtSports: []string{"tennis"}, UpdatedAt: now},
	}
	complexesStore := &mockComplexStore{
		ListActiveComplexesForHubsFn: func(ctx context.Context) ([]complexstore.HubComplex, error) {
			if !data.TenantBypassed(ctx) {
				return nil, nil
			}
			return rows, nil
		},
	}
	app, _ := newTestApplicationWithStores(t, func(m *stores.Stores) { m.Complexes = complexesStore })
	ts := newTestServer(t, app)

	t.Run("hub list", func(t *testing.T) {
		resp := getAnonymous(t, ts.URL+"/api/v1/public/hubs")
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d; want 200", resp.StatusCode)
		}

		var body struct {
			Hubs []struct {
				Slug         string `json:"slug"`
				ComplexCount int    `json:"complex_count"`
			} `json:"hubs"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode hub list: %v", err)
		}
		if len(body.Hubs) != 2 {
			t.Fatalf("hubs = %+v; want the two cities with a switched-on complex", body.Hubs)
		}
		if body.Hubs[0].Slug != "banfield" || body.Hubs[0].ComplexCount != 1 {
			t.Errorf("first hub = %+v; want banfield with one complex", body.Hubs[0])
		}
	})

	t.Run("one city's data", func(t *testing.T) {
		resp := getAnonymous(t, ts.URL+"/api/v1/public/hubs/banfield/data")
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d; want 200", resp.StatusCode)
		}

		var body struct {
			Hub struct {
				Slug string `json:"slug"`
			} `json:"hub"`
			Complexes []struct {
				Slug string `json:"slug"`
			} `json:"complexes"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode city hub: %v", err)
		}
		if body.Hub.Slug != "banfield" {
			t.Errorf("hub.slug = %q; want banfield", body.Hub.Slug)
		}
		if len(body.Complexes) != 1 || body.Complexes[0].Slug != "club-norte" {
			t.Errorf("complexes = %+v; want club-norte alone", body.Complexes)
		}
	})
}

// A city whose last switched-on complex is gone keeps its hub at the route: a 200
// with empty lists, reached by its own URL only. It stays out of the hub index and
// the sitemap, which list only the cities with a switched-on complex.
func TestCityHubRouteAnswersAnEmptyHubAfterItsLastComplexIsGone(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	complexesStore := &mockComplexStore{
		ListActiveComplexesForHubsFn: func(ctx context.Context) ([]complexstore.HubComplex, error) {
			if !data.TenantBypassed(ctx) {
				return nil, nil
			}
			return []complexstore.HubComplex{
				{ID: uuid.New(), Name: "Club Norte", Slug: "club-norte", Address: "Av. Rivadavia 100", City: "Banfield", Province: "Buenos Aires", IsActive: true, CourtSports: []string{"padel"}, UpdatedAt: now},
			}, nil
		},
		ListComplexCitiesForHubsFn: func(ctx context.Context) ([]complexstore.HubCity, error) {
			if !data.TenantBypassed(ctx) {
				return nil, nil
			}
			return []complexstore.HubCity{
				{City: "Banfield", Province: "Buenos Aires"},
				{City: "Quilmes", Province: "Buenos Aires"},
			}, nil
		},
	}
	app, _ := newTestApplicationWithStores(t, func(m *stores.Stores) { m.Complexes = complexesStore })
	ts := newTestServer(t, app)

	t.Run("the empty hub", func(t *testing.T) {
		resp := getAnonymous(t, ts.URL+"/api/v1/public/hubs/quilmes/data")
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d; want 200", resp.StatusCode)
		}

		var raw bytes.Buffer
		if _, err := raw.ReadFrom(resp.Body); err != nil {
			t.Fatalf("read city hub: %v", err)
		}
		if !strings.Contains(raw.String(), `"complexes":[]`) || !strings.Contains(raw.String(), `"sports":[]`) {
			t.Errorf("an empty hub must encode its lists as []:\n%s", raw.String())
		}

		var body struct {
			Hub struct {
				Slug     string `json:"slug"`
				Name     string `json:"name"`
				Province string `json:"province"`
			} `json:"hub"`
		}
		if err := json.NewDecoder(strings.NewReader(raw.String())).Decode(&body); err != nil {
			t.Fatalf("decode city hub: %v", err)
		}
		if body.Hub.Slug != "quilmes" || body.Hub.Name != "Quilmes" || body.Hub.Province != "Buenos Aires" {
			t.Errorf("hub = %+v; want quilmes named Quilmes in Buenos Aires", body.Hub)
		}
	})

	t.Run("left out of the index", func(t *testing.T) {
		resp := getAnonymous(t, ts.URL+"/api/v1/public/hubs")
		defer func() { _ = resp.Body.Close() }()

		var body struct {
			Hubs []struct {
				Slug string `json:"slug"`
			} `json:"hubs"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode hub list: %v", err)
		}
		if len(body.Hubs) != 1 || body.Hubs[0].Slug != "banfield" {
			t.Errorf("hubs = %+v; want banfield alone", body.Hubs)
		}
	})

	t.Run("left out of the sitemap", func(t *testing.T) {
		resp := getAnonymous(t, ts.URL+"/api/v1/sitemap.xml")
		defer func() { _ = resp.Body.Close() }()

		var raw bytes.Buffer
		if _, err := raw.ReadFrom(resp.Body); err != nil {
			t.Fatalf("read sitemap: %v", err)
		}
		if strings.Contains(raw.String(), "/canchas/quilmes") {
			t.Errorf("sitemap lists the quilmes hub, which has no switched-on complex:\n%s", raw.String())
		}
	})
}

// getAnonymous sends a GET with no credentials, the way a crawler does.
func getAnonymous(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
