package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// verifiedUser builds an account that can sign in.
func verifiedUser(t *testing.T, email, password string) *authstore.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return &authstore.User{
		ID: uuid.New(), Email: email, FirstName: "Ana", LastName: "Perez",
		PasswordHash: hash, EmailVerified: true, IsActive: true, Role: "owner",
	}
}

const registerBody = `{"email":"ana@example.com","password":"correct-horse-battery","first_name":"Ana","last_name":"Perez","phone":"+541100000000"}`

// Registration must answer identically whether or not the address is taken.
// Any difference — status, body, or timing shape — turns the endpoint into an
// oracle for which email addresses have accounts.
func TestRegisterDoesNotRevealWhetherTheEmailExists(t *testing.T) {
	fresh := newFixture(t)
	w1 := httptest.NewRecorder()
	fresh.handler.Register(w1, postJSON(t, registerBody))

	taken := newFixture(t)
	taken.users.add(verifiedUser(t, "ana@example.com", "something-else"))
	w2 := httptest.NewRecorder()
	taken.handler.Register(w2, postJSON(t, registerBody))

	if w1.Code != w2.Code {
		t.Errorf("the status differs between a new and an existing address: %d vs %d", w1.Code, w2.Code)
	}
	if w1.Body.String() != w2.Body.String() {
		t.Errorf("the body differs:\n new:      %s\n existing: %s", w1.Body.String(), w2.Body.String())
	}

	// The existing account is told, by email, that someone tried again.
	if len(taken.notify.duplicates) != 1 {
		t.Errorf("want a duplicate-registration email; got %d", len(taken.notify.duplicates))
	}
	if taken.users.inserted != nil {
		t.Error("a second account must not be created for a taken address")
	}
	if len(fresh.notify.verifications) != 1 {
		t.Errorf("a new registration must send a verification email; got %d", len(fresh.notify.verifications))
	}
}

func TestRegisterRejectsWeakInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"malformed email", `{"email":"not-an-email","password":"correct-horse-battery","first_name":"A","last_name":"B","phone":"+5411"}`},
		{"short password", `{"email":"ana@example.com","password":"short","first_name":"A","last_name":"B","phone":"+5411"}`},
		{"no name", `{"email":"ana@example.com","password":"correct-horse-battery","first_name":"","last_name":"","phone":"+5411"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			w := httptest.NewRecorder()
			f.handler.Register(w, postJSON(t, tt.body))

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if f.users.inserted != nil {
				t.Error("no account may be created from invalid input")
			}
		})
	}
}

// The stored password must never be the password.
func TestRegisterStoresOnlyAHash(t *testing.T) {
	f := newFixture(t)
	w := httptest.NewRecorder()
	f.handler.Register(w, postJSON(t, registerBody))

	if f.users.inserted == nil {
		t.Fatalf("the account was not created (%d: %s)", w.Code, w.Body.String())
	}
	if strings.Contains(string(f.users.inserted.PasswordHash), "correct-horse-battery") {
		t.Fatal("the password was stored in recoverable form")
	}
	if err := bcrypt.CompareHashAndPassword(f.users.inserted.PasswordHash, []byte("correct-horse-battery")); err != nil {
		t.Errorf("the stored hash does not verify the password: %v", err)
	}
}

func TestLoginIssuesASession(t *testing.T) {
	f := newFixture(t)
	f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))

	w := httptest.NewRecorder()
	f.handler.Login(w, postJSON(t, `{"email":"ana@example.com","password":"correct-horse-battery"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if findCookie(w.Header(), "access_token") == nil {
		t.Error("no access token cookie was set")
	}
	if findCookie(w.Header(), "refresh_token") == nil {
		t.Error("no refresh token cookie was set")
	}
	if len(f.tokens.stored) != 1 {
		t.Errorf("want the refresh token persisted; got %d", len(f.tokens.stored))
	}
	if f.users.failedIncrement != 0 {
		t.Error("a successful sign-in must not count a failed attempt")
	}
}

// The session cookies must not be readable by JavaScript, and must not travel
// over plain HTTP outside development.
func TestSessionCookiesAreHardened(t *testing.T) {
	f := newFixture(t)
	f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))

	w := httptest.NewRecorder()
	f.handler.Login(w, postJSON(t, `{"email":"ana@example.com","password":"correct-horse-battery"}`))

	for _, name := range []string{"access_token", "refresh_token"} {
		c := findCookie(w.Header(), name)
		if c == nil {
			t.Fatalf("%s cookie is missing", name)
		}
		if !c.HttpOnly {
			t.Errorf("%s must be HttpOnly, or a script can read the session", name)
		}
		if c.SameSite == http.SameSiteNoneMode {
			t.Errorf("%s must not be SameSite=None", name)
		}
	}

	// The CSRF token is deliberately readable — the browser has to send it back
	// in a header, which is what proves the request was not cross-site.
	if csrf := findCookie(w.Header(), "csrf_token"); csrf != nil && csrf.HttpOnly {
		t.Error("the CSRF cookie must be readable by the frontend to be echoed in a header")
	}
}

