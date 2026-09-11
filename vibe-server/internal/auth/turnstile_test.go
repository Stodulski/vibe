package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/turnstile"
)

// forgotPasswordAuditCount is a small helper so the ForgotPassword tests
// below can assert that a Turnstile rejection never reaches the unconditional
// audit write, without duplicating the "one entry, and what it says" style
// the rest of this package's audit assertions use.
func forgotPasswordAuditCount(f *fixture) int {
	return len(f.audit.entries)
}

func TestRegisterTurnstile(t *testing.T) {
	const bodyNoToken = registerBody
	const bodyWithToken = `{"email":"ana@example.com","password":"correct-horse-battery","first_name":"Ana","last_name":"Perez","phone":"+541100000000","turnstile_token":"good-token"}`

	t.Run("disabled verifier: unchanged behavior", func(t *testing.T) {
		f := newFixture(t) // Turnstile disabled by default.
		w := httptest.NewRecorder()
		f.handler.Register(w, postJSON(t, bodyNoToken))

		if w.Code != http.StatusCreated {
			t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
		}
		if len(f.turnstile.calls) != 0 {
			t.Error("a disabled verifier must never be asked to Verify anything")
		}
	})

	t.Run("missing token is rejected", func(t *testing.T) {
		f := newFixtureWithTurnstile(t)
		w := httptest.NewRecorder()
		f.handler.Register(w, postJSON(t, bodyNoToken))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
		}
		errs, _ := decode(t, w)["error"].(map[string]any)
		if errs["turnstile_token"] != "required" {
			t.Errorf(`turnstile_token = %v, want "required"`, errs["turnstile_token"])
		}
		if f.users.inserted != nil {
			t.Error("no account may be created when the token is missing")
		}
		if len(f.turnstile.calls) != 0 {
			t.Error("an empty token must never reach Verify")
		}
	})

	t.Run("invalid token is rejected", func(t *testing.T) {
		f := newFixtureWithTurnstile(t)
		f.turnstile.err = turnstile.ErrInvalidToken
		w := httptest.NewRecorder()
		f.handler.Register(w, postJSON(t, bodyWithToken))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
		}
		errs, _ := decode(t, w)["error"].(map[string]any)
		if errs["turnstile_token"] != "invalid" {
			t.Errorf(`turnstile_token = %v, want "invalid"`, errs["turnstile_token"])
		}
		if f.users.inserted != nil {
			t.Error("no account may be created when the token is invalid")
		}
	})

	t.Run("unavailable service is reported as unavailable", func(t *testing.T) {
		f := newFixtureWithTurnstile(t)
		f.turnstile.err = turnstile.ErrUnavailable
		w := httptest.NewRecorder()
		f.handler.Register(w, postJSON(t, bodyWithToken))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
		}
		errs, _ := decode(t, w)["error"].(map[string]any)
		if errs["turnstile_token"] != "unavailable" {
			t.Errorf(`turnstile_token = %v, want "unavailable"`, errs["turnstile_token"])
		}
		if f.users.inserted != nil {
			t.Error("no account may be created when Cloudflare could not be reached")
		}
	})

	t.Run("a valid token passes through", func(t *testing.T) {
		f := newFixtureWithTurnstile(t)
		w := httptest.NewRecorder()
		f.handler.Register(w, postJSON(t, bodyWithToken))

		if w.Code != http.StatusCreated {
			t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
		}
		if f.users.inserted == nil {
			t.Fatal("the account was not created")
		}
		if len(f.turnstile.calls) != 1 || f.turnstile.calls[0].token != "good-token" {
			t.Errorf("Verify calls = %+v, want exactly one call with the request's token", f.turnstile.calls)
		}
		if f.turnstile.calls[0].remoteIP != "192.0.2.1" {
			t.Errorf("remoteIP = %q, want the request's client IP, not empty or raw RemoteAddr", f.turnstile.calls[0].remoteIP)
		}
	})
}

