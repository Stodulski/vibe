package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/googleid"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// getRequest builds a plain GET, the shape GoogleStart's caller — a browser
// navigating here directly — actually sends.
func getRequest(t *testing.T) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
}

// finishBody is GoogleFinish's request body: the code and state Google's
// redirect carried back to the callback page.
func finishBody(code, state string) string {
	return `{"code":` + jsonQuote(code) + `,"state":` + jsonQuote(state) + `}`
}

func jsonQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// postFinish builds a POST /auth/google/finish request, with cookie as its
// state cookie unless empty.
func postFinish(t *testing.T, code, state, cookie string) *http.Request {
	t.Helper()
	r := postJSON(t, finishBody(code, state))
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: googleOAuthStateCookie, Value: cookie})
	}
	return r
}

// startAndBindNonce drives GoogleStart through the handler, so it also
// exercises the cookie the handler itself sets, and returns the state (from
// the cookie) alongside the nonce (parsed out of the redirect URL) already
// copied onto f.google.claims — the fixture the happy-path GoogleFinish tests
// build on.
func startAndBindNonce(t *testing.T, f *fixture) (state string) {
	t.Helper()

	w := httptest.NewRecorder()
	f.handler.GoogleStart(w, getRequest(t))
	if w.Code != http.StatusFound {
		t.Fatalf("GoogleStart: want 302; got %d (%s)", w.Code, w.Body.String())
	}

	cookie := findCookie(w.Header(), googleOAuthStateCookie)
	if cookie == nil {
		t.Fatal("GoogleStart set no state cookie")
	}
	location := w.Header().Get("Location")
	if !strings.HasPrefix(location, googleAuthorizationEndpoint+"?") {
		t.Fatalf("Location = %q, want a %s?... URL", location, googleAuthorizationEndpoint)
	}
	_, rawQuery, _ := strings.Cut(location, "?")
	params, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatalf("Location query does not parse: %v", err)
	}
	if params.Get("state") != cookie.Value {
		t.Fatalf("state in the redirect (%q) does not match the state cookie (%q)", params.Get("state"), cookie.Value)
	}
	f.google.claims.Nonce = params.Get("nonce")
	return cookie.Value
}

// ---------------------------------------------------------------------------
// GoogleStart
// ---------------------------------------------------------------------------

func TestGoogleStartHappyPath(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)

	w := httptest.NewRecorder()
	f.handler.GoogleStart(w, getRequest(t))

	if w.Code != http.StatusFound {
		t.Fatalf("want 302; got %d (%s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	cookie := findCookie(w.Header(), googleOAuthStateCookie)
	if cookie == nil {
		t.Fatal("no state cookie set")
	}
	if !cookie.HttpOnly {
		t.Error("state cookie is not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("state cookie SameSite = %v, want Lax", cookie.SameSite)
	}
	// The fixture's Environment is "test", not "development" — the same
	// environment value SetTokenCookies treats as needing Secure, and this
	// cookie follows the same rule.
	if !cookie.Secure {
		t.Error("state cookie is not Secure outside the development environment")
	}
	if cookie.Path != googleOAuthStateCookiePath {
		t.Errorf("state cookie Path = %q, want %q", cookie.Path, googleOAuthStateCookiePath)
	}
	if cookie.Value == "" {
		t.Error("state cookie carries no value")
	}

	location := w.Header().Get("Location")
	if !strings.HasPrefix(location, googleAuthorizationEndpoint+"?") {
		t.Fatalf("Location = %q, want a %s?... URL", location, googleAuthorizationEndpoint)
	}
}

func TestGoogleStartDisabledConfig(t *testing.T) {
	f := newFixture(t) // Google/code exchanger both disabled by default.

	w := httptest.NewRecorder()
	f.handler.GoogleStart(w, getRequest(t))

	assertRedirectedTo(t, w, testLoginPath+"google_unavailable")
}

// TestGoogleStartMissingClientSecretOnly: the client id and FrontendURL are
// configured (the fixture default), but the secret is not — the third
// condition GoogleOAuthEnabled requires.
func TestGoogleStartMissingClientSecretOnly(t *testing.T) {
	f := newFixtureWithGoogle(t) // GIS flow enabled; code exchanger left disabled.

	w := httptest.NewRecorder()
	f.handler.GoogleStart(w, getRequest(t))

	assertRedirectedTo(t, w, testLoginPath+"google_unavailable")
}

func TestGoogleStartWithoutAFrontendURL(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	f.handler.cfg.FrontendURL = ""

	w := httptest.NewRecorder()
	f.handler.GoogleStart(w, getRequest(t))

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("want 501; got %d (%s)", w.Code, w.Body.String())
	}
	if location := w.Header().Get("Location"); location != "" {
		t.Errorf("answered a Location %q with no frontend configured", location)
	}
	if findCookie(w.Header(), googleOAuthStateCookie) != nil {
		t.Error("a state cookie was set with no frontend configured")
	}
}

// ---------------------------------------------------------------------------
// GoogleFinish
// ---------------------------------------------------------------------------

func TestGoogleFinishHandlerHappyPath(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	f.users.add(activeUser(t, "ana@example.com"))
	state := startAndBindNonce(t, f)

	w := httptest.NewRecorder()
	f.handler.GoogleFinish(w, postFinish(t, "the-auth-code", state, state))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if findCookie(w.Header(), "access_token") == nil || findCookie(w.Header(), "refresh_token") == nil {
		t.Error("the session cookies were not set")
	}
	body := decode(t, w)
	if _, ok := body["csrf_token"]; !ok {
		t.Error("csrf_token missing from the response body")
	}

	cleared := findCookie(w.Header(), googleOAuthStateCookie)
	if cleared == nil {
		t.Fatal("the state cookie was not cleared")
	}
	if cleared.MaxAge >= 0 {
		t.Errorf("state cookie MaxAge = %d, want negative (expired)", cleared.MaxAge)
	}
}

func TestGoogleFinishHandlerNeedsProfile(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	state := startAndBindNonce(t, f)

	w := httptest.NewRecorder()
	f.handler.GoogleFinish(w, postFinish(t, "the-auth-code", state, state))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["needs_profile"] != true {
		t.Errorf("needs_profile = %v, want true", body["needs_profile"])
	}
	assertNoSession(t, w)
}

func TestGoogleFinishHandlerMissingCookie(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	state := startAndBindNonce(t, f)

	w := httptest.NewRecorder()
	f.handler.GoogleFinish(w, postFinish(t, "the-auth-code", state, ""))

	assertInvalidCode(t, w)
	if len(f.codeExchanger.calls) != 0 {
		t.Error("the token exchange ran with no state cookie presented")
	}
}

func TestGoogleFinishHandlerMismatchedCookie(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	state := startAndBindNonce(t, f)

	w := httptest.NewRecorder()
	f.handler.GoogleFinish(w, postFinish(t, "the-auth-code", state, "some-other-browser-entirely"))

	assertInvalidCode(t, w)
	if len(f.codeExchanger.calls) != 0 {
		t.Error("the token exchange ran with a state cookie that did not match the body")
	}
}

func TestGoogleFinishHandlerUnknownState(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)

	w := httptest.NewRecorder()
	f.handler.GoogleFinish(w, postFinish(t, "the-auth-code", "never-started", "never-started"))

	assertInvalidCode(t, w)
}

