package data

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stodulski/vibe-server/internal/crypto"
	"github.com/stodulski/vibe-server/internal/db"
)

// ---------------------------------------------------------------------------
// UserStore — segregated by responsibility
// ---------------------------------------------------------------------------

// UserCRUD defines create, read, update and delete operations for user accounts.
type UserCRUD interface {
	Insert(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	Update(ctx context.Context, user *User) error
	UpdatePassword(ctx context.Context, userID uuid.UUID, newHash []byte) error
	Delete(ctx context.Context, userID uuid.UUID) error
}

// UserVerificationManager manages email verification and onboarding state for a user.
type UserVerificationManager interface {
	SetEmailVerified(ctx context.Context, userID uuid.UUID) error
	DeleteUnverifiedStale(ctx context.Context) error
}

// UserSecurityManager tracks failed login attempts used for account lockout.
type UserSecurityManager interface {
	IncrementFailedAttempts(ctx context.Context, userID uuid.UUID) error
	ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error
}

// UserStore composes every user-related store capability.
type UserStore interface {
	UserCRUD
	UserVerificationManager
	UserSecurityManager
}

// ---------------------------------------------------------------------------
// UserIdentityStore — external identity links (Sign in with Google)
// ---------------------------------------------------------------------------

// UserIdentityStore links and looks up the external identities linked to a
// local account. Not tenant-scoped, like UserStore: an identity link belongs
// to the platform account, not to any one complex.
type UserIdentityStore interface {
	Insert(ctx context.Context, identity *UserIdentity) error
	GetByProviderSubject(ctx context.Context, provider, subject string) (*UserIdentity, error)
	GetByUser(ctx context.Context, userID uuid.UUID) ([]*UserIdentity, error)
}

// ---------------------------------------------------------------------------
// EmailVerificationStore — already well-sized
// ---------------------------------------------------------------------------

// EmailVerificationStore manages email verification tokens.
type EmailVerificationStore interface {
	Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte) error
	InsertWithCooldown(ctx context.Context, userID uuid.UUID, tokenHash []byte) error
	GetByHash(ctx context.Context, tokenHash []byte) (*EmailVerificationToken, error)
	DeleteByUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// PasswordResetStore — already well-sized
// ---------------------------------------------------------------------------

// PasswordResetStore manages password reset tokens.
type PasswordResetStore interface {
	InsertWithCooldown(ctx context.Context, userID uuid.UUID, tokenHash []byte) error
	GetByHash(ctx context.Context, tokenHash []byte) (*PasswordResetToken, error)
	DeleteByUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// BookingLinkTokenStore — a credential's lifecycle, not a booking's
// ---------------------------------------------------------------------------

// BookingLinkTokenStore mints and resolves the single-purpose access tokens
// that authorize a booking's public routes (specs/booking-link-credential).
//
// A top-level Models field, matching EmailVerificationStore/PasswordResetStore
// rather than composed into BookingStore (ISP): these three methods are a
// credential's lifecycle, not a booking's.
type BookingLinkTokenStore interface {
	Mint(ctx context.Context, bookingID uuid.UUID, expiresAt time.Time) (plaintext string, err error)
	// ResolveBooking returns the enriched booking and the token's stored
	// expiry in one JOIN, mirroring GetByID's hand-written SELECT.
	// ErrRecordNotFound means no row carries that hash — never that the row
	// expired; the caller decides expiry.
	ResolveBooking(ctx context.Context, plaintext string) (*Booking, time.Time, error)
	DeleteExpiredTerminal(ctx context.Context, retention time.Duration) error
}

// ---------------------------------------------------------------------------
// TokenStore — segregated into reader/writer
// ---------------------------------------------------------------------------

// TokenReader looks up refresh tokens by their hash.
type TokenReader interface {
	GetRefreshToken(ctx context.Context, tokenHash []byte) (*RefreshToken, error)
	GetUsedRefreshToken(ctx context.Context, tokenHash []byte) (*RefreshToken, error)
}

// TokenWriter creates, consumes and expires refresh tokens.
type TokenWriter interface {
	InsertRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash []byte, ttl time.Duration) error
	MarkRefreshTokenUsed(ctx context.Context, tokenHash []byte) error
	DeleteRefreshToken(ctx context.Context, tokenHash []byte) error
	DeleteAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

// TokenStore composes refresh-token read and write access.
type TokenStore interface {
	TokenReader
	TokenWriter
}

// ---------------------------------------------------------------------------
// ComplexStore — segregated by responsibility
// ---------------------------------------------------------------------------

// ComplexCRUD defines create, read, update and soft-delete operations for padel complexes.
type ComplexCRUD interface {
	Insert(ctx context.Context, complex *Complex) error
	GetByID(ctx context.Context, id uuid.UUID) (*Complex, error)
	GetBySlug(ctx context.Context, slug string) (*Complex, error)
	GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*Complex, error)
	Update(ctx context.Context, complex *Complex) error
	// SoftDeleteCascade soft-deletes the complex and returns how many of its
	// courts went down with it. There is no plain SoftDelete: stamping a
	// complex without closing its courts is the state the soft-delete cascade exists to
	// make unreachable, so the store does not offer a way to ask for it.
	SoftDeleteCascade(ctx context.Context, id uuid.UUID) (int, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
	SlugsWithPrefix(ctx context.Context, base string) ([]string, error)
	GetAllSlugs(ctx context.Context) ([]ComplexSlug, error)
}

// ComplexScheduleManager manages a complex's weekly opening schedule.
type ComplexScheduleManager interface {
	UpsertSchedule(ctx context.Context, schedule *Schedule) error
	GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*Schedule, error)
}

// ComplexMPManager manages a complex's MercadoPago OAuth credentials.
type ComplexMPManager interface {
	UpdateMPCredentials(ctx context.Context, complexID uuid.UUID, accessToken, refreshToken, userID string, expiresIn int) error
	ClearMPCredentials(ctx context.Context, complexID uuid.UUID) error
	GetWithMPConnected(ctx context.Context) ([]*Complex, error)
	// ListComplexesNeedingMPRefresh narrows GetWithMPConnected to complexes
	// whose token has no known expiry or expires within 30 days — what
	// cronRefreshMPTokens actually needs to refresh.
	ListComplexesNeedingMPRefresh(ctx context.Context) ([]*Complex, error)
}

// ComplexStore composes every complex-related store capability.
type ComplexStore interface {
	ComplexCRUD
	ComplexScheduleManager
	ComplexMPManager
}

// ---------------------------------------------------------------------------
// CourtStore — segregated by responsibility
// ---------------------------------------------------------------------------

// CourtCRUD defines create, read, update and soft-delete operations for courts.
type CourtCRUD interface {
	Insert(ctx context.Context, court *Court) error
	GetByID(ctx context.Context, id uuid.UUID) (*Court, error)
	GetByComplex(ctx context.Context, complexID uuid.UUID) ([]*Court, error)
	Update(ctx context.Context, court *Court) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// CourtPricingManager manages per-court, time-based price rules.
type CourtPricingManager interface {
	InsertPrice(ctx context.Context, price *CourtPrice) error
	GetPrices(ctx context.Context, courtID uuid.UUID) ([]*CourtPrice, error)
	UpdatePrice(ctx context.Context, price *CourtPrice) error
	DeletePrice(ctx context.Context, id uuid.UUID) error
	DeletePricesByCourtID(ctx context.Context, courtID uuid.UUID) error
	GetPricesByCourtIDs(ctx context.Context, courtIDs []uuid.UUID) ([]*CourtPrice, error)
	// ReplacePrices atomically replaces a court's whole price table — see its
	// comment in courts.go (H-07). Declared here, alongside the two calls it
	// replaces in internal/courts.Handler.UpdatePrices, because this interface
	// is what cmd/api's own store wiring is built against
	// (courts.NewHandler(d.models.Courts, ...) in cmd/api/app.go): a mock
	// implementing CourtStore there needs an additive stub for this method to
	// keep compiling, and cmd/api/mock_stores_test.go carries one.
	ReplacePrices(ctx context.Context, courtID uuid.UUID, prices []*CourtPrice) (failedIndex int, err error)
}

// CourtBlockedSlotManager manages manually blocked (unbookable) court slots.
type CourtBlockedSlotManager interface {
	InsertBlockedSlot(ctx context.Context, slot *BlockedSlot) error
	GetBlockedSlots(ctx context.Context, courtID uuid.UUID, date time.Time) ([]*BlockedSlot, error)
	GetBlockedSlotByID(ctx context.Context, id uuid.UUID) (*BlockedSlot, error)
	GetBlockedSlotsByComplex(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time) ([]*BlockedSlot, error)
	GetBlockedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]*BlockedSlot, error)
	DeleteBlockedSlot(ctx context.Context, id uuid.UUID) error
}

