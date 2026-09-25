package mp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

// APIError is returned when the MercadoPago API responds with a non-success status.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("mp: API error %d: %s", e.StatusCode, e.Body)
}

// IsUnauthorized returns true if the error is a 401 from MercadoPago.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusUnauthorized
	}
	return false
}

// ErrNoSellerToken is returned by AsSeller when handed an empty token, and by
// every MPClient method handed the zero Caller that an ignored AsSeller error
// leaves behind.
//
// The empty string used to be a second, unwritten way of saying "the
// platform": UpdatePreferenceExpired, GetPayment and RefundPayment each read
// an empty seller token as permission to authenticate with the marketplace's
// own credentials, so a credential that failed to load turned a per-seller
// call into a platform call with nothing at the call site to show for it.
// Naming the caller is now the only way to reach either account, and the zero
// Caller names neither.
var ErrNoSellerToken = errors.New("mp: no seller access token; say mp.AsPlatform() to call as the marketplace")

// ErrPlatformCannotSell is returned by CreatePreference when handed
// AsPlatform.
//
// Every other method may legitimately run as either party, but MercadoPago
// answers a preference created under the marketplace's own credentials by
// settling the money into the marketplace's account. It is a wrong-payee
// error rather than a retryable one, and the only one of these calls the
// platform must never be allowed to make.
var ErrPlatformCannotSell = errors.New("mp: a checkout preference must be created as the seller, not as the platform")

// Caller names which MercadoPago account a request is made as. Its zero value
// names nobody and every method refuses it.
//
// The type exists so that calling as the platform is something a call site
// writes down (see AsPlatform) rather than something it falls into by handing
// over a token that happened to be empty.
type Caller struct {
	sellerToken string
	platform    bool
}

// AsSeller names the seller whose OAuth access token authorizes the call.
//
// An empty token is the bug this package spent a release chasing, not a
// request for the marketplace's credentials, so it comes back as
// ErrNoSellerToken. A caller that means the platform says so with AsPlatform.
func AsSeller(token string) (Caller, error) {
	if token == "" {
		return Caller{}, ErrNoSellerToken
	}
	return Caller{sellerToken: token}, nil
}

// AsPlatform names the marketplace application itself.
//
// MercadoPago answers the application owner for anything created under its own
// app_id, which is the only reason internal/payments' webhook can read a
// payment before it knows which seller collected it. Everywhere else this is a
// last resort: a call made as the platform against a seller's resource is
// rejected by MercadoPago, and arrives back looking like an ordinary outage.
func AsPlatform() Caller {
	return Caller{platform: true}
}

// bearer resolves a caller to the access token that authorizes it.
func (c *MPClient) bearer(caller Caller) (string, error) {
	switch {
	case caller.platform:
		return c.accessToken, nil
	case caller.sellerToken != "":
		return caller.sellerToken, nil
	default:
		return "", ErrNoSellerToken
	}
}

// MPClient is a circuit-breaker-wrapped HTTP client for the MercadoPago API.
//
//nolint:revive // stutters as mp.MPClient, but renaming touches ~25 call sites outside this doc-comment remediation's scope; matches the existing WAClient/R2Client naming convention.
type MPClient struct {
	accessToken   string
	webhookSecret string
	appID         string
	clientSecret  string
	httpClient    *http.Client
	baseURL       string
	cb            *circuitbreaker.CircuitBreaker
	// now returns the current time. Defaults to time.Now; overridable in
	// tests to pin the webhook signature freshness check to a fixed clock.
	now func() time.Time
}

// NewMPClient builds an MPClient authenticated with the given MercadoPago credentials.
func NewMPClient(accessToken, webhookSecret, appID, clientSecret string, cb *circuitbreaker.CircuitBreaker) *MPClient {
	return &MPClient{
		accessToken:   accessToken,
		webhookSecret: webhookSecret,
		appID:         appID,
		clientSecret:  clientSecret,
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
		baseURL: "https://api.mercadopago.com",
		cb:      cb,
		now:     time.Now,
	}
}

