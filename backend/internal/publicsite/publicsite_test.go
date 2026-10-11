package publicsite

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/httpx"
)

type stubStore struct {
	slugs []complexstore.ComplexSlug
	hubs  []complexstore.HubComplex

	slugsErr error
	hubsErr  error
}

func (s *stubStore) GetAllSlugs(context.Context) ([]complexstore.ComplexSlug, error) {
	return s.slugs, s.slugsErr
}

func (s *stubStore) ListActiveComplexesForHubs(context.Context) ([]complexstore.HubComplex, error) {
	return s.hubs, s.hubsErr
}

func testResponder() *httpx.Responder {
	return httpx.NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
}

func TestSitemapListsEveryComplexAndNotTheNoindexRoot(t *testing.T) {
	store := &stubStore{slugs: []complexstore.ComplexSlug{
		{Slug: "vibe-palermo", UpdatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{Slug: "vibe-pilar", UpdatedAt: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)},
	}}

	h := NewHandler(NewService(store, "https://vibe.example/"), testResponder())
	w := httptest.NewRecorder()
	h.Sitemap(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d", w.Code)
	}
	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/xml") {
		t.Errorf("want an XML content type; got %q", got)
	}

	body := w.Body.String()
	for _, want := range []string{
		"<loc>https://vibe.example/c/vibe-palermo</loc>",
		"<lastmod>2026-03-01</lastmod>",
		"<loc>https://vibe.example/c/vibe-pilar</loc>",
		"<lastmod>2026-04-02</lastmod>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap is missing %q\n%s", want, body)
		}
	}

	// The app's own root is served with X-Robots-Tag: noindex, so listing it
	// asks a crawler to fetch a URL it is then told to throw away.
	if strings.Contains(body, "<loc>https://vibe.example/</loc>") {
		t.Errorf("sitemap still lists the noindex root\n%s", body)
	}

	// The trailing slash on the configured public URL must not double up.
	if strings.Contains(body, "//c/vibe-palermo") {
		t.Error("the base URL's trailing slash produced a doubled separator")
	}
}

// The pre-versioning path is still in search engines' indexes. A 404 there
// makes a crawler drop the pages the sitemap lists rather than look for a new
// address, so it has to be a permanent redirect for as long as it is fetched.
func TestTheOldSitemapPathRedirectsPermanently(t *testing.T) {
	h := NewHandler(NewService(&stubStore{}, "https://vibe.example"), testResponder())
	w := httptest.NewRecorder()
	h.SitemapMoved(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/sitemap.xml", nil))

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("want 301; got %d", w.Code)
	}
	if got := w.Header().Get("Location"); got != SitemapPath {
		t.Errorf("Location = %q, want %q", got, SitemapPath)
	}
}

func TestSitemapReportsStoreFailure(t *testing.T) {
	h := NewHandler(NewService(&stubStore{slugsErr: errors.New("db down")}, "https://vibe.example"), testResponder())
	w := httptest.NewRecorder()
	h.Sitemap(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500; got %d", w.Code)
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

// The sitemap lists the complex pages and the city hubs under the public origin,
// and nothing under any other origin.
func TestSitemapListsEveryPublicPageOnThePublicOrigin(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{
		slugs: []complexstore.ComplexSlug{{Slug: "club-norte", UpdatedAt: now}},
		hubs:  []complexstore.HubComplex{hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true)},
	}

	doc, err := NewService(store, "https://vibe.com.ar").Sitemap(t.Context())
	if err != nil {
		t.Fatalf("Sitemap: %v", err)
	}

	for _, want := range []string{
		"<loc>https://vibe.com.ar/c/club-norte</loc>",
		"<loc>https://vibe.com.ar/canchas/banfield</loc>",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("sitemap is missing %q:\n%s", want, doc)
		}
	}
	if strings.Contains(doc, "app.vibe.com.ar") {
		t.Errorf("sitemap publishes the owner-panel origin:\n%s", doc)
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
