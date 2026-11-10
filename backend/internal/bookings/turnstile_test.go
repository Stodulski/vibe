package bookings

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/turnstile"
)

// stubTurnstile is a TurnstileVerifier double, the same shape as auth's. A nil
// err from Verify means an enabled verifier that accepts every token; set err
// to drive a rejection, or enabled to false for the disabled path.
type stubTurnstile struct {
	enabled bool
	err     error
	// calls records every token/remoteIP pair Verify was asked to check.
	calls []struct{ token, remoteIP string }
}

func (s *stubTurnstile) Enabled() bool { return s.enabled }

func (s *stubTurnstile) Verify(_ context.Context, token, remoteIP string) error {
	s.calls = append(s.calls, struct{ token, remoteIP string }{token, remoteIP})
	return s.err
}

// withTurnstile rebuilds the fixture's handler with a Turnstile verifier and
// the booking enforcement flag.
func withTurnstile(f *fixture, verifier *stubTurnstile, enforce bool) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	f.handler = NewHandler(f.service, f.whatsapp, verifier, enforce, httpx.NewResponder(logger), logger, httpx.TrustedProxies{})
}

// withToken adds a turnstile_token to a publicBookBody.
func withToken(body, token string) string {
	return strings.TrimSuffix(body, "}") + `,"turnstile_token":"` + token + `"}`
}

// bookingFieldError returns the message FailedValidation wrote for field.
func bookingFieldError(body map[string]any, field string) string {
	errs, _ := body["errors"].([]any)
	for _, e := range errs {
		entry, ok := e.(map[string]any)
		if ok && entry["field"] == field {
			msg, _ := entry["message"].(string)
			return msg
		}
	}
	return ""
}

func TestPublicBookTurnstile(t *testing.T) {
	// httptest.NewRequest's peer address; trustProxies is off in the fixture.
	const clientIP = "192.0.2.1"

	t.Run("flag off: no token is booked as today and Verify is never called", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)
		verifier := &stubTurnstile{enabled: true, err: turnstile.ErrInvalidToken}
		withTurnstile(f, verifier, false)

		w := httptest.NewRecorder()
		f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

		if w.Code != http.StatusCreated {
			t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
		}
		if len(verifier.calls) != 0 {
			t.Errorf("with enforcement off Verify must not be called; got %d calls", len(verifier.calls))
		}
	})

	refused := []struct {
		name  string
		err   error
		token string
		want  string
	}{
		{"flag on: a missing token is refused as required", nil, "", "required"},
		{"flag on: an invalid token is refused as invalid", turnstile.ErrInvalidToken, "bad-token", "invalid"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			complexID, courtID := preparePublicBooking(f)
			verifier := &stubTurnstile{enabled: true, err: tc.err}
			withTurnstile(f, verifier, true)

			body := publicBookBody(complexID, courtID, 90)
			if tc.token != "" {
				body = withToken(body, tc.token)
			}
			w := httptest.NewRecorder()
			f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", body))

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if got := bookingFieldError(decode(t, w), "turnstile_token"); got != tc.want {
				t.Errorf("turnstile_token = %q, want %q", got, tc.want)
			}
			if len(f.store.inserted) != 0 || len(f.clients.created) != 0 || len(f.locks.acquired) != 0 || f.checkout.created != 0 {
				t.Errorf("a refused token must create nothing: inserted=%d clients=%d locks=%d checkouts=%d",
					len(f.store.inserted), len(f.clients.created), len(f.locks.acquired), f.checkout.created)
			}
			if tc.token == "" && len(verifier.calls) != 0 {
				t.Error("an empty token must never reach Verify")
			}
		})
	}

	booked := []struct {
		name     string
		verifier *stubTurnstile
		calls    int
	}{
		{"flag on: Cloudflare unavailable fails open", &stubTurnstile{enabled: true, err: turnstile.ErrUnavailable}, 1},
		{"flag on: a valid token is booked", &stubTurnstile{enabled: true}, 1},
		{"flag on but verifier disabled: booked without verification", &stubTurnstile{enabled: false}, 0},
	}
	for _, tc := range booked {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			complexID, courtID := preparePublicBooking(f)
			withTurnstile(f, tc.verifier, true)

			w := httptest.NewRecorder()
			f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", withToken(publicBookBody(complexID, courtID, 90), "good-token")))

			if w.Code != http.StatusCreated {
				t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
			}
			if len(tc.verifier.calls) != tc.calls {
				t.Fatalf("Verify calls = %d, want %d", len(tc.verifier.calls), tc.calls)
			}
			if tc.calls == 1 {
				got := tc.verifier.calls[0]
				if got.token != "good-token" || got.remoteIP != clientIP {
					t.Errorf("Verify(%q, %q), want (%q, %q)", got.token, got.remoteIP, "good-token", clientIP)
				}
			}
		})
	}
}
