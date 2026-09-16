package auth

import (
	"bytes"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestGenerateAccessToken(t *testing.T) {
	svc := NewTokenService(TokenServiceConfig{JWTSecret: testJWTSecret, Environment: "test"})
	userID := uuid.New()

	t.Run("valid token generation", func(t *testing.T) {
		token, err := svc.GenerateAccessToken(userID, "owner")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token == "" {
			t.Fatal("expected non-empty token")
		}
	})

	t.Run("correct claims", func(t *testing.T) {
		token, err := svc.GenerateAccessToken(userID, "owner")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		claims, err := svc.ValidateAccessToken(token)
		if err != nil {
			t.Fatalf("unexpected error validating token: %v", err)
		}

		if claims.Subject != userID.String() {
			t.Errorf("want subject %q; got %q", userID.String(), claims.Subject)
		}
		if claims.Role != "owner" {
			t.Errorf("want role %q; got %q", "owner", claims.Role)
		}
		if claims.Issuer != "vibe" {
			t.Errorf("want issuer %q; got %q", "vibe", claims.Issuer)
		}

		// Expiry should be approximately 15 minutes from now.
		expiry := claims.ExpiresAt.Time
		expected := time.Now().Add(15 * time.Minute)
		diff := expiry.Sub(expected)
		if diff < -5*time.Second || diff > 5*time.Second {
			t.Errorf("want expiry ~15min from now; got diff %v", diff)
		}
	})
}

func TestValidateAccessToken(t *testing.T) {
	svc := NewTokenService(TokenServiceConfig{JWTSecret: testJWTSecret, Environment: "test"})
	userID := uuid.New()

	t.Run("valid token", func(t *testing.T) {
		token, err := svc.GenerateAccessToken(userID, "owner")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		claims, err := svc.ValidateAccessToken(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if claims.Subject != userID.String() {
			t.Errorf("want subject %q; got %q", userID.String(), claims.Subject)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		claims := Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   userID.String(),
				Issuer:    "vibe",
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-30 * time.Minute)),
			},
			Role: "owner",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString([]byte(testJWTSecret))
		if err != nil {
			t.Fatalf("unexpected error signing: %v", err)
		}

		_, err = svc.ValidateAccessToken(signed)
		if err == nil {
			t.Fatal("expected error for expired token")
		}
	})

	t.Run("wrong signing method", func(t *testing.T) {
		claims := Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   userID.String(),
				Issuer:    "vibe",
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			},
			Role: "owner",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
		signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
		if err != nil {
			t.Fatalf("unexpected error signing: %v", err)
		}

		_, err = svc.ValidateAccessToken(signed)
		if err == nil {
			t.Fatal("expected error for wrong signing method")
		}
	})

	t.Run("tampered token", func(t *testing.T) {
		token, err := svc.GenerateAccessToken(userID, "owner")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Tamper with the payload section of the token (between the two dots).
		parts := strings.SplitN(token, ".", 3)
		if len(parts) != 3 {
			t.Fatal("expected JWT with 3 parts")
		}
		// Flip a character in the middle of the payload.
		payload := []byte(parts[1])
		payload[len(payload)/2] ^= 0xFF
		tampered := parts[0] + "." + string(payload) + "." + parts[2]
		_, err = svc.ValidateAccessToken(tampered)
		if err == nil {
			t.Fatal("expected error for tampered token")
		}
	})
}

func TestGenerateRefreshToken(t *testing.T) {
	t.Run("returns non-empty plaintext and hash", func(t *testing.T) {
		plaintext, hash := generateRefreshToken()
		if plaintext == "" {
			t.Fatal("expected non-empty plaintext")
		}
		if len(hash) == 0 {
			t.Fatal("expected non-empty hash")
		}
	})

	t.Run("hash is SHA256 of plaintext", func(t *testing.T) {
		plaintext, hash := generateRefreshToken()

		expected := sha256.Sum256([]byte(plaintext))
		if !bytes.Equal(hash, expected[:]) {
			t.Error("hash does not match SHA256 of plaintext")
		}
	})
}

