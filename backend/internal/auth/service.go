package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/googleid"
	"github.com/stodulski/vibe-server/internal/notifications"
)

// The refusals this module raises. Each one is a decision the handler turns
// into a status and a sentence; none of them carries anything an
// unauthenticated caller could learn from.
var (
	// ErrInvalidCredentials is every refusal of a sign-in, whatever caused it.
	// A wrong password, an unknown address, a locked account, an unverified one
	// and a deactivated one all raise it, because telling them apart is exactly
	// what turns this endpoint into an oracle for which addresses have accounts.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidSession reports a refresh that established nothing: no token, an
	// unknown one, a rotated one, or an account that is no longer active.
	ErrInvalidSession = errors.New("invalid session")
	// ErrInvalidToken reports a verification or reset token that is missing,
	// unknown, expired or spent.
	ErrInvalidToken = errors.New("invalid or expired token")
	// ErrEditConflict reports that the account row moved out from under a read.
	ErrEditConflict = fmt.Errorf("account changed before the update: %w", data.ErrEditConflict)
	// ErrActiveBookings reports that an account cannot be deleted because a
	// venue it owns still has live bookings.
	ErrActiveBookings = errors.New("account still has active bookings")
	// ErrAccountExists reports that the address a Google sign-up is completing
	// was claimed between the two halves of the flow.
	ErrAccountExists = errors.New("account already exists")
	// ErrGoogleRejected reports that Google refused the ID token the client
	// presented.
	ErrGoogleRejected = errors.New("google rejected the credential")
)

// Actor is who a request came from, as the handler read it off. The service
// needs it for the audit trail and for the one log line that names the peer.
type Actor struct {
	// IP is the address the audit trail records.
	IP string
	// RemoteAddr is the peer as the server saw it, logged when an
	// already-rotated refresh token is presented.
	RemoteAddr string
}

// Session is a freshly established session: the account it belongs to and the
// two tokens the handler puts into cookies.
type Session struct {
	User         *authstore.User
	AccessToken  string
	RefreshToken string
}

// Service holds this module's rules: what establishes a session, what ends one,
// what may change an account, and what every one of those writes to the trail.
//
//nolint:govet // field order follows the dependency list, not padding
type Service struct {
	// The four account ports, all satisfied by the one store Dependencies
	// carries. They are separate fields so a rule reads through the port it
	// actually needs: what a call site touches is visible at the call site.
	users         UserReader
	userWrites    UserWriter
	credentials   CredentialStore
	lockout       LockoutStore
	tokens        TokenStore
	tokenService  *TokenService
	verifications VerificationStore
	resets        PasswordResetStore
	complexes     OwnershipReader
	bookings      BookingReader
	blacklist     Blacklist
	notify        Notifier
	cache         UserCache
	audit         Recorder
	turnstile     TurnstileVerifier
	google        GoogleVerifier
	identities    IdentityStore
	logger        *slog.Logger
	cfg           Config
}

// NewService returns a Service.
//
// A nil recorder is refused here, as internal/payments refuses one, and for a
// sharper version of the same reason. Every route in this module is reachable
// by a stranger, so the first thing a nil recorder would break is sign-in — in
// production, for everyone, and only once traffic arrived. Failing at
// construction moves that discovery to the deploy that caused it.
//
// It is refused rather than made nil-safe. A trail that quietly drops entries
// is the one kind of broken this table cannot survive: it still answers every
// question and every answer is short, and "there is no record of a sign-in from
// that address" then means nothing at all.
func NewService(d Dependencies, cfg Config) *Service {
	if d.Audit == nil {
		panic("auth: NewService needs an audit recorder; a session and credential trail is not optional")
	}
	turnstileVerifier := d.Turnstile
	if turnstileVerifier == nil {
		// A nil interface value cannot be asked Enabled() without a type check
		// at every call site; disabledTurnstile makes "no verifier configured"
		// and "verifier configured but reports disabled" the same code path
		// everywhere below.
		turnstileVerifier = disabledTurnstile{}
	}
	googleVerifier := d.Google
	if googleVerifier == nil {
		// Same reasoning as disabledTurnstile above.
		googleVerifier = disabledGoogle{}
	}
	return &Service{
		users:       d.Users,
		userWrites:  d.Users,
		credentials: d.Users,
		lockout:     d.Users,
		tokens:      d.Tokens,
		tokenService: NewTokenService(TokenServiceConfig{
			JWTSecret:    cfg.JWTSecret,
			CookieDomain: cfg.CookieDomain,
			Environment:  cfg.Environment,
		}),
		verifications: d.Verifications,
		resets:        d.Resets,
		complexes:     d.Complexes,
		bookings:      d.Bookings,
		blacklist:     d.Blacklist,
		notify:        d.Notify,
		cache:         d.Cache,
		audit:         d.Audit,
		turnstile:     turnstileVerifier,
		google:        googleVerifier,
		identities:    d.Identities,
		logger:        d.Logger,
		cfg:           cfg,
	}
}

