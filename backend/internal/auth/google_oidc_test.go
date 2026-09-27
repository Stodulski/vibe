package auth

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/stodulski/vibe-server/internal/googleid"
)

// authURLParams parses a GoogleStartResult's AuthorizationURL into its query
// values, failing the test if it is not the shape GoogleStart promises.
func authURLParams(t *testing.T, authorizationURL string) url.Values {
	t.Helper()
	if !strings.HasPrefix(authorizationURL, googleAuthorizationEndpoint+"?") {
		t.Fatalf("AuthorizationURL = %q, want a %s?... URL", authorizationURL, googleAuthorizationEndpoint)
	}
	_, rawQuery, _ := strings.Cut(authorizationURL, "?")
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatalf("AuthorizationURL query does not parse: %v", err)
	}
	return values
}

func TestGoogleStartBuildsTheAuthorizationRequest(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)

	start, err := f.service.GoogleStart(t.Context())
	if err != nil {
		t.Fatalf("GoogleStart() = %v, want nil", err)
	}
	if start.State == "" || start.Nonce == "" {
		t.Fatalf("GoogleStart() left State/Nonce empty: %+v", start)
	}
	if start.State == start.Nonce {
		t.Error("state and nonce must not be the same value")
	}

	params := authURLParams(t, start.AuthorizationURL)
	want := map[string]string{
		"response_type":         "code",
		"client_id":             "test-client-id.apps.googleusercontent.com",
		"redirect_uri":          "https://vibe.test/auth/google/callback",
		"scope":                 "openid email profile",
		"state":                 start.State,
		"nonce":                 start.Nonce,
		"code_challenge_method": "S256",
		"prompt":                "select_account",
	}
	for key, val := range want {
		if got := params.Get(key); got != val {
			t.Errorf("param %q = %q, want %q", key, got, val)
		}
	}
	if params.Get("code_challenge") == "" {
		t.Error("no code_challenge in the authorization URL")
	}
}

// exchangedNonce runs GoogleStart and returns it with f.google.claims.Nonce
// already set to match, so GoogleFinish's nonce check passes — the fixture
// for every test below that exercises the happy path or a failure past that
// check.
func exchangedNonce(t *testing.T, f *fixture) *GoogleStartResult {
	t.Helper()
	start, err := f.service.GoogleStart(t.Context())
	if err != nil {
		t.Fatalf("GoogleStart() = %v, want nil", err)
	}
	f.google.claims.Nonce = start.Nonce
	return start
}

func TestGoogleFinishEstablishesTheSession(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	f.users.add(activeUser(t, "ana@example.com"))
	start := exchangedNonce(t, f)

	result, err := f.service.GoogleFinish(t.Context(), Actor{}, "the-auth-code", start.State)
	if err != nil {
		t.Fatalf("GoogleFinish() = %v, want nil", err)
	}
	if result.Session == nil {
		t.Fatal("GoogleFinish did not establish a session")
	}
	if result.NeedsProfile != nil {
		t.Error("an existing account should not answer needs_profile")
	}

	if len(f.codeExchanger.calls) != 1 {
		t.Fatalf("Exchange called %d times, want 1", len(f.codeExchanger.calls))
	}
	call := f.codeExchanger.calls[0]
	if call.code != "the-auth-code" {
		t.Errorf("Exchange code = %q, want %q", call.code, "the-auth-code")
	}
	if call.verifier == "" {
		t.Error("Exchange was called with no PKCE verifier")
	}
	if call.redirectURI != "https://vibe.test/auth/google/callback" {
		t.Errorf("Exchange redirectURI = %q, want the same one GoogleStart advertised", call.redirectURI)
	}
}

func TestGoogleFinishNeedsProfile(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	start := exchangedNonce(t, f)

	result, err := f.service.GoogleFinish(t.Context(), Actor{}, "the-auth-code", start.State)
	if err != nil {
		t.Fatalf("GoogleFinish() = %v, want nil", err)
	}
	if result.NeedsProfile == nil {
		t.Fatal("an address with no account should answer needs_profile")
	}
	if result.Session != nil {
		t.Error("needs_profile must not carry a session")
	}
}

// TestGoogleFinishStateIsSingleUse is what makes a leaked callback URL worth
// nothing twice, the same property TestGoogleExchangeSpendsTheCodeOnce pins
// for the redirect-mode flow.
func TestGoogleFinishStateIsSingleUse(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	f.users.add(activeUser(t, "ana@example.com"))
	start := exchangedNonce(t, f)

	if _, err := f.service.GoogleFinish(t.Context(), Actor{}, "the-auth-code", start.State); err != nil {
		t.Fatalf("first GoogleFinish() = %v, want nil", err)
	}

	_, err := f.service.GoogleFinish(t.Context(), Actor{}, "the-auth-code", start.State)
	if !errors.Is(err, ErrGoogleCodeInvalid) {
		t.Fatalf("second GoogleFinish() = %v, want an error wrapping ErrGoogleCodeInvalid", err)
	}
}

func TestGoogleFinishUnknownState(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)

	_, err := f.service.GoogleFinish(t.Context(), Actor{}, "the-auth-code", "never-minted")
	if !errors.Is(err, ErrGoogleCodeInvalid) {
		t.Fatalf("GoogleFinish() = %v, want an error wrapping ErrGoogleCodeInvalid", err)
	}
	if len(f.codeExchanger.calls) != 0 {
		t.Error("an unknown state reached the token exchange")
	}
}

