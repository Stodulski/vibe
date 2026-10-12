package publicsite

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
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
	slugs  []complexstore.ComplexSlug
	hubs   []complexstore.HubComplex
	cities []complexstore.HubCity

	slugsErr  error
	hubsErr   error
	citiesErr error
}

func (s *stubStore) GetAllSlugs(context.Context) ([]complexstore.ComplexSlug, error) {
	return s.slugs, s.slugsErr
}

func (s *stubStore) ListActiveComplexesForHubs(context.Context) ([]complexstore.HubComplex, error) {
	return s.hubs, s.hubsErr
}

func (s *stubStore) ListComplexCitiesForHubs(context.Context) ([]complexstore.HubCity, error) {
	return s.cities, s.citiesErr
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

// A city whose complexes are all gone keeps its hub page but stays out of the
// sitemap, as it stays out of the hub index: the sitemap lists only the cities
// with a switched-on complex.
func TestSitemapOmitsACityWhoseComplexesAreAllGone(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{
		hubs: []complexstore.HubComplex{
			hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true),
			hubRow("Canchas Sur", "canchas-sur", "b", "Banfield", nil, now, true),
		},
		cities: []complexstore.HubCity{{City: "Banfield"}, {City: "Ushuaia"}},
	}

	doc, err := NewService(store, "https://vibe.example").Sitemap(t.Context())
	if err != nil {
		t.Fatalf("Sitemap: %v", err)
	}
	if urlBlock(doc, "https://vibe.example/canchas/banfield") == "" {
		t.Errorf("sitemap omits the banfield hub, which has two complexes:\n%s", doc)
	}
	if urlBlock(doc, "https://vibe.example/canchas/ushuaia") != "" {
		t.Errorf("sitemap lists the ushuaia hub, which has no switched-on complex:\n%s", doc)
	}
}

// A hub with one complex funnels every search for its city to a single page, which
// search engines read as a doorway. It stays out of the sitemap, though its page
// still answers and it is still linked from the hub index.
func TestSitemapLeavesOutAHubWithOneComplex(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{hubs: []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true),
		hubRow("Canchas Sur", "canchas-sur", "b", "banfield", nil, now, true),
		hubRow("Club Solo", "club-solo", "c", "Quilmes", nil, now, true),
	}}

	doc, err := NewService(store, "https://vibe.example").Sitemap(t.Context())
	if err != nil {
		t.Fatalf("Sitemap: %v", err)
	}
	if urlBlock(doc, "https://vibe.example/canchas/banfield") == "" {
		t.Errorf("sitemap omits the banfield hub, which has two complexes:\n%s", doc)
	}
	if urlBlock(doc, "https://vibe.example/canchas/quilmes") != "" {
		t.Errorf("sitemap lists the quilmes hub, which has one complex:\n%s", doc)
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
		hubs: []complexstore.HubComplex{
			hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true),
			hubRow("Canchas Sur", "canchas-sur", "b", "Banfield", nil, now, true),
		},
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

// googleImageNS is the namespace of Google's image sitemap extension.
const googleImageNS = "http://www.google.com/schemas/sitemap-image/1.1"

func urlPtr(s string) *string { return &s }

// photoSlug is a switched-on complex of the sitemap test fixtures with the given photos.
func photoSlug(slug string, cover, logo *string) complexstore.ComplexSlug {
	return complexstore.ComplexSlug{
		Slug:      slug,
		UpdatedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		CoverURL:  cover,
		LogoURL:   logo,
	}
}

func sitemapFor(t *testing.T, slugs ...complexstore.ComplexSlug) string {
	t.Helper()
	doc, err := NewService(&stubStore{slugs: slugs}, "https://vibe.example").Sitemap(t.Context())
	if err != nil {
		t.Fatalf("Sitemap: %v", err)
	}
	return doc
}

// The image sitemap extension lists a venue's photos under its page: the cover first,
// then the logo, each only when it is an absolute http(s) URL.
func TestSitemapListsTheCoverThenTheLogoAsImagesOfTheComplex(t *testing.T) {
	doc := sitemapFor(t, photoSlug("club-norte", urlPtr("https://cdn.example/cover.jpg"), urlPtr("https://cdn.example/logo.png")))

	block := urlBlock(doc, "https://vibe.example/c/club-norte")
	cover := strings.Index(block, "<image:loc>https://cdn.example/cover.jpg</image:loc>")
	logo := strings.Index(block, "<image:loc>https://cdn.example/logo.png</image:loc>")
	if cover < 0 || logo < 0 {
		t.Fatalf("the complex's <url> is missing its cover or its logo:\n%s", block)
	}
	if cover > logo {
		t.Errorf("the logo is listed before the cover; want the cover first:\n%s", block)
	}
	if n := strings.Count(block, "<image:image>"); n != 2 {
		t.Errorf("want 2 image entries; got %d:\n%s", n, block)
	}
}

