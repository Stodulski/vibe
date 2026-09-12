// Package bookings owns a reservation from the moment a slot is held to the
// moment it is cancelled or played.
//
// Two flows create bookings and they are not the same. The owner books from
// the dashboard, against a court they own, with no payment step. A client
// books from the public page with no account at all: that path holds the slot
// with a lock, creates a MercadoPago preference, and only confirms once the
// payment webhook arrives. The lock is what stops two people paying for the
// same slot at once.
package bookings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// Store is the booking persistence this module uses.
//
// The last block is what the scheduler drives: the four booking cron jobs moved
// out of cmd/api into Service, so the sweeps they run are reads and writes of
// this domain like any other.
type Store interface {
	// The reads other domains enter this one through, which Service's own
	// rules go through too. They live on FacadeStore (facade.go) because
	// *Facade is built over them alone, before any domain service exists.
	FacadeStore

	InsertSafe(ctx context.Context, b *bookingstore.Booking) error

	// The scheduled sweeps.
	GetForReminder2hEnriched(ctx context.Context, now time.Time) ([]*bookingstore.CronBooking, error)
	MarkReminderSent2h(ctx context.Context, id uuid.UUID) error
	GetExpiredPendingEnriched(ctx context.Context, expiry time.Duration) ([]*bookingstore.CronBooking, error)
	CompletePastBookings(ctx context.Context) (int64, error)
}

// ClientStore resolves the person a booking is for. Public bookings create the
// client record on the fly, since they arrive with no account.
type ClientStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*clientstore.Client, error)
	// allowNameUpdate must be true only from the authenticated owner path
	// (create.go) and false from the public one (public.go) — see
	// clientstore.Store.GetOrCreate's comment for why an unauthenticated
	// caller must never be able to overwrite an existing client's name.
	GetOrCreate(ctx context.Context, complexID uuid.UUID, firstName, lastName, phone, email string, allowNameUpdate bool) (*clientstore.Client, error)
	IncrementNoShows(ctx context.Context, clientID uuid.UUID) error
}

// ComplexReader supplies the venue and its opening hours.
type ComplexReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*complexstore.Complex, error)
	GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*complexstore.Schedule, error)
	UpdateMPCredentials(ctx context.Context, complexID uuid.UUID, accessToken, refreshToken, userID string, expiresIn int) error
}

// CourtReader supplies the court, its price bands, and the hours its owner has
// taken off sale.
//
// The blocked slots belong here rather than in a separate interface because
// they answer the same question the court and its bands do — what this surface
// is selling on a given day — and because a write path that cannot see them
// sells hours the storefront has already withdrawn.
type CourtReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*courtstore.Court, error)
	GetPrices(ctx context.Context, courtID uuid.UUID) ([]*courtstore.CourtPrice, error)
	GetBlockedSlots(ctx context.Context, courtID uuid.UUID, date time.Time) ([]*courtstore.BlockedSlot, error)
}

// PaymentStore records the payment a public booking is waiting on.
type PaymentStore interface {
	Insert(ctx context.Context, p *paymentstore.Payment) error
	GetByBookingID(ctx context.Context, bookingID uuid.UUID) (*paymentstore.Payment, error)
	ListByBookingID(ctx context.Context, bookingID uuid.UUID) ([]*paymentstore.Payment, error)
	InsertAndConfirmBooking(ctx context.Context, payment *paymentstore.Payment, booking *bookingstore.Booking) error
	// RecordManualRefund closes out a partial_refund booking's remaining
	// cash/transfer rows, in one transaction with the booking's move to
	// 'refunded'. See ManualRefund in actions.go.
	RecordManualRefund(ctx context.Context, bookingID uuid.UUID) (returnedCentavos int, err error)
}

// SlotLocker holds a slot while a client goes through checkout.
//
// Without it two people can both reach MercadoPago for the same court and
// time, and one of them pays for a slot that is already gone.
type SlotLocker interface {
	AcquireLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime, endTime string, bookingID *uuid.UUID, ttl time.Duration) error
	ReleaseLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime string) error
}

// Checkout is the MercadoPago side of a public booking.
type Checkout interface {
	CreatePreference(ctx context.Context, input mp.CreatePreferenceInput) (*mp.Preference, error)
	RefreshOAuthToken(ctx context.Context, refreshToken string) (*mp.OAuthTokens, error)
	UpdatePreferenceExpired(ctx context.Context, preferenceID string, caller mp.Caller) error
}

// WhatsAppVerifier authenticates Meta's inbound webhook.
//
// The webhook lives in this module because the messages it carries are replies
// to booking confirmations. Today those replies are only logged — cancelling
// still happens through the web app — so nothing here writes to a store.
type WhatsAppVerifier interface {
	VerifyWebhook(r *http.Request) (string, error)
	VerifySignature(r *http.Request, body []byte) error
}

