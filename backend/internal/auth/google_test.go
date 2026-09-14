package auth

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/googleid"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// TestGoogleSignInDisabledConfig covers the 503 a caller gets when no
// GOOGLE_OAUTH_CLIENT_ID is configured — newFixture's stub verifier starts
// disabled, exactly like a real Verifier with an empty client id.
func TestGoogleSignInDisabledConfig(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.GoogleSignIn(w, postJSON(t, `{"credential":"whatever"}`))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.google.calls) != 0 {
		t.Error("Verify must not be called while Google sign-in is disabled")
	}
}

func TestGoogleCompleteDisabledConfig(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.GoogleComplete(w, postJSON(t, `{"profile_token":"whatever","phone":"+5491112345678"}`))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestGoogleSignInExistingUser is the whole existing-account contract in one
// place: a session exactly like Login's, email_verified flipped from false to
// true, failed attempts reset, and the identity link written.
// The pre-hijack this guards against: somebody registered ana@example.com
// with a password of their choosing and never verified it; Ana then signs in
// with Google. The account is hers now — with the registrant's password gone
// and nothing they were issued still valid.
func TestGoogleSignInExistingUser(t *testing.T) {
	f := newFixtureWithGoogle(t)
	user := &authstore.User{
		ID: uuid.New(), Email: "ana@example.com", FirstName: "Ana", LastName: "Perez",
		Phone: "+5491112345678", Role: "owner", IsActive: true, EmailVerified: false,
		FailedLoginAttempts: 3,
	}
	if err := user.SetPassword("registrant-chose-this", bcrypt.MinCost); err != nil {
		t.Fatal(err)
	}
	registrantHash := append([]byte(nil), user.PasswordHash...)
	f.users.add(user)

	w := httptest.NewRecorder()
	f.handler.GoogleSignIn(w, postJSON(t, `{"credential":"good-id-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if findCookie(w.Header(), "access_token") == nil {
		t.Error("no access token cookie was set")
	}
	if findCookie(w.Header(), "refresh_token") == nil {
		t.Error("no refresh token cookie was set")
	}
	body := decode(t, w)
	if _, ok := body["csrf_token"]; !ok {
		t.Error("csrf_token missing from the response body")
	}
	if _, ok := body["user"]; !ok {
		t.Error("user missing from the response body")
	}

	if f.users.verified == nil || *f.users.verified != user.ID {
		t.Error("email_verified was not persisted for a previously-unverified account")
	}
	if f.users.failedReset == 0 {
		t.Error("failed login attempts were not reset")
	}
	if f.users.passwordUpdated == nil || *f.users.passwordUpdated != user.ID || bytes.Equal(user.PasswordHash, registrantHash) {
		t.Error("the password the registrant chose still opens the account Google just handed to Ana")
	}
	if len(f.tokens.allWiped) != 1 || f.tokens.allWiped[0] != user.ID {
		t.Errorf("refresh tokens wiped for %v, want exactly [%s]", f.tokens.allWiped, user.ID)
	}
	if len(f.blacklist.invalidated) != 1 || f.blacklist.invalidated[0] != user.ID {
		t.Errorf("access tokens invalidated for %v, want exactly [%s]", f.blacklist.invalidated, user.ID)
	}

	if len(f.identities.inserted) != 1 {
		t.Fatalf("want exactly 1 identity link written; got %d", len(f.identities.inserted))
	}
	link := f.identities.inserted[0]
	if link.UserID != user.ID || link.Provider != "google" || link.Subject != f.google.claims.Subject {
		t.Errorf("identity link = %+v, want user %s, provider google, subject %s", link, user.ID, f.google.claims.Subject)
	}

	if len(f.audit.entries) != 2 {
		t.Fatalf("want the credential reset and the sign-in recorded, got %d entries: %+v", len(f.audit.entries), f.audit.entries)
	}
	for _, action := range []string{actionPasswordReset, actionLogin} {
		event, ok := findEntry(t, f.audit.entries, action).NewValue.(accountEvent)
		if !ok {
			t.Fatalf("%s: audit NewValue is not accountEvent", action)
		}
		if event.Method != "google" {
			t.Errorf("%s: audit Method = %q, want %q", action, event.Method, "google")
		}
	}
}

// The control: an account whose owner proved the address themselves keeps
// the password they chose. Only the never-verified case is a credential reset.
func TestGoogleSignInVerifiedAccountKeepsCredentials(t *testing.T) {
	f := newFixtureWithGoogle(t)
	user := &authstore.User{
		ID: uuid.New(), Email: "ana@example.com", FirstName: "Ana", LastName: "Perez",
		Phone: "+5491112345678", Role: "owner", IsActive: true, EmailVerified: true,
	}
	f.users.add(user)

	w := httptest.NewRecorder()
	f.handler.GoogleSignIn(w, postJSON(t, `{"credential":"good-id-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.users.passwordUpdated != nil || len(f.tokens.allWiped) != 0 || len(f.blacklist.invalidated) != 0 {
		t.Error("a verified account's own credentials were reset by a Google sign-in")
	}
	if entry := f.audit.only(t); entry.Action != actionLogin {
		t.Errorf("audit action = %q, want %q", entry.Action, actionLogin)
	}
}

// If the reset cannot be persisted, no session may start: it would sit on
// top of a password somebody else chose.
func TestGoogleSignInClaimNotPersisted(t *testing.T) {
	f := newFixtureWithGoogle(t)
	user := &authstore.User{
		ID: uuid.New(), Email: "ana@example.com", FirstName: "Ana", LastName: "Perez",
		Phone: "+5491112345678", Role: "owner", IsActive: true, EmailVerified: false,
	}
	f.users.add(user)
	f.users.updatePasswordErr = errors.New("db down")

	w := httptest.NewRecorder()
	f.handler.GoogleSignIn(w, postJSON(t, `{"credential":"good-id-token"}`))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500; got %d (%s)", w.Code, w.Body.String())
	}
	if findCookie(w.Header(), "access_token") != nil || findCookie(w.Header(), "refresh_token") != nil {
		t.Error("a session was started on an account whose registrant password is still in place")
	}
	if f.users.verified != nil || len(f.identities.inserted) != 0 || len(f.audit.entries) != 0 {
		t.Error("the account was promoted, linked or recorded although the credential reset failed")
	}
}

// TestGoogleSignInInactiveAccount and TestGoogleSignInLockedAccount cover the
// same generic 401 Login gives a wrong password — see loginFailed.
func TestGoogleSignInInactiveAccount(t *testing.T) {
	f := newFixtureWithGoogle(t)
	user := &authstore.User{
		ID: uuid.New(), Email: "ana@example.com", FirstName: "Ana", LastName: "Perez",
		Phone: "+5491112345678", Role: "owner", IsActive: false, EmailVerified: true,
	}
	f.users.add(user)

	w := httptest.NewRecorder()
	f.handler.GoogleSignIn(w, postJSON(t, `{"credential":"good-id-token"}`))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.tokens.stored) != 0 {
		t.Error("no session may be issued to a deactivated account")
	}
	if len(f.identities.inserted) != 0 {
		t.Error("no identity link may be written when the sign-in is refused")
	}
}

func TestGoogleSignInLockedAccount(t *testing.T) {
	f := newFixtureWithGoogle(t)
	lockedUntil := time.Now().Add(time.Hour)
	user := &authstore.User{
		ID: uuid.New(), Email: "ana@example.com", FirstName: "Ana", LastName: "Perez",
		Phone: "+5491112345678", Role: "owner", IsActive: true, EmailVerified: true,
		LockedUntil: &lockedUntil,
	}
	f.users.add(user)

	w := httptest.NewRecorder()
	f.handler.GoogleSignIn(w, postJSON(t, `{"credential":"good-id-token"}`))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.tokens.stored) != 0 {
		t.Error("no session may be issued to a locked account")
	}
}

