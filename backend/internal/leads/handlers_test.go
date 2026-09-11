package leads

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/httpx"
)

func newTestHandler(t *testing.T, webhookURL string) (*Handler, *bytes.Buffer) {
	t.Helper()
	logs := &bytes.Buffer{}
	respond := httpx.NewResponder(slog.New(slog.NewTextHandler(logs, nil)))
	h := NewHandler(respond, Config{WebhookURL: webhookURL, Token: "test-token"})
	return h, logs
}

func doCapture(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/public/leads/abandoned-registration", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.CaptureAbandonedRegistration(rr, req)
	return rr
}

func TestCaptureAbandonedRegistration_ValidEmail_ForwardsToWebhook(t *testing.T) {
	var received sheetPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h, _ := newTestHandler(t, srv.URL)
	rr := doCapture(t, h, `{"email":"juan@example.com"}`)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
	}
	if received.Email != "juan@example.com" {
		t.Errorf("forwarded email = %q, want %q", received.Email, "juan@example.com")
	}
	if received.Token != "test-token" {
		t.Errorf("forwarded token = %q, want %q", received.Token, "test-token")
	}
	if received.Origen != captureOrigin {
		t.Errorf("forwarded origen = %q, want %q", received.Origen, captureOrigin)
	}
}

func TestCaptureAbandonedRegistration_GoogleSource_ForwardsWithGoogleOrigin(t *testing.T) {
	var received sheetPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h, _ := newTestHandler(t, srv.URL)
	rr := doCapture(t, h, `{"email":"juan@example.com","source":"google"}`)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
	}
	if received.Email != "juan@example.com" {
		t.Errorf("forwarded email = %q, want %q", received.Email, "juan@example.com")
	}
	if received.Origen != captureOriginGoogle {
		t.Errorf("forwarded origen = %q, want %q", received.Origen, captureOriginGoogle)
	}
}

func TestCaptureAbandonedRegistration_UnknownSource_Rejected(t *testing.T) {
	h, _ := newTestHandler(t, "")
	rr := doCapture(t, h, `{"email":"juan@example.com","source":"landing"}`)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rr.Body.String(), `"source"`) {
		t.Errorf("body should name the source field; got %s", rr.Body.String())
	}
}

func TestCaptureAbandonedRegistration_InvalidEmail_Rejected(t *testing.T) {
	h, _ := newTestHandler(t, "")
	rr := doCapture(t, h, `{"email":"not-an-email"}`)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestCaptureAbandonedRegistration_MissingWebhookURL_StillAccepts(t *testing.T) {
	h, _ := newTestHandler(t, "")
	rr := doCapture(t, h, `{"email":"juan@example.com"}`)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
	}
}

func TestCaptureAbandonedRegistration_WebhookDown_StillAcceptsAndLogs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	srv.Close() // dead upstream: connection refused

	h, logs := newTestHandler(t, srv.URL)
	rr := doCapture(t, h, `{"email":"juan@example.com"}`)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (a webhook failure must not fail the caller)", rr.Code, http.StatusAccepted)
	}
	if logs.Len() == 0 {
		t.Error("expected the webhook failure to be logged")
	}
}

// navigator.sendBeacon can only send CORS-safelisted content types across
// origins (it can't do a preflight), so the frontend sends this body as
// text/plain rather than application/json. The decoder must not care.
func TestCaptureAbandonedRegistration_TextPlainContentType_StillAccepted(t *testing.T) {
	h, _ := newTestHandler(t, "")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/v1/public/leads/abandoned-registration",
		strings.NewReader(`{"email":"juan@example.com"}`))
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	rr := httptest.NewRecorder()
	h.CaptureAbandonedRegistration(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
	}
}

func TestCaptureAbandonedRegistration_MalformedBody_BadRequest(t *testing.T) {
	h, _ := newTestHandler(t, "")
	rr := doCapture(t, h, `not json`)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestCaptureAbandonedRegistration_EmailLengthBoundary pins H-21 at the edge
// rather than at the 10,000-character payload that reported it. A bound is
// only meaningfully tested one character either side of itself: a test that
// only rejects 10,000 characters still passes if the limit is silently moved
// to 9,000, and this endpoint's whole exposure is that it forwards caller
// input to a third party.
//
// The domain is fixed at 12 characters ("@example.com") so the local part
// carries the arithmetic and the totals below are exact.
func TestCaptureAbandonedRegistration_EmailLengthBoundary(t *testing.T) {
	const domain = "@example.com"

	emailOfLength := func(n int) string {
		return strings.Repeat("a", n-len(domain)) + domain
	}

	tests := []struct {
		name    string
		length  int
		want    int
		forward bool
	}{
		{name: "at the limit", length: 254, want: http.StatusAccepted, forward: true},
		{name: "one over the limit", length: 255, want: http.StatusUnprocessableEntity, forward: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forwarded := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				forwarded = true
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			email := emailOfLength(tt.length)
			if len(email) != tt.length {
				t.Fatalf("test built an email of %d characters, want %d", len(email), tt.length)
			}

			h, _ := newTestHandler(t, srv.URL)
			body, err := json.Marshal(map[string]string{"email": email})
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}
			rr := doCapture(t, h, string(body))

			if rr.Code != tt.want {
				t.Fatalf("status = %d, want %d", rr.Code, tt.want)
			}
			// The status alone would not catch the version of this bug that
			// matters: rejecting the caller while still forwarding the payload
			// leaves the spreadsheet exactly as exposed as before.
			if forwarded != tt.forward {
				t.Errorf("webhook called = %v, want %v", forwarded, tt.forward)
			}
		})
	}
}

// TestCaptureAbandonedRegistration_FormulaShapedEmail_IsForwardedEscaped is
// H-24, asserted at the only place it can be: the value that actually leaves
// for the webhook.
//
// The three payloads are deliberately different in kind. The first is hostile.
// The second and third are ordinary addresses real people hold, and they are
// here to pin the decision that this is escaped rather than refused — a test
// that only covered "=1+1@" would stay green if someone "fixed" H-24 by adding
// a validator rule, which would reject these two and break legitimate signups.
// All three must be accepted AND arrive as text.
func TestCaptureAbandonedRegistration_FormulaShapedEmail_IsForwardedEscaped(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "a formula", in: "=1+1@example.com", want: "'=1+1@example.com"},
		{name: "a legitimate plus address", in: "+qa@example.com", want: "'+qa@example.com"},
		{name: "a legitimate minus address", in: "-qa@example.com", want: "'-qa@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var received sheetPayload
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&received)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			h, _ := newTestHandler(t, srv.URL)
			body, err := json.Marshal(map[string]string{"email": tt.in})
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}
			rr := doCapture(t, h, string(body))

			// Accepted: the address is legal and refusing it is the wrong fix.
			if rr.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want %d — a formula-shaped address is still a valid address",
					rr.Code, http.StatusAccepted)
			}
			if received.Email != tt.want {
				t.Errorf("forwarded email = %q, want %q — the sheet cell would open as a formula",
					received.Email, tt.want)
			}
		})
	}
}