// TurnstileEnabled reports whether Cloudflare Turnstile verification is
// configured at all. When it is not, the handler skips verification entirely —
// this module is not allowed to force self-hosters into using Turnstile.
func (s *Service) TurnstileEnabled() bool { return s.turnstile.Enabled() }

// VerifyTurnstile checks a Turnstile token, reporting why it was rejected with
// the turnstile package's own sentinels.
func (s *Service) VerifyTurnstile(ctx context.Context, token, remoteIP string) error {
	return s.turnstile.Verify(ctx, token, remoteIP)
}

// GoogleEnabled reports whether Google sign-in is configured at all.
func (s *Service) GoogleEnabled() bool { return s.google.Enabled() }

// RegisterInput is a validated registration. Phone is already in E.164 form.
type RegisterInput struct {
	Email     string
	Password  string
	FirstName string
	LastName  string
	Phone     string
}

// Register creates an account and sends its verification email.
//
// It reports success whether or not the address is taken, and tells the
// existing account by email that someone tried — otherwise the endpoint is an
// oracle for which addresses have accounts.
func (s *Service) Register(ctx context.Context, in RegisterInput) error {
	user := &authstore.User{
		Email:     in.Email,
		FirstName: in.FirstName,
		LastName:  in.LastName,
		Phone:     in.Phone,
		Role:      "owner",
	}

	if err := user.SetPassword(in.Password, s.cfg.PasswordHashCost); err != nil {
		return err
	}

	err := s.userWrites.Insert(ctx, user)
	if err != nil {
		if !errors.Is(err, authstore.ErrDuplicateEmail) {
			return err
		}
		s.logger.Info("register: duplicate email attempt")

		// Notify the existing account owner so they know someone tried to
		// register.
		//
		// The greeting is the account's own first name, read back here, and
		// never the name the caller typed. This email goes to the person who
		// already owns the address, and its whole subject is that a stranger
		// just touched their account — greeting them with the name that
		// stranger typed put an attacker's chosen words in Vibe's voice,
		// addressed to the victim. A lookup that fails greets them without a
		// name; the email says the same thing either way.
		existingName := ""
		if existing, lookupErr := s.users.GetByEmail(ctx, in.Email); lookupErr == nil && existing != nil {
			existingName = existing.FirstName
		}
		s.notify.DuplicateRegistration(notifications.DuplicateRegistrationEmail{
			To:        in.Email,
			FirstName: existingName,
			LoginURL:  s.cfg.FrontendURL + "/login",
			ResetURL:  s.cfg.FrontendURL + "/forgot-password",
		})
		return nil
	}

	// In development, auto-verify the email so E2E tests can login immediately.
	if s.cfg.Environment == "development" {
		if verifyErr := s.credentials.SetEmailVerified(ctx, user.ID); verifyErr != nil {
			s.logger.Error("dev: auto-verify failed", "error", verifyErr, "user_id", user.ID)
		}
	}

	plaintext := uuid.New().String()
	hash := hashRefreshToken(plaintext)

	if err := s.verifications.Insert(ctx, user.ID, hash); err != nil {
		return err
	}

	s.notify.EmailVerification(notifications.VerificationEmail{
		To: user.Email, FirstName: user.FirstName, VerifyURL: s.cfg.FrontendURL + "/verify-email?token=" + plaintext,
	})

	return nil
}

// Login establishes a session for a password sign-in.
//
// Every refusal answers ErrInvalidCredentials and is recorded identically; see
// the comments below for why a locked account is not told apart from a wrong
// password. A store failure is neither: nobody presented bad credentials, the
// database could not answer, and it is not written down as a failed sign-in.
//
// It is one cohesive decision and splitting it would relocate sequential steps
// into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (s *Service) Login(ctx context.Context, actor Actor, email, password string) (*Session, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			// Run for its cost, not its answer: this compare must always fail.
			_ = authstore.ComparePassword(authstore.DummyPasswordHash(s.cfg.PasswordHashCost), password) //nolint:errcheck // see above
			return nil, s.loginFailed(actor, email)
		}
		return nil, err
	}

	// Always perform bcrypt comparison first for timing safety, even if the
	// account is locked.
	match, err := user.PasswordMatches(password)
	if err != nil {
		return nil, err
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
		return nil, s.loginFailed(actor, email)
	}

	if !match {
		// Increment failed attempts (handles progressive lockout).
		if incErr := s.lockout.IncrementFailedAttempts(ctx, user.ID); incErr != nil {
			s.logger.Error("login: failed to increment login attempts", "error", incErr, "user_id", user.ID)
		}
		// The cached record carries failed_login_attempts and locked_until,
		// and this write just moved both. Leaving the entry in place serves
		// the pre-attempt counts for the next ten minutes (RED-03).
		//
		// It is harmless today only because the lockout check above reads the
		// row through GetByEmail rather than through the cache — which is a
		// property of one call site, not of the cache, and the entry is what
		// the next reader of those fields would get.
		s.cache.InvalidateUser(ctx, user.ID)
		return nil, s.loginFailed(actor, email)
	}

	// Successful password match — reset failed attempts.
	if user.FailedLoginAttempts > 0 {
		if resetErr := s.lockout.ResetFailedAttempts(ctx, user.ID); resetErr != nil {
			s.logger.Error("login: failed to reset login attempts", "error", resetErr, "user_id", user.ID)
		}
		// Same reason as the increment above, and it cannot be left to
		// startSession: the three refusals between here and there — an
		// unverified address, a deactivated account, a lockout — all return
		// without ever reaching it.
		s.cache.InvalidateUser(ctx, user.ID)
	}

	if !user.EmailVerified {
		// Return the same generic refusal to prevent account enumeration.
		// Silently resend the verification email as a convenience.
		//
		//nolint:contextcheck // autoResendVerification intentionally uses its own detached
		// 10s timeout (not the request's) so the cooldown-checked resend still completes even
		// though the response is written immediately after.
		s.autoResendVerification(user)
		return nil, s.loginFailed(actor, email)
	}

	if !user.IsActive {
		return nil, s.loginFailed(actor, email)
	}

	return s.startSession(ctx, actor, user, "")
}

