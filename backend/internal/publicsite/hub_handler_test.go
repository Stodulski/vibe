package publicsite

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
)

// serveHub calls the city hub handler the way the router does: the {city} path
// value is set, then the handler runs. The request line stays ASCII so accented
// spellings can be sent as the path value alone.
func serveHub(h *Handler, city string) *httptest.ResponseRecorder {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/public/hubs/x", nil)
	r.SetPathValue("city", city)
	w := httptest.NewRecorder()
	h.CityHub(w, r)
	return w
}

func newHubHandler(store Store) *Handler {
	return NewHandler(NewService(store, hubFrontend), testResponder())
}

func TestCityHubIsServedAsHTMLWithItsCacheHeader(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{hubs: []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "Av. Rivadavia 100", "Banfield", []string{"padel"}, now, true),
	}}

	w := serveHub(newHubHandler(store), "banfield")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q; want text/html; charset=utf-8", got)
	}
	if got := w.Header().Get("Cache-Control"); got != hubCacheControl {
		t.Errorf("Cache-Control = %q; want %q", got, hubCacheControl)
	}
	if !strings.Contains(w.Body.String(), "<title>Canchas en Banfield - Reservá tu cancha | Vibe</title>") {
		t.Errorf("body is not the Banfield hub page:\n%s", w.Body.String())
	}
}

func TestCityHubIsNotFoundForACityWithNoActiveComplex(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{hubs: []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true),
	}}
	h := newHubHandler(store)

	if w := serveHub(h, "ushuaia"); w.Code != http.StatusNotFound {
		t.Errorf("unknown city: status = %d; want 404", w.Code)
	}

	switchedOff := &stubStore{hubs: []complexstore.HubComplex{
		hubRow("Club Off", "club-off", "a", "Quilmes", nil, now, false),
	}}
	if w := serveHub(newHubHandler(switchedOff), "quilmes"); w.Code != http.StatusNotFound {
		t.Errorf("city whose only complex is switched off: status = %d; want 404", w.Code)
	}
}

func TestCityHubLeavesOutSwitchedOffComplexes(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{hubs: []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true),
		hubRow("Club Off", "club-off", "b", "Banfield", nil, now, false),
	}}

	body := serveHub(newHubHandler(store), "banfield").Body.String()

	if !strings.Contains(body, `href="https://app.vibe.com.ar/club-norte"`) {
		t.Error("the switched-on complex is missing from its city hub")
	}
	if strings.Contains(body, "club-off") || strings.Contains(body, "Club Off") {
		t.Error("a switched-off complex was listed on its city hub")
	}
}

// The lookup ignores case and accents, so a hub is reached by whatever spelling a
// link carries, and its canonical URL is the one slug.
func TestCityHubLookupIgnoresCaseAndAccents(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{hubs: []complexstore.HubComplex{
		hubRow("Club Lopez", "club-lopez", "a", "Vicente López", nil, now, true),
	}}
	h := newHubHandler(store)

	for _, city := range []string{"vicente-lopez", "Vicente-López", "VICENTE-LÓPEZ"} {
		w := serveHub(h, city)
		if w.Code != http.StatusOK {
			t.Errorf("%q: status = %d; want 200", city, w.Code)
			continue
		}
		if !strings.Contains(w.Body.String(), `<link rel="canonical" href="https://app.vibe.com.ar/canchas/vicente-lopez">`) {
			t.Errorf("%q: canonical URL is not the one slug", city)
		}
	}
}

func TestCityHubStoreFailureAnswersUnavailableWithRetryAfter(t *testing.T) {
	store := &stubStore{hubsErr: errors.New("db down")}

	w := serveHub(newHubHandler(store), "banfield")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want 503 so a crawler retries instead of indexing a fault", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("a 503 without Retry-After")
	}
}

func TestSitemapListsEachCityHubWithItsLatestUpdate(t *testing.T) {
	jan := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	store := &stubStore{hubs: []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "a", "Banfield", nil, jan, true),
		hubRow("Canchas Sur", "canchas-sur", "b", "banfield", nil, feb, true),
	}}

	doc, err := NewService(store, "https://vibe.example").Sitemap(t.Context())
	if err != nil {
		t.Fatalf("Sitemap: %v", err)
	}

	block := urlBlock(doc, "https://vibe.example/canchas/banfield")
	if block == "" {
		t.Fatalf("sitemap does not list the banfield hub:\n%s", doc)
	}
	if !strings.Contains(block, "<lastmod>2026-02-02</lastmod>") {
		t.Errorf("hub entry lastmod is not its latest complex update:\n%s", block)
	}
}

func TestSitemapOmitsACityWithNoActiveComplex(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{hubs: []complexstore.HubComplex{
		hubRow("Club Off", "club-off", "a", "Ushuaia", nil, now, false),
	}}

	doc, err := NewService(store, "https://vibe.example").Sitemap(t.Context())
	if err != nil {
		t.Fatalf("Sitemap: %v", err)
	}
	if strings.Contains(doc, "canchas/ushuaia") {
		t.Errorf("sitemap lists a city with no active complex:\n%s", doc)
	}
}

func TestSitemapReportsAHubReadFailure(t *testing.T) {
	store := &stubStore{hubsErr: errors.New("db down")}

	if _, err := NewService(store, "https://vibe.example").Sitemap(t.Context()); err == nil {
		t.Error("a failed hub read was published as a sitemap without the hubs")
	}
}

// urlBlock returns the <url> entry whose <loc> is loc, or "" when there is none.
func urlBlock(doc, loc string) string {
	for _, block := range strings.Split(doc, "<url>") {
		if strings.Contains(block, "<loc>"+loc+"</loc>") {
			return block
		}
	}
	return ""
}
