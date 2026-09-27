package googleid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

// defaultTokenURL is Google's OAuth 2.0 token endpoint, where an
// authorization code is traded for an ID token. See
// https://developers.google.com/identity/protocols/oauth2/web-server#exchange-authorization-code.
//
//nolint:gosec // G101: "token" makes gosec suspect a credential; this is a URL, not a secret
const defaultTokenURL = "https://oauth2.googleapis.com/token"

// tokenResponseMaxBytes caps how much of the token endpoint's answer is read.
// A real response is an id_token plus a few short fields, a few KB at most;
// anything past this is not a response worth decoding, and a misbehaving
// endpoint must not be able to make this process buffer it.
const tokenResponseMaxBytes = 64 << 10

// ErrCodeRejected is returned when Google's token endpoint refuses the
// authorization code — most commonly `invalid_grant`: unknown, expired,
// already exchanged, or issued against a different redirect_uri, client or
// PKCE verifier. It is the caller's fault, never counted against the circuit
// breaker, and safe to answer as a generic validation failure; the wrapped
// detail is for logs only.
var ErrCodeRejected = errors.New("googleid: authorization code rejected")

// ExchangeConfig configures a CodeExchanger.
type ExchangeConfig struct {
	// ClientID and ClientSecret are this application's OAuth client
	// credentials — the same ClientID Verifier checks tokens' "aud" against.
	ClientID     string
	ClientSecret string
	// TokenURL overrides defaultTokenURL. Tests point it at a local server;
	// production leaves it empty.
	TokenURL string
	CB       *circuitbreaker.CircuitBreaker
}

// CodeExchanger trades an OIDC authorization code — together with the PKCE
// verifier it was requested with — for an ID token, following this
// codebase's external-service-client pattern (see Verifier above): a
// constructor takes config plus an optional circuit breaker, and every
// outbound call runs AllowRequest -> Do -> RecordSuccess/RecordFailure.
type CodeExchanger struct {
	clientID     string
	clientSecret string
	tokenURL     string
	httpClient   *http.Client
	cb           *circuitbreaker.CircuitBreaker
}

// NewCodeExchanger returns a CodeExchanger.
func NewCodeExchanger(cfg ExchangeConfig) *CodeExchanger {
	tokenURL := cfg.TokenURL
	if tokenURL == "" {
		tokenURL = defaultTokenURL
	}
	return &CodeExchanger{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		tokenURL:     tokenURL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		cb:           cfg.CB,
	}
}

// Enabled reports whether the authorization-code flow is configured: a
// client secret alongside the client id Verifier already requires. A
// deployment that never sets GOOGLE_OAUTH_CLIENT_SECRET is not forced to
// offer it — callers check this before calling Exchange.
func (c *CodeExchanger) Enabled() bool {
	return c.clientID != "" && c.clientSecret != ""
}

// tokenResponse is the subset of Google's token endpoint response this
// package reads: the ID token on success, and the error vocabulary
// (https://www.rfc-editor.org/rfc/rfc6749#section-5.2) on the 4xx a rejected
// code answers with.
type tokenResponse struct {
	IDToken          string `json:"id_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Exchange trades code and verifier — the PKCE code_verifier /start minted —
// for an ID token, presenting redirectURI exactly as /start sent it to
// Google: the token endpoint refuses the exchange if it does not match
// byte for byte.
//
// It returns ErrCodeRejected for a 4xx Google answers about the code itself
// (invalid_grant and friends), and ErrUnavailable for everything that is this
// deployment's problem rather than the caller's: an unreachable endpoint, a
// malformed response, or a 5xx.
func (c *CodeExchanger) Exchange(ctx context.Context, code, verifier, redirectURI string) (string, error) {
	if err := c.cb.AllowRequest(); err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		c.cb.RecordFailure()
		return "", fmt.Errorf("%w: building request: %w", ErrUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil { // caller gave up; not Google's fault
			return "", fmt.Errorf("%w: %w", ErrUnavailable, ctx.Err())
		}
		c.cb.RecordFailure()
		return "", fmt.Errorf("%w: request failed: %w", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var doc tokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, tokenResponseMaxBytes)).Decode(&doc); err != nil {
		c.cb.RecordFailure()
		return "", fmt.Errorf("%w: decoding response: %w", ErrUnavailable, err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			// The caller's code, not this deployment's health: never
			// recorded against the breaker, exactly like Verifier's own
			// ErrInvalidToken branch never is.
			return "", fmt.Errorf("%w: %s: %s", ErrCodeRejected, doc.Error, doc.ErrorDescription)
		}
		c.cb.RecordFailure()
		return "", fmt.Errorf("%w: token endpoint returned status %d", ErrUnavailable, resp.StatusCode)
	}
	if doc.IDToken == "" {
		c.cb.RecordFailure()
		return "", fmt.Errorf("%w: token response carried no id_token", ErrUnavailable)
	}

	c.cb.RecordSuccess()
	return doc.IDToken, nil
}
