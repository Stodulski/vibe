package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	"github.com/stodulski/vibe-server/internal/turnstile"
	"github.com/stodulski/vibe-server/internal/validator"
)

// actor reads who a request came from, off the request. It is the only thing
// the audit trail and the reuse-detection log line need that lives on the HTTP
// side.
func (h *Handler) actor(r *http.Request) Actor {
	return Actor{
		IP:         httpx.ClientIP(r, h.cfg.TrustProxies),
		RemoteAddr: r.RemoteAddr,
	}
}

// respondWithSession sets the session cookies for a freshly established session
// and answers with the account and its CSRF token.
func (h *Handler) respondWithSession(w http.ResponseWriter, r *http.Request, session *Session) {
	csrfToken := h.svc.tokenService.SetTokenCookies(w, session.AccessToken, session.RefreshToken)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"user":       toGenUser(session.User),
		"csrf_token": csrfToken,
	})
}

// checkTurnstile verifies the optional Cloudflare Turnstile token before a
// handler does anything else, when a verifier is configured. It reports
// whether the caller may proceed; on false it has already written the 422
// response, in this module's usual FailedValidation shape, and the caller
// must return immediately.
//
// Disabled always proceeds: self-hosters who never set TURNSTILE_SECRET_KEY
// are not forced to use it.
func (h *Handler) checkTurnstile(w http.ResponseWriter, r *http.Request, token string, failOpenOnUnavailable bool) bool {
	if !h.svc.TurnstileEnabled() {
		return true
	}

	v := validator.New()
	if token == "" {
		v.Check(false, "turnstile_token", "required")
		h.respond.FailedValidation(w, r, v.Errors)
		return false
	}

	err := h.svc.VerifyTurnstile(r.Context(), token, httpx.ClientIP(r, h.cfg.TrustProxies))
	if err == nil {
		return true
	}

	switch {
	case errors.Is(err, turnstile.ErrInvalidToken):
		h.logger.Warn("turnstile: token rejected", "path", r.URL.Path, "error", err)
		v.Check(false, "turnstile_token", "invalid")
	default:
		h.logger.Error("turnstile: verification unavailable", "path", r.URL.Path, "error", err)
		if failOpenOnUnavailable {
			return true
		}
		v.Check(false, "turnstile_token", "unavailable")
	}
	h.respond.FailedValidation(w, r, v.Errors)
	return false
}

// Register handles POST /api/v1/auth/register.
//
// It answers identically whether or not the address is taken, and tells the
// existing account by email that someone tried — otherwise the endpoint is an
// oracle for which addresses have accounts.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email          string `json:"email"`
		Password       string `json:"password"`
		FirstName      string `json:"first_name"`
		LastName       string `json:"last_name"`
		Phone          string `json:"phone"`
		TurnstileToken string `json:"turnstile_token"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	if !h.checkTurnstile(w, r, input.TurnstileToken, false) {
		return
	}

	v := validator.New()
	v.Check(input.Email != "", "email", "must be provided")
	v.Check(validator.Matches(input.Email, validator.EmailRX), "email", "must be a valid email address")
	v.Check(input.Password != "", "password", "must be provided")
	v.Check(len(input.Password) >= 8, "password", "must be at least 8 characters")
	v.Check(len(input.Password) <= 72, "password", "must not be more than 72 characters")
	v.Check(input.FirstName != "", "first_name", "must be provided")
	v.Check(len(input.FirstName) <= 100, "first_name", "must not be more than 100 characters")
	v.Check(input.LastName != "", "last_name", "must be provided")
	v.Check(len(input.LastName) <= 100, "last_name", "must not be more than 100 characters")
	v.Check(input.Phone != "", "phone", "must be provided")
	if input.Phone != "" {
		normalized, err := validator.NormalizePhone(input.Phone)
		if err != nil {
			v.AddError("phone", "must be a valid phone number (E.164 format, e.g. +5491112345678)")
		} else {
			input.Phone = normalized
		}
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	err = h.svc.Register(r.Context(), RegisterInput{
		Email:     input.Email,
		Password:  input.Password,
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Phone:     input.Phone,
	})
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{
		"message": "verification email sent",
	})
}

// Login handles POST /api/v1/auth/login, establishing the session cookies.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email          string `json:"email"`
		Password       string `json:"password"`
		TurnstileToken string `json:"turnstile_token"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	if !h.checkTurnstile(w, r, input.TurnstileToken, true) {
		return
	}

	v := validator.New()
	v.Check(input.Email != "", "email", "must be provided")
	v.Check(input.Password != "", "password", "must be provided")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	session, err := h.svc.Login(r.Context(), h.actor(r), input.Email, input.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			h.respond.InvalidCredentials(w, r)
		} else {
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.respondWithSession(w, r, session)
}

