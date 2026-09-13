package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenServiceConfig is what token minting needs.
type TokenServiceConfig struct {
	// JWTSecret signs and verifies access tokens. It is the active key: every
	// token this service mints is signed with it and names it in the `kid`
	// header.
	JWTSecret string
	// JWTKeyID names the active key in that header. Empty derives the name
	// from the secret — see keyID.
	JWTKeyID string
	// JWTSecretPrevious is the retired key: it verifies and never signs, so
	// that rotating JWTSecret does not invalidate the sessions minted under
	// the old one. Empty means there is no retired key.
	JWTSecretPrevious string
	// JWTKeyIDPrevious names the retired key. It has to repeat whatever
	// JWTKeyID held while that key was active, or the tokens naming it stop
	// resolving; empty derives it from the secret, which is right whenever
	// JWTKeyID was empty too.
	JWTKeyIDPrevious string
	// CookieDomain scopes the session cookies. Empty means host-only.
	CookieDomain string
	// Environment decides whether cookies are marked Secure; local development
	// runs over plain HTTP, where a Secure cookie would never be sent.
	Environment string
}

// NewTokenService returns a TokenService.
func NewTokenService(cfg TokenServiceConfig) *TokenService {
	return &TokenService{cfg: cfg, keys: newJWTKeyring(cfg)}
}

// TokenService mints and verifies the three tokens a session is made of: the
// short-lived JWT access token, the opaque refresh token, and the CSRF token
// derived from the access token.
//
// It is separate from Handler because the authentication middleware verifies
// tokens on every request and must not depend on the auth handlers to do it.
type TokenService struct {
	cfg TokenServiceConfig
	// keys is the signing keyring built from cfg at construction: the active
	// key and, while a rotation is in flight, the retired one. See keyring.go.
	keys jwtKeyring
}

// ActiveKeyID is the name the tokens this service mints carry in their `kid`
// header. It is exported for the boot log line, so an operator can see which
// key a running instance is signing with without decoding a token.
func (s *TokenService) ActiveKeyID() string { return s.keys.active.id }

// sign mints and signs a token with the active key, naming it in the header so
// the verifier can resolve it after a rotation.
func (s *TokenService) sign(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = s.keys.active.id
	signed, err := token.SignedString(s.keys.active.secret)
	if err != nil {
		return "", fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, nil
}

// errUnnamedKey is what a token with no `kid` header is refused with. It is a
// distinct error because "this token predates key rotation" and "this token
// names a key we retired" are different operator situations.
var errUnnamedKey = errors.New("token does not name a signing key")

// keyFor resolves the secret a token names. It is the key function every
// verification in this package passes to jwt.ParseWithClaims.
//
// It never falls back to the active key: see keyring.go for why a kid-less
// token is refused rather than accommodated.
func (s *TokenService) keyFor(token *jwt.Token) (any, error) {
	kid, _ := token.Header["kid"].(string)
	if kid == "" {
		return nil, errUnnamedKey
	}
	secret, ok := s.keys.lookup(kid)
	if !ok {
		return nil, fmt.Errorf("unknown signing key %q", kid)
	}
	return secret, nil
}

const (
	jwtIssuer = "vibe"
	// jwtAudience is the only audience this service mints and the only one it
	// accepts. It is what stops a token minted for some other consumer of the
	// same signing key — a signed download link, a service token, whatever is
	// added next — from being spent here, and vice versa. The issuer check
	// above answers "who made this"; this one answers "who was it for", and a
	// key with two uses needs both.
	jwtAudience        = "vibe-api"
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

// parseOptions are the validations every token this service verifies has to
// pass, stated once so the two verifiers cannot drift apart.
//
// WithValidMethods is the one that has to be an option rather than a check
// inside the key function: the "alg":"none" and RS256-verified-as-HMAC attacks
// both work by making the parser choose the algorithm from the header, and the
// only fix is an allowlist the parser consults before it asks anybody for a
// key. A method check inside the key function ran after the parser had already
// decided.
//
// WithExpirationRequired turns a missing exp from "this token never expires"
// into a refusal. jwt/v5 validates exp when it is present and accepts a token
// without one, so every token minted here carries one and every token verified
// here is required to.
//
// WithIssuer and WithAudience are the two halves of "this token was made by us,
// for us" — see jwtAudience.
func parseOptions() []jwt.ParserOption {
	return []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(jwtIssuer),
		jwt.WithAudience(jwtAudience),
	}
}

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
			Audience:  jwt.ClaimStrings{jwtAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenExpiry)),
		},
		Role: role,
	}

	signed, err := s.sign(claims)
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
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, s.keyFor, parseOptions()...)
	if err != nil {
		return nil, fmt.Errorf("auth: parse access token: %w", err)
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
			Audience:  jwt.ClaimStrings{jwtAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(googleProfileTokenExpiry)),
		},
		Email:      email,
		GivenName:  givenName,
		FamilyName: familyName,
		Purpose:    googleProfilePurpose,
	}

	signed, err := s.sign(claims)
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
	token, err := jwt.ParseWithClaims(tokenString, &GoogleProfileClaims{}, s.keyFor, parseOptions()...)
	if err != nil {
		return nil, fmt.Errorf("auth: parse profile token: %w", err)
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
// needs no server-side storage and is bound to exactly that session. It always
// uses the active key.
func (s *TokenService) GenerateCSRFToken(accessToken string) string {
	return csrfWith(s.keys.active.secret, accessToken)
}

func csrfWith(secret []byte, accessToken string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("csrf:" + accessToken))
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidateCSRFToken checks the submitted CSRF token against the one derived
// from this session's access token, in constant time.
//
// Every key in the ring is tried, not only the active one. A CSRF token
// carries no header naming its key, and the one a browser is holding was
// derived under whichever secret was active when the session started — so
// checking only the active key would make a rotation reject every in-flight
// form submission, which is the outage the keyring exists to avoid. Each
// comparison is constant-time and the loop is at most two long.
func (s *TokenService) ValidateCSRFToken(accessToken, csrfToken string) bool {
	valid := false
	for _, secret := range s.keys.all() {
		// No early return: the loop is a fixed two iterations either way, so a
		// caller cannot learn which key matched from how long this took.
		if hmac.Equal([]byte(csrfWith(secret, accessToken)), []byte(csrfToken)) {
			valid = true
		}
	}
	return valid
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
