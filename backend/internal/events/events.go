// Package events is the open-play events domain ("canchas libres"): an event the
// owner publishes that takes courts off sale for a window of one day, and the
// people who sign up for it one by one, paying online.
//
// The domain owns the rules of the event and its sign-ups. Money is not its own:
// a sign-up is paid through the existing MercadoPago checkout and ledger, and
// refunded through the payments domain's refund lifecycle.
package events

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/events/notice"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// Store persists events and their registrations.
type Store interface {
	CreateEvent(ctx context.Context, e *eventstore.Event, courtIDs []uuid.UUID) error
	CancelEvent(ctx context.Context, id, by uuid.UUID, reason *string) (*eventstore.Event, []uuid.UUID, error)
	GetEvent(ctx context.Context, id uuid.UUID) (*eventstore.Event, error)
	ListEvents(ctx context.Context, complexID uuid.UUID, from, to time.Time, includeCancelled bool) ([]*eventstore.Event, error)
	SignUp(ctx context.Context, in eventstore.SignUpInput) (*eventstore.Registration, string, error)
	GetRegistration(ctx context.Context, id uuid.UUID) (*eventstore.Registration, error)
	GetRegistrationByToken(ctx context.Context, plaintext string) (*eventstore.Registration, error)
	ListRegistrations(ctx context.Context, eventID uuid.UUID, liveOnly bool) ([]*eventstore.RegistrationRow, error)
	CancelRegistration(ctx context.Context, id uuid.UUID, by string) (*eventstore.Registration, error)
	ExpireHolds(ctx context.Context) ([]eventstore.ExpiredHold, error)
	OwnerCancelledStillPaid(ctx context.Context, grace time.Duration, limit int) ([]eventstore.PendingRefund, error)
}

// ComplexReader supplies the venue a request names, by id or by public slug.
type ComplexReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*complexstore.Complex, error)
	GetBySlug(ctx context.Context, slug string) (*complexstore.Complex, error)
	// RefreshMPCredentials renews the seller's MercadoPago credentials; see
	// checkout.Refresher.
	RefreshMPCredentials(ctx context.Context, complexID uuid.UUID, staleAccessToken string) (string, error)
}

// CourtReader supplies the courts an event may hold.
type CourtReader interface {
	GetByComplex(ctx context.Context, complexID uuid.UUID) ([]*courtstore.Court, error)
}

// ClientReader answers whether the venue blocked a phone.
type ClientReader interface {
	GetByPhone(ctx context.Context, complexID uuid.UUID, phone string) (*clientstore.Client, error)
}

// Payments is the ledger and refund side of a sign-up, implemented by the
// payments service.
type Payments interface {
	Insert(ctx context.Context, p *paymentstore.Payment) error
	RefundRegistration(ctx context.Context, regID uuid.UUID) paymentstore.RefundOutcome
}

// Checkout is the MercadoPago side of a sign-up.
type Checkout interface {
	CreatePreference(ctx context.Context, input mp.CreatePreferenceInput) (*mp.Preference, error)
}

// Notifier sends the notices of a sign-up.
type Notifier interface {
	EventCancelled(notifications.EventCancellation)
}

// Broadcaster pushes a change to the owner's open dashboards.
type Broadcaster interface {
	PublishBookingChanged(complexID uuid.UUID)
}

// Recorder is the audit trail this module writes to.
type Recorder interface {
	Record(e audit.Entry)
}

// Config is what this module needs from application configuration.
type Config struct {
	// PublicURL is the public storefront origin the links a person receives
	// (the event page, the checkout redirects) are built on.
	PublicURL string
	// BackendURL is the origin MercadoPago sends its webhook to.
	BackendURL string
	// GracePeriod is how long after signing up a person may cancel and still be
	// refunded, regardless of the complex's own window.
	GracePeriod time.Duration
	// PaymentExpiry is how long an unpaid sign-up holds its spot.
	PaymentExpiry time.Duration
	// MaxHoldsPerIP is the most live spot holds one address may have.
	MaxHoldsPerIP int
	// RefundGrace is how long after an owner's cancellation the sweep leaves a
	// participant's refund to the cancel request itself before driving it.
	RefundGrace time.Duration
}

