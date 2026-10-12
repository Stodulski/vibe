package lifecycle

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/httpx"
)

const (
	testSecret   = "0123456789abcdef0123456789abcdef-current"
	testPrevious = "fedcba9876543210fedcba9876543210-previous"
)

func TestUnsubscribeTokenRoundTripNormalizesTheAddress(t *testing.T) {
	signer := NewSigner(testSecret, "")

	token := signer.Sign("  Ana.Perez@Example.com ")
	got, err := signer.Verify(token)
	if err != nil {
		t.Fatalf("Verify(own token): %v", err)
	}
	if got != "ana.perez@example.com" {
		t.Fatalf("Verify = %q, want the lowercased, trimmed address", got)
	}
}

func TestUnsubscribeTokenIsURLSafe(t *testing.T) {
	token := NewSigner(testSecret, "").Sign("ana@example.com")
	if strings.ContainsAny(token, "+/= ?&#") {
		t.Fatalf("token %q is not URL-safe", token)
	}
}

func TestUnsubscribeTokenRejectsForgeries(t *testing.T) {
	signer := NewSigner(testSecret, "")
	token := signer.Sign("ana@example.com")
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		t.Fatalf("token %q has %d parts, want payload.signature", token, len(parts))
	}

	otherPayload := base64.RawURLEncoding.EncodeToString([]byte("eve@example.com"))
	cases := map[string]string{
		"empty":              "",
		"no separator":       "abc",
		"three parts":        parts[0] + "." + parts[1] + ".x",
		"bad base64 payload": "!!!." + parts[1],
		"bad base64 mac":     parts[0] + ".!!!",
		"swapped payload":    otherPayload + "." + parts[1],
		"truncated mac":      parts[0] + "." + parts[1][:len(parts[1])-2],
		"other secret":       NewSigner(testPrevious, "").Sign("ana@example.com"),
	}
	for name, bad := range cases {
		if _, err := signer.Verify(bad); !errors.Is(err, ErrInvalidUnsubscribeToken) {
			t.Errorf("%s: Verify(%q) = %v, want ErrInvalidUnsubscribeToken", name, bad, err)
		}
	}
}

func TestUnsubscribeTokenRejectsAMalformedAddress(t *testing.T) {
	signer := NewSigner(testSecret, "")
	// A correctly signed payload that is not an address must still be refused:
	// the signature proves who minted it, not that it is a usable address.
	payload := base64.RawURLEncoding.EncodeToString([]byte("not-an-address"))
	mac := NewSigner(testSecret, "").macFor("not-an-address")
	if _, err := signer.Verify(payload + "." + mac); !errors.Is(err, ErrInvalidUnsubscribeToken) {
		t.Fatalf("Verify(signed non-address) = %v, want ErrInvalidUnsubscribeToken", err)
	}
}

func TestUnsubscribeTokenVerifiesUnderThePreviousSecretDuringRotation(t *testing.T) {
	old := NewSigner(testPrevious, "").Sign("ana@example.com")

	if _, err := NewSigner(testSecret, "").Verify(old); err == nil {
		t.Fatal("a token minted under a retired secret verified with no previous key configured")
	}
	rotating := NewSigner(testSecret, testPrevious)
	if got, err := rotating.Verify(old); err != nil || got != "ana@example.com" {
		t.Fatalf("Verify under rotation = %q, %v; want the address", got, err)
	}
	if rotating.Sign("ana@example.com") == old {
		t.Fatal("signing must use the active key, not the previous one")
	}
}

func TestUnsubscribeTokenDoesNotBareKeyTheJWTSecret(t *testing.T) {
	// The address MAC is keyed by a derivation of the secret, so a token can never
	// be replayed as a CSRF or access token signed with the raw secret.
	signer := NewSigner(testSecret, "")
	if signer.macFor("ana@example.com") == rawHMACForTest(testSecret, "ana@example.com") {
		t.Fatal("unsubscribe MAC equals the raw HMAC of the JWT secret; keys are not domain-separated")
	}
}

