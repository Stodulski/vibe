package auth

import (
	"crypto/rand"
	"crypto/subtle"
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

// ---------------------------------------------------------------------------
// Google sign-in, OIDC authorization-code + PKCE flow
// ---------------------------------------------------------------------------
//
// See internal/auth/google_oidc.go for the Service half of this flow (the
// state store, the token exchange and the nonce check) and its own comment
// for how the pieces fit together and why. This file only has the HTTP
// concerns: reading and writing the state cookie, and mapping the Service's
// errors onto the same status/field vocabulary GoogleComplete already uses.

// googleErrorUnavailable is one of the two values the frontend's /login page
// reads out of ?error= and turns into a sentence: a redirect target is a URL
// a person can read and share, so it says that the sign-in did not happen and
// nothing about why.
const googleErrorUnavailable = "google_unavailable"

// frontendNotConfigured is the answer GoogleStart gives when FRONTEND_URL is
// empty. Every other failure of that endpoint is a redirect to the frontend,
// which is precisely what this deployment has no address for — so it is the
// one case that answers a problem document instead.
var frontendNotConfigured = httpx.NotImplemented(
	"google sign-in through the OIDC flow needs FRONTEND_URL to be configured")

// googleRedirectFailed sends the browser back to the frontend's login page
// with one of the known error values, having logged why.
//
// The log line is the only place the reason exists: the person reading the
// address bar learns that the sign-in did not happen, and nothing that would
// help somebody probing this endpoint.
func (h *Handler) googleRedirectFailed(
	w http.ResponseWriter, r *http.Request, errorValue, reason string, cause error, extra ...any,
) {
	args := append([]any{"reason", reason, "error_value", errorValue}, extra...)
	if cause != nil {
		args = append(args, "error", cause)
	}
	if errorValue == googleErrorUnavailable {
		h.logger.Error("google redirect sign-in failed", args...)
	} else {
		h.logger.Warn("google redirect sign-in refused", args...)
	}

	h.redirect(w, r, h.cfg.FrontendURL+"/login?error="+errorValue)
}

// redirect answers a request whose caller is a browser mid-navigation, not a
// client reading JSON, with 303 See Other.
func (h *Handler) redirect(w http.ResponseWriter, r *http.Request, location string) {
	http.Redirect(w, r, location, http.StatusSeeOther)
}

const (
	// googleOAuthStateCookie is the state cookie's name.
	googleOAuthStateCookie = "google_oauth_state"
	// googleOAuthStateCookiePath scopes the cookie to this flow's own two
	// routes — it is read by GoogleFinish and by nothing else, so a browser
	// carries it nowhere outside them.
	googleOAuthStateCookiePath = "/api/v1/auth/google"
)

// setOAuthStateCookie sets the state cookie GoogleFinish reads back, with the
// TTL the stored entry itself carries (googleOAuthStateTTL).
func (h *Handler) setOAuthStateCookie(w http.ResponseWriter, state string) {
	//nolint:gosec // G124: HttpOnly/SameSite are literal true/Lax below; Secure is env-conditional (false only in
	// local development) so gosec's literal-value check cannot prove it, but the cookie is always fully secured.
	http.SetCookie(w, &http.Cookie{
		Name:     googleOAuthStateCookie,
		Value:    state,
		Path:     googleOAuthStateCookiePath,
		MaxAge:   int(googleOAuthStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.Environment != "development",
		SameSite: http.SameSiteLaxMode,
	})
}

// clearOAuthStateCookie expires the state cookie. GoogleFinish calls this
// whatever the outcome: the attempt the cookie named is over either way,
// spent by a real callback or refused as somebody else's, and a state left
// behind is one more single-use value with nothing left to spend.
func (h *Handler) clearOAuthStateCookie(w http.ResponseWriter) {
	//nolint:gosec // G124: HttpOnly/SameSite are literal true/Lax below; Secure is env-conditional (false only in
	// local development) so gosec's literal-value check cannot prove it, but the cookie is always fully secured.
	http.SetCookie(w, &http.Cookie{
		Name:     googleOAuthStateCookie,
		Value:    "",
		Path:     googleOAuthStateCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.Environment != "development",
		SameSite: http.SameSiteLaxMode,
	})
}

// GoogleStart handles GET /api/v1/auth/google/start: sends the browser to
// Google's own consent screen, the entry point of the standard OIDC
// authorization-code flow. Reached by navigating here directly — an ordinary
// link, not an XHR — so every failure is a redirect the person looking at
// the address bar can act on: there is no client here to hand a JSON body to.
func (h *Handler) GoogleStart(w http.ResponseWriter, r *http.Request) {
	// Nothing about this response may be cached or replayed: a cached 302
	// would send every later visitor back to a stale, already-spent state.
	w.Header().Set("Cache-Control", "no-store")

	if h.cfg.FrontendURL == "" {
		h.respond.Refuse(w, r, frontendNotConfigured)
		return
	}
	if !h.svc.GoogleOAuthEnabled() {
		h.googleRedirectFailed(w, r, googleErrorUnavailable, "oauth_not_configured", nil)
		return
	}

	start, err := h.svc.GoogleStart(r.Context())
	if err != nil {
		h.googleRedirectFailed(w, r, googleErrorUnavailable, "oauth_start_failed", err)
		return
	}

	h.setOAuthStateCookie(w, start.State)
	http.Redirect(w, r, start.AuthorizationURL, http.StatusFound)
}

// GoogleFinish handles POST /api/v1/auth/google/finish: the frontend's
// callback page calls this once Google sends the browser back with code and
// state, completing the flow GoogleStart began.
//
// The state cookie is this route's whole login-CSRF defence: whoever holds it
// is the browser GoogleStart redirected, and a code/state pair presented
// without it — or with a state that does not match — never reaches Google.
// It is cleared whatever the outcome, because the attempt it named is over
// either way.
func (h *Handler) GoogleFinish(w http.ResponseWriter, r *http.Request) {
	// A session, or a profile token, is about to be minted: no copy of this
	// response may be cached or replayed.
	w.Header().Set("Cache-Control", "no-store")

	if !h.svc.GoogleOAuthEnabled() {
		h.respond.Refuse(w, r, googleNotConfigured)
		return
	}

	var body gen.AuthGoogleFinishJSONBody
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(body.Code != "", "code", "must be provided")
	v.Check(body.State != "", "state", "must be provided")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	cookie, cookieErr := r.Cookie(googleOAuthStateCookie)
	h.clearOAuthStateCookie(w)
	if cookieErr != nil || cookie.Value == "" ||
		subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(body.State)) != 1 {
		// The same answer an unknown, expired or replayed state gets from the
		// service below — see GoogleFinish's own comment for why they must
		// not be told apart.
		v.AddError("code", "invalid or expired")
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	result, err := h.svc.GoogleFinish(r.Context(), h.actor(r), body.Code, body.State)
	if err != nil {
		switch {
		case errors.Is(err, ErrGoogleCodeInvalid):
			v.AddError("code", "invalid or expired")
			h.respond.FailedValidation(w, r, v.Errors)
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