func TestSitemapListsALogoAloneAsOneImage(t *testing.T) {
	doc := sitemapFor(t, photoSlug("club-norte", nil, urlPtr("https://cdn.example/logo.png")))

	block := urlBlock(doc, "https://vibe.example/c/club-norte")
	if n := strings.Count(block, "<image:image>"); n != 1 {
		t.Errorf("want 1 image entry for a logo alone; got %d:\n%s", n, block)
	}
	if !strings.Contains(block, "<image:loc>https://cdn.example/logo.png</image:loc>") {
		t.Errorf("the logo is not listed:\n%s", block)
	}
}

func TestSitemapListsNoImageForAComplexWithoutPhotos(t *testing.T) {
	doc := sitemapFor(t,
		photoSlug("sin-fotos", nil, nil),
		photoSlug("fotos-vacias", urlPtr(""), urlPtr("   ")),
	)

	for _, loc := range []string{"https://vibe.example/c/sin-fotos", "https://vibe.example/c/fotos-vacias"} {
		if block := urlBlock(doc, loc); strings.Contains(block, "image:") {
			t.Errorf("%s lists image entries although it has no photos:\n%s", loc, block)
		}
	}
}

func TestSitemapEscapesAnImageURLForXML(t *testing.T) {
	doc := sitemapFor(t, photoSlug("club-norte", urlPtr("https://cdn.example/cover.jpg?w=640&h=480"), nil))

	block := urlBlock(doc, "https://vibe.example/c/club-norte")
	if !strings.Contains(block, "<image:loc>https://cdn.example/cover.jpg?w=640&amp;h=480</image:loc>") {
		t.Errorf("the image URL is not XML-escaped:\n%s", block)
	}
}

func TestSitemapSkipsAPhotoThatIsNotAnAbsoluteHTTPURL(t *testing.T) {
	bad := []string{"/relative/cover.jpg", "javascript:alert(1)", "ftp://cdn.example/cover.jpg", "https://cdn.example/with space.jpg"}
	var slugs []complexstore.ComplexSlug
	for i, u := range bad {
		slugs = append(slugs, photoSlug("club-"+string(rune('a'+i)), urlPtr(u), nil))
	}

	doc := sitemapFor(t, slugs...)
	for i, u := range bad {
		loc := "https://vibe.example/c/club-" + string(rune('a'+i))
		if block := urlBlock(doc, loc); strings.Contains(block, "image:") {
			t.Errorf("the photo %q was listed:\n%s", u, block)
		}
	}
}

// The image entries must keep the document well-formed for a namespace-aware parser:
// the image prefix has to resolve to Google's namespace, and the escaped URL has to
// read back as the original. Captions and titles are deprecated and never written.
func TestSitemapWithImagesParsesAsXMLAndBindsTheImageNamespace(t *testing.T) {
	doc := sitemapFor(t,
		photoSlug("club-norte", urlPtr("https://cdn.example/cover.jpg?w=640&h=480"), urlPtr("https://cdn.example/logo.png")),
		photoSlug("sin-fotos", nil, nil),
	)

	if !strings.Contains(doc, `xmlns:image="`+googleImageNS+`"`) {
		t.Errorf("the urlset does not declare the image namespace:\n%s", doc)
	}
	if strings.Contains(doc, "image:caption") || strings.Contains(doc, "image:title") {
		t.Errorf("the sitemap writes deprecated image caption or title elements:\n%s", doc)
	}

	dec := xml.NewDecoder(strings.NewReader(doc))
	images := 0
	var locs []string
	inImageLoc := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("the sitemap is not well-formed XML: %v\n%s", err, doc)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			if el.Name.Space == googleImageNS && el.Name.Local == "image" {
				images++
			}
			inImageLoc = el.Name.Space == googleImageNS && el.Name.Local == "loc"
		case xml.EndElement:
			inImageLoc = false
		case xml.CharData:
			if inImageLoc {
				locs = append(locs, string(el))
			}
		}
	}

	if images != 2 {
		t.Errorf("want 2 image elements in Google's namespace; got %d:\n%s", images, doc)
	}
	want := []string{"https://cdn.example/cover.jpg?w=640&h=480", "https://cdn.example/logo.png"}
	if strings.Join(locs, "|") != strings.Join(want, "|") {
		t.Errorf("image locations = %q; want %q", locs, want)
	}
}
