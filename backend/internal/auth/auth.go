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
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/googleid"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/notifications"
)

// An account is read, edited, has its credentials changed, and is locked out
// after too many failed sign-ins. Those are four different reasons to touch a
// user row and four different blast radiuses, so they are four ports: the two
// that change what a credential is are nameable on their own, and a rule that
// only reads an account cannot reach a writer by accident.

// UserReader finds an account.
type UserReader interface {
	GetByEmail(ctx context.Context, email string) (*authstore.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*authstore.User, error)
}

// UserWriter creates, edits and deletes an account.
type UserWriter interface {
	Insert(ctx context.Context, user *authstore.User) error
	Update(ctx context.Context, user *authstore.User) error
	Delete(ctx context.Context, userID uuid.UUID) error
}

// CredentialStore changes what an account signs in with, and what it has
// proved about its address.
type CredentialStore interface {
	SetEmailVerified(ctx context.Context, userID uuid.UUID) error
	UpdatePassword(ctx context.Context, userID uuid.UUID, newHash []byte) error
}

// LockoutStore is the failed-sign-in counter behind the account lockout.
type LockoutStore interface {
	IncrementFailedAttempts(ctx context.Context, userID uuid.UUID) error
	ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error
}

// UserStore is all four together: one concrete store implements them, and the
// composition is what Dependencies takes, so a caller still passes one value.
type UserStore interface {
	UserReader
	UserWriter
	CredentialStore
	LockoutStore
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

// GoogleCodeStore holds the one-time codes that carry a redirect-mode Google
// sign-in between the two requests it is split across: Google's form POST,
// whose answer is a redirect, and the frontend's exchange, which is where the
// session is finally established.
//
// It is declared here, by the consumer, like every other port in this file.
// GoogleCodes satisfies it; a nil one means "no store was wired", which
// NewService turns into a purely in-memory GoogleCodes rather than a nil
// check at each call site.
type GoogleCodeStore interface {
	// Store records payload under code for ttl.
	Store(ctx context.Context, code string, payload []byte, ttl time.Duration) error
	// Consume returns what code carries and spends it in the same operation,
	// so a code is usable exactly once. An unknown, expired or already-spent
	// code is ErrGoogleCodeInvalid.
	Consume(ctx context.Context, code string) ([]byte, error)
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
	// JWTSecret signs and verifies access tokens: the active key.
	JWTSecret string
	// JWTKeyID, JWTSecretPrevious and JWTKeyIDPrevious are the rest of the
	// signing keyring, passed straight through to TokenServiceConfig — see
	// internal/auth/keyring.go for what a rotation looks like.
	JWTKeyID          string
	JWTSecretPrevious string
	JWTKeyIDPrevious  string
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

// Handler serves the auth routes. It decodes, validates, sets and clears the
// session cookies, and maps the service's domain errors onto HTTP; every rule
// lives in the Service.
type Handler struct {
	svc     *Service
	respond *httpx.Refuser
	logger  *slog.Logger
	cfg     Config
}

// refusals is this module's whole error-to-status table: every domain error of
// its own that is a refusal rather than a fault, and the status and message it
// earns. Everything absent from it — the shared sentinels, and the errors this
// module answers as a validation failure or a 401 rather than as a status of
// their own — is answered elsewhere.
//
// ErrInvalidToken is the entry with two messages: it is the same failure for an
// email verification link and for a password reset link, but the person reading
// it needs to know which link died, so each handler overrides the message
// through DomainErrorWith and keeps this status.
var refusals = httpx.Refusals{
	ErrInvalidToken: httpx.BadRequest("invalid or expired verification token"),
	ErrActiveBookings: httpx.Conflict(
		"cannot delete account while you have active bookings, cancel them first"),
	ErrAccountExists:        httpx.Conflict("account already exists"),
	googleid.ErrUnavailable: httpx.Unavailable("google sign-in is temporarily unavailable"),
}

// googleNotConfigured is the answer both Google routes give while
// GOOGLE_OAUTH_CLIENT_ID is empty. It is a guard rather than a domain error —
// there is no call to fail — so it has no entry in the table above.
var googleNotConfigured = httpx.Unavailable("google sign-in is not configured")

// Dependencies groups what NewService needs. It is a struct because the list is
// thirteen long, and a positional call at that width is unreadable and easy to
// mis-order between two values of the same type.
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
	// GoogleCodes holds the one-time codes redirect-mode Google sign-in is
	// exchanged with. Nil means in-memory — see GoogleCodeStore.
	GoogleCodes GoogleCodeStore
	Respond     *httpx.Responder
	Logger      *slog.Logger
}

// NewHandler returns a Handler backed by the given service.
func NewHandler(svc *Service, respond *httpx.Responder, logger *slog.Logger, cfg Config) *Handler {
	return &Handler{
		svc:     svc,
		respond: respond.WithRefusals(refusals),
		logger:  logger,
		cfg:     cfg,
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
