package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/googleid"
)

// The property under test in this whole file, stated once: a request an
// attacker can cause a browser to make must never end in a session.
//
// Google's redirect mode posts the ID token cross-site, as a top-level form
// navigation, so the endpoint receiving it is reachable by anybody who can get
// a browser to submit a form. Two things stop that being a sign-in oracle: the
// g_csrf_token double submit, and the fact that this endpoint establishes
// nothing at all — it hands back a one-time code, and the session is minted a
// request later, by the frontend, from its own origin.
//
// So every test below asserts the same two things about the redirect endpoint
// whatever it is fed: the answer is a redirect, and no cookie came with it.

const (
	testFrontendURL = "https://vibe.test"
	testReturnPath  = testFrontendURL + "/auth/google/return?code="
	testLoginPath   = testFrontendURL + "/login?error="
)

// postGoogleForm builds the form post Google's redirect mode makes: the
// x-www-form-urlencoded body, and the g_csrf_token cookie the frontend's proxy
// forwards. An empty cookie value sends no cookie at all.
func postGoogleForm(t *testing.T, form url.Values, csrfCookie string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if csrfCookie != "" {
		r.AddCookie(&http.Cookie{Name: "g_csrf_token", Value: csrfCookie})
	}
	return r
}

// googleForm is the body of a well-formed redirect-mode post.
func googleForm(credential, csrfToken string) url.Values {
	return url.Values{
		"credential":   {credential},
		"g_csrf_token": {csrfToken},
		"select_by":    {"btn"},
	}
}

// assertRedirectedTo fails unless w is a 303 to location that wrote no cookie
// at all.
func assertRedirectedTo(t *testing.T, w *httptest.ResponseRecorder, location string) {
	t.Helper()
	if w.Code != http.StatusSeeOther {
		t.Fatalf("want 303; got %d (%s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Location"); got != location {
		t.Errorf("Location = %q, want %q", got, location)
	}
	assertNoCookiesWritten(t, w)
}

// assertNoCookiesWritten fails if the redirect endpoint wrote any Set-Cookie
// header at all — not merely no session.
//
// Two invariants in one assertion. It must not establish a session, because
// the request that reaches it is a top-level cross-site form navigation. And
// it must not touch g_csrf_token either: that cookie is what binds the code it
// just issued to this browser, and clearing, rotating or overwriting it on any
// path — success or failure — would leave the return page unable to produce
// the value the exchange needs.
func assertNoCookiesWritten(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if written := w.Header().Values("Set-Cookie"); len(written) != 0 {
		t.Errorf("the redirect endpoint wrote %v; it must set no cookie at all, "+
			"and must never disturb the g_csrf_token the code is bound to", written)
	}
}

// assertNoSession fails if the response carries either session cookie.
func assertNoSession(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	for _, name := range []string{"access_token", "refresh_token"} {
		if findCookie(w.Header(), name) != nil {
			t.Errorf("a %s cookie was set where no session should have been established", name)
		}
	}
}

// codeFromLocation pulls the one-time code out of a successful redirect.
func codeFromLocation(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	location := w.Header().Get("Location")
	if !strings.HasPrefix(location, testReturnPath) {
		t.Fatalf("Location = %q, want a redirect to %s<code>", location, testReturnPath)
	}
	code := strings.TrimPrefix(location, testReturnPath)
	if code == "" {
		t.Fatal("the redirect carried no code")
	}
	unescaped, err := url.QueryUnescape(code)
	if err != nil {
		t.Fatalf("the code in %q is not a usable query value: %v", location, err)
	}
	return unescaped
}

// exchangeBody is the frontend's exchange request: the one-time code, plus the
// g_csrf_token it read back off the cookie Google set on the app's origin.
func exchangeBody(code, csrfToken string) string {
	body, err := json.Marshal(map[string]string{"code": code, "g_csrf_token": csrfToken})
	if err != nil {
		panic(err)
	}
	return string(body)
}

// activeUser is an account that can sign in, for the exchange tests.
func activeUser(t *testing.T, email string) *authstore.User {
	t.Helper()
	user := &authstore.User{
		ID: uuid.New(), Email: email, FirstName: "Ana", LastName: "Perez",
		Phone: "+5491112345678", Role: "owner", IsActive: true, EmailVerified: true,
	}
	if err := user.SetPassword("irrelevant-here", bcrypt.MinCost); err != nil {
		t.Fatal(err)
	}
	return user
}

// ---------------------------------------------------------------------------
// The CSRF double submit
// ---------------------------------------------------------------------------

// TestGoogleRedirectCSRFMismatch is the attack this endpoint's own defence is
// for: a form somebody else's page submitted can carry any field it likes, but
// it cannot read the cookie Google set on our origin. The two disagree, so
// nothing happens — and, because the answer is a redirect rather than a
// problem page, the person who was navigated here lands somewhere they can
// act on.
func TestGoogleRedirectCSRFMismatch(t *testing.T) {
	f := newFixtureWithGoogle(t)

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, postGoogleForm(t, googleForm("good-id-token", "attacker-chose-this"), "google-set-this"))

	assertRedirectedTo(t, w, testLoginPath+"google_rejected")
	if len(f.google.calls) != 0 {
		t.Error("the credential was verified despite the CSRF tokens disagreeing")
	}
}

