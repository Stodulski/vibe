package googleid

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

const testClientID = "test-client-id.apps.googleusercontent.com"
const testKid = "test-kid-1"

// testKeyPair generates one RSA key for signing test tokens and serving from
// a fake JWKS endpoint.
func testKeyPair(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	return key
}

// b64url encodes a big-endian byte slice as base64url, no padding — the JWK
// wire format for "n" and "e".
func b64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// jwksBody renders one RSA key as a JWKS document. An empty kid produces a
// key with no usable "kid", exercising the skip-malformed-entries path.
func jwksBody(key *rsa.PrivateKey, kid string) string {
	n := b64url(key.N.Bytes())
	eBytes := []byte{byte(key.E >> 16), byte(key.E >> 8), byte(key.E)}
	e := b64url(eBytes)
	return fmt.Sprintf(`{"keys":[{"kty":"RSA","kid":%q,"n":%q,"e":%q,"alg":"RS256","use":"sig"}]}`, kid, n, e)
}

// jwksServer serves body for every request, counting how many it received.
type jwksServer struct {
	*httptest.Server
	hits int
}

func newJWKSServer(t *testing.T, body string) *jwksServer {
	t.Helper()
	s := &jwksServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

// signedToken builds a signed Google-shaped ID token, letting the caller
// mutate the claims before signing.
func signedToken(t *testing.T, key *rsa.PrivateKey, kid string, mutate func(*googleClaims)) string {
	t.Helper()
	now := time.Now()
	claims := googleClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://accounts.google.com",
			Subject:   "10769150350006150715113082367",
			Audience:  jwt.ClaimStrings{testClientID},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
		Email:         "ana@example.com",
		EmailVerified: true,
		GivenName:     "Ana",
		FamilyName:    "Perez",
		Name:          "Ana Perez",
		Picture:       "https://example.com/photo.jpg",
	}
	if mutate != nil {
		mutate(&claims)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("signing test token: %v", err)
	}
	return signed
}

func newVerifier(t *testing.T, jwksURL string) *Verifier {
	t.Helper()
	cb := circuitbreaker.New(circuitbreaker.Config{Name: "googleid-test", MaxFailures: 1, ResetTimeout: time.Hour})
	return NewVerifier(Config{ClientID: testClientID, JWKSURL: jwksURL, CB: cb})
}

func TestVerifySuccess(t *testing.T) {
	key := testKeyPair(t)
	srv := newJWKSServer(t, jwksBody(key, testKid))
	v := newVerifier(t, srv.URL)

	token := signedToken(t, key, testKid, nil)

	claims, err := v.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("Verify() = %v, want nil", err)
	}
	if claims.Email != "ana@example.com" {
		t.Errorf("Email = %q, want ana@example.com", claims.Email)
	}
	if claims.Subject != "10769150350006150715113082367" {
		t.Errorf("Subject = %q, want the token's sub", claims.Subject)
	}
	if claims.GivenName != "Ana" || claims.FamilyName != "Perez" {
		t.Errorf("GivenName/FamilyName = %q/%q, want Ana/Perez", claims.GivenName, claims.FamilyName)
	}
}

func TestVerifyWrongAudience(t *testing.T) {
	key := testKeyPair(t)
	srv := newJWKSServer(t, jwksBody(key, testKid))
	v := newVerifier(t, srv.URL)

	token := signedToken(t, key, testKid, func(c *googleClaims) {
		c.Audience = jwt.ClaimStrings{"someone-elses-client-id"}
	})

	_, err := v.Verify(context.Background(), token)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Verify() = %v, want an error wrapping ErrInvalidToken", err)
	}
}

func TestVerifyWrongIssuer(t *testing.T) {
	key := testKeyPair(t)
	srv := newJWKSServer(t, jwksBody(key, testKid))
	v := newVerifier(t, srv.URL)

	token := signedToken(t, key, testKid, func(c *googleClaims) {
		c.Issuer = "https://evil.example.com"
	})

	_, err := v.Verify(context.Background(), token)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Verify() = %v, want an error wrapping ErrInvalidToken", err)
	}
}

