package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/stodulski/vibe-server/internal/googleid"
)

// ---------------------------------------------------------------------------
// Google sign-in, OIDC authorization-code flow (GoogleStart / GoogleFinish)
// ---------------------------------------------------------------------------
//
// This is the standard OpenID Connect authorization-code flow with PKCE, and
// the only way to sign in with Google: a GET /auth/google/start that sends
// the browser to Google's own consent screen, and a POST /auth/google/finish
// the SPA calls from its callback page once Google sends the browser back
// with a code. /auth/google/complete is its own second step, for a first-time
// sign-in that still needs a phone number Google never provides.
//
// The two requests are bound together the same way a login-CSRF defence
// always works: an opaque value only the browser that started the attempt can
// present again. Here it is `state` in a cookie this server sets, read back
// by the callback page and sent in the finish request body. GoogleFinish
// compares the two in constant time before it will spend anything.
//
// PKCE is the second half of the standard's own defence: the verifier this
// server generated and kept to itself never travels in a URL or a redirect a
// network intermediary or a nosy browser extension could observe, only its
// SHA-256 challenge does, in the /start redirect. Without it, a code
// intercepted off the /auth/google/callback redirect would be exchangeable by
// whoever intercepted it; with it, the token endpoint refuses an exchange
// presenting the wrong verifier.
//
// The nonce closes the remaining gap PKCE does not cover: PKCE proves this
// server made the token request, but says nothing about which authorization
// attempt the ID token Google hands back actually answers. The nonce this
// server minted at /start and asked Google to echo into the token is what
// ties the two together — GoogleFinish refuses an ID token whose nonce does
// not match what /start stored.

const (
	// googleOAuthCallbackPath is appended to FrontendURL to build the
	// redirect_uri both GoogleStart (in the authorization request) and
	// GoogleFinish (in the token exchange) present to Google. The two must
	// agree byte for byte, which is why this is the one place either of them
	// builds it.
	googleOAuthCallbackPath = "/auth/google/callback"
	// googleAuthorizationEndpoint is Google's OAuth 2.0 authorization
	// endpoint. See
	// https://developers.google.com/identity/protocols/oauth2/web-server#creatingclient.
	googleAuthorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	// googleOAuthStateTTL bounds how long a started attempt stays
	// completable: long enough for a person to pick an account on Google's
	// consent screen, short enough that a state nobody finished is not a
	// standing entry in the store.
	googleOAuthStateTTL = 10 * time.Minute
	// googleOAuthEntropyBytes is the entropy behind state, nonce and the PKCE
	// verifier alike — a full 256 bits for each, matching googleCodeBytes
	// above. Raw URL-safe base64 of 32 bytes is also a valid PKCE
	// code_verifier under RFC 7636 (43 characters, within its 43-128 bound).
	googleOAuthEntropyBytes = 32
)

// googleOAuthState is what GoogleStart stores under state, and what
// GoogleFinish reads back to complete the attempt: the nonce the ID token
// must echo, and the PKCE verifier the token exchange must present. Both are
// single-use — see GoogleCodeStore.Consume — so a state cannot be replayed
// against a second finish, whatever it carries.
type googleOAuthState struct {
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
}

// oidcStateKeyPrefix namespaces this flow's entries inside GoogleCodes (see
// Dependencies.GoogleCodes) — a store built for exactly this shape: an opaque
// value, single-use, with a TTL.
const oidcStateKeyPrefix = "oidc-state:"

func oidcStateKey(state string) string { return oidcStateKeyPrefix + state }

