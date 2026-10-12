package social

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GitHubIssuer is the issuer of GitHub Actions OIDC tokens.
const GitHubIssuer = "https://token.actions.githubusercontent.com"

// GitHubJWKSURL is where GitHub publishes the keys that sign those tokens.
const GitHubJWKSURL = GitHubIssuer + "/.well-known/jwks"

// SyncAudience is the audience the content sync requests its OIDC token for.
const SyncAudience = "https://api.vibe.com.ar"

// SyncRef is the only ref the content sync may run from.
const SyncRef = "refs/heads/main"

// ErrUnauthenticated means a bearer token is not an accepted GitHub Actions
// token: wrong signature, algorithm, issuer, audience or lifetime, or none.
var ErrUnauthenticated = errors.New("social: token not accepted")

// ErrForbidden means the token is a valid GitHub Actions token but was issued
// to another repository or ref than the one allowed to sync posts.
var ErrForbidden = errors.New("social: token is not from the sync workflow")

var errUnknownKey = errors.New("social: no signing key with that kid")

const (
	keyCacheTTL   = time.Hour
	keyRefetchGap = 5 * time.Minute
	jwksMaxBytes  = 1 << 20
	clockLeeway   = time.Minute
)

// Claims is the part of a GitHub Actions OIDC token this service reads.
type Claims struct {
	jwt.RegisteredClaims
	Repository string `json:"repository"`
	Ref        string `json:"ref"`
}

// VerifierConfig is what a token must match. Now and HTTPClient default to the
// wall clock and a client with a ten second timeout.
type VerifierConfig struct {
	Issuer     string
	Audience   string
	Repository string
	Ref        string
	JWKSURL    string
	HTTPClient *http.Client
	Now        func() time.Time
}

// Verifier checks GitHub Actions OIDC tokens against the issuer's published
// keys. It caches the keys for an hour and refetches at most once per five
// minutes for an unknown key ID, so a flood of forged tokens cannot turn into
// a flood of key requests.
type Verifier struct {
	cfg    VerifierConfig
	parser *jwt.Parser

	mu          sync.Mutex
	keys        map[string]*rsa.PublicKey
	fetchedAt   time.Time
	attemptedAt time.Time
}

// NewVerifier returns a Verifier for cfg.
func NewVerifier(cfg VerifierConfig) *Verifier {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Verifier{
		cfg: cfg,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
			jwt.WithIssuer(cfg.Issuer),
			jwt.WithAudience(cfg.Audience),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
			jwt.WithLeeway(clockLeeway),
			jwt.WithTimeFunc(cfg.Now),
		),
	}
}

// Verify accepts raw only when it is a valid RS256 token from the configured
// issuer and audience, unexpired, and issued for the configured repository and
// ref. It returns ErrUnauthenticated for a bad token and ErrForbidden for a
// good token from another workflow.
func (v *Verifier) Verify(ctx context.Context, raw string) error {
	var claims Claims
	_, err := v.parser.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return v.key(ctx, kid)
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnauthenticated, err)
	}
	if claims.Repository != v.cfg.Repository || claims.Ref != v.cfg.Ref {
		return ErrForbidden
	}
	return nil
}

// key returns the signing key for kid, fetching the set when the cache is
// stale or the kid is unknown and the last attempt is old enough.
func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := v.cfg.Now()
	stale := v.keys == nil || now.Sub(v.fetchedAt) > keyCacheTTL
	unknown := v.keys[kid] == nil && now.Sub(v.attemptedAt) > keyRefetchGap
	if stale || unknown {
		v.attemptedAt = now
		keys, err := fetchKeys(ctx, v.cfg.HTTPClient, v.cfg.JWKSURL)
		switch {
		case err == nil:
			v.keys = keys
			v.fetchedAt = now
		case v.keys == nil:
			return nil, fmt.Errorf("social: fetch signing keys: %w", err)
		}
	}

	key, ok := v.keys[kid]
	if !ok {
		return nil, errUnknownKey
	}
	return key, nil
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func fetchKeys(ctx context.Context, client *http.Client, url string) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build jwks request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get jwks: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks answered %d", resp.StatusCode)
	}

	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, jwksMaxBytes)).Decode(&set); err != nil {
		return nil, fmt.Errorf("decode jwks: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if pub, ok := k.publicKey(); ok {
			keys[k.Kid] = pub
		}
	}
	if len(keys) == 0 {
		return nil, errors.New("jwks holds no RSA signing key")
	}
	return keys, nil
}

func (k jwk) publicKey() (*rsa.PublicKey, bool) {
	if k.Kty != "RSA" || k.Kid == "" || (k.Use != "" && k.Use != "sig") {
		return nil, false
	}
	n, errN := base64.RawURLEncoding.DecodeString(k.N)
	e, errE := base64.RawURLEncoding.DecodeString(k.E)
	if errN != nil || errE != nil || len(n) == 0 || len(e) == 0 {
		return nil, false
	}
	exp := new(big.Int).SetBytes(e)
	if !exp.IsInt64() {
		return nil, false
	}
	//nolint:gosec // G115: exp.IsInt64 holds, and an RSA public exponent is a small prime, so the narrowing is exact.
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(exp.Int64())}, true
}
