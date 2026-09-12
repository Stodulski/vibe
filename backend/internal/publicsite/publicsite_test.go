package publicsite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

type stubStore struct {
	slugs     []complexstore.ComplexSlug
	complex   *complexstore.Complex
	schedules []*complexstore.Schedule

	slugsErr     error
	complexErr   error
	schedulesErr error
}

func (s *stubStore) GetAllSlugs(context.Context) ([]complexstore.ComplexSlug, error) {
	return s.slugs, s.slugsErr
}

func (s *stubStore) GetBySlug(context.Context, string) (*complexstore.Complex, error) {
	if s.complexErr != nil {
		return nil, s.complexErr
	}
	return s.complex, nil
}

func (s *stubStore) GetSchedules(context.Context, uuid.UUID) ([]*complexstore.Schedule, error) {
	return s.schedules, s.schedulesErr
}

func testResponder() *httpx.Responder {
	return httpx.NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
}

// frontendServing stands up a stub frontend returning the given index.html.
func frontendServing(t *testing.T, indexHTML string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(indexHTML))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// baseTemplate carries the placeholders the frontend actually ships, so a
// mismatch between this package's constants and that file shows up as a test
// failure rather than as silently unprerendered pages.
const baseTemplate = `<!DOCTYPE html><html><head>` +
	placeholderTitleTag +
	`<meta name="description" ` + placeholderDescription + `>` +
	`<meta property="og:title" ` + placeholderOGTitle + `>` +
	`<meta property="og:description" ` + placeholderOGDesc + `>` +
	`<meta property="og:image" ` + placeholderImage + `>` +
	`</head><body></body></html>`

func slugRequest(t *testing.T, slug string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	params := httprouter.Params{{Key: "slug", Value: slug}}
	return r.WithContext(context.WithValue(r.Context(), httprouter.ParamsKey, params))
}

func TestSitemapListsHomepageAndEveryComplex(t *testing.T) {
	store := &stubStore{slugs: []complexstore.ComplexSlug{
		{Slug: "vibe-palermo", UpdatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{Slug: "vibe-pilar", UpdatedAt: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)},
	}}

	h := NewHandler(store, testResponder(), "https://vibe.example/")
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
		"<loc>https://vibe.example/</loc>",
		"<loc>https://vibe.example/vibe-palermo</loc>",
		"<lastmod>2026-03-01</lastmod>",
		"<loc>https://vibe.example/vibe-pilar</loc>",
		"<lastmod>2026-04-02</lastmod>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap is missing %q\n%s", want, body)
		}
	}

	// The trailing slash on the configured frontend URL must not double up.
	if strings.Contains(body, "//vibe-palermo") {
		t.Error("the base URL's trailing slash produced a doubled separator")
	}
}

func TestSitemapReportsStoreFailure(t *testing.T) {
	h := NewHandler(&stubStore{slugsErr: errors.New("db down")}, testResponder(), "https://vibe.example")
	w := httptest.NewRecorder()
	h.Sitemap(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500; got %d", w.Code)
	}
}

