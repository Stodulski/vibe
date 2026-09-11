package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenServiceConfig is what token minting needs.
type TokenServiceConfig struct {
	// JWTSecret signs and verifies access tokens.
	JWTSecret string
	// CookieDomain scopes the session cookies. Empty means host-only.
	CookieDomain string
	// Environment decides whether cookies are marked Secure; local development
	// runs over plain HTTP, where a Secure cookie would never be sent.
	Environment string
}

// NewTokenService returns a TokenService.
func NewTokenService(cfg TokenServiceConfig) *TokenService {
	return &TokenService{cfg: cfg}
}

// TokenService mints and verifies the three tokens a session is made of: the
// short-lived JWT access token, the opaque refresh token, and the CSRF token
// derived from the access token.
//
// It is separate from Handler because the authentication middleware verifies
// tokens on every request and must not depend on the auth handlers to do it.
type TokenService struct {
	cfg TokenServiceConfig
}

const (
	jwtIssuer          = "vibe"
	accessTokenExpiry  = 15 * time.Minute
	refreshTokenExpiry = 30 * 24 * time.Hour // 30 days
	// googleProfileTokenExpiry is how long a needs_profile answer's
	// profile_token stays valid — long enough to fill in a phone number, short
	// enough that a leaked token is not a standing way to claim the address it
	// names (GoogleComplete still refuses if the address was taken meanwhile).
	googleProfileTokenExpiry = 10 * time.Minute
	// googleProfilePurpose is GoogleProfileClaims.Purpose. It is what keeps a
	// profile token from being usable anywhere an ordinary access token or a
	// verification/reset token is accepted, and vice versa: every token this
	// service mints is HS256 with the same secret, so the purpose claim is the
	// only thing that tells them apart.
	googleProfilePurpose = "google_profile"
	// refreshReuseGrace is how long after a refresh token was rotated a second
	// presentation of it is treated as a concurrent refresh rather than theft.
	//
	// Two browser tabs share one cookie jar. When the access token expires,
	// each tab's first 401 triggers its own refresh, and the second request
	// leaves before the first response has replaced the cookie, so it carries
	// the token the first one just spent. Seen in dev: two refreshes 0.8s
	// apart, the second revoking every session on the account. Within this
	// window the late one is refused without revoking anything; the tab's next
	// attempt sends the cookie its sibling set and succeeds. A replay after the
	// window is still treated as a stolen token.
	refreshReuseGrace = 30 * time.Second
)

// Claims is what a verified access token carries. The middleware reads it on
// every authenticated request.
type Claims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

// GenerateAccessToken mints a short-lived JWT carrying the user id and role.
// It is self-contained, which is why revoking one needs the blacklist.
func (s *TokenService) GenerateAccessToken(userID uuid.UUID, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    jwtIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenExpiry)),
		},
		Role: role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}
	return signed, nil
}