// randomURLToken returns n bytes of crypto/rand, base64 raw-URL-encoded.
func randomURLToken(n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("auth: generating random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// pkceChallenge returns verifier's S256 code_challenge (RFC 7636 §4.2):
// base64url, no padding, of its SHA-256.
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// GoogleStartResult is what a browser needs to begin the OIDC flow: the
// value the handler puts in the state cookie, and the URL to send it to.
// Nonce is included for observability and tests — it is already public
// inside AuthorizationURL's own "nonce" parameter, so exposing it here
// carries nothing AuthorizationURL does not already.
type GoogleStartResult struct {
	State            string
	Nonce            string
	AuthorizationURL string
}

// GoogleOAuthEnabled reports whether the OIDC authorization-code flow is
// configured: the client id and secret Google's console handed out, and
// FrontendURL to build a redirect_uri and a return address from. All three or
// none — a deployment missing any of them gets the same "not configured"
// answer GoogleStart and GoogleFinish already give the GIS flow when
// GOOGLE_OAUTH_CLIENT_ID is empty.
func (s *Service) GoogleOAuthEnabled() bool {
	return s.google.Enabled() && s.codeExchanger.Enabled() && s.cfg.FrontendURL != ""
}

// GoogleStart mints state, a nonce and a PKCE verifier, stores {nonce,
// verifier} single-use under state for googleOAuthStateTTL, and returns both
// state (for the handler's cookie) and the URL to send the browser to.
func (s *Service) GoogleStart(ctx context.Context) (*GoogleStartResult, error) {
	state, err := randomURLToken(googleOAuthEntropyBytes)
	if err != nil {
		return nil, err
	}
	nonce, err := randomURLToken(googleOAuthEntropyBytes)
	if err != nil {
		return nil, err
	}
	verifier, err := randomURLToken(googleOAuthEntropyBytes)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(googleOAuthState{Nonce: nonce, Verifier: verifier})
	if err != nil {
		return nil, fmt.Errorf("auth: encoding google oauth state: %w", err)
	}
	if err := s.googleCodes.Store(ctx, oidcStateKey(state), payload, googleOAuthStateTTL); err != nil {
		return nil, err
	}

	values := url.Values{
		"response_type":         {"code"},
		"client_id":             {s.cfg.OAuthClientID},
		"redirect_uri":          {s.cfg.FrontendURL + googleOAuthCallbackPath},
		"scope":                 {"openid email profile"},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {pkceChallenge(verifier)},
		"code_challenge_method": {"S256"},
		"prompt":                {"select_account"},
	}
	return &GoogleStartResult{
		State:            state,
		Nonce:            nonce,
		AuthorizationURL: googleAuthorizationEndpoint + "?" + values.Encode(),
	}, nil
}

// GoogleFinish completes an OIDC attempt: it consumes the entry GoogleStart
// stored under state (single-use — a replay finds nothing), exchanges code
// at Google's token endpoint using the PKCE verifier that entry carried and
// the redirect_uri GoogleStart advertised, verifies the returned ID token
// exactly as the GIS flow does, and additionally checks that its nonce
// matches what GoogleStart minted. From there it is the same outcome as
// every other verified Google sign-in: googleSignInWithClaims decides
// between a session and needs_profile.
//
// An unknown, expired or already-spent state and a code Google refuses
// (googleid.ErrCodeRejected, typically invalid_grant) both answer
// ErrGoogleCodeInvalid, for one reason: telling them apart would answer
// questions about states or codes the caller never held. An ID token that fails
// verification, or whose nonce does not match, answers ErrGoogleRejected —
// the same error a rejected credential raises everywhere else in this
// module.
func (s *Service) GoogleFinish(ctx context.Context, actor Actor, code, state string) (*GoogleResult, error) {
	payload, err := s.googleCodes.Consume(ctx, oidcStateKey(state))
	if err != nil {
		return nil, err
	}
	var held googleOAuthState
	if err := json.Unmarshal(payload, &held); err != nil {
		return nil, fmt.Errorf("auth: decoding google oauth state: %w", err)
	}

	redirectURI := s.cfg.FrontendURL + googleOAuthCallbackPath
	idToken, err := s.codeExchanger.Exchange(ctx, code, held.Verifier, redirectURI)
	if err != nil {
		switch {
		case errors.Is(err, googleid.ErrCodeRejected):
			s.logger.Warn("google oidc finish: code rejected by google", "error", err)
			return nil, ErrGoogleCodeInvalid
		case errors.Is(err, googleid.ErrUnavailable):
			s.logger.Error("google oidc finish: token exchange unavailable", "error", err)
			return nil, err
		default:
			return nil, err
		}
	}

	claims, err := s.verifyGoogleCredential(ctx, idToken)
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(held.Nonce)) != 1 {
		s.logger.Warn("google oidc finish: id token nonce did not match the one this attempt started with")
		return nil, ErrGoogleRejected
	}

	return s.googleSignInWithClaims(ctx, actor, claims)
}
