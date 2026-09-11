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
