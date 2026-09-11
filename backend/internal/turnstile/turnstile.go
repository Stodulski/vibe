// Package turnstile verifies Cloudflare Turnstile tokens against Cloudflare's
// siteverify endpoint.
//
// It follows this codebase's external-service-client pattern (see
// internal/mailer): a constructor takes config plus an optional circuit
// breaker, every call runs AllowRequest -> Do -> RecordSuccess/RecordFailure,
// and the breaker is nil-safe so a caller that never configures one still
// works. Only infrastructure failures — a request that could not be sent, a
// non-2xx status, an unreadable body — count against the breaker; a token
// Cloudflare judged invalid on its own merits does not, for the same reason a
// message Brevo rejected on its own merits does not (see mailer.isInfraError).
package turnstile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

// siteverifyURL is Cloudflare's Turnstile verification endpoint. It is a
// field on Client rather than used inline so tests can point the client at a
// local server — see mailer.brevoAPIURL for the same reason.
const siteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// ErrMissingToken is returned by Verify when token is empty. Callers in
// internal/auth check this themselves before calling Verify (so the
// validation error can name the field), but Verify refuses an empty token
// on its own too, so nothing depends on that ordering to stay safe.
var ErrMissingToken = errors.New("turnstile: token is empty")

// ErrInvalidToken is returned when Cloudflare processed the request and
// rejected the token (success=false). Verify wraps it with the error-codes
// Cloudflare returned, which is useful in logs but must never reach the
// client verbatim — see internal/auth, which maps every ErrInvalidToken to
// the same generic "invalid" validation code regardless of the codes inside.
var ErrInvalidToken = errors.New("turnstile: token rejected")

// ErrUnavailable is returned when Cloudflare could not be reached, answered
// with something other than a well-formed 2xx JSON body, or the circuit
// breaker is open. It says nothing about the token itself: retried once
// Cloudflare (or the breaker) recovers, the same token may well succeed.
var ErrUnavailable = errors.New("turnstile: verification service unavailable")

// Config configures a Client.
type Config struct {
	// SecretKey is the Cloudflare Turnstile secret. Empty disables
	// verification entirely — see Enabled.
	SecretKey string
	CB        *circuitbreaker.CircuitBreaker
}

// Client verifies Turnstile tokens.
type Client struct {
	secretKey  string
	apiURL     string
	httpClient *http.Client
	cb         *circuitbreaker.CircuitBreaker
}

// New returns a Client.
func New(cfg Config) *Client {
	return &Client{
		secretKey:  cfg.SecretKey,
		apiURL:     siteverifyURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		cb:         cfg.CB,
	}
}

// Enabled reports whether a secret key is configured. A self-hosted
// deployment that never sets TURNSTILE_SECRET_KEY is not forced to use
// Turnstile — callers check this before asking Verify to do anything.
func (c *Client) Enabled() bool {
	return c.secretKey != ""
}

// siteverifyResponse is the subset of Cloudflare's response this client
// reads. See https://developers.cloudflare.com/turnstile/get-started/server-side-validation/.
type siteverifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify checks token against Cloudflare's siteverify endpoint. remoteIP is
// the caller's address, forwarded to Cloudflare as an extra signal; an empty
// remoteIP is sent as no remoteip parameter at all rather than a blank one.
func (c *Client) Verify(ctx context.Context, token, remoteIP string) error {
	if token == "" {
		return ErrMissingToken
	}

	if err := c.cb.AllowRequest(); err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	form := url.Values{}
	form.Set("secret", c.secretKey)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		c.cb.RecordFailure()
		return fmt.Errorf("%w: building request: %w", ErrUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil { // caller gave up; not Cloudflare's fault
			return fmt.Errorf("%w: %w", ErrUnavailable, ctx.Err())
		}
		c.cb.RecordFailure()
		return fmt.Errorf("%w: request failed: %w", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.cb.RecordFailure()
		return fmt.Errorf("%w: siteverify returned status %d", ErrUnavailable, resp.StatusCode)
	}

	var body siteverifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		c.cb.RecordFailure()
		return fmt.Errorf("%w: decoding response: %w", ErrUnavailable, err)
	}

	// The request itself succeeded — Cloudflare answered, on time, with a
	// well-formed body. Whether it approved or refused the token is a
	// judgment about this one token, not about Cloudflare's health, so it is
	// recorded as a success either way; only the branch below decides what
	// the caller is told.
	c.cb.RecordSuccess()

	if !body.Success {
		return fmt.Errorf("%w: %s", ErrInvalidToken, strings.Join(body.ErrorCodes, ", "))
	}
	return nil
}
