// Package payments handles money moving in and out: the MercadoPago webhook
// that confirms a booking was paid for, the refunds that follow a
// cancellation, and the retry queue for refunds the provider rejected.
//
// Everything here is driven by MercadoPago rather than by a user, so it is
// written to be safe under retries: MercadoPago delivers a webhook more than
// once, out of order, and sometimes for a payment we have never seen.
package payments

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
)

// PaymentStore is the payment persistence this module uses.
type PaymentStore interface {
	GetByMPPaymentID(ctx context.Context, mpPaymentID string) (*data.Payment, error)
	GetByBookingID(ctx context.Context, bookingID uuid.UUID) (*data.Payment, error)
	ListByBookingID(ctx context.Context, bookingID uuid.UUID) ([]*data.Payment, error)
	InsertAndConfirmBooking(ctx context.Context, payment *data.Payment, booking *data.Booking) error
	ConfirmWebhookPayment(ctx context.Context, payment *data.Payment, booking *data.Booking) error
	Update(ctx context.Context, payment *data.Payment) error

	// The refund lifecycle, in the order it runs. ClaimRefund commits a durable
	// reservation, the provider is then called with no database resource held, and
	// exactly one of the two recorders closes the attempt out.
	ClaimRefund(ctx context.Context, paymentID uuid.UUID) (*data.RefundClaim, error)
	// manualOwedCentavos is the booking's cash/transfer balance still owed by
	// hand — computed by manualBalance or manualOwedForBooking — so the
	// booking is written 'partial_refund' rather than 'refunded' whenever this
	// refund settles the automatic half but leaves that balance outstanding.
	RecordRefundSuccess(ctx context.Context, claim data.RefundClaim, manualOwedCentavos int) (refundTotal int, err error)
	RecordRefundFailure(ctx context.Context, claim data.RefundClaim, cause string) (exhausted bool, err error)
}

// BookingStore is the booking side of confirming and cancelling.
type BookingStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*data.Booking, error)
	Update(ctx context.Context, booking *data.Booking) error
}

// RefundIntentStore is the reconciliation sweep's store layer
// (refund-intent-durability spec): finding an orphaned refund intent,
// claiming it exclusively for one sweep run, and clearing it once
// AutoRefundIfPaid is done with a booking. Kept separate from BookingStore
// above, which stays its ordinary two methods — the sweep queue is a
// different responsibility from confirming and cancelling, the same
// separation FailedRefundStore already draws from PaymentStore.
type RefundIntentStore interface {
	GetRefundIntentOrphans(ctx context.Context, olderThan time.Duration, limit int) ([]*data.Booking, error)
	ClaimRefundIntent(ctx context.Context, id uuid.UUID, seen time.Time) error
	ClearRefundIntent(ctx context.Context, id uuid.UUID) error
}

// LinkMinter mints a fresh access token for a booking. It is this module's
// only use of the booking-link-token store (internal/data/booking_link_tokens.go):
// the webhook-confirmation path runs in a separate request from the booking's
// own insert (which already minted its own token via InsertSafe), so it
// cannot reuse that mint and has to make its own — see design.md's "two live
// tokens" consequence. Declared here, by the consumer, one method, matching
// this codebase's other narrow consumer-declared interfaces (e.g. bookings.Refunder).
type LinkMinter interface {
	Mint(ctx context.Context, bookingID uuid.UUID, expiresAt time.Time) (plaintext string, err error)
}

// ClientStore is the client side: a confirmed booking updates their counters.
type ClientStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*clientstore.Client, error)
	Update(ctx context.Context, c *clientstore.Client) error
}

// ComplexReader supplies the complex, including the seller credentials a
// refund has to be issued against.
type ComplexReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*complexstore.Complex, error)
}

// CourtReader supplies the court named in a confirmation message.
type CourtReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*courtstore.Court, error)
}

// FailedRefundStore is the queue of refunds still owed to a client.
//
// It is only read and claimed here. Rows are created by ClaimRefund and closed by
// RecordRefundSuccess / RecordRefundFailure, because an attempt has to be written
// and resolved in the same transaction as the money state it describes — which is
// exactly what this module used to get wrong by issuing them as separate calls.
type FailedRefundStore interface {
	GetPendingDue(ctx context.Context) ([]*data.FailedRefund, error)
	MarkProcessing(ctx context.Context, id uuid.UUID) error
}

// WebhookEventStore is the durable inbox of provider notifications.
//
// The endpoint records an event through it and commits before answering 200, so
// the acknowledgement means "this is ours now" rather than "the bytes arrived".
// Everything after that point works from the committed row: a failure requeues
// it instead of dropping it with the goroutine that hit the failure.
type WebhookEventStore interface {
	Insert(ctx context.Context, e *data.WebhookEvent) error
	GetPendingDue(ctx context.Context) ([]*data.WebhookEvent, error)
	Claim(ctx context.Context, id uuid.UUID) (claimed bool, err error)
	MarkProcessed(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, cause string) (exhausted bool, err error)
}

