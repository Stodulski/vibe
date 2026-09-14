package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
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

// ---------------------------------------------------------------------------
// Google sign-in, redirect mode
// ---------------------------------------------------------------------------

const (
	// googleRedirectMaxBody bounds Google's form post. The credential is a
	// JWT of a couple of kilobytes and the CSRF token is a short random
	// string; 16 KiB is generous for both and small enough that a flood of
	// oversized posts costs nothing to refuse.
	googleRedirectMaxBody = 16 << 10
	// googleFormContentType is the only content type Google's redirect mode
	// sends, and so the only one this endpoint parses.
	googleFormContentType = "application/x-www-form-urlencoded"
	// googleCSRFField is the double-submit token's name in both places it
	// arrives: the cookie Google sets on the app's origin and the form field
	// it posts. Google's guide requires them to be present and equal.
	googleCSRFField = "g_csrf_token"
	// googleCredentialField is the ID token's field name in the form post.
	googleCredentialField = "credential"
)

const (
	// googleErrorRejected and googleErrorUnavailable are the two values the
	// frontend's /login page reads out of ?error= and turns into a sentence.
	// They are the whole vocabulary on purpose: a redirect target is a URL a
	// person can read and share, so it says that the sign-in did not happen
	// and nothing about why.
	googleErrorRejected    = "google_rejected"
	googleErrorUnavailable = "google_unavailable"
)

// frontendNotConfigured is the answer the redirect endpoint gives when
// FRONTEND_URL is empty. Every other failure of that endpoint is a redirect
// to the frontend, which is precisely what this deployment has no address
// for — so it is the one case that answers a problem document instead.
var frontendNotConfigured = httpx.NotImplemented(
	"google sign-in through redirect mode needs FRONTEND_URL to be configured")

// GoogleRedirect handles POST /api/v1/auth/google/redirect: the `login_uri`
// Google posts to in redirect mode (`ux_mode: 'redirect'`), reached through
// the frontend's own proxy so that the cookie Google sets on the app's origin
// arrives with it.
//
// Popup mode opens a blank Google page on a good share of mobile browsers, and
// redirect mode is the way out — but it changes what this request is. It is a
// top-level, cross-site form navigation whose response the person sees as a
// page, so it may not establish anything: a POST an attacker can cause must
// never end in a session. So the answer is always a redirect, the session is
// never started here, and what travels in the URL is an opaque one-time code
// the frontend spends from its own origin against GoogleExchange.
//
// Every failure answers 303 to the frontend as well. The alternative is a
// problem document rendered as a dead-end page in the address bar, on the one
// endpoint in this API whose caller is a human being looking at a browser
// rather than a client reading JSON.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) GoogleRedirect(w http.ResponseWriter, r *http.Request) {
	// Nothing about this response may be cached or replayed: it carries a
	// one-time code in its Location.
	w.Header().Set("Cache-Control", "no-store")

	if h.cfg.FrontendURL == "" {
		h.respond.Refuse(w, r, frontendNotConfigured)
		return
	}
	if !h.svc.GoogleEnabled() {
		h.googleRedirectFailed(w, r, googleErrorUnavailable, "not_configured", nil)
		return
	}

	if contentType := r.Header.Get("Content-Type"); !isGoogleFormPost(contentType) {
		h.googleRedirectFailed(w, r, googleErrorRejected, "unexpected_content_type", nil,
			"content_type", contentType)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, googleRedirectMaxBody)
	if err := r.ParseForm(); err != nil {
		// An oversized body and a malformed one are the same answer: neither
		// is a post Google made.
		h.googleRedirectFailed(w, r, googleErrorRejected, "unreadable_body", err)
		return
	}

	// The double submit from Google's own guide: the token is in a cookie on
	// the app's origin and in the form, and a cross-site forgery can write
	// the form but not read the cookie. Compared in constant time, because
	// the comparison itself must not report how much of a guess was right.
	cookie, err := r.Cookie(googleCSRFField)
	if err != nil || cookie.Value == "" {
		h.googleRedirectFailed(w, r, googleErrorRejected, "csrf_cookie_missing", nil)
		return
	}
	field := r.PostFormValue(googleCSRFField)
	if field == "" {
		h.googleRedirectFailed(w, r, googleErrorRejected, "csrf_field_missing", nil)
		return
	}
	if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(field)) != 1 {
		h.googleRedirectFailed(w, r, googleErrorRejected, "csrf_mismatch", nil)
		return
	}

	credential := r.PostFormValue(googleCredentialField)
	if credential == "" {
		h.googleRedirectFailed(w, r, googleErrorRejected, "credential_missing", nil)
		return
	}

	code, err := h.svc.GoogleRedirectStart(r.Context(), credential)
	if err != nil {
		if errors.Is(err, ErrGoogleRejected) {
			h.googleRedirectFailed(w, r, googleErrorRejected, "credential_rejected", err)
			return
		}
		// A verifier that could not reach Google, a Redis that would not hold
		// the code, an encoding failure: none of them is the caller's fault,
		// and all of them mean this sign-in cannot continue.
		h.googleRedirectFailed(w, r, googleErrorUnavailable, "sign_in_failed", err)
		return
	}

	h.redirect(w, r, h.cfg.FrontendURL+"/auth/google/return?code="+url.QueryEscape(code))
}

// isGoogleFormPost reports whether a Content-Type header is the form encoding
// Google's redirect mode posts. The parameters after the media type (a
// charset) are Content-Type's own syntax and are ignored, as they are
// everywhere else that reads this header.
func isGoogleFormPost(contentType string) bool {
	media, _, err := mime.ParseMediaType(contentType)
	return err == nil && media == googleFormContentType
}

// googleRedirectFailed sends the browser back to the frontend's login page
// with one of the two error values it knows, having logged why.
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

// redirect answers a redirect-mode request with 303 See Other, which is what
// turns a POST into the browser's following GET.
func (h *Handler) redirect(w http.ResponseWriter, r *http.Request, location string) {
	http.Redirect(w, r, location, http.StatusSeeOther)
}

// GoogleExchange handles POST /api/v1/auth/google/exchange: the frontend
// spends the one-time code GoogleRedirect put in the URL, from its own origin,
// and gets exactly what POST /auth/google answers — a session, or the profile
// it still has to complete.
//
// This is where the cookies are set, and it is the only endpoint of the two
// that sets any.
func (h *Handler) GoogleExchange(w http.ResponseWriter, r *http.Request) {
	if !h.svc.GoogleEnabled() {
		h.respond.Refuse(w, r, googleNotConfigured)
		return
	}

	var body gen.AuthGoogleExchangeJSONBody
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(body.Code != "", "code", "must be provided")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	result, err := h.svc.GoogleExchange(r.Context(), h.actor(r), body.Code)
	if err != nil {
		switch {
		case errors.Is(err, ErrGoogleCodeInvalid):
			// One message for unknown, expired and already-spent: which of
			// the three it was is not this caller's business, and answering
			// it would turn this endpoint into an oracle for codes.
			v.AddError("code", "invalid or expired")
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
