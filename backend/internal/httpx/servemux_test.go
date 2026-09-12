package httpx_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/httpx"
)

func newTestServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()
	rs := newResponder()
	mux := httpx.NewServeMux(http.HandlerFunc(rs.NotFound), http.HandlerFunc(rs.MethodNotAllowed))
	mux.HandlerFunc(http.MethodGet, "/a", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("get-a")) })
	mux.HandlerFunc(http.MethodPost, "/a", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
	mux.HandlerFunc(http.MethodGet, "/b/{id}", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("get-b-" + r.PathValue("id"))) })
	ts := httptest.NewServer(mux.Build())
	t.Cleanup(ts.Close)
	return ts, &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func do(t *testing.T, ts *httptest.Server, client *http.Client, method, path string) (status int, header http.Header, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, ts.URL+path, nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

func allowSet(header string) map[string]bool {
	set := make(map[string]bool)
	for _, m := range strings.Split(header, ", ") {
		if m != "" {
			set[m] = true
		}
	}
	return set
}

func TestServeMuxMiss(t *testing.T) {
	ts, client := newTestServer(t)
	t.Run("unknown path answers the JSON 404", func(t *testing.T) {
		status, header, _ := do(t, ts, client, http.MethodGet, "/nope")
		if status != http.StatusNotFound || !strings.Contains(header.Get("Content-Type"), "application/json") {
			t.Errorf("GET /nope = %d %q, want 404 JSON", status, header.Get("Content-Type"))
		}
	})
	t.Run("known path, unregistered method answers 405 with exactly the registered methods", func(t *testing.T) {
		status, header, _ := do(t, ts, client, http.MethodDelete, "/a")
		got, want := allowSet(header.Get("Allow")), map[string]bool{"GET": true, "HEAD": true, "POST": true, "OPTIONS": true}
		if status != http.StatusMethodNotAllowed || len(got) != len(want) {
			t.Fatalf("DELETE /a = %d Allow=%v, want 405 %v", status, got, want)
		}
		for m := range want {
			if !got[m] {
				t.Errorf("Allow = %v, missing %s", got, m)
			}
		}
	})
	t.Run("a GET-only path answers HEAD like net/http itself", func(t *testing.T) {
		if status, _, body := do(t, ts, client, http.MethodHead, "/a"); status != http.StatusOK || body != "" {
			t.Errorf("HEAD /a = %d %q, want 200 empty", status, body)
		}
	})
	t.Run("a bare OPTIONS answers 200 with Allow", func(t *testing.T) {
		if status, header, _ := do(t, ts, client, http.MethodOptions, "/a"); status != http.StatusOK || header.Get("Allow") == "" {
			t.Errorf("OPTIONS /a = %d Allow=%q, want 200 with Allow", status, header.Get("Allow"))
		}
	})
	t.Run("registered method+path reaches its own handler ahead of the methodless fallback", func(t *testing.T) {
		if status, _, body := do(t, ts, client, http.MethodGet, "/a"); status != http.StatusOK || body != "get-a" {
			t.Errorf("GET /a = %d %q, want 200 get-a", status, body)
		}
		if status, _, _ := do(t, ts, client, http.MethodPost, "/a"); status != http.StatusCreated {
			t.Errorf("POST /a = %d, want 201", status)
		}
		if status, _, body := do(t, ts, client, http.MethodGet, "/b/123"); status != http.StatusOK || body != "get-b-123" {
			t.Errorf("GET /b/123 = %d %q, want 200 get-b-123", status, body)
		}
	})
}

func TestServeMuxRedirectsAMissThatCleansToARegisteredPath(t *testing.T) {
	ts, client := newTestServer(t)
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantLoc    string
	}{
		{"trailing slash on GET redirects 301", http.MethodGet, "/a/", http.StatusMovedPermanently, "/a"},
		{"trailing slash on POST redirects 308, method-preserving", http.MethodPost, "/a/", http.StatusPermanentRedirect, "/a"},
		{"the query string rides along to the canonical path", http.MethodGet, "/a/?foo=bar", http.StatusMovedPermanently, "/a?foo=bar"},
		{"a wildcard pattern is repaired the same way", http.MethodGet, "/b/123/", http.StatusMovedPermanently, "/b/123"},
		{"a non-repairable path still answers 404, not a redirect", http.MethodGet, "/nope/", http.StatusNotFound, ""},
		{"the root path is never redirected", http.MethodGet, "/", http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, header, _ := do(t, ts, client, tt.method, tt.path)
			if status != tt.wantStatus || header.Get("Location") != tt.wantLoc {
				t.Errorf("%s %s = %d Location=%q, want %d Location=%q", tt.method, tt.path, status, header.Get("Location"), tt.wantStatus, tt.wantLoc)
			}
		})
	}
}