func TestPrerenderSubstitutesTheComplexMetadata(t *testing.T) {
	logo := "https://cdn.example/logo.png"
	store := &stubStore{
		complex: &complexstore.Complex{
			ID: uuid.New(), Name: "Vibe Palermo", Phone: "+541100000000",
			Address: "Av. Santa Fe 1234", City: "CABA", Province: "Buenos Aires",
			CountryCode: "AR", LogoURL: &logo, IsActive: true,
		},
		schedules: []*complexstore.Schedule{
			{Day: "monday", OpenTime: "08:00", CloseTime: "23:00"},
			{Day: "sunday", IsClosed: true},
		},
	}

	h := NewHandler(store, testResponder(), frontendServing(t, baseTemplate))
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "vibe-palermo"))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("want an HTML content type; got %q", got)
	}

	body := w.Body.String()
	for _, want := range []string{
		"<title>Vibe Palermo - Reserva tu cancha | Vibe</title>",
		`content="Reserva canchas en Vibe Palermo. Rapido y seguro."`,
		`content="https://cdn.example/logo.png"`,
		`<link rel="canonical"`,
		`<script type="application/ld+json">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("prerendered page is missing %q", want)
		}
	}

	// Every placeholder must be gone; a leftover means the frontend's markup
	// and this package's constants have drifted apart.
	for _, placeholder := range []string{
		placeholderTitleTag, placeholderDescription, placeholderOGTitle, placeholderOGDesc,
	} {
		if strings.Contains(body, placeholder) {
			t.Errorf("placeholder %q survived substitution", placeholder)
		}
	}
}

// Complex names are owner-supplied and end up inside markup, so they must be
// escaped rather than interpolated raw.
func TestPrerenderEscapesOwnerSuppliedText(t *testing.T) {
	store := &stubStore{
		complex: &complexstore.Complex{ID: uuid.New(), Name: `Vibe <script>alert(1)</script>`, IsActive: true},
	}

	h := NewHandler(store, testResponder(), frontendServing(t, baseTemplate))
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "x"))

	body := w.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Errorf("an owner-supplied name was injected unescaped:\n%s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("the name should appear escaped in the title")
	}
}

// The JSON-LD block sits inside a <script> tag, so a name containing </script>
// must not be able to close it.
func TestStructuredDataCannotBreakOutOfItsScriptTag(t *testing.T) {
	complex := &complexstore.Complex{ID: uuid.New(), Name: `</script><img src=x onerror=alert(1)>`}

	got := structuredData(complex, nil, "https://vibe.example/x")

	if strings.Contains(got, "</script>") {
		t.Errorf("the structured data closed its own script tag:\n%s", got)
	}
	// It must still be valid JSON carrying the original name.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("structured data is not valid JSON: %v", err)
	}
	if parsed["name"] != complex.Name {
		t.Errorf("the name was altered; got %v", parsed["name"])
	}
}

func TestStructuredDataOmitsAbsentFieldsAndClosedDays(t *testing.T) {
	complex := &complexstore.Complex{ID: uuid.New(), Name: "Vibe", Phone: "+5411"}
	schedules := []*complexstore.Schedule{
		{Day: "monday", OpenTime: "08:00", CloseTime: "23:00"},
		{Day: "sunday", IsClosed: true},
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(structuredData(complex, schedules, "https://vibe.example/x")), &parsed); err != nil {
		t.Fatalf("structured data is not valid JSON: %v", err)
	}

	// An empty value in structured data is treated as an error by validators,
	// so absent optional fields must be omitted rather than emitted blank.
	for _, key := range []string{"description", "image", "email", "geo"} {
		if _, present := parsed[key]; present {
			t.Errorf("%q should be omitted when the complex has no value for it", key)
		}
	}

	hours, ok := parsed["openingHoursSpecification"].([]any)
	if !ok {
		t.Fatalf("opening hours are missing; got %v", parsed["openingHoursSpecification"])
	}
	if len(hours) != 1 {
		t.Fatalf("a closed day must not be published as opening hours; got %d entries", len(hours))
	}
	if day := hours[0].(map[string]any)["dayOfWeek"]; day != "Monday" {
		t.Errorf("schema.org needs the capitalised day name; got %v", day)
	}
}

func TestPrerenderReportsUnknownSlugAsNotFound(t *testing.T) {
	h := NewHandler(&stubStore{complexErr: data.ErrRecordNotFound}, testResponder(), "https://vibe.example")
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "missing"))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404; got %d", w.Code)
	}
}

func TestPrerenderRejectsAnEmptySlug(t *testing.T) {
	h := NewHandler(&stubStore{}, testResponder(), "https://vibe.example")
	w := httptest.NewRecorder()
	h.Prerender(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404; got %d", w.Code)
	}
}

func TestTemplateIsCachedWithinItsTTL(t *testing.T) {
	var fetches int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches++
		_, _ = w.Write([]byte(baseTemplate))
	}))
	t.Cleanup(srv.Close)

	tc := newTemplateCache(time.Minute, time.Second)
	for range 3 {
		if _, err := tc.get(t.Context(), srv.URL); err != nil {
			t.Fatalf("fetching the template: %v", err)
		}
	}

	if fetches != 1 {
		t.Errorf("want a single fetch within the TTL; got %d", fetches)
	}
}

// A briefly unreachable frontend must not cost a crawler its page: serving the
// last known markup beats serving a 500.
func TestStaleTemplateIsServedWhenTheFrontendIsDown(t *testing.T) {
	var up = true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !up {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(baseTemplate))
	}))
	t.Cleanup(srv.Close)

	// A zero TTL forces a refetch on every call.
	tc := newTemplateCache(0, time.Second)
	first, err := tc.get(t.Context(), srv.URL)
	if err != nil || first == "" {
		t.Fatalf("the first fetch should succeed; got %q, %v", first, err)
	}

	up = false
	srv.Close()

	second, err := tc.get(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("want the stale copy rather than an error; got %v", err)
	}
	if second != first {
		t.Error("the stale copy was not returned")
	}
}

func TestTemplateFetchFailsWhenNothingIsCached(t *testing.T) {
	tc := newTemplateCache(time.Minute, 100*time.Millisecond)

	// A port nothing listens on, so the fetch fails with no cached fallback.
	if _, err := tc.get(t.Context(), "http://127.0.0.1:1"); err == nil {
		t.Error("want an error on the first fetch when the frontend is unreachable")
	}
}

// FINDING 4. The sitemap already filters on is_active, so a deactivated venue
// stayed reachable only through this page — indexable and shareable through a
// link a crawler had already seen, with its address, phone and opening hours in
// structured data.
func TestPrerenderIsClosedForADeactivatedComplex(t *testing.T) {
	store := &stubStore{
		complex: &complexstore.Complex{
			ID: uuid.New(), Name: "Vibe Palermo", Phone: "+541100000000",
			Address: "Av. Santa Fe 1234", City: "CABA", IsActive: false,
		},
		schedules: []*complexstore.Schedule{{Day: "monday", OpenTime: "08:00", CloseTime: "23:00"}},
	}

	h := NewHandler(store, testResponder(), frontendServing(t, baseTemplate))
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "vibe"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("a deactivated venue must not be prerendered; got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "Av. Santa Fe 1234") {
		t.Errorf("its address must not reach a crawler; got %s", w.Body.String())
	}
}

// An error page is not a template. The fetch reported only transport failures,
// so a frontend that answered 404 or 503 — a deploy in flight, a CDN error
// page — was a *successful* fetch of whatever body came back. That body then
// replaced the good cached copy and was served to crawlers under every
// complex's canonical URL with a 200 and a public cache header.
//
// It also defeated the stale-is-better-than-an-error rule one line below the
// fetch, which only runs when the fetch returns an error: the exact case that
// rule exists for was the one case that never reached it. Note that
// TestStaleTemplateIsServedWhenTheFrontendIsDown closes the server as well as
// setting its status, so it proves the transport-failure path and not this one.
func TestAnErrorPageIsNotCachedAsTheTemplate(t *testing.T) {
	t.Run("a non-2xx body never becomes the template", func(t *testing.T) {
		tc := newTemplateCache(time.Minute, time.Second)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("<html><body>404 - Not Found</body></html>"))
		}))
		t.Cleanup(srv.Close)

		got, err := tc.get(t.Context(), srv.URL)
		if err == nil {
			t.Fatalf("want an error for a 404 index.html; got the body %q", got)
		}
		if tc.template != "" {
			t.Errorf("a 404 page was stored as the template: %q", tc.template)
		}
	})

	t.Run("a good template survives the frontend answering 503", func(t *testing.T) {
		healthy := true
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if !healthy {
				// Still listening, still answering — just not with the app.
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("<html><body>Service Unavailable</body></html>"))
				return
			}
			_, _ = w.Write([]byte(baseTemplate))
		}))
		t.Cleanup(srv.Close)

		// A zero TTL forces a refetch on every call.
		tc := newTemplateCache(0, time.Second)
		first, err := tc.get(t.Context(), srv.URL)
		if err != nil {
			t.Fatalf("the first fetch should succeed: %v", err)
		}

		healthy = false

		second, err := tc.get(t.Context(), srv.URL)
		if err != nil {
			t.Fatalf("want the stale copy rather than an error; got %v", err)
		}
		if second != first {
			t.Errorf("the 503 page replaced the cached template; got %q", second)
		}
	})
}
