package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/turnstile"
	"github.com/stodulski/vibe-server/internal/validator"
)

// checkTurnstile verifies the optional Cloudflare Turnstile token before a
// handler does anything else, when a verifier is configured. It reports
// whether the caller may proceed; on false it has already written the 422
// response, in this module's usual FailedValidation shape, and the caller
// must return immediately.
//
// Disabled (h.turnstile.Enabled() == false) always proceeds: self-hosters who
// never set TURNSTILE_SECRET_KEY are not forced to use it.
func (h *Handler) checkTurnstile(w http.ResponseWriter, r *http.Request, token string, failOpenOnUnavailable bool) bool {
	if !h.turnstile.Enabled() {
		return true
	}

	v := validator.New()
	if token == "" {
		v.Check(false, "turnstile_token", "required")
		h.respond.FailedValidation(w, r, v.Errors)
		return false
	}

	err := h.turnstile.Verify(r.Context(), token, httpx.ClientIP(r, h.cfg.TrustProxies))
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

	user := &data.User{
		Email:     input.Email,
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Phone:     input.Phone,
		Role:      "owner",
	}

	err = user.SetPassword(input.Password)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	err = h.users.Insert(r.Context(), user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			// Return same response as success to prevent email enumeration.
			h.logger.Info("register: duplicate email attempt")

			// Notify the existing account owner so they know someone tried to register.
			//
			// The greeting is the account's own first name, read back here, and
			// never input.FirstName. This email goes to the person who already
			// owns the address, and its whole subject is that a stranger just
			// touched their account — greeting them with the name that stranger
			// typed put an attacker's chosen words in Vibe's voice, addressed
			// to the victim. A lookup that fails greets them without a name;
			// the email says the same thing either way.
			existingName := ""
			if existing, lookupErr := h.users.GetByEmail(r.Context(), input.Email); lookupErr == nil && existing != nil {
				existingName = existing.FirstName
			}
			h.notify.DuplicateRegistration(notifications.DuplicateRegistrationEmail{
				To:        input.Email,
				FirstName: existingName,
				LoginURL:  h.cfg.FrontendURL + "/login",
				ResetURL:  h.cfg.FrontendURL + "/forgot-password",
			})

			h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{
				"message": "verification email sent",
			})
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	// In development, auto-verify the email so E2E tests can login immediately.
	if h.cfg.Environment == "development" {
		if verifyErr := h.users.SetEmailVerified(r.Context(), user.ID); verifyErr != nil {
			h.logger.Error("dev: auto-verify failed", "error", verifyErr, "user_id", user.ID)
		}
	}

	plaintext := uuid.New().String()
	hash := hashRefreshToken(plaintext)

	err = h.verifications.Insert(r.Context(), user.ID, hash)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	verifyURL := h.cfg.FrontendURL + "/verify-email?token=" + plaintext

	h.notify.EmailVerification(notifications.VerificationEmail{
		To: user.Email, FirstName: user.FirstName, VerifyURL: verifyURL,
	})

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{
		"message": "verification email sent",
	})
}