type fakeSuppressions struct {
	calls []string
	err   error
}

func (f *fakeSuppressions) Suppress(_ context.Context, email, _ string) error {
	f.calls = append(f.calls, email)
	return f.err
}

func newUnsubscribeHandler(t *testing.T, store SuppressionStore) *UnsubscribeHandler {
	t.Helper()
	return NewUnsubscribeHandler(httpx.NewResponder(discardLogger()), NewSigner(testSecret, ""), store)
}

func TestUnsubscribeOneClickPostWithTokenInTheQuery(t *testing.T) {
	store := &fakeSuppressions{}
	h := newUnsubscribeHandler(t, store)
	token := NewSigner(testSecret, "").Sign("ana@example.com")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/v1/public/email/unsubscribe?token="+token, strings.NewReader("List-Unsubscribe=One-Click"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	tok := token
	h.Unsubscribe(rr, req, &tok)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body %q", rr.Code, rr.Body.String())
	}
	if len(store.calls) != 1 || store.calls[0] != "ana@example.com" {
		t.Fatalf("suppressed %v, want [ana@example.com]", store.calls)
	}
}

func TestUnsubscribeJSONBodyCarriesTheToken(t *testing.T) {
	store := &fakeSuppressions{}
	h := newUnsubscribeHandler(t, store)
	token := NewSigner(testSecret, "").Sign("ana@example.com")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/v1/public/email/unsubscribe", strings.NewReader(`{"token":"`+token+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Unsubscribe(rr, req, nil)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body %q", rr.Code, rr.Body.String())
	}
	if len(store.calls) != 1 {
		t.Fatalf("suppressed %d addresses, want 1", len(store.calls))
	}
}

func TestUnsubscribeRefusesABadTokenWithBadRequest(t *testing.T) {
	cases := map[string]struct {
		query string
		body  string
		ctype string
	}{
		"forged query":       {query: "?token=forged.token", ctype: "application/x-www-form-urlencoded", body: "List-Unsubscribe=One-Click"},
		"no token at all":    {body: "", ctype: "application/json"},
		"form without token": {body: "List-Unsubscribe=One-Click", ctype: "application/x-www-form-urlencoded"},
		"malformed json":     {body: `{"token":`, ctype: "application/json"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			store := &fakeSuppressions{}
			h := newUnsubscribeHandler(t, store)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
				"/api/v1/public/email/unsubscribe"+tc.query, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.ctype)
			rr := httptest.NewRecorder()
			var q *string
			if tc.query != "" {
				v := strings.TrimPrefix(tc.query, "?token=")
				q = &v
			}
			h.Unsubscribe(rr, req, q)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body %q", rr.Code, rr.Body.String())
			}
			if len(store.calls) != 0 {
				t.Fatalf("a refused token suppressed %v", store.calls)
			}
		})
	}
}

func TestUnsubscribeIsIdempotent(t *testing.T) {
	store := &fakeSuppressions{}
	h := newUnsubscribeHandler(t, store)
	token := NewSigner(testSecret, "").Sign("ana@example.com")
	for range 2 {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
			"/api/v1/public/email/unsubscribe", strings.NewReader(`{"token":"`+token+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.Unsubscribe(rr, req, nil)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204 on every repeat", rr.Code)
		}
	}
}

func TestUnsubscribeStoreFailureIsAServerError(t *testing.T) {
	store := &fakeSuppressions{err: errors.New("db down")}
	h := newUnsubscribeHandler(t, store)
	token := NewSigner(testSecret, "").Sign("ana@example.com")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/v1/public/email/unsubscribe", strings.NewReader(`{"token":"`+token+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Unsubscribe(rr, req, nil)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 so the sender can retry", rr.Code)
	}
}

// rawHMACForTest is the MAC the signer would produce if it keyed on the JWT
// secret directly. The signer must not match it.
func rawHMACForTest(secret, email string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("unsubscribe:" + email))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