// Refunder returns a cancelled booking's money and says what became of it.
//
// It is declared here, by the consumer, rather than imported from the payments
// package: bookings needs one operation, and naming it locally keeps the
// dependency to that one operation.
//
// The outcome type it answers with lives in data rather than in payments for the
// same reason. Both sides of this boundary need the vocabulary — the payments
// module produces it, these handlers turn it into a sentence for the client — and
// neither may depend on the other. data is where this codebase already keeps the
// domain nouns both layers share (Booking, Payment, RefundClaim).
//
// It used to return nothing at all, and every caller had to infer the answer from
// a struct the refund path never writes to.
type Refunder interface {
	AutoRefundIfPaid(ctx context.Context, booking *bookingstore.Booking) paymentstore.RefundOutcome
}

// LinkResolver resolves the plaintext access token presented to the three
// public routes (specs/booking-link-credential) into the booking it was
// minted for. Declared here, by the consumer, one method, matching this
// package's other narrow consumer-declared interfaces (e.g. Refunder).
//
// ErrRecordNotFound means no row carries that token's hash — never that it
// expired; resolveLink (public.go) is what decides expiry, via
// pricing.LinkLive.
type LinkResolver interface {
	ResolveBooking(ctx context.Context, plaintext string) (*bookingstore.Booking, time.Time, error)
}

// LinkTokenStore is the scheduled half of the booking-link tokens: minting a
// fresh one for a reminder, because booking_link_tokens stores only a hash and
// the plaintext sent at confirmation cannot be read back, and sweeping the
// tokens of bookings that have reached a terminal state.
//
// Kept apart from LinkResolver, which is the request path's single read, the
// same segregation stores.BookingLinkTokenStore's own comment draws.
type LinkTokenStore interface {
	Mint(ctx context.Context, bookingID uuid.UUID, expiresAt time.Time) (plaintext string, err error)
	DeleteExpiredTerminal(ctx context.Context, retention time.Duration) error
}

// Notifier tells the client what happened to their booking.
type Notifier interface {
	BookingConfirmed(c notifications.BookingConfirmation)
	BookingCancelled(c notifications.Cancellation)
	// ReminderDue is the two-hour reminder the scheduler sends.
	ReminderDue(r notifications.Reminder)
}

// Broadcaster pushes a change to the owner's open dashboards.
type Broadcaster interface {
	PublishBookingChanged(complexID uuid.UUID)
}

// Recorder writes the audit trail.
type Recorder interface {
	Record(e audit.Entry)
}

// Config is what this module needs from application configuration.
type Config struct {
	// FrontendURL builds the links a client receives.
	FrontendURL string
	// BackendURL is the origin MercadoPago sends its webhook to.
	BackendURL string
	// Environment gates behaviour that differs locally.
	Environment string
	// GracePeriod is how long after booking a client may cancel and still be
	// refunded, regardless of the complex's own window.
	GracePeriod time.Duration
	// PaymentExpiry is how long an unpaid booking holds its slot.
	PaymentExpiry time.Duration
	// SlotLockTTL is how long a slot is held during checkout.
	SlotLockTTL time.Duration
	// TrustProxies decides which address the audit trail records.
	TrustProxies bool
	// WhatsAppEnabled reflects whether the channel is configured.
	WhatsAppEnabled bool
	// LinkTokenBuffer is added to a booking's end time to compute the
	// expires_at of the token the two-hour reminder mints. Same value
	// bookingstore.Store.InsertSafe uses for its own mint — both come from
	// the one -booking-link-token-buffer flag.
	LinkTokenBuffer time.Duration
}

// Dependencies groups what NewService needs.
type Dependencies struct {
	// Facade is the cross-domain entry point this Service embeds, so the
	// handler and cron paths reach those reads through the same code every
	// other domain does. It must wrap the same store as Store below; cmd/api
	// builds it from that store and hands it to both.
	Facade       *Facade
	Store        Store
	Clients      ClientStore
	Complexes    ComplexReader
	Courts       CourtReader
	Payments     PaymentStore
	Locks        SlotLocker
	Checkout     Checkout
	Refunds      Refunder
	LinkResolver LinkResolver
	LinkTokens   LinkTokenStore
	Notify       Notifier
	Realtime     Broadcaster
	Audit        Recorder
	Logger       *slog.Logger
	Run          func(func())
}

// Actor is who a change is attributed to, as the handler read it off the
// request. The service needs it for the audit trail and for nothing else.
type Actor struct {
	// UserID is the authenticated owner, or nil for a client acting on a
	// public route and for a scheduled sweep.
	UserID *uuid.UUID
	// IP is the address the request arrived from.
	IP string
}