// startSession mints an access token, rotates in a fresh refresh token, records
// the sign-in, and hands the two tokens back for the handler to set as cookies.
//
// method distinguishes how the session was established for the audit trail
// (accountEvent.Method) — empty for the ordinary password login, "google" for
// Sign in with Google, both of which reuse this exactly rather than duplicating
// Login's session-issuing steps.
func (s *Service) startSession(ctx context.Context, actor Actor, user *authstore.User, method string) (*Session, error) {
	s.cache.InvalidateUser(ctx, user.ID)

	accessToken, err := s.tokenService.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		return nil, err
	}

	refreshPlain, refreshHash := generateRefreshToken()

	if err := s.tokens.InsertRefreshToken(ctx, user.ID, refreshHash, refreshTokenExpiry); err != nil {
		return nil, err
	}

	// Recorded once the session exists, not once the password matched: the two
	// writes above can each refuse, and an entry saying a session was
	// established when none was is worse than no entry at all.
	//
	// UserID is nil and is not read from the request, unlike the entries written
	// from the guarded routes. This request did not carry a session; it created
	// one, and a stale cookie on it would not change that. Pinning it means a
	// sign-in entry looks the same whether or not the browser still had an old
	// session lying around. The account is named by EntityID either way.
	//
	//nolint:contextcheck // Record deliberately detaches: the entry describes
	// something that already happened, so it must still be written when the
	// client hangs up mid-response. See audit.Recorder.Record.
	s.record(actor, audit.Entry{
		Action:   actionLogin,
		EntityID: accountID(user.ID),
		NewValue: accountEvent{Email: recordedAddress(user.Email), Method: method},
	})

	return &Session{User: user, AccessToken: accessToken, RefreshToken: refreshPlain}, nil
}

// loginFailed records one sign-in attempt that established no session and
// returns the generic refusal.
//
// Every exit that refuses a sign-in goes through here, so that the uniformity
// the trail depends on is structural rather than a convention five call sites
// have to keep. attempted is the address as typed, which is the whole value of
// the entry: the actor is unknown and deliberately unnamed, so the address is
// the only thing linking one attempt to the next.
func (s *Service) loginFailed(actor Actor, attempted string) error {
	s.record(actor, audit.Entry{
		Action:   actionLoginFailed,
		NewValue: accountEvent{Email: recordedAddress(attempted)},
	})
	return ErrInvalidCredentials
}

// VerifyEmail consumes the token from the emailed link.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	tokenHash := hashRefreshToken(token)

	vToken, err := s.verifications.GetByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return ErrInvalidToken
		}
		return err
	}

	// SetEmailVerified is idempotent — safe against concurrent requests.
	if err := s.credentials.SetEmailVerified(ctx, vToken.UserID); err != nil {
		return err
	}
	s.cache.InvalidateUser(ctx, vToken.UserID)

	// Delete tokens immediately to reduce exposure surface.
	if delErr := s.verifications.DeleteByUser(ctx, vToken.UserID); delErr != nil {
		s.logger.Error("failed to delete verification tokens", "error", delErr)
	}

	return nil
}

// ResendVerification sends another verification link.
//
// It is rate-limited by the store's cooldown, and reports success either way:
// an unknown address, an already-verified account and a live cooldown are all
// silent, because telling them apart would confirm which addresses have
// accounts.
func (s *Service) ResendVerification(ctx context.Context, email string) error {
	// A lookup failure is deliberately swallowed rather than reported: an
	// unknown address must be indistinguishable from a known one here, and a
	// store failure that answered differently would say which.
	//
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil || user.EmailVerified {
		return nil //nolint:nilerr // see the note above
	}

	plaintext := uuid.New().String()
	hash := hashRefreshToken(plaintext)

	// Atomic: deletes stale tokens (>3 min) and inserts a new one only if the
	// cooldown has passed.
	if err := s.verifications.InsertWithCooldown(ctx, user.ID, hash); err != nil {
		if errors.Is(err, data.ErrCooldownActive) {
			return nil
		}
		return err
	}

	s.notify.EmailVerification(notifications.VerificationEmail{
		To: user.Email, FirstName: user.FirstName, VerifyURL: s.cfg.FrontendURL + "/verify-email?token=" + plaintext,
	})

	return nil
}

