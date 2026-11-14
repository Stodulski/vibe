// Package tournaments is the tournaments domain: an owner runs a padel
// tournament in categories, pairs sign up and pay online, the owner draws
// groups and/or a knockout bracket, schedules matches on courts and loads
// results, and the bracket advances by itself.
//
// It is built on the open-play events domain. A category's sign-up is an event
// registration (held, paid, confirmed, refunded and swept by the code canchas
// libres uses), a pair is one registration plus its partner, and a scheduled
// match holds its court like an event does. This package owns the rules of the
// tournament and maps them onto the owner's and the visitor's API; the draw,
// result and schedule rules that need the database lock live in its store.
package tournaments

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/events"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// Store is the tournaments persistence this domain needs.
type Store interface {
	Create(ctx context.Context, t *tournamentstore.Tournament, cats []tournamentstore.NewCategory) error
	Get(ctx context.Context, id uuid.UUID) (*tournamentstore.Tournament, error)
	List(ctx context.Context, complexID uuid.UUID, from, to time.Time, includeCancelled bool) ([]*tournamentstore.Tournament, error)
	Categories(ctx context.Context, tournamentID uuid.UUID) ([]*tournamentstore.Category, error)
	Category(ctx context.Context, tournamentID, categoryID uuid.UUID) (*tournamentstore.Category, error)
	Detail(ctx context.Context, id uuid.UUID) (*tournamentstore.Detail, error)
	Cancel(ctx context.Context, id, by uuid.UUID, reason *string) (*tournamentstore.Tournament, error)
	CategoryOfEvent(ctx context.Context, eventID uuid.UUID) (tournamentID, categoryID uuid.UUID, err error)
	PairOf(ctx context.Context, registrationID uuid.UUID) (*tournamentstore.Pair, error)
	DrawGenerated(ctx context.Context, eventID uuid.UUID) (bool, error)
	GenerateDraw(ctx context.Context, in tournamentstore.DrawInput) error
	MovePair(ctx context.Context, categoryID, pairA, pairB uuid.UUID) error
	SetResult(ctx context.Context, in tournamentstore.ResultInput) (*tournamentstore.Match, error)
	WithdrawPair(ctx context.Context, registrationID uuid.UUID) error
	Schedule(ctx context.Context, in tournamentstore.ScheduleInput) (*tournamentstore.Match, error)
	Unschedule(ctx context.Context, matchID uuid.UUID) (*tournamentstore.Match, error)
	// SetCourtFee changes what each pair pays per match; ChargeCourtFee and
	// UndoCourtFee collect and take back that fee at the complex, through the cash.
	SetCourtFee(ctx context.Context, categoryID uuid.UUID, fee int) error
	ChargeCourtFee(ctx context.Context, in tournamentstore.ChargeInput) (*tournamentstore.MatchFee, error)
	UndoCourtFee(ctx context.Context, matchID uuid.UUID, side int, by uuid.UUID) error
	PairHooks(payerPhone string, partner tournamentstore.Player) *eventstore.SignUpHooks
}

// Events is the open-play events domain this one is built on: the sign-up, its
// money, and the cancellation of a category's container.
type Events interface {
	PublicComplex(ctx context.Context, slug string) (*complexstore.Complex, error)
	AnyEvent(ctx context.Context, complexID, eventID uuid.UUID) (*eventstore.Event, error)
	SignUpWith(ctx context.Context, actor events.Actor, complex *complexstore.Complex, ev *eventstore.Event, in events.SignUpInput, hooks *eventstore.SignUpHooks) (*events.SignUpResult, error)
	CancelAny(ctx context.Context, complex *complexstore.Complex, actor events.Actor, eventID uuid.UUID, reason *string) (*events.CancelResult, error)
	Status(ctx context.Context, token string) (*events.View, error)
	CancelRegistration(ctx context.Context, actor events.Actor, token string) (*events.CancelResultPublic, error)
}

// CourtReader supplies the courts a match may be scheduled on.
type CourtReader interface {
	GetByComplex(ctx context.Context, complexID uuid.UUID) ([]*courtstore.Court, error)
}

// Broadcaster pushes a change to the owner's open dashboards.
type Broadcaster interface {
	PublishBookingChanged(complexID uuid.UUID)
}

// Recorder is the audit trail this module writes to.
type Recorder interface {
	Record(e audit.Entry)
}

// Dependencies groups what NewService needs.
type Dependencies struct {
	Store    Store
	Events   Events
	Courts   CourtReader
	Realtime Broadcaster
	Audit    Recorder
	Logger   *slog.Logger
}

// Service holds this module's rules.
type Service struct {
	store    Store
	events   Events
	courts   CourtReader
	realtime Broadcaster
	audit    Recorder
	logger   *slog.Logger
}

// NewService returns a Service backed by the given dependencies.
func NewService(d Dependencies) *Service {
	return &Service{store: d.Store, events: d.Events, courts: d.Courts, realtime: d.Realtime, audit: d.Audit, logger: d.Logger}
}

// Actor is who is acting; it is the events domain's, so a sign-up passes through
// unchanged.
type Actor = events.Actor

// Errors an owner's or a visitor's request can end in.
var (
	// ErrNoActor is returned when an owner route runs without an authenticated user.
	ErrNoActor = events.ErrNoActor
	// ErrSamePlayer is returned when a pair's two players share a phone.
	ErrSamePlayer = errors.New("tournaments: the two players of a pair must have different phones")
)

// ValidationError is one or more fields of a request the rules reject.
type ValidationError = events.ValidationError

// StateError is an operation the current state does not allow (400).
type StateError = events.StateError

// ConflictError is a rule the request breaks about state or hours (409).
type ConflictError = events.ConflictError

func (s *Service) record(actor Actor, complexID uuid.UUID, action, entityType string, entityID uuid.UUID, newVal any) {
	s.audit.Record(audit.Entry{
		UserID: actor.UserID, ComplexID: &complexID, Action: action,
		EntityType: entityType, EntityID: &entityID, NewValue: newVal, IPAddress: actor.IP,
	})
}

// DrawGenerated implements events.TournamentGate.
func (s *Service) DrawGenerated(ctx context.Context, eventID uuid.UUID) (bool, error) {
	return s.store.DrawGenerated(ctx, eventID)
}

// PairWithdrawn implements events.TournamentGate: a pair that leaves after the
// draw forfeits its matches.
func (s *Service) PairWithdrawn(ctx context.Context, registrationID uuid.UUID) error {
	return s.store.WithdrawPair(ctx, registrationID)
}