func TestGoogleRedirectMissingCSRFCookie(t *testing.T) {
	f := newFixtureWithGoogle(t)

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), ""))

	assertRedirectedTo(t, w, testLoginPath+"google_rejected")
	if len(f.google.calls) != 0 {
		t.Error("the credential was verified with no CSRF cookie present")
	}
}

func TestGoogleRedirectMissingCSRFField(t *testing.T) {
	f := newFixtureWithGoogle(t)

	form := url.Values{"credential": {"good-id-token"}}

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, postGoogleForm(t, form, "csrf-abc"))

	assertRedirectedTo(t, w, testLoginPath+"google_rejected")
	if len(f.google.calls) != 0 {
		t.Error("the credential was verified with no CSRF field in the form")
	}
}

// ---------------------------------------------------------------------------
// What the endpoint will not read
// ---------------------------------------------------------------------------

func TestGoogleRedirectWrongContentType(t *testing.T) {
	f := newFixtureWithGoogle(t)

	r := postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc")
	r.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, r)

	assertRedirectedTo(t, w, testLoginPath+"google_rejected")
	if len(f.google.calls) != 0 {
		t.Error("a body that is not Google's form encoding was parsed anyway")
	}
}

// TestGoogleRedirectAcceptsACharsetParameter guards the other direction of the
// check above: the media type is what matters, and a charset parameter after
// it is ordinary Content-Type syntax, not a different content type.
func TestGoogleRedirectAcceptsACharsetParameter(t *testing.T) {
	f := newFixtureWithGoogle(t)

	r := postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc")
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, r)

	if w.Code != http.StatusSeeOther || !strings.HasPrefix(w.Header().Get("Location"), testReturnPath) {
		t.Fatalf("want the sign-in to proceed; got %d to %q", w.Code, w.Header().Get("Location"))
	}
}

func TestGoogleRedirectOversizedBody(t *testing.T) {
	f := newFixtureWithGoogle(t)

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, postGoogleForm(t,
		googleForm(strings.Repeat("A", googleRedirectMaxBody+1), "csrf-abc"), "csrf-abc"))

	assertRedirectedTo(t, w, testLoginPath+"google_rejected")
	if len(f.google.calls) != 0 {
		t.Error("an oversized body reached the verifier")
	}
}

func TestGoogleRedirectMissingCredential(t *testing.T) {
	f := newFixtureWithGoogle(t)

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, postGoogleForm(t, url.Values{"g_csrf_token": {"csrf-abc"}}, "csrf-abc"))

	assertRedirectedTo(t, w, testLoginPath+"google_rejected")
}