// CourtStore composes every court-related store capability.
type CourtStore interface {
	CourtCRUD
	CourtPricingManager
	CourtBlockedSlotManager
}

// ---------------------------------------------------------------------------
// BookingStore — segregated by responsibility
// ---------------------------------------------------------------------------

// BookingCreator creates new bookings, with a race-safe variant for concurrent slot claims.
type BookingCreator interface {
	Insert(ctx context.Context, booking *Booking) error
	InsertSafe(ctx context.Context, booking *Booking) error
}

// BookingReader queries bookings and the slots they occupy.
type BookingReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Booking, error)
	GetByComplex(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time, filters Filters) ([]*Booking, Metadata, error)
	GetBookedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]BookedSpan, error)
	GetByClient(ctx context.Context, complexID, clientID uuid.UUID, limit int) ([]*Booking, error)
}

// BookingUpdater persists changes to an existing booking.
type BookingUpdater interface {
	Update(ctx context.Context, booking *Booking) error
}

// BookingStatsQuerier computes dashboard and reporting aggregates over bookings.
type BookingStatsQuerier interface {
	GetDashboardStats(ctx context.Context, complexID uuid.UUID, today time.Time) (*DashboardStats, error)
	GetUpcomingToday(ctx context.Context, complexID uuid.UUID, today time.Time, nowTime string, limit int) ([]*Booking, error)
	GetRevenueByDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]RevenueDataPoint, error)
	GetOccupancyByHourDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]OccupancyDataPoint, error)
	GetPaymentSummary(ctx context.Context, complexID uuid.UUID, today time.Time) (*PaymentSummary, error)
}