// TestGoogleSignInUnknownEmailNeedsProfile covers the first-time sign-in
// branch: a verifiable profile_token, and the profile prefilled from claims.
func TestGoogleSignInUnknownEmailNeedsProfile(t *testing.T) {
	f := newFixtureWithGoogle(t)

	w := httptest.NewRecorder()
	f.handler.GoogleSignIn(w, postJSON(t, `{"credential":"good-id-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if needsProfile, _ := body["needs_profile"].(bool); !needsProfile {
		t.Errorf("needs_profile = %v, want true", body["needs_profile"])
	}
	profile, ok := body["profile"].(map[string]any)
	if !ok {
		t.Fatalf("profile is %T, not an object", body["profile"])
	}
	if profile["email"] != "ana@example.com" || profile["first_name"] != "Ana" || profile["last_name"] != "Perez" {
		t.Errorf("profile = %+v, want the claims' email/given_name/family_name", profile)
	}

	token, ok := body["profile_token"].(string)
	if !ok || token == "" {
		t.Fatalf("profile_token missing or not a string: %v", body["profile_token"])
	}

	claims, err := f.service.tokenService.ValidateProfileToken(token)
	if err != nil {
		t.Fatalf("the issued profile_token does not verify: %v", err)
	}
	if claims.Subject != f.google.claims.Subject || claims.Email != "ana@example.com" {
		t.Errorf("profile token claims = %+v, want the Google claims", claims)
	}
	if len(f.tokens.stored) != 0 {
		t.Error("no session may be issued before GoogleComplete")
	}
}

// mintProfileToken is the test helper every GoogleComplete test uses to get a
// verifiable profile_token without going through GoogleSignIn.
func mintProfileToken(t *testing.T, f *fixture, sub, email, givenName, familyName string) string {
	t.Helper()
	token, err := f.service.tokenService.GenerateProfileToken(sub, email, givenName, familyName)
	if err != nil {
		t.Fatalf("minting profile token: %v", err)
	}
	return token
}

func TestGoogleCompleteCreatesAccountAndSession(t *testing.T) {
	f := newFixtureWithGoogle(t)
	token := mintProfileToken(t, f, "google-sub-1", "new@example.com", "New", "User")

	w := httptest.NewRecorder()
	f.handler.GoogleComplete(w, postJSON(t,
		`{"profile_token":"`+token+`","phone":"+5491112345678"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if findCookie(w.Header(), "access_token") == nil {
		t.Error("no access token cookie was set")
	}
	if f.users.inserted == nil {
		t.Fatal("no user was inserted")
	}
	if f.users.inserted.Email != "new@example.com" {
		t.Errorf("inserted email = %q, want new@example.com", f.users.inserted.Email)
	}
	if f.users.inserted.Phone != "+5491112345678" {
		t.Errorf("inserted phone = %q, want E.164 normalized", f.users.inserted.Phone)
	}
	if f.users.inserted.FirstName != "New" || f.users.inserted.LastName != "User" {
		t.Errorf("inserted name = %s %s, want the profile token's given/family name",
			f.users.inserted.FirstName, f.users.inserted.LastName)
	}
	if f.users.verified == nil || *f.users.verified != f.users.inserted.ID {
		t.Error("the new account must be marked email_verified")
	}
	if len(f.identities.inserted) != 1 {
		t.Fatalf("want exactly 1 identity link written; got %d", len(f.identities.inserted))
	}
	if f.identities.inserted[0].Subject != "google-sub-1" {
		t.Errorf("linked subject = %q, want google-sub-1", f.identities.inserted[0].Subject)
	}
}

// TestGoogleCompleteNameOverrides covers the optional first_name/last_name
// override of the profile token's own Google-supplied names.
func TestGoogleCompleteNameOverrides(t *testing.T) {
	f := newFixtureWithGoogle(t)
	token := mintProfileToken(t, f, "google-sub-2", "over@example.com", "Given", "Family")

	w := httptest.NewRecorder()
	f.handler.GoogleComplete(w, postJSON(t,
		`{"profile_token":"`+token+`","phone":"+5491112345678","first_name":"Override","last_name":"Name"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.users.inserted.FirstName != "Override" || f.users.inserted.LastName != "Name" {
		t.Errorf("inserted name = %s %s, want the request's override", f.users.inserted.FirstName, f.users.inserted.LastName)
	}
}

// TestGoogleCompleteExpiredToken covers ValidateProfileToken's expiry check
// through the handler, with a token otherwise identical to a genuine one.
func TestGoogleCompleteExpiredToken(t *testing.T) {
	f := newFixtureWithGoogle(t)

	now := time.Now()
	claims := GoogleProfileClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "google-sub-expired",
			Issuer:    jwtIssuer,
			IssuedAt:  jwt.NewNumericDate(now.Add(-20 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-10 * time.Minute)),
		},
		Email:      "expired@example.com",
		GivenName:  "Ana",
		FamilyName: "Perez",
		Purpose:    "google_profile",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("signing expired token: %v", err)
	}

	w := httptest.NewRecorder()
	f.handler.GoogleComplete(w, postJSON(t,
		`{"profile_token":"`+signed+`","phone":"+5491112345678"}`))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401; got %d (%s)", w.Code, w.Body.String())
	}
	if f.users.inserted != nil {
		t.Error("no account may be created from an expired profile token")
	}
}

// TestGoogleCompleteMalformedToken covers a token that fails to parse at all
// (not the expiry path above).
func TestGoogleCompleteMalformedToken(t *testing.T) {
	f := newFixtureWithGoogle(t)

	w := httptest.NewRecorder()
	f.handler.GoogleComplete(w, postJSON(t,
		`{"profile_token":"not-a-real-token","phone":"+5491112345678"}`))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401; got %d (%s)", w.Code, w.Body.String())
	}
	if f.users.inserted != nil {
		t.Error("no account may be created from an invalid profile token")
	}
}

// TestGoogleCompleteWrongPurposeToken proves a token minted for a different
// purpose (an ordinary access token) is refused — the purpose claim is the
// only thing keeping the two apart, since both are HS256 with the same
// secret.
func TestGoogleCompleteWrongPurposeToken(t *testing.T) {
	f := newFixtureWithGoogle(t)

	accessToken, err := f.service.tokenService.GenerateAccessToken(uuid.New(), "owner")
	if err != nil {
		t.Fatalf("minting access token: %v", err)
	}

	w := httptest.NewRecorder()
	f.handler.GoogleComplete(w, postJSON(t,
		`{"profile_token":"`+accessToken+`","phone":"+5491112345678"}`))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGoogleCompleteExistingEmail(t *testing.T) {
	f := newFixtureWithGoogle(t)
	f.users.add(verifiedUser(t, "taken@example.com", "correct-horse-battery"))
	token := mintProfileToken(t, f, "google-sub-3", "taken@example.com", "Ana", "Perez")

	w := httptest.NewRecorder()
	f.handler.GoogleComplete(w, postJSON(t,
		`{"profile_token":"`+token+`","phone":"+5491112345678"}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.identities.inserted) != 0 {
		t.Error("no identity link may be written when account creation is refused")
	}
}

// TestGoogleCompleteExistingEmailRaceAtInsert covers the race Insert's own
// unique-constraint check catches: the address is free when GetByEmail runs
// and taken by the time Insert reaches the database — insertErr reproduces
// that outcome directly, bypassing the stub's normal duplicate-by-map check.
func TestGoogleCompleteExistingEmailRaceAtInsert(t *testing.T) {
	f := newFixtureWithGoogle(t)
	token := mintProfileToken(t, f, "google-sub-4", "race@example.com", "Ana", "Perez")
	f.users.insertErr = authstore.ErrDuplicateEmail

	w := httptest.NewRecorder()
	f.handler.GoogleComplete(w, postJSON(t,
		`{"profile_token":"`+token+`","phone":"+5491112345678"}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGoogleCompletePhoneInvalid(t *testing.T) {
	f := newFixtureWithGoogle(t)
	token := mintProfileToken(t, f, "google-sub-5", "badphone@example.com", "Ana", "Perez")

	w := httptest.NewRecorder()
	f.handler.GoogleComplete(w, postJSON(t,
		`{"profile_token":"`+token+`","phone":"not-a-phone-number"}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if f.users.inserted != nil {
		t.Error("no account may be created with an invalid phone")
	}
}

func TestGoogleCompletePhoneMissing(t *testing.T) {
	f := newFixtureWithGoogle(t)
	token := mintProfileToken(t, f, "google-sub-6", "nophone@example.com", "Ana", "Perez")

	w := httptest.NewRecorder()
	f.handler.GoogleComplete(w, postJSON(t, `{"profile_token":"`+token+`"}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestGoogleSignInVerifyUnavailable covers the 503 path when Verify itself
// reports the JWKS could not be reached, distinct from the 503 for
// "not configured".
func TestGoogleSignInVerifyUnavailable(t *testing.T) {
	f := newFixtureWithGoogle(t)
	f.google.err = googleid.ErrUnavailable

	w := httptest.NewRecorder()
	f.handler.GoogleSignIn(w, postJSON(t, `{"credential":"good-id-token"}`))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestGoogleSignInInvalidToken covers Verify rejecting the token itself
// (signature, audience, issuer...): a validation error, not a server error.
func TestGoogleSignInInvalidToken(t *testing.T) {
	f := newFixtureWithGoogle(t)
	f.google.err = errors.Join(googleid.ErrInvalidToken, errors.New("wrong audience"))

	w := httptest.NewRecorder()
	f.handler.GoogleSignIn(w, postJSON(t, `{"credential":"bad-id-token"}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGoogleSignInMissingCredential(t *testing.T) {
	f := newFixtureWithGoogle(t)

	w := httptest.NewRecorder()
	f.handler.GoogleSignIn(w, postJSON(t, `{}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.google.calls) != 0 {
		t.Error("Verify must not be called with no credential")
	}
}

// TestGoogleSignInRefusesNonJSONContentType pins the fix for a CSRF-exempt
// route reachable by a forged cross-site body: POST /api/v1/auth/google
// mints a session cookie and carries no CSRF token (it is the token's own
// source), so a plain <form enctype="text/plain"> submission could otherwise
// drive it with an attacker-chosen "body" that still decodes as the expected
// JSON shape. Refusing any Content-Type but application/json, before the
// body is read, closes that without a token check on the route that mints
// the cookie the token would be derived from — and it must happen before any
// Set-Cookie is written.
func TestGoogleSignInRefusesNonJSONContentType(t *testing.T) {
	f := newFixtureWithGoogle(t)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/",
		strings.NewReader(`{"credential":"good-id-token"}`))
	r.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	f.handler.GoogleSignIn(w, r)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if got := body["type"]; got != httpx.KindUnsupportedMediaType.URI() {
		t.Errorf("want type %q; got %v", httpx.KindUnsupportedMediaType.URI(), got)
	}
	if findCookie(w.Header(), "access_token") != nil || findCookie(w.Header(), "refresh_token") != nil {
		t.Error("a refused request must not mint a session cookie")
	}
	if len(f.google.calls) != 0 {
		t.Error("Verify must not be called before the Content-Type check passes")
	}
}
