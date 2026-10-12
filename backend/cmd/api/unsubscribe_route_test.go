package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stodulski/vibe-server/internal/lifecycle"
	"github.com/stodulski/vibe-server/internal/middleware"
	"github.com/stodulski/vibe-server/internal/stores"
)

// recordingSuppressions is the lifecycle store with a record of what was suppressed.
type recordingSuppressions struct {
	mockLifecycleStore
	mu         sync.Mutex
	suppressed []string
}

func (r *recordingSuppressions) Suppress(_ context.Context, email, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.suppressed = append(r.suppressed, email)
	return nil
}

// unsubscribeApp is the route harness with the real request validator attached, so
// the bodies a mail client and the /baja page send are checked against the document
// the same way they are in staging.
func unsubscribeApp(t *testing.T) (*application, *recordingSuppressions) {
	t.Helper()
	rec := &recordingSuppressions{}
	app, _ := newTestApplicationWithStores(t, func(s *stores.Stores) { s.Lifecycle = rec })

	v, err := middleware.NewSpecValidator(app.openapi.Document(), app.respond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSpecValidator: %v", err)
	}
	app.specValidator = v
	return app, rec
}

func signedToken(app *application, email string) string {
	return lifecycle.NewSigner(app.config.JWT.Secret, app.config.JWT.SecretPrevious).Sign(email)
}

// The one-click POST a mail client sends: the token in the query, the form body
// List-Unsubscribe=One-Click (RFC 8058). It must pass the spec and suppress the address.
func TestOneClickUnsubscribeFromAMailClientSuppressesTheAddress(t *testing.T) {
	app, rec := unsubscribeApp(t)
	ts := newTestServer(t, app)

	token := signedToken(app, "ana@example.com")
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		ts.URL+"/api/v1/public/email/unsubscribe?token="+url.QueryEscape(token),
		strings.NewReader("List-Unsubscribe=One-Click"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 204; body %s", resp.StatusCode, body)
	}
	if len(rec.suppressed) != 1 || rec.suppressed[0] != "ana@example.com" {
		t.Fatalf("suppressed %v, want [ana@example.com]", rec.suppressed)
	}
}

// The /baja page posts the token as JSON.
func TestUnsubscribeFromTheBajaPageSuppressesTheAddress(t *testing.T) {
	app, rec := unsubscribeApp(t)
	ts := newTestServer(t, app)

	token := signedToken(app, "ana@example.com")
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		ts.URL+"/api/v1/public/email/unsubscribe", strings.NewReader(`{"token":"`+token+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent || len(rec.suppressed) != 1 {
		t.Fatalf("status = %d, suppressed = %v; want 204 and one address", resp.StatusCode, rec.suppressed)
	}
}

// A forged token is refused with 400 and suppresses nothing.
func TestUnsubscribeWithAForgedTokenIsRefused(t *testing.T) {
	app, rec := unsubscribeApp(t)
	ts := newTestServer(t, app)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		ts.URL+"/api/v1/public/email/unsubscribe?token=forged.token", strings.NewReader("List-Unsubscribe=One-Click"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if len(rec.suppressed) != 0 {
		t.Fatalf("a forged token suppressed %v", rec.suppressed)
	}
}

// The validator is live on this harness: a form body the document does not
// describe is refused before the handler runs, so the 204 above is the document's
// verdict and not an accident of an unchecked route.
func TestUnsubscribeBodyOutsideTheDocumentIsRefusedBeforeTheHandler(t *testing.T) {
	app, rec := unsubscribeApp(t)
	ts := newTestServer(t, app)

	token := signedToken(app, "ana@example.com")
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		ts.URL+"/api/v1/public/email/unsubscribe?token="+url.QueryEscape(token),
		strings.NewReader("List-Unsubscribe=Two-Click"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent || len(rec.suppressed) != 0 {
		t.Fatalf("status = %d, suppressed = %v; a body the document rejects reached the handler", resp.StatusCode, rec.suppressed)
	}
}

// The /baja page runs in the browser on the storefront origin and posts JSON, so the
// browser preflights the route. CORS is one policy for every route; this pins that
// the unsubscribe route is inside it: the storefront origin is answered, POST with a
// JSON content type is allowed, and credentials are not refused.
func TestUnsubscribeRouteAnswersTheBajaPagePreflight(t *testing.T) {
	const storefront = "https://vibe.com.ar"
	app, _ := newTestApplicationWithPublicURL(t, storefront, nil)
	ts := newTestServer(t, app)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodOptions,
		ts.URL+"/api/v1/public/email/unsubscribe", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", storefront)
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != storefront {
		t.Errorf("Access-Control-Allow-Origin = %q, want the storefront origin echoed", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want true", got)
	}
	if !strings.Contains(resp.Header.Get("Access-Control-Allow-Methods"), "POST") {
		t.Errorf("Access-Control-Allow-Methods = %q, want POST", resp.Header.Get("Access-Control-Allow-Methods"))
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Access-Control-Allow-Headers")), "content-type") {
		t.Errorf("Access-Control-Allow-Headers = %q, want Content-Type", resp.Header.Get("Access-Control-Allow-Headers"))
	}
}

// The actual cross-origin POST from the storefront carries the origin back, so the
// browser lets the page read the 204 it was answered with.
func TestUnsubscribeFromTheStorefrontOriginIsAnsweredWithTheOriginEchoed(t *testing.T) {
	const storefront = "https://vibe.com.ar"
	rec := &recordingSuppressions{}
	app, _ := newTestApplicationWithPublicURL(t, storefront, func(s *stores.Stores) { s.Lifecycle = rec })
	ts := newTestServer(t, app)

	token := signedToken(app, "ana@example.com")
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		ts.URL+"/api/v1/public/email/unsubscribe", strings.NewReader(`{"token":"`+token+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", storefront)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != storefront {
		t.Errorf("Access-Control-Allow-Origin = %q, want the storefront origin so the page can read the answer", got)
	}
	if len(rec.suppressed) != 1 {
		t.Fatalf("suppressed %v, want one address", rec.suppressed)
	}
}

// An origin that is neither the frontend nor the storefront gets no grant.
func TestUnsubscribePreflightFromAnUnrelatedOriginIsNotGranted(t *testing.T) {
	app, _ := newTestApplicationWithPublicURL(t, "https://vibe.com.ar", nil)
	ts := newTestServer(t, app)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodOptions,
		ts.URL+"/api/v1/public/email/unsubscribe", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("an unrelated origin was granted %q", got)
	}
}