// ValidateAccessToken verifies an access token's signature, issuer and expiry,
// and returns its claims. It does not consult the blacklist — the caller does,
// because that requires a round trip the token itself cannot answer.
//
// The issuer is checked, not merely minted. Today the signing key has exactly
// one use, so a token carrying a foreign issuer cannot be produced without the
// key — which makes the check look redundant. It is the second use of that key
// that this guards: a webhook signature, a signed download link, a service
// token. Whoever adds one would be relying on this function to reject what it
// was not asked to accept, and the guarantee stated above needs to already be
// true when they do, rather than be discovered missing afterwards.
func (s *TokenService) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	}, jwt.WithIssuer(jwtIssuer))
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// GoogleProfileClaims is what a profile token carries between GoogleSignIn's
// needs_profile answer and GoogleComplete: the Google identity a first-time
// caller already proved ownership of, so GoogleComplete does not have to
// trust the client's own copy of it.
type GoogleProfileClaims struct {
	jwt.RegisteredClaims
	Email      string `json:"email"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	// Purpose is always googleProfilePurpose on a token this service minted;
	// see the constant's comment.
	Purpose string `json:"purpose"`
}

// GenerateProfileToken mints a 10-minute token carrying the Google profile
// gathered on a first sign-in. sub is the Google account's own id
// (googleid.Claims.Subject), carried as the JWT subject.
func (s *TokenService) GenerateProfileToken(sub, email, givenName, familyName string) (string, error) {
	now := time.Now()
	claims := GoogleProfileClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			Issuer:    jwtIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(googleProfileTokenExpiry)),
		},
		Email:      email,
		GivenName:  givenName,
		FamilyName: familyName,
		Purpose:    googleProfilePurpose,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("signing profile JWT: %w", err)
	}
	return signed, nil
}

// ValidateProfileToken verifies a profile token's signature, issuer and
// expiry, and that it carries googleProfilePurpose — so an access,
// verification or reset token, all signed with the same secret, cannot be
// replayed here.
func (s *TokenService) ValidateProfileToken(tokenString string) (*GoogleProfileClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &GoogleProfileClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	}, jwt.WithIssuer(jwtIssuer))
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*GoogleProfileClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	if claims.Purpose != googleProfilePurpose {
		return nil, fmt.Errorf("wrong token purpose %q", claims.Purpose)
	}
	return claims, nil
}

// generateRefreshToken creates a random plaintext refresh token and its SHA256 hash.
// uuid.New() and sha256.Sum256 cannot fail, so this never returns an error.
func generateRefreshToken() (plaintext string, hash []byte) {
	plaintext = uuid.New().String()
	sum := sha256.Sum256([]byte(plaintext))
	hash = sum[:]
	return plaintext, hash
}

func hashRefreshToken(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}

// GenerateCSRFToken derives a token from the access token with an HMAC, so it
// needs no server-side storage and is bound to exactly that session.
func (s *TokenService) GenerateCSRFToken(accessToken string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	mac.Write([]byte("csrf:" + accessToken))
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidateCSRFToken checks the submitted CSRF token against the one derived
// from this session's access token, in constant time.
func (s *TokenService) ValidateCSRFToken(accessToken, csrfToken string) bool {
	expected := s.GenerateCSRFToken(accessToken)
	return hmac.Equal([]byte(expected), []byte(csrfToken))
}

// SetTokenCookies writes the two session cookies and returns the CSRF token.
//
// Both cookies are HttpOnly so no script can read them. The CSRF token is not a
// cookie at all: it is returned to the caller, which echoes it back in the
// X-CSRF-Token header — and that echo, which a cross-site caller cannot produce,
// is what CSRFProtect checks against the access token it is derived from.
func (s *TokenService) SetTokenCookies(w http.ResponseWriter, accessToken, refreshToken string) string {
	secure := s.cfg.Environment != "development"

	//nolint:gosec // G124: HttpOnly/SameSite are literal true/Lax above; Secure is env-conditional (false only in
	// local development) so gosec's literal-value check cannot prove it, but the cookie is always fully secured.
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		Domain:   s.cfg.CookieDomain,
		MaxAge:   int(accessTokenExpiry.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	//nolint:gosec // G124: HttpOnly/SameSite are literal true/Lax above; Secure is env-conditional (false only in
	// local development) so gosec's literal-value check cannot prove it, but the cookie is always fully secured.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/api/v1/auth",
		Domain:   s.cfg.CookieDomain,
		MaxAge:   int(refreshTokenExpiry.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	return s.GenerateCSRFToken(accessToken)
}

// ClearTokenCookies expires both session cookies. There is no third one to
// clear: the CSRF token lives in the response body, not in a cookie.
func (s *TokenService) ClearTokenCookies(w http.ResponseWriter) {
	secure := s.cfg.Environment != "development"

	//nolint:gosec // G124: HttpOnly/SameSite are literal true/Lax below; Secure is env-conditional (false only in
	// local development), which gosec's literal-value check cannot prove, but the cookie is always fully secured.
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		Domain:   s.cfg.CookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	//nolint:gosec // G124: HttpOnly/SameSite are literal true/Lax above; Secure is env-conditional (false only in
	// local development) so gosec's literal-value check cannot prove it, but the cookie is always fully secured.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		Domain:   s.cfg.CookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
