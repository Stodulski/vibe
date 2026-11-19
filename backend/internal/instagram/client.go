// Package instagram is the client for Business Login for Instagram: the
// consent page URL, the code exchange, the long-lived token exchange and
// refresh, and the read of the account a token belongs to. It holds no state
// and stores nothing; tokens go in and out of its methods only.
//
// Hosts, per https://developers.facebook.com/documentation/instagram-platform/instagram-api-with-instagram-login/business-login
// (checked 2026-11-19):
//
//	www.instagram.com/oauth/authorize          consent page
//	api.instagram.com/oauth/access_token       code -> short-lived token (form POST)
//	graph.instagram.com/access_token           short-lived -> long-lived (60 days)
//	graph.instagram.com/refresh_access_token   long-lived -> long-lived, at least 24 h old
//	graph.instagram.com/me                     the account
//
// No error this package returns carries a token or a URL with one in it.
package instagram

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

const (
	defaultAuthorizeBase = "https://www.instagram.com"
	defaultAPIBase       = "https://api.instagram.com"
	defaultGraphBase     = "https://graph.instagram.com"
	defaultTimeout       = 15 * time.Second
	// defaultTokenLife is what Meta documents for a long-lived token, used only
	// when a response leaves expires_in out.
	defaultTokenLife = 60 * 24 * time.Hour
	maxResponseBytes = 1 << 20
	// Scopes are the permissions the consent page asks for: the account's
	// basics and publishing.
	Scopes = "instagram_business_basic,instagram_business_content_publish"
)

var (
	// ErrRejected says Meta answered and refused: a used or expired code, an
	// invalid or expired token, a mismatched redirect URI. Retrying the same
	// call does not help.
	ErrRejected = errors.New("instagram: request rejected")
	// ErrUnavailable says Meta could not be reached, failed, throttled, or the
	// circuit breaker is open. A later retry may succeed.
	ErrUnavailable = errors.New("instagram: service unavailable")
)

// Config is the app and the hosts. Empty hosts take the real ones; tests point
// them at an httptest server.
type Config struct {
	AppID         string
	AppSecret     string
	RedirectURI   string
	AuthorizeBase string
	APIBase       string
	GraphBase     string
	HTTP          *http.Client
	CB            *circuitbreaker.CircuitBreaker
}

// Client talks to Instagram for any account; the token is always an argument.
type Client struct {
	cfg  Config
	http *http.Client
	cb   *circuitbreaker.CircuitBreaker
}

// ShortToken is the result of the code exchange.
type ShortToken struct {
	AccessToken string
	// UserID is the app-scoped Instagram user id, the same value /me answers as id.
	UserID string
}

// Token is a long-lived token and how long it lives.
type Token struct {
	AccessToken string
	ExpiresIn   time.Duration
}

// Account is what /me reports about the token's account.
type Account struct {
	// ID is the app-scoped id; it matches ShortToken.UserID.
	ID string
	// UserID is the Instagram professional account id (<IG_ID>), what the
	// publishing endpoints name.
	UserID      string
	Username    string
	AccountType string
}

// New returns a Client. A nil HTTP client gets one with an explicit timeout.
func New(cfg Config) *Client {
	if cfg.AuthorizeBase == "" {
		cfg.AuthorizeBase = defaultAuthorizeBase
	}
	if cfg.APIBase == "" {
		cfg.APIBase = defaultAPIBase
	}
	if cfg.GraphBase == "" {
		cfg.GraphBase = defaultGraphBase
	}
	httpClient := cfg.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{cfg: cfg, http: httpClient, cb: cfg.CB}
}

// AuthorizeURL is the consent page for state.
func (c *Client) AuthorizeURL(state string) string {
	q := url.Values{
		"client_id":     {c.cfg.AppID},
		"redirect_uri":  {c.cfg.RedirectURI},
		"response_type": {"code"},
		"scope":         {Scopes},
		"state":         {state},
	}
	return c.cfg.AuthorizeBase + "/oauth/authorize?" + q.Encode()
}

// ExchangeCode trades the consent code for a short-lived token. Meta appends
// "#_" to the code in the redirect; it is not part of the code and is dropped.
func (c *Client) ExchangeCode(ctx context.Context, code string) (ShortToken, error) {
	form := url.Values{
		"client_id":     {c.cfg.AppID},
		"client_secret": {c.cfg.AppSecret},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.cfg.RedirectURI},
		"code":          {strings.TrimSuffix(code, "#_")},
	}
	type shortBody struct {
		AccessToken string      `json:"access_token"`
		UserID      json.Number `json:"user_id"`
	}
	// The documentation shows the token inside a "data" array; the endpoint
	// has also been seen answering it flat. Both are read.
	var out struct {
		shortBody
		Data []shortBody `json:"data"`
	}
	if err := c.post(ctx, c.cfg.APIBase+"/oauth/access_token", form, &out); err != nil {
		return ShortToken{}, err
	}
	body := out.shortBody
	if body.AccessToken == "" && len(out.Data) > 0 {
		body = out.Data[0]
	}
	if body.AccessToken == "" || body.UserID.String() == "" {
		return ShortToken{}, fmt.Errorf("%w: the code exchange returned no token", ErrRejected)
	}
	return ShortToken{AccessToken: body.AccessToken, UserID: body.UserID.String()}, nil
}