// ---------------------------------------------------------------------------
// The two failures that are not the caller's fault
// ---------------------------------------------------------------------------

func TestGoogleRedirectRejectedCredential(t *testing.T) {
	f := newFixtureWithGoogle(t)
	f.google.err = googleid.ErrInvalidToken

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, postGoogleForm(t, googleForm("forged", "csrf-abc"), "csrf-abc"))

	assertRedirectedTo(t, w, testLoginPath+"google_rejected")
}

func TestGoogleRedirectVerifierUnavailable(t *testing.T) {
	f := newFixtureWithGoogle(t)
	f.google.err = googleid.ErrUnavailable

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))

	assertRedirectedTo(t, w, testLoginPath+"google_unavailable")
}

// TestGoogleRedirectNotConfigured: a deployment with no GOOGLE_OAUTH_CLIENT_ID
// still answers this endpoint with a redirect rather than the 503 the JSON
// routes give, because its caller is a browser mid-navigation.
func TestGoogleRedirectNotConfigured(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))

	assertRedirectedTo(t, w, testLoginPath+"google_unavailable")
}

// TestGoogleRedirectWithoutAFrontendURL is the one answer this endpoint gives
// that is not a redirect: with FRONTEND_URL empty there is nowhere to send the
// browser, and a redirect to "/login?error=..." would be a redirect to the API
// itself.
func TestGoogleRedirectWithoutAFrontendURL(t *testing.T) {
	f := newFixtureWithGoogle(t)
	f.handler.cfg.FrontendURL = ""

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("want 501; got %d (%s)", w.Code, w.Body.String())
	}
	if location := w.Header().Get("Location"); location != "" {
		t.Errorf("answered a Location %q with no frontend configured", location)
	}
}

// ---------------------------------------------------------------------------
// The happy path, and the round trip it starts
// ---------------------------------------------------------------------------

func TestGoogleRedirectHappyPath(t *testing.T) {
	f := newFixtureWithGoogle(t)

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))

	if w.Code != http.StatusSeeOther {
		t.Fatalf("want 303; got %d (%s)", w.Code, w.Body.String())
	}
	assertNoCookiesWritten(t, w)
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store: the location carries a one-time code", got)
	}

	code := codeFromLocation(t, w)
	if strings.Contains(code, "@") || strings.Contains(code, f.google.claims.Email) {
		t.Errorf("the code %q carries something readable; it must be opaque", code)
	}
	if len(f.google.calls) != 1 || f.google.calls[0] != "good-id-token" {
		t.Errorf("verifier saw %v, want exactly [good-id-token]", f.google.calls)
	}
}

// TestGoogleRedirectThenExchangeEstablishesTheSession is the whole flow: the
// redirect mints nothing, and the exchange the frontend makes with the code
// answers exactly what POST /auth/google answers.
func TestGoogleRedirectThenExchangeEstablishesTheSession(t *testing.T) {
	f := newFixtureWithGoogle(t)
	f.users.add(activeUser(t, "ana@example.com"))

	redirected := httptest.NewRecorder()
	f.handler.GoogleRedirect(redirected, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))
	code := codeFromLocation(t, redirected)

	w := httptest.NewRecorder()
	f.handler.GoogleExchange(w, postJSON(t, exchangeBody(code, "csrf-abc")))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if findCookie(w.Header(), "access_token") == nil || findCookie(w.Header(), "refresh_token") == nil {
		t.Error("the exchange did not set the session cookies")
	}
	body := decode(t, w)
	if _, ok := body["csrf_token"]; !ok {
		t.Error("csrf_token missing from the response body")
	}
	if _, ok := body["user"]; !ok {
		t.Error("user missing from the response body")
	}
	// One verification for the whole flow: the exchange runs against the
	// claims the redirect already proved, not a second JWKS round trip.
	if len(f.google.calls) != 1 {
		t.Errorf("verifier called %d times, want 1 for the whole flow", len(f.google.calls))
	}
}