// VerifyEmail handles POST /api/v1/auth/verify-email, consuming the token from
// the emailed link.
func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var body gen.AuthVerifyEmailJSONBody
	err := httpx.ReadJSON(w, r, &body)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	if body.Token == "" {
		h.respond.BadRequest(w, r, fmt.Errorf("token must be provided"))
		return
	}

	err = h.svc.VerifyEmail(r.Context(), body.Token)
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "email verified successfully"})
}

// ResendVerification handles POST /api/v1/auth/resend-verification. It is
// rate-limited by the store's cooldown, and answers the same either way.
func (h *Handler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var body gen.AuthResendVerificationJSONBody
	err := httpx.ReadJSON(w, r, &body)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	var email string
	if body.Email != nil {
		email = *body.Email
	}

	if err := h.svc.ResendVerification(r.Context(), email); err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"message": "if the email exists, a verification link has been sent",
	})
}

// Refresh handles POST /api/v1/auth/refresh, rotating the refresh token.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	accessToken, refreshToken, err := h.svc.Refresh(r.Context(), h.actor(r), cookie.Value)
	if err != nil {
		if errors.Is(err, ErrInvalidSession) {
			h.respond.InvalidAuthenticationToken(w, r)
		} else {
			h.respond.ServerError(w, r, err)
		}
		return
	}

	csrfToken := h.svc.tokenService.SetTokenCookies(w, accessToken, refreshToken)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"csrf_token": csrfToken,
	})
}

// Logout handles POST /api/v1/auth/logout. It succeeds even with no session,
// so a user whose session already expired can still clear their cookies.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var refreshToken string
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		refreshToken = cookie.Value
	}

	var user *authstore.User
	if u, ok := httpx.ContextGetAuthenticatedUser(r); ok {
		user = u
	}

	// The cookie only, never the Bearer header: this is the credential the
	// browser session is carried in, and it is the one being revoked.
	var accessToken string
	if cookie, err := r.Cookie("access_token"); err == nil {
		accessToken = cookie.Value
	}

	h.svc.Logout(r.Context(), h.actor(r), refreshToken, accessToken, user)

	h.svc.tokenService.ClearTokenCookies(w)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "successfully logged out"})
}