// doRequest executes an HTTP request through the circuit breaker.
// Network errors and 5xx responses count as failures; 2xx-4xx count as successes.
func (c *MPClient) doRequest(req *http.Request) (*http.Response, error) {
	if err := c.cb.AllowRequest(); err != nil {
		return nil, fmt.Errorf("mp: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.cb.RecordFailure()
		return nil, fmt.Errorf("mp: request failed: %w", err)
	}

	if resp.StatusCode >= 500 {
		c.cb.RecordFailure()
	} else {
		c.cb.RecordSuccess()
	}

	return resp, nil
}

// OAuthTokens holds tokens from MercadoPago OAuth exchange.
type OAuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int64  `json:"user_id"`
	ExpiresIn    int    `json:"expires_in"`
}

// ExchangeOAuthCode exchanges an authorization code for access/refresh tokens (MP Marketplace OAuth).
// If codeVerifier is non-empty, it is sent for PKCE validation.
func (c *MPClient) ExchangeOAuthCode(ctx context.Context, code, redirectURI, codeVerifier string) (*OAuthTokens, error) {
	body := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     c.appID,
		"client_secret": c.clientSecret,
		"code":          code,
		"redirect_uri":  redirectURI,
	}
	if codeVerifier != "" {
		body["code_verifier"] = codeVerifier
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("mp: failed to marshal oauth body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/oauth/token", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("mp: failed to create oauth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("mp: oauth request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("mp: oauth exchange failed with status %d (failed to read body: %w)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("mp: oauth exchange failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var tokens OAuthTokens
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("mp: failed to decode oauth response: %w", err)
	}

	return &tokens, nil
}

// RefreshOAuthToken refreshes an expired access token.
func (c *MPClient) RefreshOAuthToken(ctx context.Context, refreshToken string) (*OAuthTokens, error) {
	body := map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     c.appID,
		"client_secret": c.clientSecret,
		"refresh_token": refreshToken,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("mp: failed to marshal refresh body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/oauth/token", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("mp: failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("mp: refresh request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("mp: refresh failed with status %d (failed to read body: %w)", resp.StatusCode, readErr)
		}
		// Typed so callers (cronRefreshMPTokens) can tell a 4xx invalid_grant
		// rejection — the seller revoked access, or the refresh token itself
		// expired — apart from an outage.
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var tokens OAuthTokens
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("mp: failed to decode refresh response: %w", err)
	}

	return &tokens, nil
}

// BackURLs holds the three redirects MercadoPago sends the client's browser to
// after checkout. The caller builds these — internal/booklink, today always
// through internal/bookings/public.go's PublicBook — so internal/mp never
// constructs a client-facing URL itself and never needs to import
// internal/booklink or internal/bookings to do it.
type BackURLs struct {
	Success string
	Failure string
	Pending string
}

// CreatePreferenceInput holds the data needed to create a MercadoPago checkout preference.
type CreatePreferenceInput struct {
	BookingID      uuid.UUID
	ComplexName    string
	CourtName      string
	Date           string
	StartTime      string
	Amount         int // total amount client pays in centavos (deposit + service fee)
	MarketplaceFee int // service fee in centavos retained by marketplace
	// Caller is the seller the preference is created for, and it decides who
	// gets paid: MercadoPago settles into whichever account authorized the
	// call. Only AsSeller is accepted here.
	Caller Caller
	// BackURLs are the three redirects MercadoPago sends the client's browser
	// to. BookingID stays separate from these — it is still sent as
	// external_reference and metadata.booking_id below, which is correlation
	// for a signature-authenticated webhook, not a credential.
	BackURLs   BackURLs
	BackendURL string
	ExpiresIn  time.Duration // how long until the preference expires (default 15m)
	// Payer info (improves approval rate).
	PayerEmail         string
	PayerFirstName     string
	PayerLastName      string
	PayerPhone         string
	PayerPhoneAreaCode string
}

// Preference represents the relevant fields from MercadoPago's preference response.
type Preference struct {
	ID        string `json:"id"`
	InitPoint string `json:"init_point"`
}

// FeeDetail represents a single fee entry from a MercadoPago payment.
type FeeDetail struct {
	Type     string  `json:"type"`
	Amount   float64 `json:"amount"`
	FeePayer string  `json:"fee_payer"`
}

// Payment represents a MercadoPago payment fetched via the API.
type Payment struct {
	ID                        int64          `json:"id"`
	Status                    string         `json:"status"`
	StatusDetail              string         `json:"status_detail"`
	TransactionAmount         float64        `json:"transaction_amount"`
	TransactionAmountRefunded float64        `json:"transaction_amount_refunded"`
	NetReceivedAmount         float64        `json:"net_received_amount"`
	FeeDetails                []FeeDetail    `json:"fee_details"`
	Metadata                  map[string]any `json:"metadata"`
	ExternalReference         string         `json:"external_reference"`
	// CollectorID is the MercadoPago user id of the seller who received the money.
	// It must be checked against the complex's own MP user id: external_reference
	// alone does not prove the payment was collected by the right seller.
	CollectorID int64 `json:"collector_id"`
}

// Refund represents a MercadoPago refund response.
type Refund struct {
	ID     int64   `json:"id"`
	Amount float64 `json:"amount"`
	Status string  `json:"status"`
}

// spanishMonths are the abbreviations titleDate uses, indexed by time.Month.
var spanishMonths = [...]string{"", "ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}

// titleDate renders a booking date (YYYY-MM-DD) for the checkout item title
// and description as "4 sep". The title used to carry "04/09", and
// MercadoPago's checkout strips the slash, so the payer saw "0409 21:00 hs"
// right before paying. A month abbreviation survives that rendering and reads
// as a date in Argentina. An unparseable input is returned unchanged.
func titleDate(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return fmt.Sprintf("%d %s", t.Day(), spanishMonths[t.Month()])
}

// CreatePreference creates a MercadoPago checkout preference.
// operation (build payload, call circuit-breaker-wrapped request, decode response); extraction
// would fragment this into a chain of internal-only helpers each called exactly once.
//
//nolint:funlen // single cohesive request-building-and-execution flow for one MercadoPago API
func (c *MPClient) CreatePreference(ctx context.Context, input CreatePreferenceInput) (*Preference, error) {
	if input.Caller.platform {
		return nil, ErrPlatformCannotSell
	}
	sellerToken, err := c.bearer(input.Caller)
	if err != nil {
		return nil, err
	}

	unitPrice := float64(input.Amount) / 100.0

	dateDisplay := titleDate(input.Date)

	// Preference expires after the configured payment expiry window.
	expiry := input.ExpiresIn
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	now := time.Now()
	expirationFrom := now.Format(time.RFC3339)
	expirationTo := now.Add(expiry).Format(time.RFC3339)

	body := map[string]any{
		"items": []map[string]any{
			{
				"id":          input.BookingID.String(),
				"title":       fmt.Sprintf("%s - %s - %s %s hs", input.ComplexName, input.CourtName, dateDisplay, input.StartTime),
				"description": fmt.Sprintf("Seña para reserva de pádel en %s. %s, %s a las %s hs.", input.ComplexName, input.CourtName, dateDisplay, input.StartTime),
				"quantity":    1,
				"currency_id": "ARS",
				"unit_price":  unitPrice,
				"category_id": "services",
			},
		},
		"back_urls": map[string]string{
			"success": input.BackURLs.Success,
			"failure": input.BackURLs.Failure,
			"pending": input.BackURLs.Pending,
		},
		"payment_methods": map[string]any{
			"excluded_payment_types": []map[string]string{
				{"id": "ticket"},
				{"id": "atm"},
			},
			"installments": 1,
		},
		"notification_url":     fmt.Sprintf("%s/api/v1/webhooks/mercadopago?source_news=webhooks", input.BackendURL),
		"auto_return":          "approved",
		"binary_mode":          true,
		"expires":              true,
		"expiration_date_from": expirationFrom,
		"expiration_date_to":   expirationTo,
		"statement_descriptor": "PADEL RESERVA",
		"external_reference":   input.BookingID.String(),
		"metadata": map[string]string{
			"booking_id":   input.BookingID.String(),
			"booking_date": input.Date,
		},
	}

	// Payer info improves approval rate.
	if input.PayerEmail != "" {
		payer := map[string]any{
			"email": input.PayerEmail,
		}
		if input.PayerFirstName != "" {
			payer["name"] = input.PayerFirstName
		}
		if input.PayerLastName != "" {
			payer["surname"] = input.PayerLastName
		}
		if input.PayerPhone != "" {
			phone := map[string]any{
				"number": input.PayerPhone,
			}
			if input.PayerPhoneAreaCode != "" {
				phone["area_code"] = input.PayerPhoneAreaCode
			} else {
				phone["area_code"] = "54"
			}
			payer["phone"] = phone
		}
		body["payer"] = payer
	}

	// Marketplace split: seller's token identifies the collector; we retain MarketplaceFee.
	if input.MarketplaceFee > 0 {
		body["marketplace_fee"] = float64(input.MarketplaceFee) / 100.0
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("mp: failed to marshal preference body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/checkout/preferences", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("mp: failed to create preference request: %w", err)
	}
	// Use the seller's access token (OAuth) so the preference is created on
	// their behalf. Refused above if the caller is anyone else.
	req.Header.Set("Authorization", "Bearer "+sellerToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", input.BookingID.String())

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("mp: preference request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body) //nolint:errcheck // the status is the error; a truncated body only makes it less legible
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var pref Preference
	if err := json.NewDecoder(resp.Body).Decode(&pref); err != nil {
		return nil, fmt.Errorf("mp: failed to decode preference response: %w", err)
	}

	return &pref, nil
}

// UpdatePreferenceExpired updates a preference to expire immediately, preventing further payments.
//
// caller may be AsPlatform: MercadoPago rejects a marketplace-authenticated
// expiry of a seller's preference, so the worst case is a checkout link that
// stays open and alerts, which is what cmd/api's cron and internal/bookings'
// cancellation both prefer to not trying at all.
func (c *MPClient) UpdatePreferenceExpired(ctx context.Context, preferenceID string, caller Caller) error {
	token, err := c.bearer(caller)
	if err != nil {
		return err
	}

	body := map[string]any{
		"expires":            true,
		"expiration_date_to": time.Now().Add(-1 * time.Minute).Format(time.RFC3339),
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("mp: failed to marshal preference update body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/checkout/preferences/"+preferenceID, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("mp: failed to create preference update request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("mp: preference update request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body) //nolint:errcheck // the status is the error; a truncated body only makes it less legible
		return &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	return nil
}

// GetPayment fetches a payment from MercadoPago by ID.
//
// AsPlatform is load-bearing here and must keep working: internal/payments'
// processPaymentWebhook reads every payment as the marketplace, because
// MercadoPago lets the app owner read anything under its own app_id and the
// webhook does not yet know which seller collected the money. That is the only
// way this method is called in production. Refusing the platform here — the
// way CreatePreference does — would break payment confirmation, the step that
// marks a booking paid. See TestFallbackAsymmetryIsPreserved in mp_test.go,
// which fails the moment it stops working.
func (c *MPClient) GetPayment(ctx context.Context, paymentID string, caller Caller) (*Payment, error) {
	token, err := c.bearer(caller)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/payments/"+paymentID, nil)
	if err != nil {
		return nil, fmt.Errorf("mp: failed to create get payment request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("mp: get payment request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("mp: get payment failed with status %d (failed to read body: %w)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("mp: get payment failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var payment Payment
	if err := json.NewDecoder(resp.Body).Decode(&payment); err != nil {
		return nil, fmt.Errorf("mp: failed to decode payment response: %w", err)
	}

	return &payment, nil
}

// RefundPayment creates a refund for a MercadoPago payment.
//
// The money comes back out of whichever account authorized the call, so
// internal/payments refuses to reach this at all when the seller's credential
// will not load rather than letting the marketplace pay for a seller's refund.
// AsPlatform stays representable because MercadoPago rejects it against a
// seller's payment; the type is what keeps it from being chosen by accident.
func (c *MPClient) RefundPayment(ctx context.Context, paymentID string, amount float64, caller Caller) (*Refund, error) {
	token, err := c.bearer(caller)
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"amount": amount,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("mp: failed to marshal refund body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/payments/"+paymentID+"/refunds", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("mp: failed to create refund request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", paymentID+"-refund-"+fmt.Sprintf("%.2f", amount))

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("mp: refund request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("mp: refund failed with status %d (failed to read body: %w)", resp.StatusCode, readErr)
		}
		// Typed so callers can distinguish a rejection (4xx: the provider
		// looked at the request and said no) from an outage (5xx/timeout),
		// which need different treatment — see issueRefund.
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var refund Refund
	if err := json.NewDecoder(resp.Body).Decode(&refund); err != nil {
		return nil, fmt.Errorf("mp: failed to decode refund response: %w", err)
	}

	return &refund, nil
}

// VerifyWebhookSignature verifies the X-Signature header from MercadoPago webhooks.
// MercadoPago sends: X-Signature: ts={ts},v1={hash}
// The signed string is: "id:{data.id};request-id:{x-request-id};ts:{ts};"
func (c *MPClient) VerifyWebhookSignature(r *http.Request, dataID string) error {
	// Without a secret there is no boundary to enforce. HMAC-SHA256 keyed on the
	// empty string is something any caller on the internet can compute, so an
	// unconfigured deployment would have accepted every forged webhook and
	// reported it as verified — the callers' whole trust decision, silently
	// inverted by a missing environment variable rather than by any code they
	// can see. Refusing here is what makes "the signature was checked" mean the
	// same thing on every path that calls this.
	if c.webhookSecret == "" {
		return fmt.Errorf("mp: no webhook secret is configured, refusing to verify a signature")
	}

	xSignature := r.Header.Get("X-Signature")
	xRequestID := r.Header.Get("X-Request-Id")

	if xSignature == "" {
		return fmt.Errorf("mp: missing X-Signature header")
	}

	// Parse ts and v1 from X-Signature header.
	var ts, v1 string
	parts := strings.SplitSeq(xSignature, ",")
	for part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "ts":
			ts = kv[1]
		case "v1":
			v1 = kv[1]
		}
	}

	if ts == "" || v1 == "" {
		return fmt.Errorf("mp: invalid X-Signature format")
	}

	// Validate timestamp to prevent replay attacks (allow 5 minute tolerance).
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("mp: invalid timestamp in X-Signature: %w", err)
	}
	now := c.now
	if now == nil {
		now = time.Now
	}
	tsTime := time.Unix(tsInt, 0)
	if now().Sub(tsTime).Abs() > 5*time.Minute {
		return fmt.Errorf("mp: webhook timestamp too old or in the future (ts=%s)", ts)
	}

	// MP docs: when data.id is alphanumeric it must be lowercased before
	// computing the signature (numeric ids are unaffected). Build the signed
	// string with the lowercased id.
	signedStr := fmt.Sprintf("id:%s;request-id:%s;ts:%s;", strings.ToLower(dataID), xRequestID, ts)

	// Compute HMAC-SHA256.
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write([]byte(signedStr))
	expectedMAC := mac.Sum(nil)
	expectedHex := hex.EncodeToString(expectedMAC)

	// Constant-time comparison.
	if !hmac.Equal([]byte(expectedHex), []byte(v1)) {
		return fmt.Errorf("mp: webhook signature verification failed")
	}

	return nil
}