// BookingReminderManager selects bookings due for reminder or confirmation notifications.
type BookingReminderManager interface {
	// Both take the caller's clock rather than reading the database's NOW():
	// the two-hour window is a range between instants, and the instant it is
	// measured from is the one thing a test has to be able to choose.
	GetForReminder2h(ctx context.Context, now time.Time) ([]*Booking, error)
	GetForReminder2hEnriched(ctx context.Context, now time.Time) ([]*CronBooking, error)
	MarkReminderSent2h(ctx context.Context, id uuid.UUID) error
}

// BookingLifecycleManager drives booking state transitions such as expiry, completion and bulk cancellation.
type BookingLifecycleManager interface {
	GetExpiredPendingEnriched(ctx context.Context, expiry time.Duration) ([]*CronBooking, error)
	HasActiveBookings(ctx context.Context, complexID uuid.UUID) (bool, error)
	HasActiveBookingsByCourt(ctx context.Context, courtID uuid.UUID) (bool, error)
	CompletePastBookings(ctx context.Context) (int64, error)
	CancelFutureByComplex(ctx context.Context, complexID uuid.UUID) error
}

// BookingRefundIntentManager is the reconciliation sweep's store layer
// (refund-intent-durability spec): finding a cancellation whose refund-intent
// marker has stood past its grace period, claiming it exclusively for one
// sweep run, and clearing it once the refund path is done. Separate from
// BookingUpdater, whose single Update method is the ordinary write path every
// other booking mutation already uses.
type BookingRefundIntentManager interface {
	GetRefundIntentOrphans(ctx context.Context, olderThan time.Duration, limit int) ([]*Booking, error)
	ClaimRefundIntent(ctx context.Context, id uuid.UUID, seen time.Time) error
	ClearRefundIntent(ctx context.Context, id uuid.UUID) error
}

// BookingStore composes every booking-related store capability.
type BookingStore interface {
	BookingCreator
	BookingReader
	BookingUpdater
	BookingStatsQuerier
	BookingReminderManager
	BookingLifecycleManager
	BookingRefundIntentManager
}

// ---------------------------------------------------------------------------
// ClientStore — segregated by responsibility
// ---------------------------------------------------------------------------

// ClientCRUD defines create, read and update operations for clients.
type ClientCRUD interface {
	Insert(ctx context.Context, client *Client) error
	GetByID(ctx context.Context, id uuid.UUID) (*Client, error)
	GetByComplex(ctx context.Context, complexID uuid.UUID, search string, filters Filters) ([]*Client, Metadata, error)
	Update(ctx context.Context, client *Client) error
}