// Wrong password, unknown address and inactive account must be indistinguishable
// to the caller.
func TestFailedLoginsAreIndistinguishable(t *testing.T) {
	wrongPassword := newFixture(t)
	wrongPassword.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
	w1 := httptest.NewRecorder()
	wrongPassword.handler.Login(w1, postJSON(t, `{"email":"ana@example.com","password":"wrong-password-here"}`))

	unknown := newFixture(t)
	w2 := httptest.NewRecorder()
	unknown.handler.Login(w2, postJSON(t, `{"email":"nobody@example.com","password":"wrong-password-here"}`))

	if w1.Code != w2.Code {
		t.Errorf("a wrong password and an unknown address answer differently: %d vs %d", w1.Code, w2.Code)
	}
	if w1.Code != http.StatusUnauthorized {
		t.Errorf("want 401; got %d", w1.Code)
	}
	if w1.Body.String() != w2.Body.String() {
		t.Errorf("the bodies differ:\n wrong password: %s\n unknown:        %s", w1.Body.String(), w2.Body.String())
	}
	if findCookie(w1.Header(), "access_token") != nil {
		t.Error("a failed sign-in must not set a session cookie")
	}
	if wrongPassword.users.failedIncrement == 0 {
		t.Error("a failed attempt must be counted, or the lockout never triggers")
	}
}

func TestLoginRefusesAnUnverifiedAccount(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	user.EmailVerified = false
	f.users.add(user)

	w := httptest.NewRecorder()
	f.handler.Login(w, postJSON(t, `{"email":"ana@example.com","password":"correct-horse-battery"}`))

	if w.Code == http.StatusOK {
		t.Errorf("an unverified account must not sign in; got %d", w.Code)
	}
	if len(f.tokens.stored) != 0 {
		t.Error("no session may be issued to an unverified account")
	}
}

func TestLoginRefusesADeactivatedAccount(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	user.IsActive = false
	f.users.add(user)

	w := httptest.NewRecorder()
	f.handler.Login(w, postJSON(t, `{"email":"ana@example.com","password":"correct-horse-battery"}`))

	if w.Code == http.StatusOK {
		t.Errorf("a deactivated account must not sign in; got %d", w.Code)
	}
	if len(f.tokens.stored) != 0 {
		t.Error("no session may be issued to a deactivated account")
	}
}

