package auth

import (
	"crypto/rand"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/googleid"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/validator"
)

// googleIdentityProvider is the provider column value every row this module
// writes to user_identities carries. It is also the CHECK constraint's only
// allowed value (db/migrations/002_user_identities.sql) — there is exactly
// one provider today.
const googleIdentityProvider = "google"

// randomPasswordBytes is how much entropy backs the unusable password hash
// setUnusablePassword gives a Google-only account. It is never typed by
// anyone: PasswordMatches always fails against it, which is the point — the
// account can only be signed into through Google until a real password is
// set.
const randomPasswordBytes = 32

// setUnusablePassword gives user a password nobody knows, so that a password
// sign-in against the account always fails until its owner sets one through
// the reset flow. Both a Google-only account at creation (GoogleComplete) and
// a never-verified local account being claimed through Google
// (claimUnverifiedAccount) get one.
func setUnusablePassword(user *authstore.User, cost int) error {
	randomPassword := make([]byte, randomPasswordBytes)
	if _, err := rand.Read(randomPassword); err != nil {
		return err
	}
	// The raw bytes, not a base64 or hex encoding of them: bcrypt hashes
	// whatever it is given, and encoding them first would spend some of the
	// 72 bytes bcrypt reads on encoding overhead instead of entropy for no
	// benefit — nobody ever types this password.
	return user.SetPassword(string(randomPassword), cost)
}