// ClientLookup resolves clients by phone, creating one when none exists.
type ClientLookup interface {
	// allowNameUpdate is what tells the authenticated owner-booking caller
	// apart from the public, unauthenticated one on a phone match — see
	// ClientModel.GetOrCreate's own comment for why the two must not share
	// one answer.
	GetOrCreate(ctx context.Context, complexID uuid.UUID, firstName, lastName, phone, email string, allowNameUpdate bool) (*Client, error)
	GetByPhone(ctx context.Context, complexID uuid.UUID, phone string) (*Client, error)
}

// ClientMetrics computes client-related counters and insights for a complex.
type ClientMetrics interface {
	IncrementNoShows(ctx context.Context, clientID uuid.UUID) error
	CountByComplex(ctx context.Context, complexID uuid.UUID) (int, error)
	GetInsights(ctx context.Context, complexID uuid.UUID, today time.Time) (*ClientInsights, error)
}

// ClientStore composes every client-related store capability.
type ClientStore interface {
	ClientCRUD
	ClientLookup
	ClientMetrics
}

// ---------------------------------------------------------------------------
// PaymentStore — segregated into reader/writer
// ---------------------------------------------------------------------------

// PaymentReader looks up payments by booking or MercadoPago payment ID.
type PaymentReader interface {
	GetByBookingID(ctx context.Context, bookingID uuid.UUID) (*Payment, error)
	ListByBookingID(ctx context.Context, bookingID uuid.UUID) ([]*Payment, error)
	GetByMPPaymentID(ctx context.Context, mpPaymentID string) (*Payment, error)
}

// PaymentWriter creates and updates payments, including atomic booking confirmation.
type PaymentWriter interface {
	Insert(ctx context.Context, payment *Payment) error
	InsertAndConfirmBooking(ctx context.Context, payment *Payment, booking *Booking) error
	ConfirmWebhookPayment(ctx context.Context, payment *Payment, booking *Booking) error
	Update(ctx context.Context, payment *Payment) error
}

// PaymentRefunder runs the refund lifecycle: claim, then call the provider, then
// record the outcome. Each method is its own short committed transaction, so no
// database resource is held while MercadoPago is being called.
//
// It is separate from PaymentWriter because a refund spans three tables — the
// payment, its booking and the attempt record — which is a different
// responsibility from writing a payment row.
type PaymentRefunder interface {
	ClaimRefund(ctx context.Context, paymentID uuid.UUID) (*RefundClaim, error)
	// manualOwedCentavos is the booking's still-outstanding cash/transfer
	// balance; see the identical parameter on payments.PaymentStore.
	RecordRefundSuccess(ctx context.Context, claim RefundClaim, manualOwedCentavos int) (refundTotal int, err error)
	RecordRefundFailure(ctx context.Context, claim RefundClaim, cause string) (exhausted bool, err error)
	// RecordManualRefund closes out a partial_refund booking's remaining
	// cash/transfer rows once the owner confirms they returned that money by
	// hand, in one transaction with the booking write. See
	// internal/bookings/actions.go ManualRefund.
	RecordManualRefund(ctx context.Context, bookingID uuid.UUID) (returnedCentavos int, err error)
}

// PaymentStore composes payment read, write and refund access.
type PaymentStore interface {
	PaymentReader
	PaymentWriter
	PaymentRefunder
}

// ---------------------------------------------------------------------------
// FailedRefundStore — the queue of refunds still owed to a client
// ---------------------------------------------------------------------------

// FailedRefundQueue records a refund attempt and drives it to a terminal state.
// It is the whole of what the refund sweeper needs.
type FailedRefundQueue interface {
	Insert(ctx context.Context, fr *FailedRefund) error
	GetPendingDue(ctx context.Context) ([]*FailedRefund, error)
	MarkProcessing(ctx context.Context, id uuid.UUID) error
	MarkResolved(ctx context.Context, id uuid.UUID) error
	MarkExhausted(ctx context.Context, id uuid.UUID) error
	IncrementRetry(ctx context.Context, id uuid.UUID, retryCount int, errMsg string) error
}