// Login handles POST /api/v1/auth/login, establishing the session cookies.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
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

	user, err := h.users.GetByEmail(r.Context(), input.Email)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			_ = data.ComparePassword(data.DummyPasswordHash(), input.Password)
			h.loginFailed(w, r, input.Email)
			return
		}
		// A store failure is not a failed sign-in and is not recorded as one.
		// Nobody presented bad credentials; the database could not answer.
		h.respond.ServerError(w, r, err)
		return
	}

	// Always perform bcrypt comparison first for timing safety,
	// even if the account is locked.
	match, err := user.PasswordMatches(input.Password)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	// Check lockout AFTER bcrypt to maintain consistent timing.
	//
	// A locked account answers exactly as a wrong password does, because an
	// unauthenticated caller is entitled to know one thing: these credentials
	// did not establish a session. Answering 429 here told them a second thing.
	// Five wrong guesses against any address and the sixth reply named the ones
	// that have accounts — the very fact the dummy bcrypt above spends real work
	// keeping out of the response timing, handed over in the status line.
	//
	// So the lockout is no longer something the caller is told; it is only
	// something that happens. Nothing is emailed either: this endpoint is
	// unauthenticated and repeatable, so notifying on each attempt would let
	// anyone flood a stranger's inbox by guessing at their address, and the
	// cooldown that would need lives in a store this package does not own.
	//
	// The legitimate user is not stranded by the silence. The lock is temporary
	// and clears on its own, and the exit they would reach for on being told
	// "invalid credentials" — the reset link — now clears it outright, which is
	// why ResetPassword below ends with ResetFailedAttempts.
	if user.IsLocked() {
		h.loginFailed(w, r, input.Email)
		return
	}

	if !match {
		// Increment failed attempts (handles progressive lockout).
		if incErr := h.users.IncrementFailedAttempts(r.Context(), user.ID); incErr != nil {
			h.logger.Error("login: failed to increment login attempts", "error", incErr, "user_id", user.ID)
		}
		h.loginFailed(w, r, input.Email)
		return
	}

	// Successful password match — reset failed attempts.
	if user.FailedLoginAttempts > 0 {
		if resetErr := h.users.ResetFailedAttempts(r.Context(), user.ID); resetErr != nil {
			h.logger.Error("login: failed to reset login attempts", "error", resetErr, "user_id", user.ID)
		}
	}

	if !user.EmailVerified {
		// Return same generic error to prevent account enumeration.
		// Silently resend verification email as a convenience.
		//nolint:contextcheck // autoResendVerification intentionally uses its own detached
		// 10s timeout (not r.Context()) so the cooldown-checked resend still completes even
		// though the response below is written immediately after.
		h.autoResendVerification(user)
		h.loginFailed(w, r, input.Email)
		return
	}

	if !user.IsActive {
		h.loginFailed(w, r, input.Email)
		return
	}

	h.startSession(w, r, user, "")
}

// startSession establishes a new session for user: invalidates its cached
// copy, mints an access token, rotates in a fresh refresh token, sets the
// session cookies, records the sign-in, and answers with the user and the
// CSRF token.
//
// method distinguishes how the session was established for the audit trail
// (accountEvent.Method) — empty for the ordinary password login, "google" for
// Sign in with Google (Handler.GoogleSignIn, Handler.GoogleComplete), both of
// which reuse this exactly rather than duplicating Login's session-issuing
// steps.
func (h *Handler) startSession(w http.ResponseWriter, r *http.Request, user *data.User, method string) {
	h.cache.InvalidateUser(r.Context(), user.ID)

	accessToken, err := h.tokenService.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	refreshPlain, refreshHash := generateRefreshToken()

	err = h.tokens.InsertRefreshToken(r.Context(), user.ID, refreshHash, refreshTokenExpiry)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	csrfToken := h.tokenService.SetTokenCookies(w, accessToken, refreshPlain)

	// Recorded once the session exists, not once the password matched: the
	// three writes above can each refuse, and an entry saying a session was
	// established when none was is worse than no entry at all.
	//
	// UserID is nil and is not read from the context, unlike the entries written
	// from the guarded routes. This request did not carry a session; it created
	// one, and a stale cookie on it would not change that. Pinning it means a
	// sign-in entry looks the same whether or not the browser still had an old
	// session lying around. The account is named by EntityID either way.
	//
	//nolint:contextcheck // Record deliberately detaches: the entry describes
	// something that already happened, so it must still be written when the
	// client hangs up mid-response. See audit.Recorder.Record.
	h.record(r, audit.Entry{
		Action:   actionLogin,
		EntityID: accountID(user.ID),
		NewValue: accountEvent{Email: recordedAddress(user.Email), Method: method},
	})

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"user":       user,
		"csrf_token": csrfToken,
	})
}

// loginFailed records one sign-in attempt that established no session, then
// gives the caller the generic refusal.
//
// Every exit in Login that refuses a sign-in goes through here, so that the
// uniformity the trail depends on is structural rather than a convention five
// call sites have to keep. attempted is the address as typed, which is the whole
// value of the entry: the actor is unknown and deliberately unnamed, so the
// address is the only thing linking one attempt to the next.
func (h *Handler) loginFailed(w http.ResponseWriter, r *http.Request, attempted string) {
	//nolint:contextcheck // see the note on Record in Login above.
	h.record(r, audit.Entry{
		Action:   actionLoginFailed,
		NewValue: accountEvent{Email: recordedAddress(attempted)},
	})

	h.respond.InvalidCredentials(w, r)
}

