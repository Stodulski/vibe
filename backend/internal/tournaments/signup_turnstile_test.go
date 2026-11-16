package tournaments

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/turnstile"
)

// stubTurnstile is a turnstile.Verifier double: enabled, and rejecting with err.
type stubTurnstile struct {
	err   error
	calls []string
}

func (s *stubTurnstile) Enabled() bool { return true }

func (s *stubTurnstile) Verify(_ context.Context, token, _ string) error {
	s.calls = append(s.calls, token)
	return s.err
}

const pairBody = `"first_name":"Ana","last_name":"Diaz","phone":"+5491100000001",` +
	`"partner":{"first_name":"Beto","last_name":"Socio","phone":"+5491100000002"}`

func (f *fixture) gatedHandler(verifier *stubTurnstile) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	respond := httpx.NewResponder(logger)
	return NewHandler(f.svc, respond, httpx.TrustedProxies{}, turnstile.NewGate(verifier, true, respond, logger, httpx.TrustedProxies{}))
}

func (f *fixture) pairSignUp(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("slug", "vibe")
	r.SetPathValue("tournamentID", f.store.detail.Tournament.ID.String())
	r.SetPathValue("categoryID", f.category().ID.String())
	w := httptest.NewRecorder()
	h.PublicSignUp(w, r)
	return w
}

// The pair sign-up takes a spot, so it asks the same bot check as the booking
// hold: a missing or rejected token is a 422 on turnstile_token and holds nothing.
func TestPairSignUpAsksTheTurnstileGateLikeTheBookingHold(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		err   error
		want  string
		calls int
	}{
		{"missing token", `{` + pairBody + `}`, nil, "required", 0},
		{"rejected token", `{` + pairBody + `,"turnstile_token":"bad"}`, turnstile.ErrInvalidToken, "invalid", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			verifier := &stubTurnstile{err: tc.err}
			w := f.pairSignUp(t, f.gatedHandler(verifier), tc.body)
			if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "turnstile_token") ||
				!strings.Contains(w.Body.String(), `"`+tc.want+`"`) {
				t.Fatalf("want 422 turnstile_token %s; got %d (%s)", tc.want, w.Code, w.Body.String())
			}
			if len(f.events.signUps) != 0 {
				t.Error("a refused token must hold no spot")
			}
			if len(verifier.calls) != tc.calls {
				t.Errorf("Verify calls = %d, want %d", len(verifier.calls), tc.calls)
			}
		})
	}

	t.Run("valid token signs up", func(t *testing.T) {
		f := newFixture(t)
		verifier := &stubTurnstile{}
		w := f.pairSignUp(t, f.gatedHandler(verifier), `{`+pairBody+`,"turnstile_token":"ok"}`)
		if w.Code != http.StatusCreated || len(f.events.signUps) != 1 || len(verifier.calls) != 1 || verifier.calls[0] != "ok" {
			t.Fatalf("want 201 with the token verified; got %d (%s) calls=%v", w.Code, w.Body.String(), verifier.calls)
		}
	})
}
