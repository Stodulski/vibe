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
type Store interface {
	GetByComplex(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time, filters data.Filters) ([]*bookingstore.Booking, data.Metadata, error)
	GetByID(ctx context.Context, id uuid.UUID) (*bookingstore.Booking, error)
	InsertSafe(ctx context.Context, b *bookingstore.Booking) error
	Update(ctx context.Context, b *bookingstore.Booking) error
}

// ClientStore resolves the person a booking is for. Public bookings create the
// client record on the fly, since they arrive with no account.
type ClientStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*clientstore.Client, error)
	// allowNameUpdate must be true only from the authenticated owner path
	// (create.go) and false from the public one (public.go) — see
	// data.ClientModel.GetOrCreate's comment for why an unauthenticated
	// caller must never be able to overwrite an existing client's name.
	GetOrCreate(ctx context.Context, complexID uuid.UUID, firstName, lastName, phone, email string, allowNameUpdate bool) (*clientstore.Client, error)
	Update(ctx context.Context, c *clientstore.Client) error
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
	Update(ctx context.Context, p *paymentstore.Payment) error
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

// Notifier tells the client what happened to their booking.
type Notifier interface {
	BookingConfirmed(c notifications.BookingConfirmation)
	BookingCancelled(c notifications.Cancellation)
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
}

// Handler serves the booking routes.
type Handler struct {
	store        Store
	clients      ClientStore
	complexes    ComplexReader
	courts       CourtReader
	payments     PaymentStore
	locks        SlotLocker
	checkout     Checkout
	whatsapp     WhatsAppVerifier
	refunds      Refunder
	linkResolver LinkResolver
	notify       Notifier
	realtime     Broadcaster
	audit        Recorder
	respond      *httpx.Responder
	logger       *slog.Logger
	cfg          Config
	run          func(func())
}

// Dependencies groups what NewHandler needs.
type Dependencies struct {
	Store        Store
	Clients      ClientStore
	Complexes    ComplexReader
	Courts       CourtReader
	Payments     PaymentStore
	Locks        SlotLocker
	Checkout     Checkout
	WhatsApp     WhatsAppVerifier
	Refunds      Refunder
	LinkResolver LinkResolver
	Notify       Notifier
	Realtime     Broadcaster
	Audit        Recorder
	Respond      *httpx.Responder
	Logger       *slog.Logger
	Run          func(func())
}

// NewHandler returns a Handler.
func NewHandler(d Dependencies, cfg Config) *Handler {
	return &Handler{
		store:        d.Store,
		clients:      d.Clients,
		complexes:    d.Complexes,
		courts:       d.Courts,
		payments:     d.Payments,
		locks:        d.Locks,
		checkout:     d.Checkout,
		whatsapp:     d.WhatsApp,
		refunds:      d.Refunds,
		linkResolver: d.LinkResolver,
		notify:       d.Notify,
		realtime:     d.Realtime,
		audit:        d.Audit,
		respond:      d.Respond,
		logger:       d.Logger,
		cfg:          cfg,
		run:          d.Run,
	}
}

// Routes registers this module's endpoints.
//
// The /book group is public because clients book without an account; each of
// those endpoints resolves the booking by an opaque, expiring access token
// (specs/booking-link-credential) — never by the booking's primary key, which
// is not a session and not a credential.
func (h *Handler) Routes(router httpx.Router, guards httpx.Guards) {
	owner := func(next http.HandlerFunc) http.HandlerFunc {
		return guards.RequireAuth(guards.RequireComplexOwner(next))
	}

	router.HandlerFunc(http.MethodGet, "/api/v1/webhooks/whatsapp", h.WhatsAppVerify)
	router.HandlerFunc(http.MethodPost, "/api/v1/webhooks/whatsapp", h.WhatsAppWebhook)

	router.HandlerFunc(http.MethodPost, "/api/v1/book", h.PublicBook)
	router.HandlerFunc(http.MethodGet, "/api/v1/book/status", h.PublicStatus)
	router.HandlerFunc(http.MethodGet, "/api/v1/book/cancel-info", h.PublicCancelInfo)
	router.HandlerFunc(http.MethodPost, "/api/v1/book/cancel", h.PublicCancel)

	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/bookings", owner(h.List))
	router.HandlerFunc(http.MethodPost, "/api/v1/complexes/:id/bookings", owner(h.Create))
	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/bookings/:bookingID", owner(h.Get))
	router.HandlerFunc(http.MethodPut, "/api/v1/complexes/:id/bookings/:bookingID", owner(h.Update))
	router.HandlerFunc(http.MethodPost, "/api/v1/complexes/:id/bookings/:bookingID/cancel", owner(h.Cancel))
	router.HandlerFunc(http.MethodPost, "/api/v1/complexes/:id/bookings/:bookingID/confirm-payment", owner(h.ConfirmPayment))
	router.HandlerFunc(http.MethodPost, "/api/v1/complexes/:id/bookings/:bookingID/manual-refund", owner(h.ManualRefund))
}

// record writes an audit entry for a change to a booking. Every write in this
// module acts on a booking, so the entity type is fixed.
func (h *Handler) record(r *http.Request, complexID uuid.UUID, action string, bookingID *uuid.UUID, newVal any) {
	var userID *uuid.UUID
	if user, ok := httpx.ContextGetAuthenticatedUser(r); ok {
		userID = &user.ID
	}

	h.audit.Record(audit.Entry{
		UserID:     userID,
		ComplexID:  &complexID,
		Action:     action,
		EntityType: "booking",
		EntityID:   bookingID,
		NewValue:   newVal,
		IPAddress:  httpx.ClientIP(r, h.cfg.TrustProxies),
	})
}
