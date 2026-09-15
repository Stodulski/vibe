package openapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/stodulski/vibe-server/internal/httpx"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()

	respond := httpx.NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	h, err := NewHandler(respond)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return h
}

// TestEmbeddedDocumentLoadsAndValidates is the boot-time check this test
// exercises directly: a broken openapi.yaml must fail here, not in
// production.
func TestEmbeddedDocumentLoadsAndValidates(t *testing.T) {
	h := newTestHandler(t)

	if err := h.Document().Validate(context.Background()); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if h.Document().Info == nil || h.Document().Info.Title != "Vibe API" {
		t.Fatalf("unexpected Info: %+v", h.Document().Info)
	}
}

func TestJSONRoute(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/openapi.json", nil)
	rr := httptest.NewRecorder()
	h.JSON(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var doc map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &doc); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Errorf("openapi field = %v, want 3.1.0", doc["openapi"])
	}
}

func TestYAMLRoute(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/openapi.yaml", nil)
	rr := httptest.NewRecorder()
	h.YAML(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/yaml" {
		t.Errorf("Content-Type = %q, want application/yaml", ct)
	}
	if !strings.Contains(rr.Body.String(), "openapi: 3.1.0") {
		t.Error("response body does not look like the committed openapi.yaml")
	}
}

func TestDocsRoute(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/docs", nil)
	rr := httptest.NewRecorder()
	h.Docs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if got := rr.Header().Get("X-Robots-Tag"); got != "noindex" {
		t.Errorf("X-Robots-Tag = %q, want noindex", got)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "/api/v1/openapi.json") {
		t.Error("docs page does not reference /api/v1/openapi.json")
	}
	if !strings.Contains(body, "@scalar/api-reference") {
		t.Error("docs page does not load Scalar's API reference")
	}
}

// TestCatalogRoute proves GET /.well-known/api-catalog answers an RFC 9727
// linkset whose hrefs are built from the embedded document's own https
// server and paths — not hardcoded — so a renamed route or a changed
// production domain shows up here instead of only in a hand-maintained copy.
func TestCatalogRoute(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/api-catalog", nil)
	rr := httptest.NewRecorder()
	h.Catalog(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/linkset+json" {
		t.Errorf("Content-Type = %q, want application/linkset+json", ct)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}

	var body struct {
		Linkset []struct {
			Anchor      string `json:"anchor"`
			ServiceDesc []struct {
				Href string `json:"href"`
				Type string `json:"type"`
			} `json:"service-desc"`
			ServiceDoc []struct {
				Href string `json:"href"`
			} `json:"service-doc"`
			Status []struct {
				Href string `json:"href"`
			} `json:"status"`
			Author []struct {
				Href string `json:"href"`
			} `json:"author"`
		} `json:"linkset"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if len(body.Linkset) != 1 {
		t.Fatalf("linkset has %d entries, want 1", len(body.Linkset))
	}
	entry := body.Linkset[0]

	// The embedded document's first https server (see catalogBaseURL) is the
	// base every advertised href must be built from.
	var wantBase string
	for _, s := range h.Document().Servers {
		if strings.HasPrefix(s.URL, "https://") {
			wantBase = s.URL
			break
		}
	}
	if wantBase == "" {
		t.Fatal("test setup: embedded document has no https server")
	}

	if entry.Anchor != wantBase+"/api/v1" {
		t.Errorf("anchor = %q, want %q", entry.Anchor, wantBase+"/api/v1")
	}
	if len(entry.ServiceDesc) != 2 {
		t.Fatalf("service-desc has %d entries, want 2", len(entry.ServiceDesc))
	}
	wantServiceDesc := map[string]string{
		wantBase + "/api/v1/openapi.json": "application/openapi+json",
		wantBase + "/api/v1/openapi.yaml": "application/openapi+yaml",
	}
	for _, ref := range entry.ServiceDesc {
		wantType, ok := wantServiceDesc[ref.Href]
		if !ok {
			t.Errorf("unexpected service-desc href %q", ref.Href)
			continue
		}
		if ref.Type != wantType {
			t.Errorf("service-desc %q type = %q, want %q", ref.Href, ref.Type, wantType)
		}
	}
	if len(entry.ServiceDoc) != 1 || entry.ServiceDoc[0].Href != wantBase+"/api/v1/docs" {
		t.Errorf("service-doc = %+v, want one entry for %s/api/v1/docs", entry.ServiceDoc, wantBase)
	}
	if len(entry.Status) != 1 || entry.Status[0].Href != wantBase+"/api/v1/healthcheck" {
		t.Errorf("status = %+v, want one entry for %s/api/v1/healthcheck", entry.Status, wantBase)
	}
	if len(entry.Author) != 1 || entry.Author[0].Href != catalogSiteURL {
		t.Errorf("author = %+v, want one entry for %s", entry.Author, catalogSiteURL)
	}
}

// TestBuildCatalogFailsLoudlyOnAMissingAdvertisedPath proves buildCatalog
// refuses to render a catalog that would point an agent at a route the
// document does not serve, rather than silently omitting the link or
// shipping a dead href — the same failure mode
// landing/scripts/build-api-catalog.mjs guards against for the landing's own
// copy of this document.
func TestBuildCatalogFailsLoudlyOnAMissingAdvertisedPath(t *testing.T) {
	h := newTestHandler(t)

	// h.Document() is the process-wide shared document (see the "loaded"
	// comment in openapi.go): copying doc.Paths into a fresh *openapi3.Paths
	// that omits one path, rather than calling Paths.Delete on the shared
	// pointer, is what keeps this test from corrupting every other test's
	// view of the embedded document.
	doc := *h.Document()
	pruned := openapi3.NewPathsWithCapacity(doc.Paths.Len())
	for path, item := range doc.Paths.Map() {
		if path == "/api/v1/docs" {
			continue
		}
		pruned.Set(path, item)
	}
	doc.Paths = pruned

	if _, err := buildCatalog(&doc); err == nil {
		t.Fatal("buildCatalog did not fail with /api/v1/docs removed from the document")
	} else if !strings.Contains(err.Error(), "/api/v1/docs") {
		t.Errorf("error does not name the missing path: %v", err)
	}
}

// TestRoutesRegistersAllThree proves Routes wires exactly the three
// documented endpoints, so a future edit here cannot silently drop one.
func TestRoutesRegistersAllThree(t *testing.T) {
	h := newTestHandler(t)

	got := map[string]bool{}
	rec := recordingRouter{routes: &got}
	h.Routes(rec, httpx.Guards{})

	want := []string{
		"GET /api/v1/openapi.json",
		"GET /api/v1/openapi.yaml",
		"GET /api/v1/docs",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("Routes did not register %q", w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("Routes registered %d routes, want %d: %v", len(got), len(want), got)
	}
}

type recordingRouter struct {
	routes *map[string]bool
}

func (r recordingRouter) HandlerFunc(method, path string, _ http.HandlerFunc) {
	(*r.routes)[method+" "+path] = true
}

func (r recordingRouter) Handler(method, path string, _ http.Handler) {
	(*r.routes)[method+" "+path] = true
}