func TestHashRefreshToken(t *testing.T) {
	t.Run("consistent hashing", func(t *testing.T) {
		plaintext := "test-refresh-token-value"
		hash1 := hashToken(plaintext)
		hash2 := hashToken(plaintext)
		if !bytes.Equal(hash1, hash2) {
			t.Error("hash is not consistent for the same input")
		}
	})

	t.Run("matches generateRefreshToken hash", func(t *testing.T) {
		plaintext, hash := generateRefreshToken()

		computed := hashToken(plaintext)
		if !bytes.Equal(hash, computed) {
			t.Error("hashToken does not match hash from generateRefreshToken")
		}
	})
}

func TestCSRFToken(t *testing.T) {
	svc := NewTokenService(TokenServiceConfig{JWTSecret: testJWTSecret, Environment: "test"})
	userID := uuid.New()

	accessToken, err := svc.GenerateAccessToken(userID, "owner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("generate and validate round-trip", func(t *testing.T) {
		csrf := svc.GenerateCSRFToken(accessToken)
		if csrf == "" {
			t.Fatal("expected non-empty CSRF token")
		}
		if !svc.ValidateCSRFToken(accessToken, csrf) {
			t.Error("CSRF token should be valid")
		}
	})

	t.Run("wrong CSRF token fails", func(t *testing.T) {
		if svc.ValidateCSRFToken(accessToken, "wrong-csrf-token") {
			t.Error("expected validation to fail for wrong CSRF token")
		}
	})

	t.Run("different access token fails", func(t *testing.T) {
		csrf := svc.GenerateCSRFToken(accessToken)

		otherToken, err := svc.GenerateAccessToken(uuid.New(), "owner")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if svc.ValidateCSRFToken(otherToken, csrf) {
			t.Error("expected validation to fail for different access token")
		}
	})
}

