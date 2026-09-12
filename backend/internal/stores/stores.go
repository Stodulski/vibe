package stores

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	adminstore "github.com/stodulski/vibe-server/internal/admin/store"
	auditstore "github.com/stodulski/vibe-server/internal/audit/store"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	booklinkstore "github.com/stodulski/vibe-server/internal/booklink/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/crypto"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
)

// ---------------------------------------------------------------------------
// UserStore — segregated by responsibility
// ---------------------------------------------------------------------------

// UserCRUD defines create, read, update and delete operations for user accounts.
type UserCRUD interface {
	Insert(ctx context.Context, user *authstore.User) error
	GetByEmail(ctx context.Context, email string) (*authstore.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*authstore.User, error)
	Update(ctx context.Context, user *authstore.User) error
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
	Insert(ctx context.Context, identity *authstore.UserIdentity) error
	GetByProviderSubject(ctx context.Context, provider, subject string) (*authstore.UserIdentity, error)
	GetByUser(ctx context.Context, userID uuid.UUID) ([]*authstore.UserIdentity, error)
}

// ---------------------------------------------------------------------------
// EmailVerificationStore — already well-sized
// ---------------------------------------------------------------------------

// EmailVerificationStore manages email verification tokens.
type EmailVerificationStore interface {
	Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte) error
	InsertWithCooldown(ctx context.Context, userID uuid.UUID, tokenHash []byte) error
	GetByHash(ctx context.Context, tokenHash []byte) (*authstore.EmailVerificationToken, error)
	DeleteByUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// PasswordResetStore — already well-sized
// ---------------------------------------------------------------------------

// PasswordResetStore manages password reset tokens.
type PasswordResetStore interface {
	InsertWithCooldown(ctx context.Context, userID uuid.UUID, tokenHash []byte) error
	GetByHash(ctx context.Context, tokenHash []byte) (*authstore.PasswordResetToken, error)
	DeleteByUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// BookingLinkTokenStore — a credential's lifecycle, not a booking's
// ---------------------------------------------------------------------------

// BookingLinkTokenStore mints and resolves the single-purpose access tokens
// that authorize a booking's public routes (specs/booking-link-credential).
//
// A top-level Stores field, matching EmailVerificationStore/PasswordResetStore
// rather than composed into BookingStore (ISP): these three methods are a
// credential's lifecycle, not a booking's.
type BookingLinkTokenStore interface {
	Mint(ctx context.Context, bookingID uuid.UUID, expiresAt time.Time) (plaintext string, err error)
	// ResolveBooking returns the enriched booking and the token's stored
	// expiry in one JOIN, mirroring GetByID's hand-written SELECT.
	// ErrRecordNotFound means no row carries that hash — never that the row
	// expired; the caller decides expiry.
	ResolveBooking(ctx context.Context, plaintext string) (*bookingstore.Booking, time.Time, error)
	DeleteExpiredTerminal(ctx context.Context, retention time.Duration) error
}

// ---------------------------------------------------------------------------
// TokenStore — segregated into reader/writer
// ---------------------------------------------------------------------------

// TokenReader looks up refresh tokens by their hash.
type TokenReader interface {
	GetRefreshToken(ctx context.Context, tokenHash []byte) (*authstore.RefreshToken, error)
	GetUsedRefreshToken(ctx context.Context, tokenHash []byte) (*authstore.RefreshToken, error)
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
	Insert(ctx context.Context, complex *complexstore.Complex) error
	GetByID(ctx context.Context, id uuid.UUID) (*complexstore.Complex, error)
	GetBySlug(ctx context.Context, slug string) (*complexstore.Complex, error)
	GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*complexstore.Complex, error)
	Update(ctx context.Context, complex *complexstore.Complex, expectedVersion *int) error
	// SoftDeleteCascade soft-deletes the complex and returns how many of its
	// courts went down with it. There is no plain SoftDelete: stamping a
	// complex without closing its courts is the state the soft-delete cascade exists to
	// make unreachable, so the store does not offer a way to ask for it.
	SoftDeleteCascade(ctx context.Context, id uuid.UUID) (int, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
	SlugsWithPrefix(ctx context.Context, base string) ([]string, error)
	GetAllSlugs(ctx context.Context) ([]complexstore.ComplexSlug, error)
}

// ComplexScheduleManager manages a complex's weekly opening schedule.
type ComplexScheduleManager interface {
	UpsertSchedule(ctx context.Context, schedule *complexstore.Schedule) error
	GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*complexstore.Schedule, error)
}