// Handler serves the booking routes. It decodes, validates the request's shape,
// and maps the service's domain errors onto HTTP; every rule lives in the
// Service.
type Handler struct {
	svc *Service
	// whatsapp is held for the two verification calls alone: both are computed
	// over the request, so they are an HTTP concern and are checked here,
	// before anything reaches the service.
	whatsapp     WhatsAppVerifier
	respond      *httpx.Refuser
	logger       *slog.Logger
	trustProxies bool
}

// refusals is this module's whole error-to-status table: every domain error of
// its own that is a refusal rather than a fault, and the status and message it
// earns. Everything absent from it — the shared sentinels, and the errors
// refuse still has to match on their type rather than their identity — is
// answered by internal/httpx or by refuse itself.
var refusals = httpx.Refusals{
	ErrSlotTaken:               httpx.SlotUnavailable(slotTakenMessage),
	ErrClientBlocked:           httpx.Forbidden("your account is blocked, contact the complex for more information"),
	ErrMercadoPagoNotConnected: httpx.BadRequest("the complex does not have MercadoPago connected, contact the complex"),
	ErrCheckoutUnavailable:     httpx.Unavailable("no se pudo crear el enlace de pago, intente nuevamente"),
	ErrVenueGone:               httpx.Gone(venueGoneMessage),
	ErrLinkExpired:             httpx.Gone(linkExpiredMessage),
	// H-15 / R4: ConfirmPayment's status check runs against a read taken
	// before the store's own transaction opened, so a client cancellation
	// landing in that gap used to fall through every named case and answer 500
	// — after the owner had already taken the client's cash at the counter,
	// with no way to tell whether it was recorded. These two sentinels are the
	// re-read inside InsertAndConfirmBooking's own transaction catching exactly
	// that race; naming them here turns it into a 409 the owner can act on.
	bookingstore.ErrBookingNotConfirmable: httpx.Conflict(confirmPaymentRaceMessage),
	bookingstore.ErrBookingCancelled:      httpx.Conflict(confirmPaymentRaceMessage),
}

// NewHandler returns a Handler backed by the given service.
func NewHandler(svc *Service, whatsapp WhatsAppVerifier, respond *httpx.Responder, logger *slog.Logger, trustProxies bool) *Handler {
	return &Handler{
		svc:          svc,
		whatsapp:     whatsapp,
		respond:      respond.WithRefusals(refusals),
		logger:       logger,
		trustProxies: trustProxies,
	}
}

// actor reads who is making the change off the request.
func (h *Handler) actor(r *http.Request) Actor {
	var userID *uuid.UUID
	if user, ok := httpx.ContextGetAuthenticatedUser(r); ok {
		userID = &user.ID
	}
	return Actor{UserID: userID, IP: httpx.ClientIP(r, h.trustProxies)}
}

// ErrEditConflict reports that a booking moved out from under a read: the row
// the update aimed at was gone by the time it ran. It is distinct from
// data.ErrRecordNotFound, which means the booking was never this complex's to
// begin with, because the two answer the caller differently — 409 against 404.
var ErrEditConflict = fmt.Errorf("booking changed before the update: %w", data.ErrEditConflict)

// ErrNoActor reports a write that reached the service with no authenticated
// user on it, on a route whose guard should have made that impossible. The
// handler answers it the way it always did: the token is not valid.
var ErrNoActor = errors.New("bookings: the change has no authenticated user")

// ErrVenueGone reports a live booking link whose venue — or whose court — has
// since been soft-deleted. It is not the same fact as an unknown token, and it
// can never succeed by being retried.
var ErrVenueGone = errors.New("bookings: the booking's venue is no longer available")

// ErrLinkExpired reports a booking link past the point where it still
// authorizes anything (specs/booking-link-credential).
var ErrLinkExpired = errors.New("bookings: the booking link has expired")

// FieldError is a rule the request breaks that belongs to one named field. The
// handler answers 422 naming it — the same field error it used to add to its
// own validator right where the rule ran.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

// ValidationError carries a whole validator's field errors, for the one use
// case (Update) that accumulates several before deciding.
type ValidationError struct{ Errors map[string]string }

func (e *ValidationError) Error() string { return "bookings: the request is not valid" }

// ConflictError is a rule the request breaks that is about the booking's state
// rather than one field. The handler answers 409 with its message, unchanged.
type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

// StateError is an operation the booking's current state does not allow. The
// handler answers 400 with its message, unchanged.
type StateError struct{ Message string }

func (e *StateError) Error() string { return e.Message }

// record writes an audit entry for a change to a booking. Every write in this
// module acts on a booking, so the entity type is fixed.
func (s *Service) record(actor Actor, complexID uuid.UUID, action string, bookingID *uuid.UUID, newVal any) {
	s.audit.Record(audit.Entry{
		UserID:     actor.UserID,
		ComplexID:  &complexID,
		Action:     action,
		EntityType: "booking",
		EntityID:   bookingID,
		NewValue:   newVal,
		IPAddress:  actor.IP,
	})
}
