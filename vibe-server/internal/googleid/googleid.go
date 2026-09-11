// Package googleid verifies Google Identity Services ID tokens — the JWT a
// browser hands back after "Sign in with Google" — against Google's published
// JWKS. It follows this codebase's external-service-client pattern (see
// internal/turnstile): a constructor takes config plus an optional circuit
// breaker, every outbound call runs AllowRequest -> Do ->
// RecordSuccess/RecordFailure, and the breaker is nil-safe.
//
// There is no Google SDK dependency: an ID token is an ordinary RS256 JWT,
// and golang-jwt/jwt/v5 (already a dependency for this server's own session
// tokens) verifies it once this package resolves the signing key from
// Google's JWKS by "kid".
package googleid

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/sync/singleflight"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

// defaultJWKSURL is Google's published JWKS for ID token verification. See
// https://developers.google.com/identity/openid-connect/openid-connect#discovery.
const defaultJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"

// defaultJWKSCacheTTL is the cache lifetime used when Google's response
// carries no Cache-Control max-age, or one that fails to parse.
const defaultJWKSCacheTTL = time.Hour

// clockSkew is the leeway given to exp/iat comparisons, absorbing ordinary
// clock drift between this server and Google's token issuance.
const clockSkew = 60 * time.Second

// ErrInvalidToken is returned when credential is malformed, its signature
// does not verify, or one of its claims (aud, iss, exp, iat, email_verified)
// fails the check described on Verify. It is safe to answer a generic
// validation error to the caller; the wrapped detail is for logs only.
var ErrInvalidToken = errors.New("googleid: invalid token")

// ErrUnavailable is returned when Google's JWKS endpoint could not be
// reached, answered with something other than a well-formed 2xx JSON body,
// or the circuit breaker is open. It says nothing about the token itself.
var ErrUnavailable = errors.New("googleid: verification service unavailable")

// googleIssuers is the set of "iss" values Google ID tokens carry — with and
// without the scheme, per Google's own documentation.
var googleIssuers = map[string]bool{
	"https://accounts.google.com": true,
	"accounts.google.com":         true,
}

// Claims is what a verified Google ID token carries, the subset this
// codebase uses.
type Claims struct {
	// Subject is Google's stable, unique id for the account ("sub").
	Subject string
	Email   string
	// GivenName and FamilyName are Google's split name claims. Either or
	// both may be empty even on a verified token; see Name.
	GivenName  string
	FamilyName string
	// Name is the account's full display name, present even when
	// GivenName/FamilyName are not.
	Name    string
	Picture string
}

// Config configures a Verifier.
type Config struct {
	// ClientID is this application's OAuth client id. Every token must carry
	// it in "aud". Empty disables verification entirely — see Enabled.
	ClientID string
	// JWKSURL overrides defaultJWKSURL. Tests point it at a local server;
	// production leaves it empty.
	JWKSURL string
	CB      *circuitbreaker.CircuitBreaker
}

// Verifier verifies Google Identity Services ID tokens.
type Verifier struct {
	clientID   string
	jwksURL    string
	httpClient *http.Client
	cb         *circuitbreaker.CircuitBreaker

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
	sf        singleflight.Group
}

// NewVerifier returns a Verifier.
func NewVerifier(cfg Config) *Verifier {
	jwksURL := cfg.JWKSURL
	if jwksURL == "" {
		jwksURL = defaultJWKSURL
	}
	return &Verifier{
		clientID:   cfg.ClientID,
		jwksURL:    jwksURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		cb:         cfg.CB,
	}
}

// Enabled reports whether a client id is configured. A deployment that never
// sets GOOGLE_OAUTH_CLIENT_ID is not forced to offer Google sign-in —
// callers check this before calling Verify.
func (v *Verifier) Enabled() bool {
	return v.clientID != ""
}

// jwksDoc is Google's JWKS response shape.
type jwksDoc struct {
	Keys []jwkKey `json:"keys"`
}

// jwkKey is one RSA public key entry.
type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// fetchKeys fetches and parses the JWKS, caching every key it carries for the
// duration Google's own Cache-Control header names.
func (v *Verifier) fetchKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	if err := v.cb.AllowRequest(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		v.cb.RecordFailure()
		return nil, fmt.Errorf("%w: building request: %w", ErrUnavailable, err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil { // caller gave up; not Google's fault
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, ctx.Err())
		}
		v.cb.RecordFailure()
		return nil, fmt.Errorf("%w: request failed: %w", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		v.cb.RecordFailure()
		return nil, fmt.Errorf("%w: jwks endpoint returned status %d", ErrUnavailable, resp.StatusCode)
	}

	var doc jwksDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		v.cb.RecordFailure()
		return nil, fmt.Errorf("%w: decoding response: %w", ErrUnavailable, err)
	}

	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		pub, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue // malformed entry; skip rather than fail the whole set
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		v.cb.RecordFailure()
		return nil, fmt.Errorf("%w: jwks response carried no usable RSA keys", ErrUnavailable)
	}

	v.cb.RecordSuccess()
	v.cacheKeys(keys, cacheTTL(resp.Header.Get("Cache-Control")))
	return keys, nil
}