// ComplexMPManager manages a complex's MercadoPago OAuth credentials.
type ComplexMPManager interface {
	UpdateMPCredentials(ctx context.Context, complexID uuid.UUID, accessToken, refreshToken, userID string, expiresIn int) error
	ClearMPCredentials(ctx context.Context, complexID uuid.UUID) error
	GetWithMPConnected(ctx context.Context) ([]*complexstore.Complex, error)
	// ListComplexesNeedingMPRefresh narrows GetWithMPConnected to complexes
	// whose token has no known expiry or expires within 30 days — what
	// cronRefreshMPTokens actually needs to refresh.
	ListComplexesNeedingMPRefresh(ctx context.Context) ([]*complexstore.Complex, error)
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
	Insert(ctx context.Context, court *courtstore.Court) error
	GetByID(ctx context.Context, id uuid.UUID) (*courtstore.Court, error)
	GetByComplex(ctx context.Context, complexID uuid.UUID) ([]*courtstore.Court, error)
	Update(ctx context.Context, court *courtstore.Court, expectedVersion *int) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// CourtPricingManager manages per-court, time-based price rules.
type CourtPricingManager interface {
	InsertPrice(ctx context.Context, price *courtstore.CourtPrice) error
	GetPrices(ctx context.Context, courtID uuid.UUID) ([]*courtstore.CourtPrice, error)
	UpdatePrice(ctx context.Context, price *courtstore.CourtPrice) error
	DeletePrice(ctx context.Context, id uuid.UUID) error
	DeletePricesByCourtID(ctx context.Context, courtID uuid.UUID) error
	GetPricesByCourtIDs(ctx context.Context, courtIDs []uuid.UUID) ([]*courtstore.CourtPrice, error)
	// ReplacePrices atomically replaces a court's whole price table — see its
	// comment in courts.go (H-07). Declared here, alongside the two calls it
	// replaces in internal/courts.Handler.UpdatePrices, because this interface
	// is what cmd/api's own store wiring is built against
	// (courts.NewHandler(d.models.Courts, ...) in cmd/api/app.go): a mock
	// implementing CourtStore there needs an additive stub for this method to
	// keep compiling, and cmd/api/mock_stores_test.go carries one.
	ReplacePrices(ctx context.Context, courtID uuid.UUID, prices []*courtstore.CourtPrice,
		expectedVersion *int) (failedIndex int, err error)
}

// CourtBlockedSlotManager manages manually blocked (unbookable) court slots.
type CourtBlockedSlotManager interface {
	InsertBlockedSlot(ctx context.Context, slot *courtstore.BlockedSlot) error
	GetBlockedSlots(ctx context.Context, courtID uuid.UUID, date time.Time) ([]*courtstore.BlockedSlot, error)
	GetBlockedSlotByID(ctx context.Context, id uuid.UUID) (*courtstore.BlockedSlot, error)
	GetBlockedSlotsByComplex(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time) ([]*courtstore.BlockedSlot, error)
	GetBlockedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]*courtstore.BlockedSlot, error)
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
	Insert(ctx context.Context, booking *bookingstore.Booking) error
	InsertSafe(ctx context.Context, booking *bookingstore.Booking) error
}

// BookingReader queries bookings and the slots they occupy.
type BookingReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*bookingstore.Booking, error)
	GetByComplex(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time, filters data.Filters) ([]*bookingstore.Booking, data.Metadata, error)
	GetBookedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]bookingstore.BookedSpan, error)
	GetByClient(ctx context.Context, complexID, clientID uuid.UUID, limit int) ([]*bookingstore.Booking, error)
}

// BookingUpdater persists changes to an existing booking.
type BookingUpdater interface {
	Update(ctx context.Context, booking *bookingstore.Booking) error
}

// BookingStatsQuerier computes dashboard and reporting aggregates over bookings.
type BookingStatsQuerier interface {
	GetDashboardStats(ctx context.Context, complexID uuid.UUID, today time.Time) (*bookingstore.DashboardStats, error)
	GetUpcomingToday(ctx context.Context, complexID uuid.UUID, today time.Time, nowTime string, limit int) ([]*bookingstore.Booking, error)
	GetRevenueByDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]bookingstore.RevenueDataPoint, error)
	GetOccupancyByHourDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]bookingstore.OccupancyDataPoint, error)
	GetPaymentSummary(ctx context.Context, complexID uuid.UUID, today time.Time) (*bookingstore.PaymentSummary, error)
}