// Dependencies groups what NewService needs.
type Dependencies struct {
	Store     Store
	Complexes ComplexReader
	Courts    CourtReader
	Clients   ClientReader
	Payments  Payments
	Checkout  Checkout
	Notify    Notifier
	Realtime  Broadcaster
	Audit     Recorder
	Logger    *slog.Logger
	Run       func(func())
}

// Service holds this module's rules.
type Service struct {
	store     Store
	complexes ComplexReader
	courts    CourtReader
	clients   ClientReader
	payments  Payments
	checkout  Checkout
	notify    Notifier
	realtime  Broadcaster
	audit     Recorder
	logger    *slog.Logger
	cfg       Config
}

// NewService returns a Service backed by the given dependencies.
func NewService(d Dependencies, cfg Config) *Service {
	return &Service{
		store: d.Store, complexes: d.Complexes, courts: d.Courts, clients: d.Clients,
		payments: d.Payments, checkout: d.Checkout, notify: d.Notify, realtime: d.Realtime,
		audit: d.Audit, logger: d.Logger, cfg: cfg,
	}
}

// Actor is who is acting: the authenticated owner, or nobody on a public route.
type Actor struct {
	UserID *uuid.UUID
	IP     string
}

// ErrNoActor is returned when an owner route runs without an authenticated user.
var ErrNoActor = errors.New("events: no authenticated user")

// Errors a visitor's request can end in.
var (
	// ErrMercadoPagoNotConnected reports a venue that cannot take money.
	ErrMercadoPagoNotConnected = errors.New("events: the complex has no usable MercadoPago credential")
	// ErrCheckoutUnavailable reports that MercadoPago would not create the
	// checkout; the held spot was released.
	ErrCheckoutUnavailable = errors.New("events: the checkout link could not be created")
	// ErrClientBlocked reports a phone the venue blocked.
	ErrClientBlocked = errors.New("events: the phone is blocked by the complex")
	// ErrEventFull, ErrEventClosed, ErrAlreadyRegistered and ErrTooManyHolds are
	// the reasons a public sign-up is refused; each answers its own problem kind.
	ErrEventFull         = errors.New("events: the event is full")
	ErrEventClosed       = errors.New("events: sign-up for the event is closed")
	ErrAlreadyRegistered = errors.New("events: the phone is already signed up for the event")
	ErrTooManyHolds      = errors.New("events: too many spots held from this address")
	// ErrNotEditable reports an operation the event's state does not allow.
	ErrNotEditable = errors.New("events: the event no longer allows that")
)

// ValidationError is one or more fields of a request the rules reject.
type ValidationError struct{ Errors map[string]string }

func (e *ValidationError) Error() string { return "events: invalid request" }

// StateError is an operation the current state does not allow; the handler
// answers 400 with its message.
type StateError struct{ Message string }

func (e *StateError) Error() string { return e.Message }

// ConflictError is a rule the request breaks that is about the event's state or
// its hours rather than one field; the handler answers 409 with its message.
type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

// record writes an audit entry for a change to an event.
func (s *Service) record(actor Actor, complexID uuid.UUID, action, entityType string, entityID uuid.UUID, newVal any) {
	s.audit.Record(audit.Entry{
		UserID: actor.UserID, ComplexID: &complexID, Action: action,
		EntityType: entityType, EntityID: &entityID, NewValue: newVal, IPAddress: actor.IP,
	})
}

// notifyCancelled sends the cancellation notice to a person, never failing the
// cancellation: a notice that cannot be built is logged.
func (s *Service) notifyCancelled(reg *eventstore.Registration, ev *eventstore.Event, complex *complexstore.Complex, byOwner bool, outcome paymentstore.RefundOutcome) {
	line, amount := refundLine(outcome)
	s.notify.EventCancelled(notice.Cancelled(reg, ev, complex, s.cfg.PublicURL, byOwner, line, amount))
}

// refundLine is the sentence and figure a cancellation notice carries about the
// person's money.
func refundLine(outcome paymentstore.RefundOutcome) (line string, centavos int) {
	switch outcome.Result {
	case paymentstore.RefundIssued, paymentstore.RefundAlreadyIssued:
		return notice.LineRefunded, outcome.AmountCentavos
	case paymentstore.RefundQueued, paymentstore.RefundManual:
		return notice.LineRefundQueued, 0
	case paymentstore.RefundNotEligible:
		return notice.LineKept, 0
	case paymentstore.RefundNone:
		return "", 0
	}
	return "", 0
}
