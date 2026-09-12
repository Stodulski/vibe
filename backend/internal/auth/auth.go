// Package auth owns who the caller is: registration, sign-in, the session
// cookies, email verification, password reset, and the account itself.
//
// Sessions are cookie-based. A short-lived JWT access token authenticates
// requests, a long-lived opaque refresh token renews it, and a CSRF token
// derived from the access token guards state-changing requests. The refresh
// token rotates on every use, and a reuse of an already-rotated one is treated
// as theft: every session for that account is revoked.
package auth

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/googleid"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/notifications"
)

// UserStore is the account persistence this module uses.
type UserStore interface {
	GetByEmail(ctx context.Context, email string) (*authstore.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*authstore.User, error)
	Insert(ctx context.Context, user *authstore.User) error
	Update(ctx context.Context, user *authstore.User) error
	Delete(ctx context.Context, userID uuid.UUID) error
	IncrementFailedAttempts(ctx context.Context, userID uuid.UUID) error
	ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error
	SetEmailVerified(ctx context.Context, userID uuid.UUID) error
	UpdatePassword(ctx context.Context, userID uuid.UUID, newHash []byte) error
}

// TokenStore holds the refresh tokens that back a session.
type TokenStore interface {
	InsertRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash []byte, ttl time.Duration) error
	GetRefreshToken(ctx context.Context, tokenHash []byte) (*authstore.RefreshToken, error)
	GetUsedRefreshToken(ctx context.Context, tokenHash []byte) (*authstore.RefreshToken, error)
	MarkRefreshTokenUsed(ctx context.Context, tokenHash []byte) error
	DeleteRefreshToken(ctx context.Context, tokenHash []byte) error
	DeleteAllForUser(ctx context.Context, userID uuid.UUID) error
}

// VerificationStore holds the single-use email-verification tokens.
type VerificationStore interface {
	Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte) error
	InsertWithCooldown(ctx context.Context, userID uuid.UUID, tokenHash []byte) error
	GetByHash(ctx context.Context, tokenHash []byte) (*authstore.EmailVerificationToken, error)
	DeleteByUser(ctx context.Context, userID uuid.UUID) error
}

// PasswordResetStore holds the single-use password-reset tokens.
type PasswordResetStore interface {
	InsertWithCooldown(ctx context.Context, userID uuid.UUID, tokenHash []byte) error
	GetByHash(ctx context.Context, tokenHash []byte) (*authstore.PasswordResetToken, error)
	DeleteByUser(ctx context.Context, userID uuid.UUID) error
}

// OwnershipReader is what account deletion needs: an account with a complex
// still trading cannot simply vanish.
type OwnershipReader interface {
	GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*complexstore.Complex, error)
}

// IdentityStore links a local account to an external identity provider's
// account. Only Insert is declared here (ISP): this module records a link, it
// never has to look one up — the account itself is always found by email.
type IdentityStore interface {
	Insert(ctx context.Context, identity *authstore.UserIdentity) error
}

// BookingReader answers whether a complex still has live bookings.
type BookingReader interface {
	HasActiveBookings(ctx context.Context, complexID uuid.UUID) (bool, error)
}

// Blacklist revokes issued access tokens before they expire on their own.
//
// Access tokens are self-contained JWTs, so signing out or deleting an account
// cannot invalidate one by deleting a row — the blacklist is what makes
// revocation possible at all.
//
// Both writes return an error so a failure to record a revocation cannot pass
// unnoticed. An error does not mean nothing was revoked: the implementation
// still enforces it locally (see TokenBlacklist), it just could not share it
// with the other instances.
type Blacklist interface {
	BlacklistToken(ctx context.Context, rawToken string, expiry time.Time) error
	InvalidateUserTokens(ctx context.Context, userID uuid.UUID) error
}

// Notifier sends the account emails.
type Notifier interface {
	EmailVerification(e notifications.VerificationEmail)
	PasswordReset(e notifications.PasswordResetEmail)
	DuplicateRegistration(e notifications.DuplicateRegistrationEmail)
}

// UserCache is the cached-user invalidation this module triggers when an
// account's own record changes.
type UserCache interface {
	InvalidateUser(ctx context.Context, id uuid.UUID)
}

// TurnstileVerifier checks a Cloudflare Turnstile token before a handler does
// anything else. It is declared here, by the consumer, matching this
// module's other dependencies — Handler depends on this interface, never on
// internal/turnstile's concrete Client.
//
// A nil TurnstileVerifier means disabled: NewHandler treats it exactly like
// one whose Enabled() returns false, so existing callers and tests that never
// set Dependencies.Turnstile keep working unchanged.
type TurnstileVerifier interface {
	// Enabled reports whether verification is configured at all. When it is
	// not, callers must skip Verify entirely — this module is not allowed to
	// force self-hosters into using Turnstile.
	Enabled() bool
	// Verify checks token, which the caller received in the request body, and
	// reports why it was rejected via the package's sentinel errors
	// (turnstile.ErrMissingToken, turnstile.ErrInvalidToken,
	// turnstile.ErrUnavailable).
	Verify(ctx context.Context, token, remoteIP string) error
}

// GoogleVerifier verifies a Google Identity Services ID token. It is declared
// here, by the consumer, matching TurnstileVerifier — Handler depends on this
// interface, never on internal/googleid's concrete Verifier.
//
// A nil GoogleVerifier means disabled, exactly like a nil TurnstileVerifier:
// NewHandler substitutes disabledGoogle{}, so GoogleSignIn and GoogleComplete
// answer 503 without a nil check at every call site.
type GoogleVerifier interface {
	// Enabled reports whether Google sign-in is configured at all (a client
	// id is set). When it is not, callers must answer 503 without calling
	// Verify.
	Enabled() bool
	// Verify checks credential, a Google Identity Services ID token from the
	// request body, and reports why it was rejected via googleid.ErrInvalidToken
	// or googleid.ErrUnavailable.
	Verify(ctx context.Context, credential string) (*googleid.Claims, error)
}