// TestGoogleExchangeNeedsProfile: an address with no account gets the same
// profile token /auth/google gives it, and still no session.
func TestGoogleExchangeNeedsProfile(t *testing.T) {
	f := newFixtureWithGoogle(t)

	redirected := httptest.NewRecorder()
	f.handler.GoogleRedirect(redirected, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))
	code := codeFromLocation(t, redirected)

	w := httptest.NewRecorder()
	f.handler.GoogleExchange(w, postJSON(t, exchangeBody(code, "csrf-abc")))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["needs_profile"] != true {
		t.Errorf("needs_profile = %v, want true", body["needs_profile"])
	}
	if token, _ := body["profile_token"].(string); token == "" {
		t.Error("no profile token in the needs_profile answer")
	}
	assertNoSession(t, w)
}

// TestGoogleExchangeReadsNoCookie pins where the exchange gets the binding
// value from: the JSON field, and only the JSON field.
//
// The API is served from api.vibe.com.ar and the app from app.vibe.com.ar, so
// the g_csrf_token cookie Google set on the app's origin never reaches this
// endpoint at all. A handler that read it would work in a test that forged one
// and fail for every real caller. The request below carries no Cookie header
// whatsoever, which is what a real exchange looks like, and it succeeds.
func TestGoogleExchangeReadsNoCookie(t *testing.T) {
	f := newFixtureWithGoogle(t)
	f.users.add(activeUser(t, "ana@example.com"))

	minted := httptest.NewRecorder()
	f.handler.GoogleRedirect(minted, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))
	code := codeFromLocation(t, minted)

	r := postJSON(t, exchangeBody(code, "csrf-abc"))
	if len(r.Header.Values("Cookie")) != 0 {
		t.Fatalf("this test is meaningless with cookies on the request: %v", r.Header.Values("Cookie"))
	}

	w := httptest.NewRecorder()
	f.handler.GoogleExchange(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 with no cookie on the request; got %d (%s)", w.Code, w.Body.String())
	}
	if findCookie(w.Header(), "access_token") == nil {
		t.Error("no session was established")
	}
}