// VerifyEmail handles POST /api/v1/auth/verify-email, consuming the token from
// the emailed link.
func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}
	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	if input.Token == "" {
		h.respond.BadRequest(w, r, fmt.Errorf("token must be provided"))
		return
	}

	tokenHash := hashRefreshToken(input.Token)

	vToken, err := h.verifications.GetByHash(r.Context(), tokenHash)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.Error(w, r, http.StatusBadRequest, "invalid or expired verification token")
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	// SetEmailVerified is idempotent — safe against concurrent requests.
	err = h.users.SetEmailVerified(r.Context(), vToken.UserID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	h.cache.InvalidateUser(r.Context(), vToken.UserID)

	// Delete tokens immediately to reduce exposure surface.
	if delErr := h.verifications.DeleteByUser(r.Context(), vToken.UserID); delErr != nil {
		h.logger.Error("failed to delete verification tokens", "error", delErr)
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "email verified successfully"})
}

// ResendVerification handles POST /api/v1/auth/resend-verification. It is
// rate-limited by the store's cooldown, and answers the same either way.
func (h *Handler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	genericMsg := "if the email exists, a verification link has been sent"

	// Always return success to prevent email enumeration.
	user, err := h.users.GetByEmail(r.Context(), input.Email)
	if err != nil || user.EmailVerified {
		h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": genericMsg})
		return
	}

	plaintext := uuid.New().String()
	hash := hashRefreshToken(plaintext)

	// Atomic: deletes stale tokens (>3 min) and inserts new one only if cooldown passed.
	err = h.verifications.InsertWithCooldown(r.Context(), user.ID, hash)
	if err != nil {
		if errors.Is(err, data.ErrCooldownActive) {
			h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": genericMsg})
			return
		}
		h.respond.ServerError(w, r, err)
		return
	}

	verifyURL := h.cfg.FrontendURL + "/verify-email?token=" + plaintext

	h.notify.EmailVerification(notifications.VerificationEmail{
		To: user.Email, FirstName: user.FirstName, VerifyURL: verifyURL,
	})

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": genericMsg})
}