// signIn returns the refresh token cookie from a successful login.
func signIn(t *testing.T, f *fixture) *http.Cookie {
	t.Helper()
	f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))

	w := httptest.NewRecorder()
	f.handler.Login(w, postJSON(t, `{"email":"ana@example.com","password":"correct-horse-battery"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("sign-in failed: %d (%s)", w.Code, w.Body.String())
	}

	c := findCookie(w.Header(), "refresh_token")
	if c == nil {
		t.Fatal("no refresh token cookie")
	}
	return c
}

func TestRefreshRotatesTheToken(t *testing.T) {
	f := newFixture(t)
	old := signIn(t, f)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	r.AddCookie(old)

	w := httptest.NewRecorder()
	f.handler.Refresh(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	fresh := findCookie(w.Header(), "refresh_token")
	if fresh == nil {
		t.Fatal("refresh did not issue a new token")
	}
	if fresh.Value == old.Value {
		t.Error("the refresh token must rotate, or a stolen one stays valid forever")
	}
	if len(f.tokens.markedUse) == 0 {
		t.Error("the spent token must be marked used, or reuse cannot be detected")
	}
}

// Presenting an already-rotated refresh token means either the client or an
// attacker holds a copy. Which one cannot be known, so every session for the
// account is revoked — once the token has been spent for longer than the
// concurrent-refresh grace window (see refreshReuseGrace).
func TestReusingARotatedTokenRevokesEverySession(t *testing.T) {
	f := newFixture(t)
	old := signIn(t, f)

	first := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	first.AddCookie(old)
	f.handler.Refresh(httptest.NewRecorder(), first)
	f.tokens.ageUsedTokens(2 * refreshReuseGrace)

	// Present the same, long-spent token again.
	replay := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	replay.AddCookie(old)

	w := httptest.NewRecorder()
	f.handler.Refresh(w, replay)

	if w.Code == http.StatusOK {
		t.Errorf("a reused refresh token must be refused; got %d", w.Code)
	}
	if len(f.tokens.allWiped) == 0 {
		t.Fatal("reuse must revoke every session for the account")
	}
	// Refresh tokens alone are not enough: an access token already issued is a
	// self-contained JWT and stays valid until it expires on its own.
	if len(f.blacklist.invalidated) == 0 {
		t.Error("reuse must also revoke the access tokens already issued")
	}
}

// Two tabs sharing a cookie jar both refresh when the access token expires;
// the second request leaves before the first response has replaced the cookie
// and so carries the token just spent. That is not theft, and treating it as
// theft signed people out of every device. Within the grace window the late
// request is refused and nothing else happens.
func TestASecondRefreshMomentsLaterIsRefusedWithoutRevokingSessions(t *testing.T) {
	f := newFixture(t)
	old := signIn(t, f)

	first := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	first.AddCookie(old)
	f.handler.Refresh(httptest.NewRecorder(), first)

	late := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	late.AddCookie(old)
	w := httptest.NewRecorder()
	f.handler.Refresh(w, late)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("the late refresh must still be refused; got %d", w.Code)
	}
	if len(f.tokens.allWiped) != 0 {
		t.Error("a refresh moments after rotation is a sibling tab, not a thief; sessions must survive")
	}
	if len(f.blacklist.invalidated) != 0 {
		t.Error("access tokens must survive a concurrent refresh")
	}
	if strings.Contains(f.logs.String(), "reuse detected") {
		t.Error("a concurrent refresh must not be logged as a security incident")
	}
}

func TestRefreshWithoutACookieIsRefused(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.Refresh(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))

	if w.Code == http.StatusOK {
		t.Errorf("want a rejection; got %d", w.Code)
	}
}

func TestRefreshWithAnUnknownTokenIsRefused(t *testing.T) {
	f := newFixture(t)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	r.AddCookie(&http.Cookie{Name: "refresh_token", Value: "not-a-real-token"})

	w := httptest.NewRecorder()
	f.handler.Refresh(w, r)

	if w.Code == http.StatusOK {
		t.Errorf("want a rejection; got %d", w.Code)
	}
	if len(f.tokens.allWiped) != 0 {
		t.Error("an unrecognised token is not evidence of reuse and must not revoke sessions")
	}
}

func TestLogoutClearsAndRevokes(t *testing.T) {
	f := newFixture(t)
	refresh := signIn(t, f)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	r.AddCookie(refresh)

	w := httptest.NewRecorder()
	f.handler.Logout(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if c := findCookie(w.Header(), "refresh_token"); c == nil || c.MaxAge >= 0 && c.Value != "" {
		t.Error("logout must clear the refresh cookie")
	}
	if len(f.tokens.deleted) == 0 {
		t.Error("the refresh token must be deleted server-side, not just from the browser")
	}
}

// Signing out has to work even when the session is already gone, or a user
// with an expired session can never clear it.
func TestLogoutWithoutASessionStillSucceeds(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.Logout(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))

	if w.Code != http.StatusOK {
		t.Errorf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestForgotPasswordDoesNotRevealWhetherTheAccountExists(t *testing.T) {
	known := newFixture(t)
	known.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
	w1 := httptest.NewRecorder()
	known.handler.ForgotPassword(w1, postJSON(t, `{"email":"ana@example.com"}`))

	unknown := newFixture(t)
	w2 := httptest.NewRecorder()
	unknown.handler.ForgotPassword(w2, postJSON(t, `{"email":"nobody@example.com"}`))

	if w1.Code != w2.Code || w1.Body.String() != w2.Body.String() {
		t.Errorf("the response reveals whether the account exists:\n known:   %d %s\n unknown: %d %s",
			w1.Code, w1.Body.String(), w2.Code, w2.Body.String())
	}
	if len(known.notify.resets) != 1 {
		t.Errorf("a known address must be sent a reset link; got %d", len(known.notify.resets))
	}
	if len(unknown.notify.resets) != 0 {
		t.Errorf("an unknown address must not be emailed; got %d", len(unknown.notify.resets))
	}
}

func TestResetPasswordRevokesEverySession(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	f.users.add(user)

	w := httptest.NewRecorder()
	f.handler.ForgotPassword(w, postJSON(t, `{"email":"ana@example.com"}`))
	if len(f.notify.resets) != 1 {
		t.Fatalf("no reset email was sent (%d: %s)", w.Code, w.Body.String())
	}

	token := tokenFromURL(t, f.notify.resets[0].ResetURL)

	w = httptest.NewRecorder()
	f.handler.ResetPassword(w, postJSON(t, `{"token":"`+token+`","password":"a-brand-new-password"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.users.passwordUpdated == nil {
		t.Fatal("the password was not changed")
	}
	// Changing a password is how someone recovers from a compromise, so it has
	// to end every session that was open, not just future ones.
	if len(f.tokens.allWiped) == 0 {
		t.Error("a password reset must revoke every refresh token")
	}
	if len(f.blacklist.invalidated) == 0 {
		t.Error("a password reset must revoke the outstanding access tokens")
	}
}

// The reset token is single-use: a second attempt with the same link fails.
func TestResetPasswordTokenIsSingleUse(t *testing.T) {
	f := newFixture(t)
	f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))

	f.handler.ForgotPassword(httptest.NewRecorder(), postJSON(t, `{"email":"ana@example.com"}`))
	token := tokenFromURL(t, f.notify.resets[0].ResetURL)
	body := `{"token":"` + token + `","password":"a-brand-new-password"}`

	first := httptest.NewRecorder()
	f.handler.ResetPassword(first, postJSON(t, body))
	if first.Code != http.StatusOK {
		t.Fatalf("the first reset should succeed; got %d (%s)", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	f.handler.ResetPassword(second, postJSON(t, body))
	// Pinned to 400, not merely "not 200": a 500 would also have satisfied the
	// old assertion, and a 500 here is the symptom of the defect the test
	// below covers.
	if second.Code != http.StatusBadRequest {
		t.Errorf("the same reset link must be refused as invalid; want 400, got %d (%s)", second.Code, second.Body.String())
	}
}

// A store failure is not a bad token. ResetPassword answered every GetByHash
// error with the same 400 "invalid or expired reset token", so a database
// outage told the one person holding a valid link that their link was bad —
// sending them to request another, which fails identically. Neither the user
// nor the logs can tell the two apart: the 400 is indistinguishable from the
// ordinary traffic of people clicking an already-used link, so an outage in
// the recovery flow looks like nothing at all.
func TestResetPasswordSeparatesABadTokenFromABrokenStore(t *testing.T) {
	t.Run("an unknown token is refused as invalid", func(t *testing.T) {
		f := newFixture(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))

		w := httptest.NewRecorder()
		f.handler.ResetPassword(w, postJSON(t, `{"token":"never-issued","password":"a-brand-new-password"}`))

		if w.Code != http.StatusBadRequest {
			t.Errorf("want 400 for a token that was never issued; got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("a store failure is reported as a server error", func(t *testing.T) {
		f := newFixture(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
		f.resets.getErr = errors.New("connection refused")

		w := httptest.NewRecorder()
		f.handler.ResetPassword(w, postJSON(t, `{"token":"a-perfectly-good-token","password":"a-brand-new-password"}`))

		if w.Code != http.StatusInternalServerError {
			t.Errorf("a broken store must not be reported as a bad link; want 500, got %d (%s)", w.Code, w.Body.String())
		}
		if f.users.passwordUpdated != nil {
			t.Error("no password may be changed when the token could not be read")
		}
	})
}

func TestDeleteAccountIsRefusedWhileAComplexHasLiveBookings(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	f.users.add(user)
	f.complexes.owned = []*complexstore.Complex{{ID: uuid.New()}}
	f.bookings.hasActive = true

	r := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", nil)
	r = withUser(r, user)

	w := httptest.NewRecorder()
	f.handler.DeleteAccount(w, r)

	if w.Code == http.StatusOK {
		t.Errorf("an account with live bookings must not be deletable; got %d", w.Code)
	}
	if f.users.deleted != nil {
		t.Error("the account must not be deleted")
	}
}

// The sessions used to be revoked before the row was deleted. When the delete
// then failed, the client showed the error and immediately lost its session,
// which reads as a successful deletion — with the account fully intact.
func TestAFailedAccountDeleteLeavesTheSessionAlone(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	f.users.add(user)
	f.users.deleteErr = errors.New("foreign key still points at a complex")

	r := withUser(httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", nil), user)
	w := httptest.NewRecorder()
	f.handler.DeleteAccount(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500 for a failed delete; got %d", w.Code)
	}
	if len(f.tokens.allWiped) != 0 {
		t.Error("a failed delete must not revoke the refresh tokens")
	}
	if len(f.blacklist.invalidated) != 0 {
		t.Error("a failed delete must not revoke the access tokens")
	}
	if findCookie(w.Header(), "access_token") != nil {
		t.Error("a failed delete must not clear the auth cookies")
	}
}

func TestCurrentUserReturnsTheAuthenticatedAccount(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	f.users.add(user)

	r := withUser(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), user)

	w := httptest.NewRecorder()
	f.handler.CurrentUser(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	returned, _ := decode(t, w)["user"].(map[string]any)
	if returned["email"] != "ana@example.com" {
		t.Errorf("the wrong account was returned; got %v", returned["email"])
	}
	// The hash must never leave the server, under any field name.
	if strings.Contains(w.Body.String(), "password") {
		t.Errorf("the response mentions a password field: %s", w.Body.String())
	}
}

// The frontend bootstraps each page load from GET /auth/me instead of spending
// the refresh token, so the answer must carry the CSRF token bound to the
// access token the request authenticated with, read the way the middleware
// reads it: cookie first, then the Bearer header.
func TestCurrentUserCarriesTheCSRFTokenOfTheSession(t *testing.T) {
	f := newFixture(t)
	user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
	f.users.add(user)

	accessToken, err := f.service.tokenService.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		t.Fatalf("minting access token: %v", err)
	}
	want := f.service.tokenService.GenerateCSRFToken(accessToken)

	t.Run("from the access_token cookie", func(t *testing.T) {
		r := withUser(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), user)
		r.AddCookie(&http.Cookie{Name: "access_token", Value: accessToken})

		w := httptest.NewRecorder()
		f.handler.CurrentUser(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
		}
		if got := decode(t, w)["csrf_token"]; got != want {
			t.Errorf("csrf_token must be derived from the cookie's access token; got %v", got)
		}
	})

	t.Run("from the Bearer header when there is no cookie", func(t *testing.T) {
		r := withUser(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), user)
		r.Header.Set("Authorization", "Bearer "+accessToken)

		w := httptest.NewRecorder()
		f.handler.CurrentUser(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
		}
		if got := decode(t, w)["csrf_token"]; got != want {
			t.Errorf("csrf_token must be derived from the Bearer token; got %v", got)
		}
	})

	t.Run("omitted when the request carries no credential", func(t *testing.T) {
		r := withUser(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), user)

		w := httptest.NewRecorder()
		f.handler.CurrentUser(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
		}
		if _, present := decode(t, w)["csrf_token"]; present {
			t.Error("csrf_token must be omitted when there is no access token to derive it from")
		}
	})
}

// observable renders everything an unauthenticated caller can see of a response.
// Asserting the status alone would miss the oracle simply moving channel — into
// a Retry-After header, a WWW-Authenticate challenge, or the error message.
func observable(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()

	names := make([]string, 0, len(w.Header()))
	for name := range w.Header() {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	fmt.Fprintf(&b, "status: %d\n", w.Code)
	for _, name := range names {
		fmt.Fprintf(&b, "header: %s: %s\n", name, strings.Join(w.Header()[name], ", "))
	}
	fmt.Fprintf(&b, "body: %s", w.Body.String())
	return b.String()
}

// lockedUser builds an account that is currently locked out.
func lockedUser(t *testing.T, email, password string) *authstore.User {
	t.Helper()
	u := verifiedUser(t, email, password)
	until := time.Now().Add(15 * time.Minute)
	u.FailedLoginAttempts = 5
	u.LockedUntil = &until
	return u
}

// A locked account used to answer 429 while every other failure answered 401,
// which made the endpoint an oracle: five wrong guesses at any address, and the
// sixth reply said whether that address has an account. The dummy bcrypt in the
// unknown-address branch exists to keep exactly this fact out of the response
// timing; the status line was handing it over anyway.
func TestLoginDoesNotRevealThatAnAccountIsLocked(t *testing.T) {
	// The baseline an attacker compares everything against: an address with no
	// account at all.
	unknown := newFixture(t)
	baseline := httptest.NewRecorder()
	unknown.handler.Login(baseline, postJSON(t, `{"email":"nobody@example.com","password":"wrong-password-here"}`))

	if baseline.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for an unknown address; got %d (%s)", baseline.Code, baseline.Body.String())
	}

	tests := []struct {
		name string
		user func(t *testing.T) *authstore.User
		body string
	}{
		{
			// The sharpest case: the account exists and is locked, so today's
			// 429 names it outright.
			name: "a locked account, guessed at again",
			user: func(t *testing.T) *authstore.User { return lockedUser(t, "ana@example.com", "correct-horse-battery") },
			body: `{"email":"ana@example.com","password":"wrong-password-here"}`,
		},
		{
			// Even the real password must not distinguish a locked account:
			// otherwise the lockout confirms the address to anyone who reaches it.
			name: "a locked account, with the correct password",
			user: func(t *testing.T) *authstore.User { return lockedUser(t, "ana@example.com", "correct-horse-battery") },
			body: `{"email":"ana@example.com","password":"correct-horse-battery"}`,
		},
		{
			name: "an unlocked account, wrong password",
			user: func(t *testing.T) *authstore.User { return verifiedUser(t, "ana@example.com", "correct-horse-battery") },
			body: `{"email":"ana@example.com","password":"wrong-password-here"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.users.add(tt.user(t))

			w := httptest.NewRecorder()
			f.handler.Login(w, postJSON(t, tt.body))

			if got, want := observable(t, w), observable(t, baseline); got != want {
				t.Errorf("this case is distinguishable from an address with no account:\n got:\n%s\n want:\n%s", got, want)
			}
			if findCookie(w.Header(), "access_token") != nil {
				t.Error("no session may be issued")
			}
			if len(f.tokens.stored) != 0 {
				t.Error("no refresh token may be persisted")
			}
		})
	}
}

// Being locked out is what sends someone to the reset link, so the reset has to
// end the lockout. UpdatePassword writes only the hash, leaving locked_until
// untouched — without clearing it the brand-new password is refused for up to
// four hours, and refused with the same generic "invalid credentials" a locked
// account now gets, so nothing explains why the password just set does not work.
func TestResetPasswordClearsTheLoginLockout(t *testing.T) {
	f := newFixture(t)
	f.users.add(lockedUser(t, "ana@example.com", "correct-horse-battery"))

	f.handler.ForgotPassword(httptest.NewRecorder(), postJSON(t, `{"email":"ana@example.com"}`))
	if len(f.notify.resets) != 1 {
		t.Fatalf("no reset email was sent; got %d", len(f.notify.resets))
	}
	token := tokenFromURL(t, f.notify.resets[0].ResetURL)

	w := httptest.NewRecorder()
	f.handler.ResetPassword(w, postJSON(t, `{"token":"`+token+`","password":"a-brand-new-password"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.users.failedReset == 0 {
		t.Error("the lockout must be cleared, or the new password is refused and nothing says why")
	}
}

// MarkRefreshTokenUsed used to commit before the replacement token was stored,
// so any failure after it left the client holding a token the database already
// considered spent. The client's ordinary retry then looked exactly like a
// replayed stolen token, and the response to that is the nuclear one: every
// session revoked, every issued access token blacklisted, and the user named as
// an attacker in the log. One database hiccup, and a user is signed out of every
// device with a SECURITY line filed against them.
func TestRefreshDoesNotTreatAStoreFailureAsTokenTheft(t *testing.T) {
	f := newFixture(t)
	old := signIn(t, f)

	// The replacement token cannot be written — a transient database failure.
	f.tokens.insertErr = errors.New("connection reset by peer")

	first := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	first.AddCookie(old)
	w1 := httptest.NewRecorder()
	f.handler.Refresh(w1, first)

	if w1.Code != http.StatusInternalServerError {
		t.Fatalf("a store failure must be reported as a server error; got %d (%s)", w1.Code, w1.Body.String())
	}
	if findCookie(w1.Header(), "refresh_token") != nil {
		t.Error("no replacement cookie may be handed out when the replacement was not stored")
	}

	// The database recovers and the client retries with the only token it has.
	f.tokens.insertErr = nil

	retry := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	retry.AddCookie(old)
	w2 := httptest.NewRecorder()
	f.handler.Refresh(w2, retry)

	if w2.Code != http.StatusOK {
		t.Errorf("the retry must succeed — the client's token was never replaced; got %d (%s)", w2.Code, w2.Body.String())
	}
	if len(f.tokens.allWiped) != 0 {
		t.Error("a database failure is not token theft: no session may be revoked")
	}
	if len(f.blacklist.invalidated) != 0 {
		t.Error("a database failure must not blacklist the account's access tokens")
	}
	if strings.Contains(f.logs.String(), "reuse detected") {
		t.Errorf("a database failure was logged as a security incident:\n%s", f.logs.String())
	}
}

// The phone number is what WhatsApp delivers booking notifications to.
// Registration has always demanded E.164; this path took anything non-empty, so
// the number could be replaced with one that can never receive a message and
// nothing would say so — the failure being a message that never arrives.
func TestUpdateCurrentUserRequiresADeliverablePhoneNumber(t *testing.T) {
	t.Run("a number that cannot be delivered to is refused", func(t *testing.T) {
		f := newFixture(t)
		user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
		user.Phone = "+541112345678"
		f.users.add(user)

		r := withUser(postJSON(t, `{"phone":"not-a-phone"}`), user)
		w := httptest.NewRecorder()
		f.handler.UpdateCurrentUser(w, r)

		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("want 422; got %d (%s)", w.Code, w.Body.String())
		}
		if f.users.updated != nil {
			t.Errorf("nothing may be persisted from invalid input; stored phone %q", f.users.updated.Phone)
		}
	})

	t.Run("a formatted number is stored in E.164", func(t *testing.T) {
		f := newFixture(t)
		user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
		user.Phone = "+541112345678"
		f.users.add(user)

		r := withUser(postJSON(t, `{"phone":"+54 (11) 9876-5432"}`), user)
		w := httptest.NewRecorder()
		f.handler.UpdateCurrentUser(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
		}
		if f.users.updated == nil {
			t.Fatal("the account was never persisted")
		}
		if got := f.users.updated.Phone; got != "+541198765432" {
			t.Errorf("the number was stored as %q, not normalised to %q", got, "+541198765432")
		}
	})

	// The reason this was left alone: rejecting outright would 422 someone who
	// came to change their surname, over a number they never touched.
	t.Run("an unchanged legacy number does not block an unrelated edit", func(t *testing.T) {
		f := newFixture(t)
		user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
		user.Phone = "011 4321-8765" // stored before this path validated anything
		f.users.add(user)

		r := withUser(postJSON(t, `{"last_name":"Gomez","phone":"011 4321-8765"}`), user)
		w := httptest.NewRecorder()
		f.handler.UpdateCurrentUser(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("re-saving an untouched legacy number must not block the edit; got %d (%s)", w.Code, w.Body.String())
		}
		if f.users.updated == nil {
			t.Fatal("the account was never persisted")
		}
		if f.users.updated.LastName != "Gomez" {
			t.Errorf("the edit the user actually came to make was lost; last name is %q", f.users.updated.LastName)
		}
		if f.users.updated.Phone != "011 4321-8765" {
			t.Errorf("the untouched number must be left as found; got %q", f.users.updated.Phone)
		}
	})

	// Standing still is free; a deliberate edit is not.
	t.Run("changing a legacy number to another bad one is still refused", func(t *testing.T) {
		f := newFixture(t)
		user := verifiedUser(t, "ana@example.com", "correct-horse-battery")
		user.Phone = "011 4321-8765"
		f.users.add(user)

		r := withUser(postJSON(t, `{"phone":"011 0000-1111"}`), user)
		w := httptest.NewRecorder()
		f.handler.UpdateCurrentUser(w, r)

		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("want 422 for a deliberate edit to another undeliverable number; got %d (%s)", w.Code, w.Body.String())
		}
	})
}

// Every InsertWithCooldown error returned the generic success and wrote nothing
// anywhere. During a database problem every password reset silently failed while
// each user was told their mail was on its way — an outage in the one flow
// people reach for when they are already locked out, looking like a quiet
// afternoon. The generic answer has to stay, because it is what stops this
// endpoint confirming which addresses have accounts; the silence inside does not.
func TestForgotPasswordKeepsTheGenericAnswerButRecordsAStoreFailure(t *testing.T) {
	unknown := newFixture(t)
	baseline := httptest.NewRecorder()
	unknown.handler.ForgotPassword(baseline, postJSON(t, `{"email":"nobody@example.com"}`))

	t.Run("a store failure is invisible outside and logged inside", func(t *testing.T) {
		f := newFixture(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
		f.resets.err = errors.New("connection refused")

		w := httptest.NewRecorder()
		f.handler.ForgotPassword(w, postJSON(t, `{"email":"ana@example.com"}`))

		if got, want := observable(t, w), observable(t, baseline); got != want {
			t.Errorf("a store failure changed what the caller sees, which is an enumeration signal:\n got:\n%s\n want:\n%s", got, want)
		}
		if len(f.notify.resets) != 0 {
			t.Error("no email can have been sent when the token was never stored")
		}
		if !strings.Contains(f.logs.String(), "forgot-password") {
			t.Errorf("the failure left no trace anywhere; log was:\n%s", f.logs.String())
		}
		if !strings.Contains(f.logs.String(), "connection refused") {
			t.Errorf("the log must carry the underlying error; log was:\n%s", f.logs.String())
		}
	})

	// The other half, without which a fix that logs unconditionally still passes:
	// an active cooldown is somebody clicking twice, not an outage.
	t.Run("an active cooldown is ordinary traffic and stays quiet", func(t *testing.T) {
		f := newFixture(t)
		f.users.add(verifiedUser(t, "ana@example.com", "correct-horse-battery"))
		f.resets.err = data.ErrCooldownActive

		w := httptest.NewRecorder()
		f.handler.ForgotPassword(w, postJSON(t, `{"email":"ana@example.com"}`))

		if got, want := observable(t, w), observable(t, baseline); got != want {
			t.Errorf("a cooldown changed what the caller sees:\n got:\n%s\n want:\n%s", got, want)
		}
		if strings.Contains(f.logs.String(), "forgot-password") {
			t.Errorf("an active cooldown must not be logged as a failure; log was:\n%s", f.logs.String())
		}
	})
}