// TestVerifyIssuerWithoutScheme covers the second form Google documents:
// "accounts.google.com" with no "https://" prefix.
func TestVerifyIssuerWithoutScheme(t *testing.T) {
	key := testKeyPair(t)
	srv := newJWKSServer(t, jwksBody(key, testKid))
	v := newVerifier(t, srv.URL)

	token := signedToken(t, key, testKid, func(c *googleClaims) {
		c.Issuer = "accounts.google.com"
	})

	if _, err := v.Verify(context.Background(), token); err != nil {
		t.Fatalf("Verify() = %v, want nil", err)
	}
}

func TestVerifyExpiredToken(t *testing.T) {
	key := testKeyPair(t)
	srv := newJWKSServer(t, jwksBody(key, testKid))
	v := newVerifier(t, srv.URL)

	token := signedToken(t, key, testKid, func(c *googleClaims) {
		past := time.Now().Add(-time.Hour)
		c.IssuedAt = jwt.NewNumericDate(past.Add(-time.Minute))
		c.ExpiresAt = jwt.NewNumericDate(past)
	})

	_, err := v.Verify(context.Background(), token)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Verify() = %v, want an error wrapping ErrInvalidToken", err)
	}
}

func TestVerifyEmailNotVerified(t *testing.T) {
	key := testKeyPair(t)
	srv := newJWKSServer(t, jwksBody(key, testKid))
	v := newVerifier(t, srv.URL)

	token := signedToken(t, key, testKid, func(c *googleClaims) {
		c.EmailVerified = false
	})

	_, err := v.Verify(context.Background(), token)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Verify() = %v, want an error wrapping ErrInvalidToken", err)
	}
}

// TestVerifyUnknownKidRefetches covers key rotation: the first attempt caches
// an old key set, the token carries a kid minted after that, and Verify must
// fetch again rather than failing on the stale cache.
func TestVerifyUnknownKidRefetches(t *testing.T) {
	oldKey := testKeyPair(t)
	newKey := testKeyPair(t)

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_, _ = w.Write([]byte(jwksBody(oldKey, "old-kid")))
			return
		}
		_, _ = w.Write([]byte(jwksBody(newKey, "new-kid")))
	}))
	t.Cleanup(srv.Close)

	v := newVerifier(t, srv.URL)

	// Prime the cache with the old key set.
	if _, err := v.keyForKID(context.Background(), "old-kid"); err != nil {
		t.Fatalf("priming cache: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d after priming, want 1", calls)
	}

	token := signedToken(t, newKey, "new-kid", nil)

	claims, err := v.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("Verify() = %v, want nil", err)
	}
	if claims.Email == "" {
		t.Error("claims were not populated")
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2 (one refetch for the unknown kid)", calls)
	}
}

func TestVerifyJWKS500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	key := testKeyPair(t)
	v := newVerifier(t, srv.URL)
	token := signedToken(t, key, testKid, nil)

	_, err := v.Verify(context.Background(), token)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Verify() = %v, want an error wrapping ErrUnavailable", err)
	}
}

func TestKeyForKIDStaleCacheSurvivesOutage(t *testing.T) {
	key := testKeyPair(t)
	okSrv := newJWKSServer(t, jwksBody(key, testKid))
	v := newVerifier(t, okSrv.URL)
	want, err := v.keyForKID(context.Background(), testKid)
	if err != nil {
		t.Fatalf("priming cache: %v", err)
	}
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }))
	t.Cleanup(down.Close)
	v.jwksURL, v.expiresAt = down.URL, time.Now().Add(-time.Minute)
	if got, err := v.keyForKID(context.Background(), testKid); err != nil || got != want {
		t.Errorf("stale hit: got (%v, %v), want (%v, nil)", got, err, want)
	}
	// A kid this process has never seen, with the JWKS down, is unverifiable
	// rather than invalid: a key rotation during an outage must not reject
	// the token, cache or no cache.
	if _, err := v.keyForKID(context.Background(), "other-kid"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("unknown kid with cache during outage: err = %v, want ErrUnavailable", err)
	}
}

func TestEnabled(t *testing.T) {
	if (&Verifier{}).Enabled() {
		t.Error("a Verifier with no client id must report disabled")
	}
	v := NewVerifier(Config{ClientID: testClientID})
	if !v.Enabled() {
		t.Error("a Verifier with a client id must report enabled")
	}
}