// Recorder writes the audit trail for what happens to an account.
//
// It is declared here, by the consumer, matching internal/bookings,
// internal/courts, internal/complexes and internal/payments — this module
// depends on one method, not on the audit package's Recorder type.
type Recorder interface {
	Record(e audit.Entry)
}

// Config is what this module needs from application configuration.
type Config struct {
	// JWTSecret signs and verifies access tokens.
	JWTSecret string
	// CookieDomain scopes the session cookies. Empty means host-only.
	CookieDomain string
	// Environment decides whether cookies are marked Secure; local development
	// runs over plain HTTP.
	Environment string
	// FrontendURL is the origin the emailed verification and reset links point at.
	FrontendURL string
	// PasswordHashCost is the bcrypt cost this module hashes passwords at.
	// Zero means authstore.DefaultHashCost, the production value; a test
	// binary passes bcrypt.MinCost so that a suite which registers or signs in
	// hundreds of users does not spend a quarter of a second on each one.
	PasswordHashCost int
	// TrustProxies decides which address the audit trail records — the peer, or
	// the one the forwarded headers claim. Same flag every other module's
	// trail reads (cfg.trustedProxies).
	TrustProxies bool
}

// Handler serves the auth routes.
type Handler struct {
	users         UserStore
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
	respond       *httpx.Responder
	logger        *slog.Logger
	cfg           Config
}

// Dependencies groups what NewHandler needs. It is a struct because the list
// is twelve long, and a positional call at that width is unreadable and easy
// to mis-order between two values of the same type.
type Dependencies struct {
	Users         UserStore
	Tokens        TokenStore
	Verifications VerificationStore
	Resets        PasswordResetStore
	Complexes     OwnershipReader
	Bookings      BookingReader
	Blacklist     Blacklist
	Notify        Notifier
	Cache         UserCache
	Audit         Recorder
	// Turnstile verifies the optional turnstile_token on register, login and
	// forgot-password. Nil means disabled — see TurnstileVerifier.
	Turnstile TurnstileVerifier
	// Google verifies a Google Identity Services ID token for /auth/google
	// and /auth/google/complete. Nil means disabled — see GoogleVerifier.
	Google GoogleVerifier
	// Identities links a local account to the Google account it signed in
	// with.
	Identities IdentityStore
	Respond    *httpx.Responder
	Logger     *slog.Logger
}

// NewHandler returns a Handler.
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
func NewHandler(d Dependencies, cfg Config) *Handler {
	if d.Audit == nil {
		panic("auth: NewHandler needs an audit recorder; a session and credential trail is not optional")
	}
	turnstileVerifier := d.Turnstile
	if turnstileVerifier == nil {
		// A nil interface value cannot be asked Enabled() without a type
		// check at every call site; disabledTurnstile makes "no verifier
		// configured" and "verifier configured but reports disabled" the same
		// code path everywhere below.
		turnstileVerifier = disabledTurnstile{}
	}
	googleVerifier := d.Google
	if googleVerifier == nil {
		// Same reasoning as disabledTurnstile above.
		googleVerifier = disabledGoogle{}
	}
	return &Handler{
		users:  d.Users,
		tokens: d.Tokens,
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
		respond:       d.Respond,
		logger:        d.Logger,
		cfg:           cfg,
	}
}

// disabledTurnstile is the zero-value TurnstileVerifier: always disabled,
// never called.
type disabledTurnstile struct{}

func (disabledTurnstile) Enabled() bool { return false }

func (disabledTurnstile) Verify(context.Context, string, string) error {
	return nil
}

// disabledGoogle is the zero-value GoogleVerifier: always disabled, its
// Verify never actually called (GoogleSignIn and GoogleComplete check
// Enabled first).
type disabledGoogle struct{}

func (disabledGoogle) Enabled() bool { return false }

func (disabledGoogle) Verify(context.Context, string) (*googleid.Claims, error) {
	return nil, googleid.ErrUnavailable
}

// Routes registers this module's endpoints.
//
// The public group is public by necessity, not by oversight: registering
// creates the session, signing in establishes it, refresh runs on an expired
// access token by design, and the verification and reset flows are reached
// from an emailed link by someone who cannot sign in. Each is authenticated by
// something other than a session — a password, a rotating token, or a
// single-use emailed token.
func (h *Handler) Routes(router httpx.Router, guards httpx.Guards) {
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/register", h.Register)
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/login", h.Login)
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/refresh", h.Refresh)
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/logout", h.Logout)
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/verify-email", h.VerifyEmail)
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/resend-verification", h.ResendVerification)
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/forgot-password", h.ForgotPassword)
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/reset-password", h.ResetPassword)

	// Google sign-in is always registered, even when GOOGLE_OAUTH_CLIENT_ID
	// is unset — GoogleSignIn and GoogleComplete answer 503 in that case
	// (h.google.Enabled()) rather than 404, so the route table, the OpenAPI
	// document and the CSRF/rate-limit/tenant exemption tables stay static
	// regardless of configuration.
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/google", h.GoogleSignIn)
	router.HandlerFunc(http.MethodPost, "/api/v1/auth/google/complete", h.GoogleComplete)

	router.HandlerFunc(http.MethodGet, "/api/v1/auth/me", guards.RequireAuth(h.CurrentUser))
	router.HandlerFunc(http.MethodPut, "/api/v1/auth/me", guards.RequireAuth(h.UpdateCurrentUser))
	router.HandlerFunc(http.MethodDelete, "/api/v1/auth/me", guards.RequireAuth(h.DeleteAccount))
}