// CurrentUser handles GET /api/v1/auth/me.
//
// The answer carries the session's CSRF token next to the user. The frontend
// bootstraps every fresh document load from this endpoint, so a page load never
// spends the refresh token: rotation happens only once the access token has
// expired and this request answered 401. The token is a pure HMAC of the access
// token (see TokenService.GenerateCSRFToken), so returning it costs no storage
// and rotates nothing; it is the same value the sign-in and refresh answers
// carry for that access token.
//
// The raw credential is read the way middleware.credential reads it, cookie
// first and then the Bearer header. That helper is not imported because the
// middleware package already depends on this one. RequireAuth put the user in
// the context, so one of the two is present; if neither is, the field is left
// out rather than turned into a 500.
func (h *Handler) CurrentUser(w http.ResponseWriter, r *http.Request) {
	user, ok := httpx.ContextGetAuthenticatedUser(r)
	if !ok {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	envelope := httpx.Envelope{"user": toGenUser(user)}
	if accessToken := rawAccessToken(r); accessToken != "" {
		envelope["csrf_token"] = h.svc.tokenService.GenerateCSRFToken(accessToken)
	}
	envelope["pending_email"] = h.pendingEmailOrNil(r, user.ID)

	h.respond.JSON(w, r, http.StatusOK, envelope)
}

// pendingEmailOrNil answers the pending_email field both GET and PUT
// /auth/me carry, reporting nil rather than failing the whole request when
// the lookup itself fails.
//
// This is a side feature riding on two requests that matter far more than it
// does: GET bootstraps every session, and PUT has, by the time this is
// called, already committed the write the caller asked for. A failed read of
// an unrelated table must never turn either of those into a 500 — the
// failure is logged instead, and the caller sees "no pending request" until
// the next successful read tells it otherwise.
func (h *Handler) pendingEmailOrNil(r *http.Request, userID uuid.UUID) *string {
	pending, err := h.svc.PendingEmail(r.Context(), userID)
	if err != nil {
		h.logger.Error("pending-email: lookup failed, reporting none", "error", err, "user_id", userID)
		return nil
	}
	return pending
}

// rawAccessToken returns the access token the request authenticated with: the
// cookie first, then a Bearer header, in the order the authentication
// middleware consults them.
func rawAccessToken(r *http.Request) string {
	if cookie, err := r.Cookie("access_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if found && scheme == "Bearer" && token != "" {
		return token
	}
	return ""
}

// DeleteAccount handles DELETE /api/v1/auth/me. It is refused while any complex
// the account owns still has live bookings, because deleting it would strand
// the clients holding them.
func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	user, ok := httpx.ContextGetAuthenticatedUser(r)
	if !ok {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	// The cookie only, as in Logout above.
	var accessToken string
	if cookie, err := r.Cookie("access_token"); err == nil {
		accessToken = cookie.Value
	}

	err := h.svc.DeleteAccount(r.Context(), h.actor(r), user, accessToken)
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	// Clear auth cookies so the browser can't reuse them.
	h.svc.tokenService.ClearTokenCookies(w)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "account deleted"})
}