// Refresh handles POST /api/v1/auth/refresh, rotating the refresh token.
//
// Presenting an already-rotated token means a copy is in circulation, so every
// session for that account is revoked rather than just this one refused.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	oldHash := hashRefreshToken(cookie.Value)

	storedToken, err := h.tokens.GetRefreshToken(r.Context(), oldHash)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			// Token not found — check if it was already used (reuse detection).
			usedToken, usedErr := h.tokens.GetUsedRefreshToken(r.Context(), oldHash)
			if usedErr == nil && time.Since(usedToken.UsedAt) < refreshReuseGrace {
				// A sibling tab rotated this token moments ago; see
				// refreshReuseGrace. Refuse this request, revoke nothing.
				h.logger.Info("refresh token presented again within the concurrent-refresh grace window",
					"user_id", usedToken.UserID,
					"used_ago", time.Since(usedToken.UsedAt).String(),
				)
				h.respond.InvalidAuthenticationToken(w, r)
				return
			}
			if usedErr == nil {
				// REUSE DETECTED: this token was already rotated.
				// Nuclear option: invalidate ALL tokens for this user.
				h.logger.Error("SECURITY: refresh token reuse detected, invalidating all sessions",
					"user_id", usedToken.UserID,
					"remote_addr", r.RemoteAddr,
				)
				if delErr := h.tokens.DeleteAllForUser(r.Context(), usedToken.UserID); delErr != nil {
					h.logger.Error("SECURITY: failed to invalidate sessions after token reuse",
						"user_id", usedToken.UserID,
						"error", delErr,
					)
				}

				// Deleting the refresh tokens stops new access tokens being
				// minted, but the ones already issued are self-contained JWTs
				// valid for their full lifetime. Without this the holder of a
				// stolen token keeps access for up to accessTokenExpiry after
				// the theft was detected.
				if blErr := h.blacklist.InvalidateUserTokens(r.Context(), usedToken.UserID); blErr != nil {
					h.revocationFailed("refresh-token reuse", usedToken.UserID.String(), blErr)
				}

				// Every session on the account was just destroyed, and nobody
				// asked for it. Until now the only record of that was the log
				// line above, so the account holder's "why was I signed out of
				// everything, everywhere?" had no durable answer.
				//
				// UserID stays nil. The account is named by EntityID, which is
				// the party this happened to; who presented the rotated token is
				// exactly what is not known, and putting the victim's id in the
				// actor column would record the theft as something they did.
				//
				//nolint:contextcheck // see the note on Record in Login.
				h.record(r, audit.Entry{
					Action:   actionRefreshReuse,
					EntityID: accountID(usedToken.UserID),
					NewValue: accountEvent{Actor: actorUnknown},
				})
			}
			h.respond.InvalidAuthenticationToken(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	user, err := h.users.GetByID(r.Context(), storedToken.UserID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	if !user.IsActive {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	h.cache.InvalidateUser(r.Context(), user.ID)

	accessToken, err := h.tokenService.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	refreshPlain, refreshHash := generateRefreshToken()

	err = h.tokens.InsertRefreshToken(r.Context(), user.ID, refreshHash, refreshTokenExpiry)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	// Mark the old token as used (instead of deleting) for reuse detection —
	// and only now, once the replacement is durably stored.
	//
	// Marking it earlier meant every step between the mark and this point could
	// fail with the client still holding a token the database already considered
	// spent. Its perfectly ordinary retry then looked exactly like a replayed
	// stolen token, and the answer to that is the nuclear one: every session on
	// the account revoked, every issued access token blacklisted, and a
	// "SECURITY: refresh token reuse detected" line naming the user as an
	// attacker. One transient database error signed a user out of every device
	// and filed it as a break-in.
	//
	// Ordering it this way removes the false positive outright rather than
	// narrowing it, and costs genuine theft detection nothing — the alternative,
	// forgiving a used token inside a grace window, would hand a real thief a
	// window in which a replayed token quietly mints a session instead of
	// tripping the alarm, and an exfiltrated token is replayed by tooling within
	// seconds, squarely inside it.
	//
	// If this write is the one that fails, the client keeps a token that still
	// works and no session was issued, because the cookies are set below. The
	// stored replacement is then unreachable and expires on its own.
	err = h.tokens.MarkRefreshTokenUsed(r.Context(), oldHash)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	csrfToken := h.tokenService.SetTokenCookies(w, accessToken, refreshPlain)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"csrf_token": csrfToken,
	})
}

// revocationFailed reports a revocation the blacklist could not share with the
// other instances, and is the single place that decides what the user is told
// about it: nothing.
//
// The request keeps its normal successful response on purpose. The revocation
// did land — the blacklist records a failed Redis write in its own memory, so
// this instance still enforces it — and what is missing is only its propagation
// to the other instances, which the person clicking "log out" can neither see
// nor do anything about. Answering "logout failed" would invite them to retry
// something that already worked as far as they are concerned, while the real
// problem is a Redis outage that only we can fix. So the alarm is raised at
// error level and in Sentry, where it reaches the people who can act on it.
func (h *Handler) revocationFailed(op, subject string, err error) {
	h.logger.Error("SECURITY: token revocation not shared with other instances — enforced on this instance only",
		"op", op, "subject", subject, "error", err)
	sentry.CaptureMessage(fmt.Sprintf(
		"TOKEN REVOCATION DEGRADED (enforced on this instance only, other instances still accept the token): op=%s subject=%s error=%v",
		op, subject, err))
}