// GoogleSignIn handles POST /api/v1/auth/google: verifies a Google Identity
// Services ID token from the client and either starts a session for the
// account already registered under that address, or hands back a
// short-lived profile token so the client can collect the one field Google
// never provides — a phone number — before GoogleComplete creates the
// account.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) GoogleSignIn(w http.ResponseWriter, r *http.Request) {
	if !h.google.Enabled() {
		h.respond.Error(w, r, http.StatusServiceUnavailable, "google sign-in is not configured")
		return
	}

	var input struct {
		Credential string `json:"credential"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.Credential != "", "credential", "must be provided")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	claims, err := h.google.Verify(r.Context(), input.Credential)
	if err != nil {
		if errors.Is(err, googleid.ErrUnavailable) {
			h.logger.Error("google sign-in: verification unavailable", "error", err)
			h.respond.Error(w, r, http.StatusServiceUnavailable, "google sign-in is temporarily unavailable")
			return
		}
		h.logger.Warn("google sign-in: token rejected", "error", err)
		v.AddError("credential", "invalid")
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	user, err := h.users.GetByEmail(r.Context(), claims.Email)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			h.respondNeedsProfile(w, r, claims)
			return
		}
		h.respond.ServerError(w, r, err)
		return
	}

	// Same generic refusal Login gives a wrong password or a locked account —
	// see Login's comment on why a lockout is never told apart from bad
	// credentials in the response, only in what loginFailed records.
	if !user.IsActive || user.IsLocked() {
		h.loginFailed(w, r, claims.Email)
		return
	}

	if user.FailedLoginAttempts > 0 {
		if resetErr := h.users.ResetFailedAttempts(r.Context(), user.ID); resetErr != nil {
			h.logger.Error("google sign-in: failed to reset login attempts", "error", resetErr, "user_id", user.ID)
		}
	}

	// Google already proved this address, so an unverified account is
	// promoted rather than turned away the way Login turns an unverified
	// password sign-in away — but never with its credentials intact. Whoever
	// registered this address with a password never proved they own the
	// address (an unverified account cannot sign in), and carrying that
	// password into the account Google just handed its real owner is the
	// classic pre-hijack: the registrant's password would open the owner's
	// account. So the merge is a credential reset first.
	if !user.EmailVerified && !h.claimUnverifiedAccount(w, r, user) {
		return
	}

	h.linkGoogleIdentity(r, user.ID, claims)

	h.startSession(w, r, user, googleIdentityProvider)
}

// GoogleComplete handles POST /api/v1/auth/google/complete: creates the
// account for a first-time Google sign-in, once the client has collected the
// phone number Google never provides, and starts the session.
//
//nolint:funlen // see the cohesion note on GoogleSignIn above
func (h *Handler) GoogleComplete(w http.ResponseWriter, r *http.Request) {
	if !h.google.Enabled() {
		h.respond.Error(w, r, http.StatusServiceUnavailable, "google sign-in is not configured")
		return
	}

	var input struct {
		ProfileToken string `json:"profile_token"`
		Phone        string `json:"phone"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	claims, err := h.tokenService.ValidateProfileToken(input.ProfileToken)
	if err != nil {
		// A missing, expired, forged or wrong-purpose token gets the same
		// generic refusal invalid credentials do elsewhere in this module —
		// it is not this caller's business which.
		h.respond.InvalidCredentials(w, r)
		return
	}

	firstName := strings.TrimSpace(input.FirstName)
	if firstName == "" {
		firstName = claims.GivenName
	}
	lastName := strings.TrimSpace(input.LastName)
	if lastName == "" {
		lastName = claims.FamilyName
	}

	v := validator.New()
	v.Check(firstName != "", "first_name", "must be provided")
	v.Check(len(firstName) <= 100, "first_name", "must not be more than 100 characters")
	v.Check(lastName != "", "last_name", "must be provided")
	v.Check(len(lastName) <= 100, "last_name", "must not be more than 100 characters")
	v.Check(input.Phone != "", "phone", "must be provided")
	if input.Phone != "" {
		normalized, normErr := validator.NormalizePhone(input.Phone)
		if normErr != nil {
			v.AddError("phone", "must be a valid phone number (E.164 format, e.g. +5491112345678)")
		} else {
			input.Phone = normalized
		}
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	// A second window between the needs_profile answer and this request in
	// which the address was claimed — by an ordinary registration, or by the
	// same Google account completing twice. Insert below catches the same
	// race at the database's unique constraint; this is the common case
	// answered cheaply and without a second unusable password hash minted.
	if _, err := h.users.GetByEmail(r.Context(), claims.Email); err == nil {
		h.respond.Error(w, r, http.StatusConflict, "account already exists")
		return
	} else if !errors.Is(err, data.ErrRecordNotFound) {
		h.respond.ServerError(w, r, err)
		return
	}

	user := &authstore.User{
		Email:     claims.Email,
		FirstName: firstName,
		LastName:  lastName,
		Phone:     input.Phone,
		Role:      "owner",
	}

	if err := setUnusablePassword(user, h.cfg.PasswordHashCost); err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	if err := h.users.Insert(r.Context(), user); err != nil {
		if errors.Is(err, authstore.ErrDuplicateEmail) {
			h.respond.Error(w, r, http.StatusConflict, "account already exists")
			return
		}
		h.respond.ServerError(w, r, err)
		return
	}

	// InsertUser (db/queries/users.sql) never writes email_verified, so it
	// comes back false from the RETURNING clause Insert reads onto user —
	// this is what actually persists the true the contract promises, the way
	// Register's development-only auto-verify does.
	if err := h.users.SetEmailVerified(r.Context(), user.ID); err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	user.EmailVerified = true

	h.linkGoogleIdentity(r, user.ID, &googleid.Claims{Subject: claims.Subject, Email: claims.Email})

	h.startSession(w, r, user, googleIdentityProvider)
}

// claimUnverifiedAccount hands a never-verified local account to the Google
// account that just proved its address: the password is replaced with one
// nobody knows, every session and refresh token issued for the account is
// revoked, the reset is recorded, and only then is the address marked
// verified. It reports false, after answering the request, when the reset
// itself could not be persisted — a session must not start on top of a
// password somebody else chose.
func (h *Handler) claimUnverifiedAccount(w http.ResponseWriter, r *http.Request, user *authstore.User) bool {
	if err := setUnusablePassword(user, h.cfg.PasswordHashCost); err != nil {
		h.respond.ServerError(w, r, err)
		return false
	}
	if err := h.users.UpdatePassword(r.Context(), user.ID, user.PasswordHash); err != nil {
		h.respond.ServerError(w, r, err)
		return false
	}

	// The same action ResetPassword writes, for the same reason: the
	// credential was replaced without the old one being presented. Method
	// names what authorized it — Google's proof of the address, not a reset
	// link.
	//
	//nolint:contextcheck // see the note on Record in Login.
	h.record(r, audit.Entry{
		Action:   actionPasswordReset,
		EntityID: accountID(user.ID),
		NewValue: accountEvent{Email: recordedAddress(user.Email), Method: googleIdentityProvider},
	})

	// Best effort, as in ResetPassword: an unverified account could never
	// sign in, so there should be nothing to revoke, and a failure here is
	// logged rather than allowed to keep the owner out.
	if err := h.tokens.DeleteAllForUser(r.Context(), user.ID); err != nil {
		h.logger.Error("google sign-in: failed to revoke sessions of a claimed account", "error", err, "user_id", user.ID)
	}
	if err := h.blacklist.InvalidateUserTokens(r.Context(), user.ID); err != nil {
		h.revocationFailed("google claim", user.ID.String(), err)
	}

	if err := h.users.SetEmailVerified(r.Context(), user.ID); err != nil {
		h.logger.Error("google sign-in: failed to mark email verified", "error", err, "user_id", user.ID)
		return true
	}
	user.EmailVerified = true
	return true
}

// respondNeedsProfile answers a Google sign-in for an address with no
// account yet: a short-lived profile token carrying what Google already
// verified — so GoogleComplete does not have to trust the client's own copy
// of it — plus the profile fields prefilled for the signup form.
func (h *Handler) respondNeedsProfile(w http.ResponseWriter, r *http.Request, claims *googleid.Claims) {
	givenName, familyName := claims.GivenName, claims.FamilyName
	if givenName == "" && familyName == "" {
		givenName, familyName = splitName(claims.Name)
	}

	profileToken, err := h.tokenService.GenerateProfileToken(claims.Subject, claims.Email, givenName, familyName)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"needs_profile": true,
		"profile_token": profileToken,
		"profile": httpx.Envelope{
			"email":      claims.Email,
			"first_name": givenName,
			"last_name":  familyName,
		},
	})
}

// linkGoogleIdentity records the Google account behind userID, once. A store
// failure is logged, not fatal: the session this request is about to start
// does not depend on the link row existing, and the next Google sign-in
// tries again (Insert is idempotent — see data.UserIdentityStore.Insert).
func (h *Handler) linkGoogleIdentity(r *http.Request, userID uuid.UUID, claims *googleid.Claims) {
	email := claims.Email
	err := h.identities.Insert(r.Context(), &authstore.UserIdentity{
		UserID:   userID,
		Provider: googleIdentityProvider,
		Subject:  claims.Subject,
		Email:    &email,
	})
	if err != nil {
		h.logger.Error("google: failed to record identity link", "error", err, "user_id", userID)
	}
}

// splitName splits a Google display name into first/last when the ID token
// carried no given_name/family_name — some accounts have a name but no split
// form. The first space separates the first name from the rest; a name with
// no space becomes the whole first name and an empty last name.
func splitName(name string) (first, last string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ""
	}
	parts := strings.SplitN(name, " ", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.TrimSpace(parts[1])
}