// Locker provides the advisory lock that makes webhook handling idempotent
// across instances.
type Locker interface {
	TryAdvisory(ctx context.Context, key string) (acquired bool, release func(), err error)
}

// Provider is the MercadoPago client.
type Provider interface {
	VerifyWebhookSignature(r *http.Request, dataID string) error
	GetPayment(ctx context.Context, paymentID string, caller mp.Caller) (*mp.Payment, error)
	RefundPayment(ctx context.Context, paymentID string, amount float64, caller mp.Caller) (*mp.Refund, error)
}

// Notifier tells the client what happened to their money.
type Notifier interface {
	BookingConfirmed(c notifications.BookingConfirmation)
	DepositRefunded(r notifications.Refund)
}

// Broadcaster pushes a booking change to the owner's open dashboards.
type Broadcaster interface {
	PublishBookingChanged(complexID uuid.UUID)
}

// Recorder is the audit trail this module writes to.
//
// It is declared here, by the consumer, matching internal/bookings,
// internal/courts and internal/complexes — the module depends on one method,
// not on the audit package's Recorder type.
type Recorder interface {
	Record(e audit.Entry)
}

// Config is what this module needs from application configuration.
type Config struct {
	// FrontendURL builds the cancellation links sent to clients.
	FrontendURL string
	// CancellationGracePeriod is how long after booking a client may cancel
	// and still be refunded, regardless of the complex's window.
	CancellationGracePeriod time.Duration
	// LinkTokenBuffer is added to a booking's end time to compute the
	// expires_at of the token this module mints on webhook confirmation.
	// Same value BookingModel.InsertSafe uses for its own mint
	// (data.Config.LinkTokenBuffer) — both come from the one
	// -booking-link-token-buffer flag.
	LinkTokenBuffer time.Duration
}

// Handler serves the payment webhook and owns the refund flows.
type Handler struct {
	payments      PaymentStore
	bookings      BookingStore
	clients       ClientStore
	complexes     ComplexReader
	courts        CourtReader
	failedRefunds FailedRefundStore
	webhookEvents WebhookEventStore
	refundIntents RefundIntentStore
	linkTokens    LinkMinter
	locks         Locker
	provider      Provider
	notify        Notifier
	realtime      Broadcaster
	audit         Recorder
	respond       *httpx.Responder
	logger        *slog.Logger
	cfg           Config
	// run schedules background work on the application's tracked goroutines.
	run func(func())
}

// Dependencies groups what NewHandler needs, because the list is long enough
// that a positional call would be unreadable and easy to mis-order.
type Dependencies struct {
	Payments      PaymentStore
	Bookings      BookingStore
	Clients       ClientStore
	Complexes     ComplexReader
	Courts        CourtReader
	FailedRefunds FailedRefundStore
	WebhookEvents WebhookEventStore
	RefundIntents RefundIntentStore
	LinkTokens    LinkMinter
	Locks         Locker
	Provider      Provider
	Notify        Notifier
	Realtime      Broadcaster
	Audit         Recorder
	Respond       *httpx.Responder
	Logger        *slog.Logger
	Run           func(func())
}

// NewHandler returns a Handler.
func NewHandler(d Dependencies, cfg Config) *Handler {
	// A nil recorder is refused here rather than left to panic at the first
	// refund — or, worse, made nil-safe. An audit trail that silently drops
	// entries is the one kind of broken this table cannot survive: it still
	// answers every query, and every answer is short. Failing at construction
	// is what makes "there is no entry" mean "it did not happen".
	if d.Audit == nil {
		panic("payments: NewHandler needs an audit recorder; the money path's entries are not optional")
	}
	return &Handler{
		payments:      d.Payments,
		bookings:      d.Bookings,
		clients:       d.Clients,
		complexes:     d.Complexes,
		courts:        d.Courts,
		failedRefunds: d.FailedRefunds,
		webhookEvents: d.WebhookEvents,
		refundIntents: d.RefundIntents,
		linkTokens:    d.LinkTokens,
		locks:         d.Locks,
		provider:      d.Provider,
		notify:        d.Notify,
		realtime:      d.Realtime,
		audit:         d.Audit,
		respond:       d.Respond,
		logger:        d.Logger,
		cfg:           cfg,
		run:           d.Run,
	}
}

// Routes registers the webhook.
//
// It takes no guard because MercadoPago has no session: the request is
// authenticated by its signature, which the handler verifies before reading
// anything else out of the body.
func (h *Handler) Routes(router httpx.Router, _ httpx.Guards) {
	router.HandlerFunc(http.MethodPost, "/api/v1/webhooks/mercadopago", h.MercadoPagoWebhook)
}