// Refresh rotates the refresh token and mints a new access token.
//
// Presenting an already-rotated token means a copy is in circulation, so every
// session for that account is revoked rather than just this one refused.
//
// It is one cohesive decision and splitting it would relocate sequential steps
// into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (s *Service) Refresh(ctx context.Context, actor Actor, presented string) (accessToken, refreshToken string, err error) {
	oldHash := hashRefreshToken(presented)

	storedToken, err := s.tokens.GetRefreshToken(ctx, oldHash)
	if err != nil {
		if !errors.Is(err, data.ErrRecordNotFound) {
			return "", "", err
		}
		s.handleUnknownRefreshToken(ctx, actor, oldHash)
		return "", "", ErrInvalidSession
	}

	user, err := s.users.GetByID(ctx, storedToken.UserID)
	if err != nil {
		return "", "", err
	}

	if !user.IsActive {
		return "", "", ErrInvalidSession
	}

	s.cache.InvalidateUser(ctx, user.ID)

	accessToken, err = s.tokenService.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		return "", "", err
	}

	refreshPlain, refreshHash := generateRefreshToken()

	if err := s.tokens.InsertRefreshToken(ctx, user.ID, refreshHash, refreshTokenExpiry); err != nil {
		return "", "", err
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
	// works and no session was issued, because the cookies are set by the
	// handler afterwards. The stored replacement is then unreachable and expires
	// on its own.
	if err := s.tokens.MarkRefreshTokenUsed(ctx, oldHash); err != nil {
		return "", "", err
	}

	return accessToken, refreshPlain, nil
}

// handleUnknownRefreshToken decides what an unrecognised refresh token means: a
// sibling tab that rotated moments ago, a replay of a spent token, or nothing
// this service has ever seen. Only the middle case revokes anything.
func (s *Service) handleUnknownRefreshToken(ctx context.Context, actor Actor, oldHash []byte) {
	usedToken, usedErr := s.tokens.GetUsedRefreshToken(ctx, oldHash)
	if usedErr != nil {
		return
	}

	if time.Since(usedToken.UsedAt) < refreshReuseGrace {
		// A sibling tab rotated this token moments ago; see refreshReuseGrace.
		// Refuse this request, revoke nothing.
		s.logger.Info("refresh token presented again within the concurrent-refresh grace window",
			"user_id", usedToken.UserID,
			"used_ago", time.Since(usedToken.UsedAt).String(),
		)
		return
	}

	// REUSE DETECTED: this token was already rotated. Nuclear option:
	// invalidate ALL tokens for this user.
	s.logger.Error("SECURITY: refresh token reuse detected, invalidating all sessions",
		"user_id", usedToken.UserID,
		"remote_addr", actor.RemoteAddr,
	)
	if delErr := s.tokens.DeleteAllForUser(ctx, usedToken.UserID); delErr != nil {
		s.logger.Error("SECURITY: failed to invalidate sessions after token reuse",
			"user_id", usedToken.UserID,
			"error", delErr,
		)
	}

	// Deleting the refresh tokens stops new access tokens being minted, but the
	// ones already issued are self-contained JWTs valid for their full lifetime.
	// Without this the holder of a stolen token keeps access for up to
	// accessTokenExpiry after the theft was detected.
	if blErr := s.blacklist.InvalidateUserTokens(ctx, usedToken.UserID); blErr != nil {
		s.revocationFailed("refresh-token reuse", usedToken.UserID.String(), blErr)
	}

	// Every session on the account was just destroyed, and nobody asked for it.
	// Until now the only record of that was the log line above, so the account
	// holder's "why was I signed out of everything, everywhere?" had no durable
	// answer.
	//
	// UserID stays nil. The account is named by EntityID, which is the party
	// this happened to; who presented the rotated token is exactly what is not
	// known, and putting the victim's id in the actor column would record the
	// theft as something they did.
	s.record(actor, audit.Entry{
		Action:   actionRefreshReuse,
		EntityID: accountID(usedToken.UserID),
		NewValue: accountEvent{Actor: actorUnknown},
	})
}

// Logout ends the session the request carried. Every step is best effort: the
// handler clears the cookies and reports success regardless, so that a user
// whose session already expired can still sign out.
//
// user is the authenticated account when the request carried one, and nil
// otherwise — see the note below on why only the former is recorded.
func (s *Service) Logout(ctx context.Context, actor Actor, refreshToken, accessToken string, user *authstore.User) {
	if refreshToken != "" {
		if delErr := s.tokens.DeleteRefreshToken(ctx, hashRefreshToken(refreshToken)); delErr != nil {
			s.logger.Error("failed to delete refresh token on logout", "error", delErr)
		}
	}

	// Blacklist the access token so it can't be used until it expires.
	if accessToken != "" {
		if claims, parseErr := s.tokenService.ValidateAccessToken(accessToken); parseErr == nil {
			if blErr := s.blacklist.BlacklistToken(ctx, accessToken, claims.ExpiresAt.Time); blErr != nil {
				s.revocationFailed("logout", claims.Subject, blErr)
			}
		}
	}

	// Recorded only when the request carried a session the handler could name.
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
	if user != nil {
		s.record(actor, audit.Entry{
			UserID:   accountID(user.ID),
			Action:   actionLogout,
			EntityID: accountID(user.ID),
			NewValue: accountEvent{Email: recordedAddress(user.Email)},
		})
	}
}