func TestGoogleFinishHandlerReplayedState(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	f.users.add(activeUser(t, "ana@example.com"))
	state := startAndBindNonce(t, f)

	first := httptest.NewRecorder()
	f.handler.GoogleFinish(first, postFinish(t, "the-auth-code", state, state))
	if first.Code != http.StatusOK {
		t.Fatalf("first GoogleFinish: want 200; got %d (%s)", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	f.handler.GoogleFinish(second, postFinish(t, "the-auth-code", state, state))
	assertInvalidCode(t, second)
}

func TestGoogleFinishHandlerCodeRejectedByGoogle(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	f.codeExchanger.err = googleid.ErrCodeRejected
	state := startAndBindNonce(t, f)

	w := httptest.NewRecorder()
	f.handler.GoogleFinish(w, postFinish(t, "spent-code", state, state))

	assertInvalidCode(t, w)
}

func TestGoogleFinishHandlerIDTokenRejected(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	f.google.err = googleid.ErrInvalidToken
	state := startAndBindNonce(t, f)

	w := httptest.NewRecorder()
	f.handler.GoogleFinish(w, postFinish(t, "the-auth-code", state, state))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if message, ok := fieldError(decode(t, w), "credential"); !ok || message != "invalid" {
		t.Errorf("credential error = %q (present %v), want %q", message, ok, "invalid")
	}
}

func TestGoogleFinishHandlerTokenExchangeUnavailable(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	f.codeExchanger.err = googleid.ErrUnavailable
	state := startAndBindNonce(t, f)

	w := httptest.NewRecorder()
	f.handler.GoogleFinish(w, postFinish(t, "the-auth-code", state, state))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGoogleFinishHandlerMissingCode(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	state := startAndBindNonce(t, f)

	w := httptest.NewRecorder()
	f.handler.GoogleFinish(w, postFinish(t, "", state, state))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if message, ok := fieldError(decode(t, w), "code"); !ok || message != "must be provided" {
		t.Errorf("code error = %q (present %v), want %q", message, ok, "must be provided")
	}
}

func TestGoogleFinishHandlerMissingState(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)

	w := httptest.NewRecorder()
	f.handler.GoogleFinish(w, postFinish(t, "the-auth-code", "", ""))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if message, ok := fieldError(decode(t, w), "state"); !ok || message != "must be provided" {
		t.Errorf("state error = %q (present %v), want %q", message, ok, "must be provided")
	}
}

func TestGoogleFinishHandlerDisabledConfig(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.GoogleFinish(w, postFinish(t, "the-auth-code", "whatever", "whatever"))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestGoogleFinishRefusesNonJSONContentType pins the fix for a CSRF-exempt
// route reachable by a forged cross-site body: POST /api/v1/auth/google/finish
// mints a session cookie and carries no CSRF token (its own state cookie is
// the token's source), so a plain <form enctype="text/plain"> submission
// could otherwise drive it with an attacker-chosen "body" that still decodes
// as the expected JSON shape. Refusing any Content-Type but
// application/json, before the body is read, closes that without a token
// check on the route that mints the cookie the token would be derived from —
// and it must happen before any Set-Cookie is written, or the token exchange
// runs.
func TestGoogleFinishRefusesNonJSONContentType(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	state := startAndBindNonce(t, f)

	r := postFinish(t, "the-auth-code", state, state)
	r.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	f.handler.GoogleFinish(w, r)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if got := body["type"]; got != httpx.KindUnsupportedMediaType.URI() {
		t.Errorf("want type %q; got %v", httpx.KindUnsupportedMediaType.URI(), got)
	}
	if findCookie(w.Header(), "access_token") != nil || findCookie(w.Header(), "refresh_token") != nil {
		t.Error("a refused request must not mint a session cookie")
	}
	if len(f.codeExchanger.calls) != 0 {
		t.Error("the token exchange must not run before the Content-Type check passes")
	}
}
