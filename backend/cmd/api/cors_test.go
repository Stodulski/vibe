package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/cors"
)

// newTestCORSHandler builds the same rs/cors handler routes() wires up, so a
// test exercises the exact configuration the server runs rather than a
// reconstruction of it.
func newTestCORSHandler() *cors.Cors {
	return cors.New(corsOptions("http://localhost:5173"))
}

// preflight builds and serves one OPTIONS preflight against handler and
// returns the recorder.
func preflight(handler http.Handler, reqHeaders string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/v1/auth/me", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	r.Header.Set("Access-Control-Request-Method", "GET")
	r.Header.Set("Access-Control-Request-Headers", reqHeaders)
	handler.ServeHTTP(w, r)
	return w
}

// TestBareRSCORSIsCaseSensitiveOnRequestHeaders documents and confirms, from
// the actual library rather than the source comment alone, why the QA
// observation reproduces: rs/cors (see its "Allowed Headers" comment in
// cors.go) relies on the Fetch standard's guarantee that a browser always
// lowercases CORS-unsafe request-header names before sending
// Access-Control-Request-Headers. It does not itself lowercase that incoming
// value before matching it against its own (already-lowercased)
// AllowedHeaders set. A spec-compliant browser never sends mixed case, so
// this is not reachable from real browser traffic — but a raw HTTP client,
// SDK, or test tool can send whatever case it likes, and the bare library
// then answers with no Access-Control-Allow-* headers at all.
func TestBareRSCORSIsCaseSensitiveOnRequestHeaders(t *testing.T) {
	tests := []struct {
		name        string
		reqHeaders  string
		wantAllowed bool
	}{
		{"lowercase, as a spec-compliant browser sends it", "x-csrf-token", true},
		{"mixed case, as a raw HTTP client might send it", "X-CSRF-Token", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newTestCORSHandler()
			w := preflight(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handler.HandlerFunc(w, r)
			}), tt.reqHeaders)

			if w.Code != http.StatusNoContent {
				t.Errorf("want 204 from the CORS handler; got %d", w.Code)
			}

			got := w.Header().Get("Access-Control-Allow-Headers") != ""
			if got != tt.wantAllowed {
				t.Errorf("Access-Control-Request-Headers: %q: want CORS headers present=%v; got=%v (Allow-Origin=%q, Allow-Headers=%q)",
					tt.reqHeaders, tt.wantAllowed, got,
					w.Header().Get("Access-Control-Allow-Origin"), w.Header().Get("Access-Control-Allow-Headers"))
			}
		})
	}
}

// TestNormalizeCORSPreflightHeadersFixesMixedCase proves the fix: routes()
// wraps rs/cors' handler in normalizeCORSPreflightHeaders, which lowercases
// Access-Control-Request-Headers before rs/cors ever sees it, so both a
// spec-compliant lowercase request and a raw mixed-case one now get full
// CORS headers back.
func TestNormalizeCORSPreflightHeadersFixesMixedCase(t *testing.T) {
	corsHandler := newTestCORSHandler()
	fixed := normalizeCORSPreflightHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corsHandler.HandlerFunc(w, r)
	}))

	for _, reqHeaders := range []string{"x-csrf-token", "X-CSRF-Token"} {
		t.Run(reqHeaders, func(t *testing.T) {
			w := preflight(fixed, reqHeaders)

			if w.Code != http.StatusNoContent {
				t.Errorf("want 204; got %d", w.Code)
			}
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
				t.Errorf("want Access-Control-Allow-Origin; got %q", got)
			}
			if got := w.Header().Get("Access-Control-Allow-Headers"); got == "" {
				t.Errorf("want Access-Control-Allow-Headers to be set for %q", reqHeaders)
			}
		})
	}
}

// TestTheRequestIDIsExposedToTheBrowser pins the header the frontend needs to
// show a correlation id in an error toast. A browser hides every response
// header outside the CORS-safelist unless the server names it, so without this
// the id was readable by curl and invisible to the page.
func TestTheRequestIDIsExposedToTheBrowser(t *testing.T) {
	handler := newTestCORSHandler().Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-ID", "01JABCDEF")
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/me", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	handler.ServeHTTP(w, r)

	// rs/cors writes the name back canonicalized ("X-Request-Id"); header
	// names are case-insensitive, and so is the browser's own matching.
	got := w.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(strings.ToLower(got), "x-request-id") {
		t.Errorf("Access-Control-Expose-Headers = %q, want it to name X-Request-ID", got)
	}
}
