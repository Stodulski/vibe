package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	"github.com/stodulski/vibe-server/internal/validator"
)

// googleIdentityProvider is the provider column value every row this module
// writes to user_identities carries. It is also the CHECK constraint's only
// allowed value (db/migrations/001_init.sql) — there is exactly
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
// the reset flow. Both a Google-only account at creation (Service.GoogleComplete)
// and a never-verified local account being claimed through Google
// (Service.claimUnverifiedAccount) get one.
func setUnusablePassword(user *authstore.User, cost int) error {
	randomPassword := make([]byte, randomPasswordBytes)
	if _, err := rand.Read(randomPassword); err != nil {
		return fmt.Errorf("auth: generate unusable password: %w", err)
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
func (h *Handler) GoogleSignIn(w http.ResponseWriter, r *http.Request) {
	if !h.svc.GoogleEnabled() {
		h.respond.Refuse(w, r, googleNotConfigured)
		return
	}

	var body gen.AuthGoogleJSONBody
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(body.Credential != "", "credential", "must be provided")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	result, err := h.svc.GoogleSignIn(r.Context(), h.actor(r), body.Credential)
	if err != nil {
		switch {
		case errors.Is(err, ErrGoogleRejected):
			v.AddError("credential", "invalid")
			h.respond.FailedValidation(w, r, v.Errors)
		case errors.Is(err, ErrInvalidCredentials):
			h.respond.InvalidCredentials(w, r)
		default:
			h.respond.DomainError(w, r, err)
		}
		return
	}

	if result.NeedsProfile != nil {
		h.respondNeedsProfile(w, r, result.NeedsProfile)
		return
	}

	h.respondWithSession(w, r, result.Session)
}

// GoogleComplete handles POST /api/v1/auth/google/complete: creates the
// account for a first-time Google sign-in, once the client has collected the
// phone number Google never provides, and starts the session.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) GoogleComplete(w http.ResponseWriter, r *http.Request) {
	if !h.svc.GoogleEnabled() {
		h.respond.Refuse(w, r, googleNotConfigured)
		return
	}

	var body gen.AuthGoogleCompleteJSONBody
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	claims, err := h.svc.tokenService.ValidateProfileToken(body.ProfileToken)
	if err != nil {
		// A missing, expired, forged or wrong-purpose token gets the same
		// generic refusal invalid credentials do elsewhere in this module —
		// it is not this caller's business which.
		h.respond.InvalidCredentials(w, r)
		return
	}

	var firstNameOverride, lastNameOverride string
	if body.FirstName != nil {
		firstNameOverride = *body.FirstName
	}
	if body.LastName != nil {
		lastNameOverride = *body.LastName
	}

	firstName := strings.TrimSpace(firstNameOverride)
	if firstName == "" {
		firstName = claims.GivenName
	}
	lastName := strings.TrimSpace(lastNameOverride)
	if lastName == "" {
		lastName = claims.FamilyName
	}

	v := validator.New()
	v.Check(firstName != "", "first_name", "must be provided")
	v.Check(len(firstName) <= 100, "first_name", "must not be more than 100 characters")
	v.Check(lastName != "", "last_name", "must be provided")
	v.Check(len(lastName) <= 100, "last_name", "must not be more than 100 characters")
	v.Check(body.Phone != "", "phone", "must be provided")
	if body.Phone != "" {
		normalized, normErr := validator.NormalizePhone(body.Phone)
		if normErr != nil {
			v.AddError("phone", "must be a valid phone number (E.164 format, e.g. +5491112345678)")
		} else {
			body.Phone = normalized
		}
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	session, err := h.svc.GoogleComplete(r.Context(), h.actor(r), claims, GoogleCompleteInput{
		FirstName: firstName,
		LastName:  lastName,
		Phone:     body.Phone,
	})
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respondWithSession(w, r, session)
}

// respondNeedsProfile answers a Google sign-in for an address with no account
// yet: the profile token plus the fields prefilled for the signup form.
func (h *Handler) respondNeedsProfile(w http.ResponseWriter, r *http.Request, needs *NeedsProfile) {
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"needs_profile": true,
		"profile_token": needs.ProfileToken,
		"profile": httpx.Envelope{
			"email":      needs.Email,
			"first_name": needs.FirstName,
			"last_name":  needs.LastName,
		},
	})
}