// FailedRefundPruner drops attempts that were resolved long enough ago to have no
// forensic value left. It is separated from the queue for the reason
// WebhookEventPruner is: retention is a cron job's business rather than the
// sweeper's, and the two have no caller in common.
type FailedRefundPruner interface {
	DeleteResolved(ctx context.Context, olderThan time.Duration) (int64, error)
}

// FailedRefundStore composes working and pruning the refund retry queue.
type FailedRefundStore interface {
	FailedRefundQueue
	FailedRefundPruner
}

// ---------------------------------------------------------------------------
// WebhookEventStore — the durable inbox for provider notifications
// ---------------------------------------------------------------------------

// WebhookEventRecorder records a delivered event. It is separated from the rest
// because it is the only part the HTTP handler needs: the endpoint records the
// event and answers, and everything after that is the worker's problem.
type WebhookEventRecorder interface {
	Insert(ctx context.Context, e *WebhookEvent) error
}

// WebhookEventWorker drives a recorded event to a terminal state, with the same
// claim/retry vocabulary the failed-refund queue uses.
type WebhookEventWorker interface {
	GetPendingDue(ctx context.Context) ([]*WebhookEvent, error)
	Claim(ctx context.Context, id uuid.UUID) (claimed bool, err error)
	MarkProcessed(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, cause string) (exhausted bool, err error)
}

// WebhookEventPruner drops processed events once they are old enough to have no
// forensic value left.
type WebhookEventPruner interface {
	DeleteProcessed(ctx context.Context, olderThan time.Duration) (int64, error)
}

// WebhookEventStore composes recording, working and pruning webhook events.
type WebhookEventStore interface {
	WebhookEventRecorder
	WebhookEventWorker
	WebhookEventPruner
}

// ---------------------------------------------------------------------------
// SlotLockStore — already well-sized
// ---------------------------------------------------------------------------

// SlotLockStore manages short-lived locks that reserve a court slot during checkout.
type SlotLockStore interface {
	AcquireLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime, endTime string, bookingID *uuid.UUID, ttl time.Duration) error
	ReleaseLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime string) error
	ReleaseByBooking(ctx context.Context, bookingID uuid.UUID) error
	CleanExpired(ctx context.Context) (int64, error)
}

// ---------------------------------------------------------------------------
// Models — aggregate of all stores
// ---------------------------------------------------------------------------

// MaxBookingHorizonDays bounds how far into the future a booking or a court's
// blocked slot may be dated.
//
// H-08: internal/courts.BlockSlot and internal/bookings.PublicBook — the two
// write paths an anonymous visitor can reach with no account — validated a
// date's lower bound (not in the past) but never its upper one. A booking
// dated "9999-12-31" was accepted with a real MercadoPago preference: it
// holds a slot forever and is never reaped, since cron's completeBookings
// only completes a booking whose end time has already passed. A block that
// far out is merely inert, but the booking half is a standing way for an
// anonymous caller to leave permanent rows behind, one request at a time.
//
// A year is generous for a real booking. It lives here, next to
// defaultPaymentExpiry below, rather than as a literal duplicated in two
// handlers in two different packages — so the two validators agree by
// construction, and raising it later is a one-line change rather than an
// audit of every date check in the codebase.
const MaxBookingHorizonDays = 365

// defaultPaymentExpiry is the hold a Config that names none falls back to. It is
// the default of the -booking-payment-expiry flag, and it exists so a zero Config
// cannot be read as "no hold at all" — that would make every unpaid public
// booking stale the instant it is created and stop it holding its slot.
const defaultPaymentExpiry = 15 * time.Minute

// Config is what the stores need from application configuration.
//
// It exists for one value. The slot-holding rule inside the double-booking
// defence (see slot_guard.go) used to carry a hardcoded fifteen minutes while the
// cancellation cron it mirrors ran on the configured payment expiry, so raising
// that flag widened the window in which a booking held no slot and nothing had
// cancelled it yet. Passing the number in makes the two the same by construction.
type Config struct {
	// PaymentExpiry is how long an unpaid public booking keeps its slot. It is
	// the same value cronReleaseExpiredPayments cancels on
	// (-booking-payment-expiry / BOOKING_PAYMENT_EXPIRY).
	PaymentExpiry time.Duration
	// Logger is where the stores report what no caller can be told. Only the
	// advisory-lock release needs it today: see LockModel.
	Logger *slog.Logger
	// Keys is the MercadoPago credential encryption keyring. ComplexModel and
	// BookingModel use it to seal mp_access_token/mp_refresh_token on write
	// and open them on read (internal/data/mpcred.go). A nil Keys is a valid
	// zero value that Seal and Open both refuse — see crypto.Keyring — so a
	// Config built without it fails loudly the first time a credential is
	// touched, rather than storing or returning plaintext.
	Keys *crypto.Keyring
	// LinkTokenBuffer is added to a booking's end time to compute a booking
	// link token's expires_at. BookingModel.InsertSafe and
	// BookingLinkTokenModel.Mint's callers both use it. Zero falls back to
	// defaultLinkTokenBuffer, the same "a zero Config is not a smaller hazard"
	// reasoning PaymentExpiry documents above.
	LinkTokenBuffer time.Duration
}