// BookingReminderManager selects bookings due for reminder or confirmation notifications.
type BookingReminderManager interface {
	// Both take the caller's clock rather than reading the database's NOW():
	// the two-hour window is a range between instants, and the instant it is
	// measured from is the one thing a test has to be able to choose.
	GetForReminder2h(ctx context.Context, now time.Time) ([]*bookingstore.Booking, error)
	GetForReminder2hEnriched(ctx context.Context, now time.Time) ([]*bookingstore.CronBooking, error)
	MarkReminderSent2h(ctx context.Context, id uuid.UUID) error
}

// BookingLifecycleManager drives booking state transitions such as expiry, completion and bulk cancellation.
type BookingLifecycleManager interface {
	GetExpiredPendingEnriched(ctx context.Context, expiry time.Duration) ([]*bookingstore.CronBooking, error)
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
	GetRefundIntentOrphans(ctx context.Context, olderThan time.Duration, limit int) ([]*bookingstore.Booking, error)
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
	Insert(ctx context.Context, client *clientstore.Client) error
	GetByID(ctx context.Context, id uuid.UUID) (*clientstore.Client, error)
	GetByComplex(ctx context.Context, complexID uuid.UUID, search string, filters data.Filters) ([]*clientstore.Client, data.Metadata, error)
	Update(ctx context.Context, client *clientstore.Client) error
}

// ClientLookup resolves clients by phone, creating one when none exists.
type ClientLookup interface {
	// allowNameUpdate is what tells the authenticated owner-booking caller
	// apart from the public, unauthenticated one on a phone match — see
	// clientstore.Store.GetOrCreate's own comment for why the two must not share
	// one answer.
	GetOrCreate(ctx context.Context, complexID uuid.UUID, firstName, lastName, phone, email string, allowNameUpdate bool) (*clientstore.Client, error)
	GetByPhone(ctx context.Context, complexID uuid.UUID, phone string) (*clientstore.Client, error)
}

// ClientMetrics computes client-related counters and insights for a complex.
type ClientMetrics interface {
	IncrementNoShows(ctx context.Context, clientID uuid.UUID) error
	CountByComplex(ctx context.Context, complexID uuid.UUID) (int, error)
	GetInsights(ctx context.Context, complexID uuid.UUID, today time.Time) (*clientstore.ClientInsights, error)
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
	GetByBookingID(ctx context.Context, bookingID uuid.UUID) (*paymentstore.Payment, error)
	ListByBookingID(ctx context.Context, bookingID uuid.UUID) ([]*paymentstore.Payment, error)
	GetByMPPaymentID(ctx context.Context, mpPaymentID string) (*paymentstore.Payment, error)
}

// PaymentWriter creates and updates payments, including atomic booking confirmation.
type PaymentWriter interface {
	Insert(ctx context.Context, payment *paymentstore.Payment) error
	InsertAndConfirmBooking(ctx context.Context, payment *paymentstore.Payment, booking *bookingstore.Booking) error
	ConfirmWebhookPayment(ctx context.Context, payment *paymentstore.Payment, booking *bookingstore.Booking) error
	Update(ctx context.Context, payment *paymentstore.Payment) error
}

// PaymentRefunder runs the refund lifecycle: claim, then call the provider, then
// record the outcome. Each method is its own short committed transaction, so no
// database resource is held while MercadoPago is being called.
//
// It is separate from PaymentWriter because a refund spans three tables — the
// payment, its booking and the attempt record — which is a different
// responsibility from writing a payment row.
type PaymentRefunder interface {
	ClaimRefund(ctx context.Context, paymentID uuid.UUID) (*paymentstore.RefundClaim, error)
	// manualOwedCentavos is the booking's still-outstanding cash/transfer
	// balance; see the identical parameter on payments.PaymentStore.
	RecordRefundSuccess(ctx context.Context, claim paymentstore.RefundClaim, manualOwedCentavos int) (refundTotal int, err error)
	RecordRefundFailure(ctx context.Context, claim paymentstore.RefundClaim, cause string) (exhausted bool, err error)
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
	Insert(ctx context.Context, fr *paymentstore.FailedRefund) error
	GetPendingDue(ctx context.Context) ([]*paymentstore.FailedRefund, error)
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
	Insert(ctx context.Context, e *paymentstore.WebhookEvent) error
}