// DeleteAccount destroys an account and everything cascading off it. It is
// refused while any complex the account owns still has live bookings, because
// deleting it would strand the clients holding them.
//
// accessToken is the credential the request authenticated with, blacklisted so
// that nothing issued to the account keeps working.
func (s *Service) DeleteAccount(ctx context.Context, actor Actor, user *authstore.User, accessToken string) error {
	// Check all owned complexes for active bookings.
	complexes, err := s.complexes.GetByOwner(ctx, user.ID)
	if err != nil {
		return err
	}
	for _, c := range complexes {
		hasActive, err := s.bookings.HasActiveBookings(ctx, c.ID)
		if err != nil {
			return err
		}
		if hasActive {
			return ErrActiveBookings
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
	if err := s.userWrites.Delete(ctx, user.ID); err != nil {
		return err
	}
	s.cache.InvalidateUser(ctx, user.ID)

	// The account is gone; now nothing issued to it may keep working.
	if blErr := s.blacklist.InvalidateUserTokens(ctx, user.ID); blErr != nil {
		s.revocationFailed("delete account", user.ID.String(), blErr)
	}
	if err := s.tokens.DeleteAllForUser(ctx, user.ID); err != nil {
		s.logger.Error("delete account: failed to invalidate sessions", "error", err, "user_id", user.ID)
	}

	// Blacklist the current access token.
	if accessToken != "" {
		if claims, parseErr := s.tokenService.ValidateAccessToken(accessToken); parseErr == nil {
			if blErr := s.blacklist.BlacklistToken(ctx, accessToken, claims.ExpiresAt.Time); blErr != nil {
				s.revocationFailed("delete account", claims.Subject, blErr)
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
	s.record(actor, audit.Entry{
		Action:   actionAccountDelete,
		EntityID: accountID(user.ID),
		NewValue: accountEvent{Email: recordedAddress(user.Email), Actor: actorSelf},
	})

	return nil
}

// UpdateInput is a validated change to the caller's own account. A nil field
// keeps its current value. Phone is already normalised, or nil when the stored
// value is being left exactly as found.
type UpdateInput struct {
	Email     *string
	FirstName *string
	LastName  *string
	Phone     *string
	// CurrentPassword and NewPassword are both set or both nil; the handler
	// refuses any other combination.
	CurrentPassword *string
	NewPassword     *string
}

// UpdateCurrentUser applies a change to the caller's own account. Changing the
// email address resets verification, and changing the password ends every open
// session.
//
// It is one cohesive write and splitting it would relocate sequential steps
// into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (s *Service) UpdateCurrentUser(ctx context.Context, actor Actor, user *authstore.User, in UpdateInput) (*authstore.User, error) {
	// Captured before the change is applied: after it the previous address
	// exists nowhere. It is the one thing an email change destroys, and the one
	// thing the account's owner needs to name if the change was not theirs.
	previousEmail := user.Email

	if in.Email != nil && *in.Email != user.Email {
		// Changing email requires re-verification to prevent claiming unowned
		// addresses.
		user.Email = *in.Email
		user.EmailVerified = false
	}
	if in.FirstName != nil {
		user.FirstName = *in.FirstName
	}
	if in.LastName != nil {
		user.LastName = *in.LastName
	}
	if in.Phone != nil {
		user.Phone = *in.Phone
	}

	passwordChange := in.NewPassword != nil
	if passwordChange {
		match, err := user.PasswordMatches(*in.CurrentPassword)
		if err != nil {
			return nil, err
		}
		if !match {
			return nil, ErrInvalidCredentials
		}
	}

	if err := s.userWrites.Update(ctx, user); err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, ErrEditConflict
		}
		return nil, err
	}
	s.cache.InvalidateUser(ctx, user.ID)

	// The address is the account's recovery channel: reset links go wherever it
	// points. Someone holding a stolen session can move it to an address they
	// control, verify that address, and reset the password from it — and the
	// entries below would then show the reset without showing the redirect that
	// made it possible, on an account whose owner can no longer say what their
	// address used to be. The other profile fields are not recorded; a surname
	// is data about a person, not control over an account.
	if user.Email != previousEmail {
		s.record(actor, audit.Entry{
			UserID:   accountID(user.ID),
			Action:   actionEmailChange,
			EntityID: accountID(user.ID),
			OldValue: accountEvent{Email: recordedAddress(previousEmail)},
			NewValue: accountEvent{Email: recordedAddress(user.Email)},
		})
	}

	// If the email changed, send a verification email for the new address.
	if !user.EmailVerified && in.Email != nil {
		//nolint:contextcheck // autoResendVerification intentionally uses its own detached
		// 10s timeout so the cooldown-checked resend still completes even though the rest
		// of this method continues independently of it.
		s.autoResendVerification(user)
	}

	if passwordChange {
		if err := user.SetPassword(*in.NewPassword, s.cfg.PasswordHashCost); err != nil {
			return nil, err
		}
		if err := s.credentials.UpdatePassword(ctx, user.ID, user.PasswordHash); err != nil {
			return nil, err
		}

		// Recorded the moment the credential changed, not after the sessions are
		// cleaned up below. The hash was overwritten in place, so this is the
		// only surviving record that it happened and when; the revocations that
		// follow can each fail and report themselves, and none of them can
		// un-change the password this entry describes.
		s.record(actor, audit.Entry{
			UserID:   accountID(user.ID),
			Action:   actionPasswordChange,
			EntityID: accountID(user.ID),
			NewValue: accountEvent{Email: recordedAddress(user.Email)},
		})

		if err := s.tokens.DeleteAllForUser(ctx, user.ID); err != nil {
			return nil, err
		}
		if blErr := s.blacklist.InvalidateUserTokens(ctx, user.ID); blErr != nil {
			s.revocationFailed("password change", user.ID.String(), blErr)
		}
		s.cache.InvalidateUser(ctx, user.ID)
	}

	return user, nil
}

// autoResendVerification sends another verification link on its own detached
// timeout, and says nothing when the store's cooldown refuses.
func (s *Service) autoResendVerification(user *authstore.User) {
	plaintext := uuid.New().String()
	hash := hashRefreshToken(plaintext)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.verifications.InsertWithCooldown(ctx, user.ID, hash); err != nil {
		return // Cooldown active or other error — silently ignore.
	}

	s.notify.EmailVerification(notifications.VerificationEmail{
		To: user.Email, FirstName: user.FirstName, VerifyURL: s.cfg.FrontendURL + "/verify-email?token=" + plaintext,
	})
}

// ForgotPassword sends a reset link. It does the same thing whether or not the
// address has an account, so the caller learns nothing from it.
func (s *Service) ForgotPassword(ctx context.Context, actor Actor, email string) {
	// Recorded here, before the lookup, and unconditionally.
	//
	// This flow has five exits — no such account, unverified, inactive,
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
	s.record(actor, audit.Entry{
		Action:   actionPasswordResetRequest,
		NewValue: accountEvent{Email: recordedAddress(email)},
	})

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil || !user.EmailVerified || !user.IsActive {
		return
	}

	plaintext := uuid.New().String()
	hash := hashRefreshToken(plaintext)

	if err := s.resets.InsertWithCooldown(ctx, user.ID, hash); err != nil {
		// The caller sees the same generic success either way, and deliberately
		// so: it is the one thing keeping this endpoint from confirming which
		// addresses have accounts. A 500 here would confirm it, since only a
		// known, verified, active account ever reaches this line.
		//
		// What changes is that a real store failure is no longer
		// indistinguishable from a cooldown on the inside. An active cooldown is
		// ordinary traffic — somebody clicked twice — and stays quiet. A store
		// failure meant every password reset silently failed while each user was
		// told their mail was on its way, and nothing was written down anywhere;
		// the outage in the one flow people reach for when they are already
		// locked out looked exactly like a quiet afternoon. ResendVerification,
		// the sibling flow, has always drawn this line; this is the same line.
		if !errors.Is(err, data.ErrCooldownActive) {
			s.logger.Error("forgot-password: could not store the reset token — no reset email was sent",
				"error", err, "user_id", user.ID)
		}
		return
	}

	s.notify.PasswordReset(notifications.PasswordResetEmail{
		To: user.Email, FirstName: user.FirstName, ResetURL: s.cfg.FrontendURL + "/reset-password?token=" + plaintext,
	})
}

// ResetPassword consumes a single-use reset link and ends every session the
// account had open.
//
// It is one cohesive write and splitting it would relocate sequential steps
// into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (s *Service) ResetPassword(ctx context.Context, actor Actor, token, password string) error {
	resetToken, err := s.resets.GetByHash(ctx, hashRefreshToken(token))
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return ErrInvalidToken
		}
		// A store failure is not a bad token, and saying so sends the one person
		// holding a valid link away to request another — which fails the same
		// way, because the database is what is broken. A 400 is also
		// indistinguishable from a used link in the logs, so the outage looks
		// like ordinary traffic. VerifyEmail, the sibling flow, has always split
		// these two; this is the same split.
		return err
	}

	user, err := s.users.GetByID(ctx, resetToken.UserID)
	if err != nil {
		return err
	}

	if !user.IsActive {
		return ErrInvalidToken
	}

	if err := user.SetPassword(password, s.cfg.PasswordHashCost); err != nil {
		return err
	}
	if err := s.credentials.UpdatePassword(ctx, user.ID, user.PasswordHash); err != nil {
		return err
	}

	// The takeover step, if this was one: whoever reached this line replaced the
	// account's credential without ever presenting the old one.
	//
	// UserID is nil, and the distinction from actionPasswordChange is the point.
	// That one is written for a request carrying a session the handler
	// authenticated; this route is public, and what authorized the change was a
	// single-use token out of an email — control of an address, which is not the
	// same claim. The two actions being separate is how a reader tells "the
	// owner changed their password" from "somebody with the reset link did".
	s.record(actor, audit.Entry{
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
	if resetErr := s.lockout.ResetFailedAttempts(ctx, user.ID); resetErr != nil {
		s.logger.Error("reset-password: failed to clear the login lockout — the new password may still be refused",
			"error", resetErr, "user_id", user.ID)
	}

	// Invalidate all sessions and reset tokens.
	if delErr := s.tokens.DeleteAllForUser(ctx, user.ID); delErr != nil {
		s.logger.Error("reset-password: failed to invalidate sessions — old sessions may remain active",
			"error", delErr, "user_id", user.ID)
	}
	if blErr := s.blacklist.InvalidateUserTokens(ctx, user.ID); blErr != nil {
		s.revocationFailed("password reset", user.ID.String(), blErr)
	}
	s.cache.InvalidateUser(ctx, user.ID)
	if delErr := s.resets.DeleteByUser(ctx, user.ID); delErr != nil {
		s.logger.Error("reset-password: failed to delete reset tokens", "error", delErr, "user_id", user.ID)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Google sign-in
// ---------------------------------------------------------------------------

// NeedsProfile is the answer to a Google sign-in for an address with no account
// yet: a short-lived profile token carrying what Google already verified — so
// GoogleComplete does not have to trust the client's own copy of it — plus the
// profile fields prefilled for the signup form.
type NeedsProfile struct {
	ProfileToken string
	Email        string
	FirstName    string
	LastName     string
}

// GoogleResult is either an established session or the profile the client still
// has to complete. Exactly one of the two is set.
type GoogleResult struct {
	Session      *Session
	NeedsProfile *NeedsProfile
}

// GoogleSignIn verifies a Google Identity Services ID token and either starts a
// session for the account already registered under that address, or hands back
// a profile token so the client can collect the one field Google never provides
// — a phone number — before GoogleComplete creates the account.
func (s *Service) GoogleSignIn(ctx context.Context, actor Actor, credential string) (*GoogleResult, error) {
	claims, err := s.google.Verify(ctx, credential)
	if err != nil {
		if errors.Is(err, googleid.ErrUnavailable) {
			s.logger.Error("google sign-in: verification unavailable", "error", err)
			return nil, err
		}
		s.logger.Warn("google sign-in: token rejected", "error", err)
		return nil, ErrGoogleRejected
	}

	user, err := s.users.GetByEmail(ctx, claims.Email)
	if err != nil {
		if !errors.Is(err, data.ErrRecordNotFound) {
			return nil, err
		}
		needs, buildErr := s.needsProfile(claims)
		if buildErr != nil {
			return nil, buildErr
		}
		return &GoogleResult{NeedsProfile: needs}, nil
	}

	// Same generic refusal Login gives a wrong password or a locked account —
	// see Login's comment on why a lockout is never told apart from bad
	// credentials in the response, only in what loginFailed records.
	if !user.IsActive || user.IsLocked() {
		return nil, s.loginFailed(actor, claims.Email)
	}

	if user.FailedLoginAttempts > 0 {
		if resetErr := s.lockout.ResetFailedAttempts(ctx, user.ID); resetErr != nil {
			s.logger.Error("google sign-in: failed to reset login attempts", "error", resetErr, "user_id", user.ID)
		}
	}

	// Google already proved this address, so an unverified account is promoted
	// rather than turned away the way Login turns an unverified password sign-in
	// away — but never with its credentials intact. Whoever registered this
	// address with a password never proved they own the address (an unverified
	// account cannot sign in), and carrying that password into the account
	// Google just handed its real owner is the classic pre-hijack: the
	// registrant's password would open the owner's account. So the merge is a
	// credential reset first.
	if !user.EmailVerified {
		if err := s.claimUnverifiedAccount(ctx, actor, user); err != nil {
			return nil, err
		}
	}

	s.linkGoogleIdentity(ctx, user.ID, claims)

	session, err := s.startSession(ctx, actor, user, googleIdentityProvider)
	if err != nil {
		return nil, err
	}
	return &GoogleResult{Session: session}, nil
}

// GoogleCompleteInput is the validated second half of a first-time Google
// sign-in.
type GoogleCompleteInput struct {
	FirstName string
	LastName  string
	Phone     string
}

// GoogleComplete creates the account for a first-time Google sign-in, once the
// client has collected the phone number Google never provides, and starts the
// session.
func (s *Service) GoogleComplete(ctx context.Context, actor Actor, claims *GoogleProfileClaims, in GoogleCompleteInput) (*Session, error) {
	// A second window between the needs_profile answer and this request in which
	// the address was claimed — by an ordinary registration, or by the same
	// Google account completing twice. Insert below catches the same race at the
	// database's unique constraint; this is the common case answered cheaply and
	// without a second unusable password hash minted.
	if _, err := s.users.GetByEmail(ctx, claims.Email); err == nil {
		return nil, ErrAccountExists
	} else if !errors.Is(err, data.ErrRecordNotFound) {
		return nil, err
	}

	user := &authstore.User{
		Email:     claims.Email,
		FirstName: in.FirstName,
		LastName:  in.LastName,
		Phone:     in.Phone,
		Role:      "owner",
	}

	if err := setUnusablePassword(user, s.cfg.PasswordHashCost); err != nil {
		return nil, err
	}

	if err := s.userWrites.Insert(ctx, user); err != nil {
		if errors.Is(err, authstore.ErrDuplicateEmail) {
			return nil, ErrAccountExists
		}
		return nil, err
	}

	// InsertUser (db/queries/users.sql) never writes email_verified, so it comes
	// back false from the RETURNING clause Insert reads onto user — this is what
	// actually persists the true the contract promises, the way Register's
	// development-only auto-verify does.
	if err := s.credentials.SetEmailVerified(ctx, user.ID); err != nil {
		return nil, err
	}
	user.EmailVerified = true

	s.linkGoogleIdentity(ctx, user.ID, &googleid.Claims{Subject: claims.Subject, Email: claims.Email})

	return s.startSession(ctx, actor, user, googleIdentityProvider)
}

// claimUnverifiedAccount hands a never-verified local account to the Google
// account that just proved its address: the password is replaced with one
// nobody knows, every session and refresh token issued for the account is
// revoked, the reset is recorded, and only then is the address marked verified.
// It fails when the reset itself could not be persisted — a session must not
// start on top of a password somebody else chose.
func (s *Service) claimUnverifiedAccount(ctx context.Context, actor Actor, user *authstore.User) error {
	if err := setUnusablePassword(user, s.cfg.PasswordHashCost); err != nil {
		return err
	}
	if err := s.credentials.UpdatePassword(ctx, user.ID, user.PasswordHash); err != nil {
		return err
	}

	// The same action ResetPassword writes, for the same reason: the credential
	// was replaced without the old one being presented. Method names what
	// authorized it — Google's proof of the address, not a reset link.
	s.record(actor, audit.Entry{
		Action:   actionPasswordReset,
		EntityID: accountID(user.ID),
		NewValue: accountEvent{Email: recordedAddress(user.Email), Method: googleIdentityProvider},
	})

	// Best effort, as in ResetPassword: an unverified account could never sign
	// in, so there should be nothing to revoke, and a failure here is logged
	// rather than allowed to keep the owner out.
	if err := s.tokens.DeleteAllForUser(ctx, user.ID); err != nil {
		s.logger.Error("google sign-in: failed to revoke sessions of a claimed account", "error", err, "user_id", user.ID)
	}
	if err := s.blacklist.InvalidateUserTokens(ctx, user.ID); err != nil {
		s.revocationFailed("google claim", user.ID.String(), err)
	}

	if err := s.credentials.SetEmailVerified(ctx, user.ID); err != nil {
		s.logger.Error("google sign-in: failed to mark email verified", "error", err, "user_id", user.ID)
		return nil
	}
	user.EmailVerified = true

	// This function replaced the password hash and flipped email_verified,
	// and both are cached fields (RED-03). startSession invalidates too, and
	// that is not a reason to leave this out: it is one caller away, and the
	// entry between here and there is a record whose credential material is
	// the one this account no longer has.
	s.cache.InvalidateUser(ctx, user.ID)
	return nil
}

// needsProfile builds the answer for a Google sign-in whose address has no
// account yet.
func (s *Service) needsProfile(claims *googleid.Claims) (*NeedsProfile, error) {
	givenName, familyName := claims.GivenName, claims.FamilyName
	if givenName == "" && familyName == "" {
		givenName, familyName = splitName(claims.Name)
	}

	profileToken, err := s.tokenService.GenerateProfileToken(claims.Subject, claims.Email, givenName, familyName)
	if err != nil {
		return nil, err
	}

	return &NeedsProfile{
		ProfileToken: profileToken,
		Email:        claims.Email,
		FirstName:    givenName,
		LastName:     familyName,
	}, nil
}

// linkGoogleIdentity records the Google account behind userID, once. A store
// failure is logged, not fatal: the session this request is about to start does
// not depend on the link row existing, and the next Google sign-in tries again
// (Insert is idempotent — see stores.UserIdentityStore.Insert).
func (s *Service) linkGoogleIdentity(ctx context.Context, userID uuid.UUID, claims *googleid.Claims) {
	email := claims.Email
	err := s.identities.Insert(ctx, &authstore.UserIdentity{
		UserID:   userID,
		Provider: googleIdentityProvider,
		Subject:  claims.Subject,
		Email:    &email,
	})
	if err != nil {
		s.logger.Error("google: failed to record identity link", "error", err, "user_id", userID)
	}
}

// splitName splits a Google display name into first/last when the ID token
// carried no given_name/family_name — some accounts have a name but no split
// form. The first space separates the first name from the rest; a name with no
// space becomes the whole first name and an empty last name.
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