// Logout handles POST /api/v1/auth/logout. It succeeds even with no session,
// so a user whose session already expired can still clear their cookies.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil && cookie.Value != "" {
		hash := hashRefreshToken(cookie.Value)
		if delErr := h.tokens.DeleteRefreshToken(r.Context(), hash); delErr != nil {
			h.logger.Error("failed to delete refresh token on logout", "error", delErr)
		}
	}

	// Blacklist the access token so it can't be used until it expires.
	if atCookie, atErr := r.Cookie("access_token"); atErr == nil && atCookie.Value != "" {
		if claims, parseErr := h.tokenService.ValidateAccessToken(atCookie.Value); parseErr == nil {
			if blErr := h.blacklist.BlacklistToken(r.Context(), atCookie.Value, claims.ExpiresAt.Time); blErr != nil {
				h.revocationFailed("logout", claims.Subject, blErr)
			}
		}
	}

	h.tokenService.ClearTokenCookies(w)

	// Recorded only when the request carried a session this handler could name.
	//
	// The route has no RequireAuth guard on purpose — clearing cookies has to
	// work for someone whose session already expired — so a plain POST here with
	// no cookies at all is not an act against any account, and recording it
	// would let anyone write rows into an unpruned table by curling an
	// unauthenticated endpoint.
	//
	// One real sign-out is missed by this: an expired access token alongside a
	// still-valid refresh token does end a session, but DeleteRefreshToken takes
	// a hash and returns nothing, so the account behind it is not knowable here
	// without widening TokenStore. An entry naming no account is worth less than
	// the store method it would cost.
	if user, ok := httpx.ContextGetAuthenticatedUser(r); ok {
		//nolint:contextcheck // see the note on Record in Login.
		h.record(r, audit.Entry{
			UserID:   accountID(user.ID),
			Action:   actionLogout,
			EntityID: accountID(user.ID),
			NewValue: accountEvent{Email: recordedAddress(user.Email)},
		})
	}

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

	envelope := httpx.Envelope{"user": user}
	if accessToken := rawAccessToken(r); accessToken != "" {
		envelope["csrf_token"] = h.tokenService.GenerateCSRFToken(accessToken)
	}

	h.respond.JSON(w, r, http.StatusOK, envelope)
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

	// Check all owned complexes for active bookings.
	complexes, err := h.complexes.GetByOwner(r.Context(), user.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	for _, c := range complexes {
		hasActive, err := h.bookings.HasActiveBookings(r.Context(), c.ID)
		if err != nil {
			h.respond.ServerError(w, r, err)
			return
		}
		if hasActive {
			h.respond.Error(w, r, http.StatusConflict, "cannot delete account while you have active bookings, cancel them first")
			return
		}
	}

	// Delete user — CASCADE removes complexes, courts, clients, etc.
	//
	// The row goes first and the sessions after. The sessions used to be
	// revoked before the delete, so when the delete failed the response was a
	// 500 the client showed as an error — and then every request 401'd, the
	// refresh was refused, and the app signed the owner out and sent them to
	// the login page, which reads as "deleted", with the account fully intact.
	// A failed delete must leave the session exactly as it was.
	err = h.users.Delete(r.Context(), user.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	h.cache.InvalidateUser(r.Context(), user.ID)

	// The account is gone; now nothing issued to it may keep working.
	if blErr := h.blacklist.InvalidateUserTokens(r.Context(), user.ID); blErr != nil {
		h.revocationFailed("delete account", user.ID.String(), blErr)
	}
	if err := h.tokens.DeleteAllForUser(r.Context(), user.ID); err != nil {
		h.logger.Error("delete account: failed to invalidate sessions", "error", err, "user_id", user.ID)
	}

	// Blacklist the current access token.
	if atCookie, atErr := r.Cookie("access_token"); atErr == nil && atCookie.Value != "" {
		if claims, parseErr := h.tokenService.ValidateAccessToken(atCookie.Value); parseErr == nil {
			if blErr := h.blacklist.BlacklistToken(r.Context(), atCookie.Value, claims.ExpiresAt.Time); blErr != nil {
				h.revocationFailed("delete account", claims.Subject, blErr)
			}
		}
	}

	// Recorded after the row is gone, and therefore with UserID nil.
	//
	// audit_log.user_id is a foreign key into users (audit_log_user_id_fkey), so by
	// the time this act is complete there is nothing left for that column to
	// point at: an entry naming the actor there would be refused by the database
	// and dropped, and the one deletion the trail most needs to have witnessed
	// would be the one it silently missed. So the account is named by entity_id,
	// which carries no foreign key, and the actor is said in words — the same
	// shape internal/payments uses for the same reason.
	//
	// It is recorded here rather than before the delete because the delete can
	// fail, and an entry saying an account was destroyed when it still exists is
	// the worse of the two errors.
	//
	//nolint:contextcheck // see the note on Record in Login.
	h.record(r, audit.Entry{
		Action:   actionAccountDelete,
		EntityID: accountID(user.ID),
		NewValue: accountEvent{Email: recordedAddress(user.Email), Actor: actorSelf},
	})

	// Clear auth cookies so the browser can't reuse them.
	h.tokenService.ClearTokenCookies(w)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "account deleted"})
}