// paymentExpiry is the configured hold, or the default when none was given.
func (c Config) paymentExpiry() time.Duration {
	if c.PaymentExpiry <= 0 {
		return defaultPaymentExpiry
	}
	return c.PaymentExpiry
}

// defaultLinkTokenBuffer is the hold a Config that names none falls back to —
// the default of the -booking-link-token-buffer flag.
const defaultLinkTokenBuffer = 24 * time.Hour

// linkTokenBuffer is the configured buffer, or the default when none was given.
func (c Config) linkTokenBuffer() time.Duration {
	if c.LinkTokenBuffer <= 0 {
		return defaultLinkTokenBuffer
	}
	return c.LinkTokenBuffer
}

// Models aggregates every store interface used by the application.
type Models struct {
	Users             UserStore
	UserIdentities    UserIdentityStore
	Complexes         ComplexStore
	Courts            CourtStore
	Bookings          BookingStore
	BookingLinkTokens BookingLinkTokenStore
	Tokens            TokenStore
	Clients           ClientStore
	Payments          PaymentStore
	EmailVerification EmailVerificationStore
	PasswordReset     PasswordResetStore
	FailedRefunds     FailedRefundStore
	WebhookEvents     WebhookEventStore
	SlotLocks         SlotLockStore
	Admin             AdminStore
	Reports           ReportStore
	Locks             LockStore
}

// NewModels builds a Models with every store backed by the given connection pool
// and the given configuration.
func NewModels(pool *pgxpool.Pool, cfg Config) Models {
	return newModels(NewDB(pool), cfg)
}

// newModels builds the stores over a handle that already retries.
//
// The raw pool does not reach this function, and that is the point: sqlc's
// generated queries are built against whatever is passed to db.New, so if the
// pool were in scope here it could be handed to db.New by accident and every
// generated query would quietly leave the retry behind. Taking only the
// retrying handle makes that impossible to write, and lets a test drive every
// store — generated queries included — without a database.
func newModels(pooled *DB, cfg Config) Models {
	q := db.New(pooled)
	paymentExpiry := cfg.paymentExpiry()
	return Models{
		Users:             &UserModel{DB: pooled, Q: q},
		UserIdentities:    &UserIdentityModel{DB: pooled, Q: q},
		Complexes:         &ComplexModel{DB: pooled, Q: q, Keys: cfg.Keys},
		Courts:            &CourtModel{DB: pooled, Q: q, PaymentExpiry: paymentExpiry},
		Bookings:          &BookingModel{DB: pooled, Q: q, PaymentExpiry: paymentExpiry, Keys: cfg.Keys, LinkTokenBuffer: cfg.linkTokenBuffer()},
		BookingLinkTokens: &BookingLinkTokenModel{DB: pooled},
		Tokens:            &TokenModel{DB: pooled, Q: q},
		Clients:           &ClientModel{DB: pooled, Q: q},
		Payments:          &PaymentModel{DB: pooled, Q: q, PaymentExpiry: paymentExpiry},
		EmailVerification: &EmailVerificationModel{DB: pooled, Q: q},
		PasswordReset:     &PasswordResetModel{DB: pooled},
		FailedRefunds:     &FailedRefundModel{DB: pooled},
		WebhookEvents:     &WebhookEventModel{DB: pooled},
		Reports:           &ReportModel{DB: pooled},
		Locks:             &LockModel{DB: pooled, Logger: cfg.Logger},
		SlotLocks:         &SlotLockModel{DB: pooled},
		Admin:             &AdminModel{DB: pooled},
	}
}
