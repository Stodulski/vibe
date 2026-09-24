package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/rs/cors"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi"
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

// declaredHeaderParameters walks the embedded OpenAPI document and returns
// every distinct header name declared by a Parameter Object with `in:
// header`, across every operation's own parameters, every path item's
// shared parameters, and components.parameters (the ones referenced by
// $ref, such as Idempotency-Key and If-Match). Lowercased, because that is
// how a browser sends Access-Control-Request-Headers and how rs/cors
// matches AllowedHeaders.
//
// This is what makes the CORS guard spec-driven rather than a fixed list: a
// header parameter added to openapi.yaml without a matching AllowedHeaders
// entry fails TestEveryDeclaredHeaderParameterIsAllowedByCORS, instead of
// shipping a preflight rejection that is only found in production.
func declaredHeaderParameters(t *testing.T, doc *openapi3.T) []string {
	t.Helper()

	seen := map[string]bool{}
	add := func(p *openapi3.Parameter) {
		if p != nil && strings.EqualFold(p.In, openapi3.ParameterInHeader) {
			seen[strings.ToLower(p.Name)] = true
		}
	}

	for _, ref := range doc.Components.Parameters {
		if ref != nil {
			add(ref.Value)
		}
	}

	for _, pathItem := range doc.Paths.Map() {
		for _, ref := range pathItem.Parameters {
			if ref != nil {
				add(ref.Value)
			}
		}
		for _, op := range pathItem.Operations() {
			for _, ref := range op.Parameters {
				if ref != nil {
					add(ref.Value)
				}
			}
		}
	}

	headers := make([]string, 0, len(seen))
	for h := range seen {
		headers = append(headers, h)
	}
	sort.Strings(headers)

	if len(headers) == 0 {
		t.Fatal("declaredHeaderParameters found none; the walk is broken, not the document")
	}
	return headers
}

// TestEveryDeclaredHeaderParameterIsAllowedByCORS is the guard: every header
// parameter openapi.yaml declares anywhere (an operation's own parameters, a
// path item's shared parameters, or a components.parameters entry reached by
// $ref, such as Idempotency-Key and If-Match) must be allowed by the same
// corsOptions the server actually runs. Before Idempotency-Key was added to
// AllowedHeaders, this failed on that header: a cross-origin POST carrying
// it never got past the browser's preflight (see Sentry VIBE-FRONTEND-5).
func TestEveryDeclaredHeaderParameterIsAllowedByCORS(t *testing.T) {
	respond := httpx.NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	h, err := openapi.NewHandler(respond)
	if err != nil {
		t.Fatalf("openapi.NewHandler: %v", err)
	}

	handler := newTestCORSHandler()

	for _, header := range declaredHeaderParameters(t, h.Document()) {
		t.Run(header, func(t *testing.T) {
			w := preflight(http.HandlerFunc(handler.HandlerFunc), header)

			if got := w.Header().Get("Access-Control-Allow-Headers"); got == "" {
				t.Errorf("Access-Control-Request-Headers: %q: CORS preflight has no "+
					"Access-Control-Allow-Headers; add it to corsOptions' AllowedHeaders "+
					"in routes.go (status=%d)", header, w.Code)
			}
		})
	}
}

// TestCombinedPreflightAllowsContentTypeIdempotencyKeyAndCSRFToken pins the
// exact acceptance scenario: a preflight asking for content-type,
// idempotency-key and x-csrf-token together, from FrontendURL, gets the
// allow headers back.
func TestCombinedPreflightAllowsContentTypeIdempotencyKeyAndCSRFToken(t *testing.T) {
	handler := newTestCORSHandler()
	w := preflight(http.HandlerFunc(handler.HandlerFunc), "content-type,idempotency-key,x-csrf-token")

	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204; got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("want Access-Control-Allow-Origin; got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("want Access-Control-Allow-Headers to be set for the combined preflight")
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
