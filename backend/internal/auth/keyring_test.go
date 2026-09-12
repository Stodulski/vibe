package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	activeSecret   = "active-secret-that-is-32-bytes!!"
	previousSecret = "previous-secret-32-bytes-long!!!"
)

// signedUnder mints an access token from a service configured with secret as
// its only key, which is what the deployment looked like before the rotation.
func signedUnder(t *testing.T, secret, keyID string) string {
	t.Helper()
	svc := NewTokenService(TokenServiceConfig{JWTSecret: secret, JWTKeyID: keyID, Environment: "test"})
	token, err := svc.GenerateAccessToken(uuid.New(), "owner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return token
}

func TestKeyringVerifiesTokensSignedWithThePreviousKey(t *testing.T) {
	old := signedUnder(t, previousSecret, "")

	rotated := NewTokenService(TokenServiceConfig{
		JWTSecret:         activeSecret,
		JWTSecretPrevious: previousSecret,
		Environment:       "test",
	})

	if _, err := rotated.ValidateAccessToken(old); err != nil {
		t.Fatalf("a token signed with the previous key should still verify: %v", err)
	}

	fresh, err := rotated.GenerateAccessToken(uuid.New(), "owner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := rotated.ValidateAccessToken(fresh); err != nil {
		t.Fatalf("a freshly minted token should verify: %v", err)
	}
	if rotated.ActiveKeyID() != keyID("", activeSecret) {
		t.Errorf("want the active key id derived from JWT_SECRET; got %q", rotated.ActiveKeyID())
	}
}

func TestKeyringRefusesAnUnknownKeyID(t *testing.T) {
	stranger := signedUnder(t, "a-third-secret-32-bytes-long!!!!", "")

	rotated := NewTokenService(TokenServiceConfig{
		JWTSecret:         activeSecret,
		JWTSecretPrevious: previousSecret,
		Environment:       "test",
	})

	if _, err := rotated.ValidateAccessToken(stranger); err == nil {
		t.Fatal("expected a token naming a key outside the ring to be refused")
	}
}

func TestKeyringRefusesATokenWithNoKeyID(t *testing.T) {
	svc := NewTokenService(TokenServiceConfig{JWTSecret: activeSecret, Environment: "test"})

	// The same claims the service mints, signed with the same secret, but with
	// no kid header: the shape of a token minted before rotation existed.
	access, err := svc.GenerateAccessToken(uuid.New(), "owner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	claims, err := svc.ValidateAccessToken(access)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	unnamed := signWith(t, jwtSigningMethod(), []byte(activeSecret), *claims)

	if _, err := svc.ValidateAccessToken(unnamed); err == nil {
		t.Fatal("expected a token with no kid header to be refused")
	}
}

func TestKeyringDoesNotRegisterADuplicatePreviousKey(t *testing.T) {
	same := newJWTKeyring(TokenServiceConfig{JWTSecret: activeSecret, JWTSecretPrevious: activeSecret})
	if len(same.byID) != 1 {
		t.Errorf("want one key when both secrets are equal; got %d", len(same.byID))
	}

	collide := newJWTKeyring(TokenServiceConfig{
		JWTSecret:         activeSecret,
		JWTKeyID:          "k1",
		JWTSecretPrevious: previousSecret,
		JWTKeyIDPrevious:  "k1",
	})
	if len(collide.byID) != 1 {
		t.Errorf("want one key when both ids are equal; got %d", len(collide.byID))
	}
	if got, _ := collide.lookup("k1"); string(got) != activeSecret {
		t.Error("the active key must win an id collision")
	}
}

func TestCSRFTokenSurvivesARotation(t *testing.T) {
	before := NewTokenService(TokenServiceConfig{JWTSecret: previousSecret, Environment: "test"})
	access := signedUnder(t, previousSecret, "")
	csrf := before.GenerateCSRFToken(access)

	rotated := NewTokenService(TokenServiceConfig{
		JWTSecret:         activeSecret,
		JWTSecretPrevious: previousSecret,
		Environment:       "test",
	})

	if !rotated.ValidateCSRFToken(access, csrf) {
		t.Error("a CSRF token derived under the previous key should still validate")
	}
	if rotated.ValidateCSRFToken(access, "not-the-token") {
		t.Error("a wrong CSRF token must still be refused")
	}

	stranger := NewTokenService(TokenServiceConfig{JWTSecret: "a-third-secret-32-bytes-long!!!!", Environment: "test"})
	if stranger.ValidateCSRFToken(access, csrf) {
		t.Error("a CSRF token from a key outside the ring must be refused")
	}
}

// jwtSigningMethod is HS256, named through a function so the test reads the
// same allowlist the verifier enforces rather than a second copy of it.
func jwtSigningMethod() jwt.SigningMethod { return jwt.SigningMethodHS256 }