func TestSetTokenCookies(t *testing.T) {
	svc := NewTokenService(TokenServiceConfig{JWTSecret: testJWTSecret, Environment: "test"})
	userID := uuid.New()

	accessToken, err := svc.GenerateAccessToken(userID, "owner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	refreshToken := "test-refresh-token"

	w := httptest.NewRecorder()
	csrf := svc.SetTokenCookies(w, accessToken, refreshToken)

	t.Run("returns CSRF token", func(t *testing.T) {
		if csrf == "" {
			t.Fatal("expected non-empty CSRF token")
		}
		if !svc.ValidateCSRFToken(accessToken, csrf) {
			t.Error("returned CSRF token should be valid")
		}
	})

	t.Run("sets access_token cookie", func(t *testing.T) {
		cookie := findCookie(w.Header(), "access_token")
		if cookie == nil {
			t.Fatal("access_token cookie not set")
		}
		if cookie.Value != accessToken {
			t.Errorf("want access_token value %q; got %q", accessToken, cookie.Value)
		}
		if !cookie.HttpOnly {
			t.Error("access_token cookie should be HttpOnly")
		}
	})

	t.Run("sets refresh_token cookie", func(t *testing.T) {
		cookie := findCookie(w.Header(), "refresh_token")
		if cookie == nil {
			t.Fatal("refresh_token cookie not set")
		}
		if cookie.Value != refreshToken {
			t.Errorf("want refresh_token value %q; got %q", refreshToken, cookie.Value)
		}
		if !cookie.HttpOnly {
			t.Error("refresh_token cookie should be HttpOnly")
		}
	})
}

func TestClearTokenCookies(t *testing.T) {
	svc := NewTokenService(TokenServiceConfig{JWTSecret: testJWTSecret, Environment: "test"})

	w := httptest.NewRecorder()
	svc.ClearTokenCookies(w)

	t.Run("access_token cookie has MaxAge -1", func(t *testing.T) {
		cookie := findCookie(w.Header(), "access_token")
		if cookie == nil {
			t.Fatal("access_token cookie not set")
		}
		if cookie.MaxAge != -1 {
			t.Errorf("want MaxAge -1; got %d", cookie.MaxAge)
		}
		if cookie.Value != "" {
			t.Errorf("want empty value; got %q", cookie.Value)
		}
	})

	t.Run("refresh_token cookie has MaxAge -1", func(t *testing.T) {
		cookie := findCookie(w.Header(), "refresh_token")
		if cookie == nil {
			t.Fatal("refresh_token cookie not set")
		}
		if cookie.MaxAge != -1 {
			t.Errorf("want MaxAge -1; got %d", cookie.MaxAge)
		}
		if cookie.Value != "" {
			t.Errorf("want empty value; got %q", cookie.Value)
		}
	})
}

// testJWTSecret is a signing key for tests only; it is deliberately long
// enough to satisfy the production minimum so the tests exercise the real path.
const testJWTSecret = "test-secret-key-for-testing-only-32b"

// findCookie returns the named Set-Cookie from a response header, or nil.
func findCookie(header http.Header, name string) *http.Cookie {
	for _, line := range header.Values("Set-Cookie") {
		if c, err := http.ParseSetCookie(line); err == nil && c.Name == name {
			return c
		}
	}
	return nil
}

// The doc on ValidateAccessToken promises it verifies the issuer, and nothing
// did. Today the signing key has one use, so a foreign issuer cannot be forged
// without the key and the practical impact is nil — but the guarantee has to be
// true before the key gets a second use, not discovered missing afterwards by
// whoever adds a signed download link or a service token and reasonably assumed
// this function would reject what it was not asked to accept.
func TestValidateAccessTokenVerifiesTheIssuer(t *testing.T) {
	svc := NewTokenService(TokenServiceConfig{JWTSecret: testJWTSecret, Environment: "test"})
	userID := uuid.New()

	// Correctly signed with our own key — only the issuer is foreign. This is
	// what a second use of the same secret would produce.
	signedWithIssuer := func(t *testing.T, issuer string) string {
		t.Helper()
		claims := Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   userID.String(),
				Issuer:    issuer,
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			},
			Role: "superadmin",
		}
		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
		if err != nil {
			t.Fatalf("unexpected error signing: %v", err)
		}
		return signed
	}

	t.Run("another issuer's token is refused", func(t *testing.T) {
		if _, err := svc.ValidateAccessToken(signedWithIssuer(t, "vibe-downloads")); err == nil {
			t.Fatal("a token minted for another purpose with the same key was accepted as an access token")
		}
	})

	t.Run("a token with no issuer at all is refused", func(t *testing.T) {
		if _, err := svc.ValidateAccessToken(signedWithIssuer(t, "")); err == nil {
			t.Fatal("a token carrying no issuer was accepted")
		}
	})

	// The other half: the check must not have been tightened into rejecting the
	// tokens this service actually mints.
	t.Run("our own token still validates", func(t *testing.T) {
		token, err := svc.GenerateAccessToken(userID, "owner")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		claims, err := svc.ValidateAccessToken(token)
		if err != nil {
			t.Fatalf("a genuine access token was refused: %v", err)
		}
		if claims.Issuer != jwtIssuer {
			t.Errorf("want issuer %q; got %q", jwtIssuer, claims.Issuer)
		}
	})
}

// signWith mints a token with the given claims and signing method, bypassing
// the service's own minting so a test can produce what the service never
// would: a token signed with the wrong algorithm, or one without an expiry.
func signWith(t *testing.T, method jwt.SigningMethod, key any, claims jwt.Claims) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatalf("signing the fixture token: %v", err)
	}
	return signed
}