// TestGoogleFinishCodeRejectedByGoogle is the invalid_grant case: an unknown,
// expired or already-exchanged code at Google's own token endpoint answers
// the same ErrGoogleCodeInvalid an unknown state does, so the frontend's
// existing google_expired handling covers it without a new error value.
func TestGoogleFinishCodeRejectedByGoogle(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	f.codeExchanger.err = googleid.ErrCodeRejected
	start := exchangedNonce(t, f)

	_, err := f.service.GoogleFinish(t.Context(), Actor{}, "spent-code", start.State)
	if !errors.Is(err, ErrGoogleCodeInvalid) {
		t.Fatalf("GoogleFinish() = %v, want an error wrapping ErrGoogleCodeInvalid", err)
	}
	// The state was consumed before the exchange was even attempted: a
	// retried finish against the same state must not get a second try.
	_, err = f.service.GoogleFinish(t.Context(), Actor{}, "spent-code", start.State)
	if !errors.Is(err, ErrGoogleCodeInvalid) {
		t.Fatalf("retried GoogleFinish() = %v, want an error wrapping ErrGoogleCodeInvalid", err)
	}
}

func TestGoogleFinishTokenExchangeUnavailable(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	f.codeExchanger.err = googleid.ErrUnavailable
	start := exchangedNonce(t, f)

	_, err := f.service.GoogleFinish(t.Context(), Actor{}, "the-auth-code", start.State)
	if !errors.Is(err, googleid.ErrUnavailable) {
		t.Fatalf("GoogleFinish() = %v, want an error wrapping googleid.ErrUnavailable", err)
	}
}

func TestGoogleFinishIDTokenRejected(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	start, err := f.service.GoogleStart(t.Context())
	if err != nil {
		t.Fatalf("GoogleStart() = %v, want nil", err)
	}
	// Deliberately not calling exchangedNonce: the verifier itself refuses
	// the token before the nonce is ever compared.
	f.google.err = googleid.ErrInvalidToken

	_, err = f.service.GoogleFinish(t.Context(), Actor{}, "the-auth-code", start.State)
	if !errors.Is(err, ErrGoogleRejected) {
		t.Fatalf("GoogleFinish() = %v, want an error wrapping ErrGoogleRejected", err)
	}
}

// TestGoogleFinishNonceMismatch is the check PKCE alone does not give: the
// exchange proves this server requested the token, not which authorization
// attempt the token answers. f.google.claims.Nonce is left at its zero value
// (newFixtureWithGoogle never sets it), which will not equal whatever
// GoogleStart minted.
func TestGoogleFinishNonceMismatch(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	start, err := f.service.GoogleStart(t.Context())
	if err != nil {
		t.Fatalf("GoogleStart() = %v, want nil", err)
	}

	_, err = f.service.GoogleFinish(t.Context(), Actor{}, "the-auth-code", start.State)
	if !errors.Is(err, ErrGoogleRejected) {
		t.Fatalf("GoogleFinish() = %v, want an error wrapping ErrGoogleRejected", err)
	}
}

func TestGoogleOAuthEnabled(t *testing.T) {
	f := newFixtureWithGoogleOAuth(t)
	if !f.service.GoogleOAuthEnabled() {
		t.Fatal("GoogleOAuthEnabled() = false with client id, secret and FrontendURL all set")
	}

	f.google.enabled = false
	if f.service.GoogleOAuthEnabled() {
		t.Error("GoogleOAuthEnabled() = true with no Google verifier configured")
	}
	f.google.enabled = true

	f.codeExchanger.enabled = false
	if f.service.GoogleOAuthEnabled() {
		t.Error("GoogleOAuthEnabled() = true with no client secret configured")
	}
	f.codeExchanger.enabled = true

	f.service.cfg.FrontendURL = ""
	if f.service.GoogleOAuthEnabled() {
		t.Error("GoogleOAuthEnabled() = true with no FrontendURL configured")
	}
}

// ---------------------------------------------------------------------------
// The Redis-backed store, shared with the redirect-mode codes
// ---------------------------------------------------------------------------

// newGoogleOAuthFixture is newFixtureWithGoogleOAuth with the state store
// replaced by a live miniredis, so the key name and TTL are exercised for
// real — the same reasoning newGoogleCodesFixture gives for the redirect-mode
// codes, whose store this flow reuses.
func newGoogleOAuthFixture(t *testing.T) (*fixture, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })

	f := newFixtureWithGoogleOAuth(t)
	f.service.googleCodes = NewGoogleCodes(rdb, "test")
	return f, mr
}

// TestGoogleOAuthStateIsNamespacedUnderTheSameStore pins that this flow's
// entries live under a distinct key inside the redirect-mode code store
// (Dependencies.GoogleCodes) rather than a second Redis type, and that they
// carry the same TTL GoogleStart promises.
func TestGoogleOAuthStateIsNamespacedUnderTheSameStore(t *testing.T) {
	f, mr := newGoogleOAuthFixture(t)

	start, err := f.service.GoogleStart(t.Context())
	if err != nil {
		t.Fatalf("GoogleStart() = %v, want nil", err)
	}

	key := "vibe:test:gauth:oidc-state:" + start.State
	if !mr.Exists(key) {
		t.Fatalf("no state stored at %q; keys are %v", key, mr.Keys())
	}
	if ttl := mr.TTL(key); ttl != googleOAuthStateTTL {
		t.Errorf("TTL = %v, want %v", ttl, googleOAuthStateTTL)
	}
}

func TestGoogleOAuthStateExpires(t *testing.T) {
	f, mr := newGoogleOAuthFixture(t)
	f.users.add(activeUser(t, "ana@example.com"))
	start := exchangedNonce(t, f)

	mr.FastForward(googleOAuthStateTTL + time.Second)

	_, err := f.service.GoogleFinish(t.Context(), Actor{}, "the-auth-code", start.State)
	if !errors.Is(err, ErrGoogleCodeInvalid) {
		t.Fatalf("GoogleFinish() after expiry = %v, want an error wrapping ErrGoogleCodeInvalid", err)
	}
}