// parseRSAPublicKey builds an *rsa.PublicKey from a JWK's base64url-encoded
// modulus (n) and exponent (e), per RFC 7518 §6.3.1.
func parseRSAPublicKey(nb64, eb64 string) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(nb64)
	if err != nil {
		return nil, fmt.Errorf("decoding modulus: %w", err)
	}
	eb, err := base64.RawURLEncoding.DecodeString(eb64)
	if err != nil {
		return nil, fmt.Errorf("decoding exponent: %w", err)
	}
	if len(nb) == 0 || len(eb) == 0 {
		return nil, errors.New("empty modulus or exponent")
	}

	e := 0
	for _, b := range eb {
		e = e<<8 + int(b)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e}, nil
}

// cacheTTL parses a Cache-Control header's max-age directive, falling back to
// defaultJWKSCacheTTL when absent, zero, negative or unparsable.
func cacheTTL(cacheControl string) time.Duration {
	for _, part := range strings.Split(cacheControl, ",") {
		part = strings.TrimSpace(part)
		value, ok := strings.CutPrefix(part, "max-age=")
		if !ok {
			continue
		}
		secs, err := strconv.Atoi(value)
		if err != nil || secs <= 0 {
			continue
		}
		return time.Duration(secs) * time.Second
	}
	return defaultJWKSCacheTTL
}

func (v *Verifier) cacheKeys(keys map[string]*rsa.PublicKey, ttl time.Duration) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.keys = keys
	v.expiresAt = time.Now().Add(ttl)
}

// keyForKID returns kid's key, using a cached hit even when stale (its
// refresh outcome is ignored); an unknown kid refreshes once, single-flighted,
// and a failed refresh is reported as the fetch error.
func (v *Verifier) keyForKID(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	key, ok := v.keys[kid]
	fresh := ok && time.Now().Before(v.expiresAt)
	v.mu.Unlock()
	if ok {
		if !fresh {
			_, _ = v.refreshKeys(ctx)
		}
		return key, nil
	}
	// An unknown kid with the JWKS unreachable is unverifiable, not invalid:
	// a key rotation that coincides with an outage must surface as
	// ErrUnavailable (retry later, breaker counts it), never as a rejected
	// token, whether or not older keys are cached.
	keys, err := v.refreshKeys(ctx)
	if err != nil {
		return nil, err
	}
	if k, found := keys[kid]; found {
		return k, nil
	}
	return nil, fmt.Errorf("%w: unknown key id %q", ErrInvalidToken, kid)
}

// refreshKeys single-flights a JWKS fetch across concurrent callers.
func (v *Verifier) refreshKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	res, err, _ := v.sf.Do("jwks", func() (any, error) { return v.fetchKeys(ctx) })
	if err != nil {
		return nil, err
	}
	return res.(map[string]*rsa.PublicKey), nil
}

// googleClaims is the wire shape of a Google ID token. email_verified
// arrives as a JSON bool on every token Google issues today, but the OpenID
// Connect claim is documented as string-or-bool depending on flow, so it is
// decoded as `any` and normalized by truthy.
type googleClaims struct {
	jwt.RegisteredClaims
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// truthy normalizes email_verified's bool-or-string wire representation.
func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true"
	default:
		return false
	}
}

// Verify checks credential — a Google Identity Services ID token — end to
// end: its RS256 signature against Google's JWKS (resolved by "kid", cached
// and refetched on an unknown one), "iss" against the two forms Google
// issues, "aud" against the configured client id, "exp"/"iat" with 60s skew,
// and that "email_verified" is true. It returns ErrUnavailable when the JWKS
// could not be fetched (the token itself may well be genuine — retry later),
// and ErrInvalidToken for every other rejection.
func (v *Verifier) Verify(ctx context.Context, credential string) (*Claims, error) {
	var claims googleClaims
	token, err := jwt.ParseWithClaims(credential, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", ErrInvalidToken, t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("%w: token carries no kid", ErrInvalidToken)
		}
		return v.keyForKID(ctx, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithAudience(v.clientID),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(clockSkew),
	)
	if err != nil {
		if errors.Is(err, ErrUnavailable) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	if !googleIssuers[claims.Issuer] {
		return nil, fmt.Errorf("%w: unexpected issuer %q", ErrInvalidToken, claims.Issuer)
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: missing subject", ErrInvalidToken)
	}
	if !truthy(claims.EmailVerified) {
		return nil, fmt.Errorf("%w: email not verified", ErrInvalidToken)
	}

	return &Claims{
		Subject:    claims.Subject,
		Email:      claims.Email,
		GivenName:  claims.GivenName,
		FamilyName: claims.FamilyName,
		Name:       claims.Name,
		Picture:    claims.Picture,
	}, nil
}