// LongLived trades a short-lived token for a 60-day one.
func (c *Client) LongLived(ctx context.Context, shortToken string) (Token, error) {
	return c.token(ctx, "/access_token", url.Values{
		"grant_type":    {"ig_exchange_token"},
		"client_secret": {c.cfg.AppSecret},
		"access_token":  {shortToken},
	})
}

// Refresh renews a long-lived token that is at least 24 hours old and not
// expired, for another 60 days.
func (c *Client) Refresh(ctx context.Context, longToken string) (Token, error) {
	return c.token(ctx, "/refresh_access_token", url.Values{
		"grant_type":   {"ig_refresh_token"},
		"access_token": {longToken},
	})
}

func (c *Client) token(ctx context.Context, path string, q url.Values) (Token, error) {
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := c.get(ctx, c.cfg.GraphBase+path, q, &out); err != nil {
		return Token{}, err
	}
	if out.AccessToken == "" {
		return Token{}, fmt.Errorf("%w: the token response carried no token", ErrRejected)
	}
	life := time.Duration(out.ExpiresIn) * time.Second
	if life <= 0 {
		life = defaultTokenLife
	}
	return Token{AccessToken: out.AccessToken, ExpiresIn: life}, nil
}

// Me reads the account the token belongs to.
func (c *Client) Me(ctx context.Context, token string) (Account, error) {
	type meBody struct {
		ID          json.Number `json:"id"`
		UserID      json.Number `json:"user_id"`
		Username    string      `json:"username"`
		AccountType string      `json:"account_type"`
	}
	var out struct {
		meBody
		Data []meBody `json:"data"`
	}
	q := url.Values{"fields": {"id,user_id,username,account_type"}, "access_token": {token}}
	if err := c.get(ctx, c.cfg.GraphBase+"/me", q, &out); err != nil {
		return Account{}, err
	}
	body := out.meBody
	if body.UserID.String() == "" && len(out.Data) > 0 {
		body = out.Data[0]
	}
	if body.UserID.String() == "" || body.Username == "" {
		return Account{}, fmt.Errorf("%w: the account response named no account", ErrRejected)
	}
	return Account{
		ID:          body.ID.String(),
		UserID:      body.UserID.String(),
		Username:    body.Username,
		AccountType: strings.ToUpper(body.AccountType),
	}, nil
}

func (c *Client) get(ctx context.Context, rawURL string, params url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("instagram: build request: %w", redact(err))
	}
	return c.send(req, out)
}

func (c *Client) post(ctx context.Context, rawURL string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("instagram: build request: %w", redact(err))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.send(req, out)
}

// send is the one place a request leaves this package: AllowRequest, Do,
// RecordSuccess or RecordFailure.
func (c *Client) send(req *http.Request, out any) error {
	if err := c.cb.AllowRequest(); err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.cb.RecordFailure()
		return fmt.Errorf("%w: %w", ErrUnavailable, redact(err))
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		c.cb.RecordFailure()
		return fmt.Errorf("%w: read response: %w", ErrUnavailable, redact(err))
	}
	transient := resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests
	if transient {
		c.cb.RecordFailure()
		return newAPIError(ErrUnavailable, resp.StatusCode, raw)
	}
	c.cb.RecordSuccess()
	if resp.StatusCode >= http.StatusBadRequest {
		return newAPIError(ErrRejected, resp.StatusCode, raw)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: decode response: %w", ErrUnavailable, err)
	}
	return nil
}

// APIError is an error answer from Meta: the HTTP status and, when the body is
// a Graph error object, its code and message. It wraps ErrRejected (a 4xx other
// than 429) or ErrUnavailable (5xx and 429), so errors.Is keeps working; the
// code tells the answers that need their own handling apart (GraphCode).
type APIError struct {
	kind    error
	Status  int
	Code    int
	Subcode int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%v: status %d: %s", e.kind, e.Status, e.Message)
}

func (e *APIError) Unwrap() error { return e.kind }

// The Graph error codes publishing distinguishes.
const (
	// CodeResourceMissing is "The requested resource does not exist", which
	// Meta now and then answers for a container it has just created. Retryable.
	CodeResourceMissing = 24
	// CodeInvalidToken is an expired, revoked or otherwise invalid token: the
	// owner has to connect again.
	CodeInvalidToken = 190
	// CodeMediaNotReady is media_publish while the container is still being
	// processed (subcode 2207027). Meta documents it as wait and retry.
	CodeMediaNotReady = 9007
)

// GraphCode is the Graph error code err carries, 0 when it carries none.
func GraphCode(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return 0
}

// newAPIError reads Meta's error out of either shape it uses: Graph's
// {"error":{"message","code"}} and the OAuth host's {"error_message"}. Neither
// ever echoes the token.
func newAPIError(kind error, status int, raw []byte) *APIError {
	e := &APIError{kind: kind, Status: status, Message: "no error message"}
	var body struct {
		ErrorMessage string `json:"error_message"`
		Error        struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
			Subcode int    `json:"error_subcode"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		if len(raw) > 0 {
			e.Message = "unreadable error body"
		}
		return e
	}
	e.Code, e.Subcode = body.Error.Code, body.Error.Subcode
	switch {
	case body.Error.Message != "":
		e.Message = fmt.Sprintf("%s (code %d)", body.Error.Message, body.Error.Code)
	case body.ErrorMessage != "":
		e.Message = body.ErrorMessage
	}
	return e
}

// redact drops the URL from a transport error: a GET to graph.instagram.com
// carries the token in its query string, and *url.Error prints it.
func redact(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("%s request: %w", urlErr.Op, urlErr.Err)
	}
	return err
}