// TestGoogleExchangeIgnoresACookieThatDisagrees is the same rule from the
// other side: a cookie on the exchange request decides nothing, because the
// field is the only thing compared.
func TestGoogleExchangeIgnoresACookieThatDisagrees(t *testing.T) {
	f := newFixtureWithGoogle(t)
	f.users.add(activeUser(t, "ana@example.com"))

	minted := httptest.NewRecorder()
	f.handler.GoogleRedirect(minted, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))
	code := codeFromLocation(t, minted)

	r := postJSON(t, exchangeBody(code, "csrf-abc"))
	r.AddCookie(&http.Cookie{Name: "g_csrf_token", Value: "something-else-entirely"})

	w := httptest.NewRecorder()
	f.handler.GoogleExchange(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("the exchange consulted a cookie it must ignore; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestGoogleExchangeSpendsTheCodeOnce is what makes a leaked return URL worth
// nothing twice.
func TestGoogleExchangeSpendsTheCodeOnce(t *testing.T) {
	f := newFixtureWithGoogle(t)
	f.users.add(activeUser(t, "ana@example.com"))

	redirected := httptest.NewRecorder()
	f.handler.GoogleRedirect(redirected, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))
	code := codeFromLocation(t, redirected)

	first := httptest.NewRecorder()
	f.handler.GoogleExchange(first, postJSON(t, exchangeBody(code, "csrf-abc")))
	if first.Code != http.StatusOK {
		t.Fatalf("first exchange: want 200; got %d (%s)", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	f.handler.GoogleExchange(second, postJSON(t, exchangeBody(code, "csrf-abc")))

	assertInvalidCode(t, second)
}

// TestGoogleExchangeRefusesACodeFromAnotherBrowser is the login-CSRF this
// binding exists for. An attacker holding any valid Google ID token can mint a
// code with curl — it sets both halves of Google's double submit itself — and
// send the victim the return URL. Without the binding the victim's browser
// spends it and lands in the attacker's account. With it, the victim's own
// g_csrf_token cookie is a different value, and nothing is established.
func TestGoogleExchangeRefusesACodeFromAnotherBrowser(t *testing.T) {
	f := newFixtureWithGoogle(t)
	f.users.add(activeUser(t, "ana@example.com"))

	// The attacker's own curl: cookie and field agree, so the redirect
	// endpoint issues a code. It is bound to a browser that does not exist.
	minted := httptest.NewRecorder()
	f.handler.GoogleRedirect(minted, postGoogleForm(t,
		googleForm("attacker-id-token", "attacker-csrf"), "attacker-csrf"))
	code := codeFromLocation(t, minted)

	// The victim follows the link. Their browser holds whatever g_csrf_token
	// Google last set for them, which is not the attacker's.
	w := httptest.NewRecorder()
	f.handler.GoogleExchange(w, postJSON(t, exchangeBody(code, "victim-csrf")))

	assertInvalidCode(t, w)
}

// TestGoogleExchangeSpendsACodeItRefuses: a code presented with the wrong
// token is still consumed, so an attacker cannot retry pairings against it.
func TestGoogleExchangeSpendsACodeItRefuses(t *testing.T) {
	f := newFixtureWithGoogle(t)
	f.users.add(activeUser(t, "ana@example.com"))

	minted := httptest.NewRecorder()
	f.handler.GoogleRedirect(minted, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))
	code := codeFromLocation(t, minted)

	wrong := httptest.NewRecorder()
	f.handler.GoogleExchange(wrong, postJSON(t, exchangeBody(code, "guessed")))
	assertInvalidCode(t, wrong)

	// The right token now, and it is too late: the code was spent by the
	// attempt that got it wrong.
	retried := httptest.NewRecorder()
	f.handler.GoogleExchange(retried, postJSON(t, exchangeBody(code, "csrf-abc")))
	assertInvalidCode(t, retried)
}

func TestGoogleExchangeMissingCSRFToken(t *testing.T) {
	f := newFixtureWithGoogle(t)

	minted := httptest.NewRecorder()
	f.handler.GoogleRedirect(minted, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))
	code := codeFromLocation(t, minted)

	w := httptest.NewRecorder()
	f.handler.GoogleExchange(w, postJSON(t, exchangeBody(code, "")))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if message, ok := fieldError(decode(t, w), "g_csrf_token"); !ok || message != "must be provided" {
		t.Errorf("g_csrf_token error = %q (present %v), want %q", message, ok, "must be provided")
	}
	assertNoSession(t, w)
}

func TestGoogleExchangeUnknownCode(t *testing.T) {
	f := newFixtureWithGoogle(t)

	w := httptest.NewRecorder()
	f.handler.GoogleExchange(w, postJSON(t, exchangeBody("never-minted", "csrf-abc")))

	assertInvalidCode(t, w)
}

func TestGoogleExchangeMissingCode(t *testing.T) {
	f := newFixtureWithGoogle(t)

	w := httptest.NewRecorder()
	f.handler.GoogleExchange(w, postJSON(t, exchangeBody("", "csrf-abc")))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if message, ok := fieldError(decode(t, w), "code"); !ok || message != "must be provided" {
		t.Errorf("code error = %q (present %v), want %q", message, ok, "must be provided")
	}
}

func TestGoogleExchangeDisabledConfig(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.GoogleExchange(w, postJSON(t, exchangeBody("whatever", "csrf-abc")))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503; got %d (%s)", w.Code, w.Body.String())
	}
}

// assertInvalidCode fails unless w is the one answer an unknown, expired or
// already-spent code gets — the same answer for all three, so that nothing
// about which codes have existed can be read off it.
func assertInvalidCode(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	message, ok := fieldError(body, "code")
	if !ok || message != "invalid or expired" {
		t.Errorf("code error = %q (present %v), want %q", message, ok, "invalid or expired")
	}
	for _, name := range []string{"access_token", "refresh_token"} {
		if findCookie(w.Header(), name) != nil {
			t.Errorf("a refused exchange set a %s cookie", name)
		}
	}
}

