package social

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testKID         = "kid-1"
	testAudience    = "https://api.vibe.com.ar"
	testRepository  = "Stodulski/vibe-social"
	testRef         = "refs/heads/main"
	testIssuerValue = GitHubIssuer
)

type fakeIDP struct {
	key     *rsa.PrivateKey
	server  *httptest.Server
	fetches atomic.Int32
}

func newFakeIDP(t *testing.T) *fakeIDP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	idp := &fakeIDP{key: key}
	idp.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		idp.fetches.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": testKID, "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(idp.server.Close)
	return idp
}

func (idp *fakeIDP) verifier(t *testing.T) *Verifier {
	t.Helper()
	return NewVerifier(VerifierConfig{
		Issuer:     testIssuerValue,
		Audience:   testAudience,
		Repository: testRepository,
		Ref:        testRef,
		JWKSURL:    idp.server.URL,
		HTTPClient: idp.server.Client(),
		Now:        time.Now,
	})
}

type tokenOpts struct {
	iss, aud, repo, ref string
	exp                 time.Time
	key                 *rsa.PrivateKey
	kid                 string
}

func (idp *fakeIDP) sign(t *testing.T, o tokenOpts) string {
	t.Helper()
	if o.iss == "" {
		o.iss = testIssuerValue
	}
	if o.aud == "" {
		o.aud = testAudience
	}
	if o.repo == "" {
		o.repo = testRepository
	}
	if o.ref == "" {
		o.ref = testRef
	}
	if o.exp.IsZero() {
		o.exp = time.Now().Add(5 * time.Minute)
	}
	if o.key == nil {
		o.key = idp.key
	}
	if o.kid == "" {
		o.kid = testKID
	}
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    o.iss,
			Subject:   "repo:Stodulski/vibe-social:ref:refs/heads/main",
			Audience:  jwt.ClaimStrings{o.aud},
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(o.exp),
		},
		Repository: o.repo,
		Ref:        o.ref,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = o.kid
	raw, err := tok.SignedString(o.key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return raw
}

func TestVerifyAcceptsTheSyncWorkflowToken(t *testing.T) {
	t.Parallel()
	idp := newFakeIDP(t)
	v := idp.verifier(t)

	if err := v.Verify(context.Background(), idp.sign(t, tokenOpts{})); err != nil {
		t.Fatalf("Verify() = %v, want nil", err)
	}
}

func TestVerifyRefusesAnotherRepositoryOrRef(t *testing.T) {
	t.Parallel()
	idp := newFakeIDP(t)
	v := idp.verifier(t)

	cases := map[string]tokenOpts{
		"other repository": {repo: "Stodulski/vibe"},
		"feature branch":   {ref: "refs/heads/feat/x"},
		"tag ref":          {ref: "refs/tags/v1"},
	}
	for name, o := range cases {
		err := v.Verify(context.Background(), idp.sign(t, o))
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("%s: Verify() = %v, want ErrForbidden", name, err)
		}
	}
}

func TestVerifyRefusesAnInvalidToken(t *testing.T) {
	t.Parallel()
	idp := newFakeIDP(t)
	v := idp.verifier(t)

	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate other key: %v", err)
	}
	hs := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": testIssuerValue, "aud": testAudience, "exp": time.Now().Add(time.Minute).Unix()})
	hs.Header["kid"] = testKID
	hsRaw, err := hs.SignedString([]byte("shared-secret"))
	if err != nil {
		t.Fatalf("sign hs256: %v", err)
	}

	cases := map[string]string{
		"wrong audience":   idp.sign(t, tokenOpts{aud: "https://evil.example"}),
		"wrong issuer":     idp.sign(t, tokenOpts{iss: "https://issuer.example"}),
		"bad signature":    idp.sign(t, tokenOpts{key: other}),
		"expired":          idp.sign(t, tokenOpts{exp: time.Now().Add(-time.Hour)}),
		"hs256 algorithm":  hsRaw,
		"not a jwt at all": "abc.def",
		"empty":            "",
	}
	for name, raw := range cases {
		err := v.Verify(context.Background(), raw)
		if !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("%s: Verify() = %v, want ErrUnauthenticated", name, err)
		}
	}
}

func TestVerifyCachesKeysAndRefetchesOnUnknownKID(t *testing.T) {
	t.Parallel()
	idp := newFakeIDP(t)
	v := idp.verifier(t)
	ctx := context.Background()

	for range 3 {
		if err := v.Verify(ctx, idp.sign(t, tokenOpts{})); err != nil {
			t.Fatalf("Verify() = %v", err)
		}
	}
	if got := idp.fetches.Load(); got != 1 {
		t.Fatalf("JWKS fetches = %d after three valid tokens, want 1 (cached)", got)
	}

	// An unknown kid triggers one refetch, but not one per request: a flood of
	// tokens naming a missing key must not turn into a flood of JWKS calls.
	for range 5 {
		_ = v.Verify(ctx, idp.sign(t, tokenOpts{kid: "unknown"}))
	}
	if got := idp.fetches.Load(); got > 2 {
		t.Fatalf("JWKS fetches = %d after unknown-kid flood, want at most 2", got)
	}
}