// WebhookEventWorker drives a recorded event to a terminal state, with the same
// claim/retry vocabulary the failed-refund queue uses.
type WebhookEventWorker interface {
	GetPendingDue(ctx context.Context) ([]*paymentstore.WebhookEvent, error)
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
// AuditStore — the trail, written by every module and read by two
// ---------------------------------------------------------------------------

// AuditStore records and reads the audit trail.
//
// It is its own field rather than part of the admin store because the trail
// has two readers — the platform-wide /admin/audit-log view and a venue's own
// — and one writer that is every module in the application.
type AuditStore interface {
	// InsertAuditLog takes its two values as already-encoded JSON: the caller
	// encodes on its own goroutine so the background write never reads a struct
	// the caller still owns.
	InsertAuditLog(ctx context.Context, userID, complexID *uuid.UUID, action, entityType string, entityID *uuid.UUID, oldJSON, newJSON []byte, ipAddr string) error
	ListAuditLogs(ctx context.Context, complexID *uuid.UUID, entityType string, filters data.Filters) ([]*auditstore.AuditLogRow, data.Metadata, error)
}

// ---------------------------------------------------------------------------
// Stores — aggregate of all stores
// ---------------------------------------------------------------------------

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
	// Keys is the MercadoPago credential encryption keyring. complexstore.Store and
	// bookingstore.Store use it to seal mp_access_token/mp_refresh_token on write
	// and open them on read (internal/mpcred). A nil Keys is a valid
	// zero value that Seal and Open both refuse — see crypto.Keyring — so a
	// Config built without it fails loudly the first time a credential is
	// touched, rather than storing or returning plaintext.
	Keys *crypto.Keyring
	// PasswordHashCost is the bcrypt cost the user store reports to whoever
	// hashes a password. Zero means authstore.DefaultHashCost — see
	// authstore.SetPassword for why this is configuration rather than a
	// package variable.
	PasswordHashCost int
	// LinkTokenBuffer is added to a booking's end time to compute a booking
	// link token's expires_at. bookingstore.Store.InsertSafe and
	// booklinkstore.Store.Mint's callers both use it. Zero falls back to
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

// Stores aggregates every store interface used by the application.
type Stores struct {
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
	Admin             adminstore.AdminStore
	Audit             AuditStore
	Reports           reportstore.ReportStore
	Locks             data.LockStore
}

// New builds a Stores with every store backed by the given connection pool
// and the given configuration.
func New(pool *pgxpool.Pool, cfg Config) Stores {
	return newStores(data.NewDB(pool), cfg)
}

// newStores builds the stores over a handle that already retries.
//
// The raw pool does not reach this function, and that is the point: sqlc's
// generated queries are built against whatever is passed to db.New, so if the
// pool were in scope here it could be handed to db.New by accident and every
// generated query would quietly leave the retry behind. Taking only the
// retrying handle makes that impossible to write, and lets a test drive every
// store — generated queries included — without a database.
func newStores(pooled *data.DB, cfg Config) Stores {
	q := db.New(pooled)
	paymentExpiry := cfg.paymentExpiry()
	return Stores{
		Users:             &authstore.Users{DB: pooled, Q: q, HashCost: cfg.PasswordHashCost},
		UserIdentities:    &authstore.Identities{DB: pooled, Q: q},
		Complexes:         &complexstore.Store{DB: pooled, Q: q, Keys: cfg.Keys},
		Courts:            &courtstore.Store{DB: pooled, Q: q, PaymentExpiry: paymentExpiry},
		Bookings:          &bookingstore.Store{DB: pooled, Q: q, PaymentExpiry: paymentExpiry, Keys: cfg.Keys, LinkTokenBuffer: cfg.linkTokenBuffer()},
		BookingLinkTokens: &booklinkstore.Store{DB: pooled},
		Tokens:            &authstore.Tokens{DB: pooled, Q: q},
		Clients:           &clientstore.Store{DB: pooled, Q: q},
		Payments:          &paymentstore.Payments{DB: pooled, Q: q, PaymentExpiry: paymentExpiry},
		EmailVerification: &authstore.EmailVerifications{DB: pooled, Q: q},
		PasswordReset:     &authstore.PasswordResets{DB: pooled},
		FailedRefunds:     &paymentstore.FailedRefunds{DB: pooled},
		WebhookEvents:     &paymentstore.WebhookEvents{DB: pooled},
		Reports:           &reportstore.Store{DB: pooled},
		Locks:             &data.LockModel{DB: pooled, Logger: cfg.Logger},
		SlotLocks:         &bookingstore.SlotLocks{DB: pooled},
		Admin:             &adminstore.Store{DB: pooled},
		Audit:             &auditstore.Store{DB: pooled},
	}
}