// UpdateCurrentUser handles PUT /api/v1/auth/me. Changing the email address
// resets verification, and changing the password ends every open session.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) UpdateCurrentUser(w http.ResponseWriter, r *http.Request) {
	user, ok := httpx.ContextGetAuthenticatedUser(r)
	if !ok {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	// Captured before validation, which writes the new address straight onto
	// user: after that line the previous address exists nowhere. It is the one
	// thing an email change destroys, and the one thing the account's owner
	// needs to name if the change was not theirs.
	previousEmail := user.Email

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

	v := validator.New()

	if input.Email != nil {
		v.Check(*input.Email != "", "email", "must not be empty")
		v.Check(validator.Matches(*input.Email, validator.EmailRX), "email", "must be a valid email address")
		if *input.Email != user.Email {
			// Changing email requires re-verification to prevent claiming unowned addresses.
			user.Email = *input.Email
			user.EmailVerified = false
		}
	}
	if input.FirstName != nil {
		v.Check(*input.FirstName != "", "first_name", "must not be empty")
		v.Check(len(*input.FirstName) <= 100, "first_name", "must not be more than 100 characters")
		user.FirstName = *input.FirstName
	}
	if input.LastName != nil {
		v.Check(*input.LastName != "", "last_name", "must not be empty")
		v.Check(len(*input.LastName) <= 100, "last_name", "must not be more than 100 characters")
		user.LastName = *input.LastName
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
				user.Phone = normalized
			case *input.Phone != user.Phone:
				v.AddError("phone", "must be a valid phone number (E.164 format, e.g. +5491112345678)")
			}
			// The remaining case is a number that cannot be normalised and is
			// byte-for-byte what is already stored — one that predates this
			// check. It is left exactly as found, because the alternative is a
			// 422 at someone who came to change their surname, over a field they
			// never touched, with no way to save the form until they also fix a
			// number they may no longer use. A deliberate edit still has to be
			// valid; only standing still is free. Such a number stays
			// undeliverable until its owner corrects it, which no validation on
			// this path can force — that is a data question, not a request one.
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
		match, err := user.PasswordMatches(*input.CurrentPassword)
		if err != nil {
			h.respond.ServerError(w, r, err)
			return
		}
		if !match {
			h.respond.InvalidCredentials(w, r)
			return
		}
	}

	err = h.users.Update(r.Context(), user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			h.respond.FailedValidation(w, r, v.Errors)
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.EditConflict(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}
	h.cache.InvalidateUser(r.Context(), user.ID)

	// The address is the account's recovery channel: reset links go wherever it
	// points. Someone holding a stolen session can move it to an address they
	// control, verify that address, and reset the password from it — and the
	// entries below would then show the reset without showing the redirect that
	// made it possible, on an account whose owner can no longer say what their
	// address used to be. The other profile fields are not recorded; a surname
	// is data about a person, not control over an account.
	if user.Email != previousEmail {
		//nolint:contextcheck // see the note on Record in Login.
		h.record(r, audit.Entry{
			UserID:   accountID(user.ID),
			Action:   actionEmailChange,
			EntityID: accountID(user.ID),
			OldValue: accountEvent{Email: recordedAddress(previousEmail)},
			NewValue: accountEvent{Email: recordedAddress(user.Email)},
		})
	}

	// If email changed, send verification email for the new address.
	if !user.EmailVerified && input.Email != nil {
		//nolint:contextcheck // autoResendVerification intentionally uses its own detached
		// 10s timeout (not r.Context()) so the cooldown-checked resend still completes even
		// though the rest of this handler continues independently of it.
		h.autoResendVerification(user)
	}

	if passwordChange {
		err = user.SetPassword(*input.NewPassword)
		if err != nil {
			h.respond.ServerError(w, r, err)
			return
		}

		err = h.users.UpdatePassword(r.Context(), user.ID, user.PasswordHash)
		if err != nil {
			h.respond.ServerError(w, r, err)
			return
		}

		// Recorded the moment the credential changed, not after the sessions are
		// cleaned up below. The hash was overwritten in place, so this is the
		// only surviving record that it happened and when; the revocations that
		// follow can each fail and report themselves, and none of them can
		// un-change the password this entry describes.
		//
		//nolint:contextcheck // see the note on Record in Login.
		h.record(r, audit.Entry{
			UserID:   accountID(user.ID),
			Action:   actionPasswordChange,
			EntityID: accountID(user.ID),
			NewValue: accountEvent{Email: recordedAddress(user.Email)},
		})

		err = h.tokens.DeleteAllForUser(r.Context(), user.ID)
		if err != nil {
			h.respond.ServerError(w, r, err)
			return
		}

		if blErr := h.blacklist.InvalidateUserTokens(r.Context(), user.ID); blErr != nil {
			h.revocationFailed("password change", user.ID.String(), blErr)
		}
		h.cache.InvalidateUser(r.Context(), user.ID)
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"user": user})
}