// ---------------------------------------------------------------------------
// The Redis-backed store
// ---------------------------------------------------------------------------
//
// The in-memory store the tests above exercise is the fallback; Redis is what
// runs in production, and it is the only one where a code minted on the
// instance Google posted to can be spent on the instance the frontend
// reaches. miniredis speaks the real protocol, so the key name, the TTL and
// GETDEL's atomicity are all exercised for real.

// newGoogleCodesFixture returns a fixture whose code store is a live
// miniredis, plus the server so a test can inspect keys and move time.
func newGoogleCodesFixture(t *testing.T) (*fixture, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })

	return newFixtureWithGoogleCodes(t, NewGoogleCodes(rdb, "test")), mr
}

// TestGoogleCodeIsNamespacedAndExpires pins the two things about the key that
// are not visible from the endpoint: it carries the application and
// environment prefix every other key in this service carries (so staging
// cannot mint a code production would accept), and it dies on its own.
func TestGoogleCodeIsNamespacedAndExpires(t *testing.T) {
	f, mr := newGoogleCodesFixture(t)

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))
	code := codeFromLocation(t, w)

	key := "vibe:test:gauth:" + code
	if !mr.Exists(key) {
		t.Fatalf("no code stored at %q; keys are %v", key, mr.Keys())
	}
	if ttl := mr.TTL(key); ttl != googleCodeTTL {
		t.Errorf("TTL = %v, want %v", ttl, googleCodeTTL)
	}
	if _, err := mr.Get(key); err != nil {
		t.Errorf("the code's payload is not a plain value: %v", err)
	}
}

func TestGoogleCodeExpires(t *testing.T) {
	f, mr := newGoogleCodesFixture(t)
	f.users.add(activeUser(t, "ana@example.com"))

	redirected := httptest.NewRecorder()
	f.handler.GoogleRedirect(redirected, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))
	code := codeFromLocation(t, redirected)

	mr.FastForward(googleCodeTTL + time.Second)

	w := httptest.NewRecorder()
	f.handler.GoogleExchange(w, postJSON(t, exchangeBody(code, "csrf-abc")))

	assertInvalidCode(t, w)
}

// TestGoogleCodeIsSpentAtomically drives GETDEL itself: the key is gone after
// the first exchange, so a second one finds nothing whichever instance it
// reaches.
func TestGoogleCodeIsSpentAtomically(t *testing.T) {
	f, mr := newGoogleCodesFixture(t)
	f.users.add(activeUser(t, "ana@example.com"))

	redirected := httptest.NewRecorder()
	f.handler.GoogleRedirect(redirected, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))
	code := codeFromLocation(t, redirected)

	first := httptest.NewRecorder()
	f.handler.GoogleExchange(first, postJSON(t, exchangeBody(code, "csrf-abc")))
	if first.Code != http.StatusOK {
		t.Fatalf("first exchange: want 200; got %d (%s)", first.Code, first.Body.String())
	}
	if mr.Exists("vibe:test:gauth:" + code) {
		t.Error("the code survived the exchange that spent it")
	}

	second := httptest.NewRecorder()
	f.handler.GoogleExchange(second, postJSON(t, exchangeBody(code, "csrf-abc")))
	assertInvalidCode(t, second)
}

// TestGoogleRedirectWithRedisDown: a code that could not be stored is a
// sign-in that cannot continue, and the browser is told so in the only
// vocabulary the login page reads.
func TestGoogleRedirectWithRedisDown(t *testing.T) {
	f, mr := newGoogleCodesFixture(t)
	mr.Close()

	w := httptest.NewRecorder()
	f.handler.GoogleRedirect(w, postGoogleForm(t, googleForm("good-id-token", "csrf-abc"), "csrf-abc"))

	assertRedirectedTo(t, w, testLoginPath+"google_unavailable")
}