func TestLoginTurnstile(t *testing.T) {
	const loginBody = `{"email":"ana@example.com","password":"correct-horse-battery"}`
	const loginBodyWithToken = `{"email":"ana@example.com","password":"correct-horse-battery","turnstile_token":"good-token"}`

	t.Run("disabled verifier: unchanged behavior", func(t *testing.T) {
		f := newFixture(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
		w := httptest.NewRecorder()
		f.handler.Login(w, postJSON(t, loginBody))

		if w.Code != http.StatusOK {
			t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
		}
		if len(f.turnstile.calls) != 0 {
			t.Error("a disabled verifier must never be asked to Verify anything")
		}
	})

	t.Run("missing token is rejected before any credential check", func(t *testing.T) {
		f := newFixtureWithTurnstile(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
		w := httptest.NewRecorder()
		f.handler.Login(w, postJSON(t, loginBody))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
		}
		errs, _ := decode(t, w)["error"].(map[string]any)
		if errs["turnstile_token"] != "required" {
			t.Errorf(`turnstile_token = %v, want "required"`, errs["turnstile_token"])
		}
		if findCookie(w.Header(), "access_token") != nil {
			t.Error("no session may be issued when the token is missing")
		}
		// The rejection is a 422, not the generic 401 loginFailed writes — a
		// failed sign-in was never attempted, so nothing should be recorded
		// as one.
		if len(f.audit.entries) != 0 {
			t.Errorf("a Turnstile rejection must not record a login attempt; got %d entries", len(f.audit.entries))
		}
	})

	t.Run("invalid token is rejected", func(t *testing.T) {
		f := newFixtureWithTurnstile(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
		f.turnstile.err = turnstile.ErrInvalidToken
		w := httptest.NewRecorder()
		f.handler.Login(w, postJSON(t, loginBodyWithToken))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
		}
		errs, _ := decode(t, w)["error"].(map[string]any)
		if errs["turnstile_token"] != "invalid" {
			t.Errorf(`turnstile_token = %v, want "invalid"`, errs["turnstile_token"])
		}
	})

	t.Run("unavailable service fails open and is logged", func(t *testing.T) {
		f := newFixtureWithTurnstile(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
		f.turnstile.err = turnstile.ErrUnavailable
		w := httptest.NewRecorder()
		f.handler.Login(w, postJSON(t, loginBodyWithToken))

		if w.Code != http.StatusOK {
			t.Fatalf("outage must not lock out a registered user; want 200, got %d", w.Code)
		}
		if !strings.Contains(f.logs.String(), "turnstile: verification unavailable") {
			t.Error("the unavailable verifier must be logged")
		}
	})

	t.Run("a valid token passes through", func(t *testing.T) {
		f := newFixtureWithTurnstile(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
		w := httptest.NewRecorder()
		f.handler.Login(w, postJSON(t, loginBodyWithToken))

		if w.Code != http.StatusOK {
			t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
		}
		if findCookie(w.Header(), "access_token") == nil {
			t.Error("a session must be issued once the token is accepted")
		}
		if len(f.turnstile.calls) != 1 || f.turnstile.calls[0].token != "good-token" {
			t.Errorf("Verify calls = %+v, want exactly one call with the request's token", f.turnstile.calls)
		}
	})
}

func TestForgotPasswordTurnstile(t *testing.T) {
	const forgotBody = `{"email":"ana@example.com"}`
	const forgotBodyWithToken = `{"email":"ana@example.com","turnstile_token":"good-token"}`

	t.Run("disabled verifier: unchanged behavior", func(t *testing.T) {
		f := newFixture(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
		w := httptest.NewRecorder()
		f.handler.ForgotPassword(w, postJSON(t, forgotBody))

		if w.Code != http.StatusOK {
			t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
		}
		if len(f.turnstile.calls) != 0 {
			t.Error("a disabled verifier must never be asked to Verify anything")
		}
	})

	// The generic success answer is what stops this endpoint confirming which
	// addresses have accounts (see ForgotPassword's own comment). A Turnstile
	// rejection must not become a second, different-looking failure mode —
	// it has to be the ordinary 422 validation shape, distinguishable from
	// the always-200 generic answer, but identical whether or not the
	// address exists (checkTurnstile runs before the lookup either way).
	t.Run("missing token is rejected before the unconditional audit write", func(t *testing.T) {
		f := newFixtureWithTurnstile(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
		w := httptest.NewRecorder()
		f.handler.ForgotPassword(w, postJSON(t, forgotBody))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
		}
		errs, _ := decode(t, w)["error"].(map[string]any)
		if errs["turnstile_token"] != "required" {
			t.Errorf(`turnstile_token = %v, want "required"`, errs["turnstile_token"])
		}
		if len(f.notify.resets) != 0 {
			t.Error("no reset email may be sent when the token is missing")
		}
		if got := forgotPasswordAuditCount(f); got != 0 {
			t.Errorf("a Turnstile rejection must not write the password-reset-request audit entry; got %d entries", got)
		}
	})

	t.Run("invalid token is rejected without writing the audit entry", func(t *testing.T) {
		f := newFixtureWithTurnstile(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
		f.turnstile.err = turnstile.ErrInvalidToken
		w := httptest.NewRecorder()
		f.handler.ForgotPassword(w, postJSON(t, forgotBodyWithToken))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
		}
		errs, _ := decode(t, w)["error"].(map[string]any)
		if errs["turnstile_token"] != "invalid" {
			t.Errorf(`turnstile_token = %v, want "invalid"`, errs["turnstile_token"])
		}
		if got := forgotPasswordAuditCount(f); got != 0 {
			t.Errorf("a Turnstile rejection must not write the password-reset-request audit entry; got %d entries", got)
		}
	})

	t.Run("unavailable service fails open and is logged", func(t *testing.T) {
		f := newFixtureWithTurnstile(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
		f.turnstile.err = turnstile.ErrUnavailable
		w := httptest.NewRecorder()
		f.handler.ForgotPassword(w, postJSON(t, forgotBodyWithToken))

		if w.Code != http.StatusOK {
			t.Fatalf("outage must not block the recovery path; want 200, got %d", w.Code)
		}
		if !strings.Contains(f.logs.String(), "turnstile: verification unavailable") {
			t.Error("the unavailable verifier must be logged")
		}
	})

	t.Run("a valid token passes through and the generic answer is unchanged", func(t *testing.T) {
		f := newFixtureWithTurnstile(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
		w := httptest.NewRecorder()
		f.handler.ForgotPassword(w, postJSON(t, forgotBodyWithToken))

		if w.Code != http.StatusOK {
			t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
		}
		if len(f.notify.resets) != 1 {
			t.Errorf("want a reset email once the token is accepted; got %d", len(f.notify.resets))
		}
		if got := forgotPasswordAuditCount(f); got != 1 {
			t.Errorf("want exactly one password-reset-request audit entry; got %d", got)
		}
		if len(f.turnstile.calls) != 1 || f.turnstile.calls[0].token != "good-token" {
			t.Errorf("Verify calls = %+v, want exactly one call with the request's token", f.turnstile.calls)
		}
	})
}