func (h *Handler) autoResendVerification(user *data.User) {
	plaintext := uuid.New().String()
	hash := hashRefreshToken(plaintext)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := h.verifications.InsertWithCooldown(ctx, user.ID, hash)
	if err != nil {
		return // Cooldown active or other error — silently ignore.
	}

	verifyURL := h.cfg.FrontendURL + "/verify-email?token=" + plaintext

	h.notify.EmailVerification(notifications.VerificationEmail{
		To: user.Email, FirstName: user.FirstName, VerifyURL: verifyURL,
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

	// Recorded here, before the lookup, and unconditionally.
	//
	// This handler has five exits — no such account, unverified, inactive,
	// cooldown, sent — and all five answer identically, which is the only thing
	// stopping the endpoint confirming which addresses have accounts. Recording
	// after the branch would rebuild that oracle inside the trail: an entry for
	// the real addresses and silence for the rest. Placing the call above every
	// branch means no later edit can make it conditional on the account
	// existing without deleting this comment first.
	//
	// UserID is nil for the same reason, and the address in the value is what
	// the entry is for: a burst of requests aimed at one account, or at
	// thousands, is visible nowhere else.
	//
	//nolint:contextcheck // see the note on Record in Login.
	h.record(r, audit.Entry{
		Action:   actionPasswordResetRequest,
		NewValue: accountEvent{Email: recordedAddress(input.Email)},
	})

	genericMsg := "if the email exists, a password reset link has been sent"

	user, err := h.users.GetByEmail(r.Context(), input.Email)
	if err != nil || !user.EmailVerified || !user.IsActive {
		// Always return success to prevent email enumeration.
		h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": genericMsg})
		return
	}

	plaintext := uuid.New().String()
	hash := hashRefreshToken(plaintext)

	err = h.resets.InsertWithCooldown(r.Context(), user.ID, hash)
	if err != nil {
		// The caller sees the same generic success either way, and deliberately
		// so: it is the one thing keeping this endpoint from confirming which
		// addresses have accounts. A 500 here would confirm it, since only a
		// known, verified, active account ever reaches this line.
		//
		// What changes is that a real store failure is no longer indistinguishable
		// from a cooldown on the inside. An active cooldown is ordinary traffic —
		// somebody clicked twice — and stays quiet. A store failure meant every
		// password reset silently failed while each user was told their mail was
		// on its way, and nothing was written down anywhere; the outage in the
		// one flow people reach for when they are already locked out looked
		// exactly like a quiet afternoon. ResendVerification, the sibling flow,
		// has always drawn this line; this is the same line.
		if !errors.Is(err, data.ErrCooldownActive) {
			h.logger.Error("forgot-password: could not store the reset token — no reset email was sent",
				"error", err, "user_id", user.ID)
		}
		h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": genericMsg})
		return
	}

	resetURL := h.cfg.FrontendURL + "/reset-password?token=" + plaintext

	h.notify.PasswordReset(notifications.PasswordResetEmail{
		To: user.Email, FirstName: user.FirstName, ResetURL: resetURL,
	})

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": genericMsg})
}

// ResetPassword handles POST /api/v1/auth/reset-password. The link is
// single-use, and using it ends every session the account had open.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.Token != "", "token", "must be provided")
	v.Check(input.Password != "", "password", "must be provided")
	v.Check(len(input.Password) >= 8, "password", "must be at least 8 characters")
	v.Check(len(input.Password) <= 72, "password", "must not be more than 72 characters")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	tokenHash := hashRefreshToken(input.Token)

	resetToken, err := h.resets.GetByHash(r.Context(), tokenHash)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.Error(w, r, http.StatusBadRequest, "invalid or expired reset token")
		default:
			// A store failure is not a bad token, and saying so sends the one
			// person holding a valid link away to request another — which
			// fails the same way, because the database is what is broken. The
			// 400 is also indistinguishable from a used link in the logs, so
			// the outage looks like ordinary traffic. VerifyEmail, the sibling
			// flow, has always split these two; this is the same split.
			h.respond.ServerError(w, r, err)
		}
		return
	}

	user, err := h.users.GetByID(r.Context(), resetToken.UserID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	if !user.IsActive {
		h.respond.Error(w, r, http.StatusBadRequest, "invalid or expired reset token")
		return
	}

	err = user.SetPassword(input.Password)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	err = h.users.UpdatePassword(r.Context(), user.ID, user.PasswordHash)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	// The takeover step, if this was one: whoever reached this line replaced the
	// account's credential without ever presenting the old one.
	//
	// UserID is nil, and the distinction from actionPasswordChange is the point.
	// That one is written for a request carrying a session this handler
	// authenticated; this route is public, and what authorized the change was a
	// single-use token out of an email — control of an address, which is not the
	// same claim. The two actions being separate is how a reader tells "the
	// owner changed their password" from "somebody with the reset link did".
	//
	//nolint:contextcheck // see the note on Record in Login.
	h.record(r, audit.Entry{
		Action:   actionPasswordReset,
		EntityID: accountID(user.ID),
		NewValue: accountEvent{Email: recordedAddress(user.Email)},
	})

	// Clear the login lockout, because a locked-out account is exactly the
	// account whose owner is likely to be standing here: being locked out is
	// what sends someone to the reset link in the first place. UpdatePassword
	// touches only the hash, leaving failed_login_attempts and locked_until as
	// they were, so without this the brand-new password is refused for up to
	// four hours — and refused as "invalid credentials", the same generic answer
	// Login now gives a locked account, with nothing to explain why the password
	// they just set does not work.
	//
	// Nothing is lost by clearing it: the lockout exists to stop someone
	// guessing a password, and whoever got here proved control of the email
	// address and then chose a new one. There is no longer a guess to stop.
	if resetErr := h.users.ResetFailedAttempts(r.Context(), user.ID); resetErr != nil {
		h.logger.Error("reset-password: failed to clear the login lockout — the new password may still be refused",
			"error", resetErr, "user_id", user.ID)
	}

	// Invalidate all sessions and reset tokens.
	if delErr := h.tokens.DeleteAllForUser(r.Context(), user.ID); delErr != nil {
		h.logger.Error("reset-password: failed to invalidate sessions — old sessions may remain active",
			"error", delErr, "user_id", user.ID)
	}
	if blErr := h.blacklist.InvalidateUserTokens(r.Context(), user.ID); blErr != nil {
		h.revocationFailed("password reset", user.ID.String(), blErr)
	}
	h.cache.InvalidateUser(r.Context(), user.ID)
	if delErr := h.resets.DeleteByUser(r.Context(), user.ID); delErr != nil {
		h.logger.Error("reset-password: failed to delete reset tokens", "error", delErr, "user_id", user.ID)
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "password reset successfully"})
}