// UpdateCurrentUser handles PUT /api/v1/auth/me. Changing the email address
// resets verification, and changing the password ends every open session.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen,gocognit,gocyclo // see the cohesion note above
func (h *Handler) UpdateCurrentUser(w http.ResponseWriter, r *http.Request) {
	user, ok := httpx.ContextGetAuthenticatedUser(r)
	if !ok {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	var input struct {
		Email           *string `json:"email"`
		FirstName       *string `json:"first_name"`
		LastName        *string `json:"last_name"`
		Phone           *string `json:"phone"`
		CurrentPassword *string `json:"current_password"`
		NewPassword     *string `json:"new_password"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	in := UpdateInput{
		Email:     input.Email,
		FirstName: input.FirstName,
		LastName:  input.LastName,
	}

	v := validator.New()

	if input.Email != nil {
		v.Check(*input.Email != "", "email", "must not be empty")
		v.Check(validator.Matches(*input.Email, validator.EmailRX), "email", "must be a valid email address")
	}
	if input.FirstName != nil {
		v.Check(*input.FirstName != "", "first_name", "must not be empty")
		v.Check(len(*input.FirstName) <= 100, "first_name", "must not be more than 100 characters")
	}
	if input.LastName != nil {
		v.Check(*input.LastName != "", "last_name", "must not be empty")
		v.Check(len(*input.LastName) <= 100, "last_name", "must not be more than 100 characters")
	}
	// Registration has always demanded E.164 here, because this number is what
	// WhatsApp delivers booking notifications to. This path accepted anything
	// non-empty, so the one number that matters could be replaced with something
	// undeliverable and nothing would say so — the failure is a message that
	// never arrives, which nobody sees, least of all the person expecting it.
	if input.Phone != nil {
		v.Check(*input.Phone != "", "phone", "must not be empty")
		if *input.Phone != "" {
			normalized, phoneErr := validator.NormalizePhone(*input.Phone)
			switch {
			case phoneErr == nil:
				// Spacing, dashes and parens are the user's business, not ours.
				in.Phone = &normalized
			case *input.Phone != user.Phone:
				v.AddError("phone", "must be a valid phone number (E.164 format, e.g. +5491112345678)")
			}
			// The remaining case is a number that cannot be normalised and is
			// byte-for-byte what is already stored — one that predates this
			// check. It is left exactly as found (in.Phone stays nil), because
			// the alternative is a 422 at someone who came to change their
			// surname, over a field they never touched, with no way to save the
			// form until they also fix a number they may no longer use. A
			// deliberate edit still has to be valid; only standing still is
			// free. Such a number stays undeliverable until its owner corrects
			// it, which no validation on this path can force — that is a data
			// question, not a request one.
		}
	}

	passwordChange := input.CurrentPassword != nil || input.NewPassword != nil
	if passwordChange {
		v.Check(input.CurrentPassword != nil && *input.CurrentPassword != "", "current_password", "must be provided")
		v.Check(input.NewPassword != nil && *input.NewPassword != "", "new_password", "must be provided")
		if input.NewPassword != nil {
			v.Check(len(*input.NewPassword) >= 8, "new_password", "must be at least 8 characters")
			v.Check(len(*input.NewPassword) <= 72, "new_password", "must not be more than 72 characters")
		}
	}

	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	if passwordChange {
		in.CurrentPassword = input.CurrentPassword
		in.NewPassword = input.NewPassword
	}

	updated, emailChange, err := h.svc.UpdateCurrentUser(r.Context(), h.actor(r), user, in)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			h.respond.InvalidCredentials(w, r)
		case errors.Is(err, authstore.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			h.respond.FailedValidation(w, r, v.Errors)
		case errors.Is(err, ErrEditConflict):
			h.respond.EditConflict(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"user":          toGenUser(updated),
		"pending_email": h.pendingEmailOrNil(r, updated.ID),
		"email_change":  string(emailChange),
	})
}

// ForgotPassword handles POST /api/v1/auth/forgot-password. It answers the same
// whether or not the address has an account.
func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email          string `json:"email"`
		TurnstileToken string `json:"turnstile_token"`
	}
	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	if !h.checkTurnstile(w, r, input.TurnstileToken, true) {
		return
	}

	h.svc.ForgotPassword(r.Context(), h.actor(r), input.Email)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"message": "if the email exists, a password reset link has been sent",
	})
}

// ResetPassword handles POST /api/v1/auth/reset-password. The link is
// single-use, and using it ends every session the account had open.
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var body gen.AuthResetPasswordJSONBody
	err := httpx.ReadJSON(w, r, &body)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(body.Token != "", "token", "must be provided")
	v.Check(body.Password != "", "password", "must be provided")
	v.Check(len(body.Password) >= 8, "password", "must be at least 8 characters")
	v.Check(len(body.Password) <= 72, "password", "must not be more than 72 characters")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	err = h.svc.ResetPassword(r.Context(), h.actor(r), body.Token, body.Password)
	if err != nil {
		h.respond.DomainErrorWith(w, r, err, "invalid or expired reset token")
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "password reset successfully"})
}

// ConfirmEmailChange handles POST /api/v1/auth/confirm-email-change. The link
// is single-use, is what actually moves the account's email (UpdateCurrentUser
// only ever queued the request), and ends every session the account had open —
// see auth.Service.ConfirmEmailChange.
func (h *Handler) ConfirmEmailChange(w http.ResponseWriter, r *http.Request) {
	var body gen.AuthConfirmEmailChangeJSONBody
	err := httpx.ReadJSON(w, r, &body)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(body.Token != "", "token", "must be provided")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	err = h.svc.ConfirmEmailChange(r.Context(), h.actor(r), body.Token)
	if err != nil {
		switch {
		case errors.Is(err, authstore.ErrDuplicateEmail):
			v.AddError("token", "the requested email address is no longer available")
			h.respond.FailedValidation(w, r, v.Errors)
		default:
			h.respond.DomainErrorWith(w, r, err, "invalid or expired email change token")
		}
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "email address updated"})
}
