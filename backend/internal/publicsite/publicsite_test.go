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
	r.SetPathValue("slug", slug)
	return r
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
		"<loc>https://vibe.example/vibe-palermo</loc>",
		"<lastmod>2026-03-01</lastmod>",
		"<loc>https://vibe.example/vibe-pilar</loc>",
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

	// The trailing slash on the configured frontend URL must not double up.
	if strings.Contains(body, "//vibe-palermo") {
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

	h := NewHandler(NewService(store, frontendServing(t, baseTemplate)), testResponder())
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
		`content="Reserva canchas en Vibe Palermo. Rápido y seguro."`,
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

	h := NewHandler(NewService(store, frontendServing(t, baseTemplate)), testResponder())
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
	h := NewHandler(NewService(&stubStore{complexErr: data.ErrRecordNotFound}, "https://vibe.example"), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "missing"))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404; got %d", w.Code)
	}
}

func TestPrerenderRejectsAnEmptySlug(t *testing.T) {
	h := NewHandler(NewService(&stubStore{}, "https://vibe.example"), testResponder())
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

	h := NewHandler(NewService(store, frontendServing(t, baseTemplate)), testResponder())
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

// frontendIndexHead is a literal copy of the tags frontend/index.html ships,
// taken from that file rather than built from this package's constants.
//
// That is the whole point of it. baseTemplate above is assembled FROM the
// constants, so every substitution test passes whether or not the constants
// match reality — which is exactly how three of the five came to be missing
// their accents and the og:image came to be relative while the frontend's was
// absolute, with the prerenderer silently substituting nothing and every venue
// page going out carrying Vibe's generic description.
//
// When the frontend edits one of these tags, this constant is what has to be
// updated, and the test below is what says so.
const frontendIndexHead = `` +
	`<meta name="description" content="Vibe - Gestión de complejos deportivos, reservas y canchas" />` + "\n" +
	`<meta property="og:title" content="Vibe - Reserva tu cancha" />` + "\n" +
	`<meta property="og:description" content="Reserva canchas de pádel, tenis y fútbol de forma rápida y segura." />` + "\n" +
	`<meta property="og:image" content="https://app.vibe.com.ar/logo.png" />` + "\n" +
	`<title>Vibe</title>`

func TestEveryPlaceholderIsFoundInTheFrontendsRealIndexHTML(t *testing.T) {
	for name, placeholder := range map[string]string{
		"title tag":      placeholderTitleTag,
		"description":    placeholderDescription,
		"og:title":       placeholderOGTitle,
		"og:description": placeholderOGDesc,
		"og:image":       placeholderImage,
	} {
		if !strings.Contains(frontendIndexHead, placeholder) {
			t.Errorf("the %s placeholder %q is not in frontend/index.html; prerendering substitutes nothing "+
				"and the page goes out with Vibe's generic copy", name, placeholder)
		}
	}
}

// The substitution has to work against the frontend's real markup, not only
// against a template this package assembled from its own constants.
func TestPrerenderSubstitutesIntoTheFrontendsRealMarkup(t *testing.T) {
	store := &stubStore{
		complex: &complexstore.Complex{
			ID: uuid.New(), Name: "Vibe Palermo", Phone: "+541100000000",
			Address: "Av. Santa Fe 1234", City: "CABA", Province: "Buenos Aires",
			CountryCode: "AR", IsActive: true,
		},
	}
	index := `<!doctype html><html><head>` + frontendIndexHead + `</head><body></body></html>`

	h := NewHandler(NewService(store, frontendServing(t, index)), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "vibe-palermo"))

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, body)
	}
	for _, want := range []string{
		"<title>Vibe Palermo - Reserva tu cancha | Vibe</title>",
		`content="Reserva canchas en Vibe Palermo. Rápido y seguro."`,
		`content="https://app.vibe.com.ar/logo.png"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the frontend's real markup was not substituted: missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, "Gestión de complejos deportivos") {
		t.Error("the generic description survived; the placeholder did not match")
	}
}

// A crawler treats a 5xx as transient and retries it later, but reads a 2xx
// as final and indexes whatever page came back. A store failure used to
// answer 200 with the generic shell for exactly that reason — but the shell
// then got indexed in this venue's own place, which a crawler never revisits
// on its own; a 503 it retries once the read works again costs nothing.
func TestAStoreFailureAnswers503WithRetryAfter(t *testing.T) {
	store := &stubStore{complexErr: errors.New("db down")}

	h := NewHandler(NewService(store, frontendServing(t, baseTemplate)), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "vibe-palermo"))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503; got %d (%s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After = %q, want 60", got)
	}
}

// An unknown slug is still a 404: there is no page, and telling a crawler
// otherwise would have it index a URL that means nothing.
func TestAnUnknownSlugIsStillNotFound(t *testing.T) {
	store := &stubStore{complexErr: data.ErrRecordNotFound}

	h := NewHandler(NewService(store, frontendServing(t, baseTemplate)), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "nobody"))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
	// The frontend's edge only passes a 404 through to a crawler when this
	// header says so; any other 404 becomes a 503 on its side.
	if got := w.Header().Get("X-Prerender-Result"); got != "venue-not-found" {
		t.Errorf("X-Prerender-Result = %q, want venue-not-found", got)
	}
}

// The schedules are one block of the JSON-LD. Losing them costs the opening
// hours; the title, description, image and canonical URL all come off the
// complex this already holds.
func TestAScheduleFailureStillRendersTheComplexsOwnPage(t *testing.T) {
	store := &stubStore{
		complex: &complexstore.Complex{
			ID: uuid.New(), Name: "Vibe Palermo", Phone: "+541100000000",
			Address: "Av. Santa Fe 1234", City: "CABA", Province: "Buenos Aires",
			CountryCode: "AR", IsActive: true,
		},
		schedulesErr: errors.New("db down"),
	}

	h := NewHandler(NewService(store, frontendServing(t, baseTemplate)), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "vibe-palermo"))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "<title>Vibe Palermo - Reserva tu cancha | Vibe</title>") {
		t.Error("the complex's own title was lost along with its schedules")
	}
	if strings.Contains(body, "openingHoursSpecification") {
		t.Error("opening hours were published from a failed read")
	}
}

// The template has never been fetched successfully and there is no stale copy:
// the frontend was down when this process started. The built-in shell carries
// the same placeholders, so the venue's own tags still land.
func TestAnUnreachableFrontendStillProducesAPage(t *testing.T) {
	store := &stubStore{
		complex: &complexstore.Complex{
			ID: uuid.New(), Name: "Vibe Palermo", Phone: "+541100000000",
			Address: "Av. Santa Fe 1234", City: "CABA", Province: "Buenos Aires",
			CountryCode: "AR", IsActive: true,
		},
	}

	// A frontend origin nothing answers on.
	h := NewHandler(NewService(store, "http://127.0.0.1:1"), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "vibe-palermo"))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 from the built-in shell; got %d (%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "<title>Vibe Palermo - Reserva tu cancha | Vibe</title>") {
		t.Errorf("the built-in shell did not take this complex's tags\n%s", body)
	}
	if !strings.Contains(body, `<link rel="canonical"`) {
		t.Error("the built-in shell was served without a canonical URL")
	}
}