func TestValidateAccessTokenRefusesUnacceptableTokens(t *testing.T) {
	svc := NewTokenService(TokenServiceConfig{JWTSecret: testJWTSecret, Environment: "test"})
	now := time.Now()

	base := func() jwt.RegisteredClaims {
		return jwt.RegisteredClaims{
			Subject:   uuid.New().String(),
			Issuer:    jwtIssuer,
			Audience:  jwt.ClaimStrings{jwtAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		}
	}

	t.Run("wrong signing method", func(t *testing.T) {
		// "none" is the classic downgrade: a parser that reads the algorithm
		// out of the header verifies nothing at all.
		registered := base()
		token := signWith(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType,
			Claims{RegisteredClaims: registered, Role: "owner"})

		if _, err := svc.ValidateAccessToken(token); err == nil {
			t.Fatal("expected an unsigned token to be refused")
		}
	})

	t.Run("HS512 instead of HS256", func(t *testing.T) {
		registered := base()
		token := signWith(t, jwt.SigningMethodHS512, []byte(testJWTSecret),
			Claims{RegisteredClaims: registered, Role: "owner"})

		if _, err := svc.ValidateAccessToken(token); err == nil {
			t.Fatal("expected a token signed with an algorithm outside the allowlist to be refused")
		}
	})

	t.Run("missing expiry", func(t *testing.T) {
		registered := base()
		registered.ExpiresAt = nil
		token := signWith(t, jwt.SigningMethodHS256, []byte(testJWTSecret),
			Claims{RegisteredClaims: registered, Role: "owner"})

		if _, err := svc.ValidateAccessToken(token); err == nil {
			t.Fatal("expected a token without an expiry to be refused")
		}
	})

	t.Run("wrong audience", func(t *testing.T) {
		registered := base()
		registered.Audience = jwt.ClaimStrings{"somebody-else"}
		token := signWith(t, jwt.SigningMethodHS256, []byte(testJWTSecret),
			Claims{RegisteredClaims: registered, Role: "owner"})

		if _, err := svc.ValidateAccessToken(token); err == nil {
			t.Fatal("expected a token minted for another audience to be refused")
		}
	})

	t.Run("missing audience", func(t *testing.T) {
		registered := base()
		registered.Audience = nil
		token := signWith(t, jwt.SigningMethodHS256, []byte(testJWTSecret),
			Claims{RegisteredClaims: registered, Role: "owner"})

		if _, err := svc.ValidateAccessToken(token); err == nil {
			t.Fatal("expected a token without an audience to be refused")
		}
	})

	t.Run("wrong issuer", func(t *testing.T) {
		registered := base()
		registered.Issuer = "somebody-else"
		token := signWith(t, jwt.SigningMethodHS256, []byte(testJWTSecret),
			Claims{RegisteredClaims: registered, Role: "owner"})

		if _, err := svc.ValidateAccessToken(token); err == nil {
			t.Fatal("expected a token from another issuer to be refused")
		}
	})
}

func TestGeneratedTokensCarryTheAudience(t *testing.T) {
	svc := NewTokenService(TokenServiceConfig{JWTSecret: testJWTSecret, Environment: "test"})

	access, err := svc.GenerateAccessToken(uuid.New(), "owner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	claims, err := svc.ValidateAccessToken(access)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := []string(claims.Audience); len(got) != 1 || got[0] != jwtAudience {
		t.Errorf("want access-token audience [%s]; got %v", jwtAudience, got)
	}

	profile, err := svc.GenerateProfileToken("google-sub", "a@example.com", "A", "B")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	profileClaims, err := svc.ValidateProfileToken(profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := []string(profileClaims.Audience); len(got) != 1 || got[0] != jwtAudience {
		t.Errorf("want profile-token audience [%s]; got %v", jwtAudience, got)
	}
}

func TestValidateProfileTokenRefusesWrongAudience(t *testing.T) {
	svc := NewTokenService(TokenServiceConfig{JWTSecret: testJWTSecret, Environment: "test"})
	now := time.Now()

	token := signWith(t, jwt.SigningMethodHS256, []byte(testJWTSecret), GoogleProfileClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "google-sub",
			Issuer:    jwtIssuer,
			Audience:  jwt.ClaimStrings{"somebody-else"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
		Purpose: googleProfilePurpose,
	})

	if _, err := svc.ValidateProfileToken(token); err == nil {
		t.Fatal("expected a profile token minted for another audience to be refused")
	}
}
