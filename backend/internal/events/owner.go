package events

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// Limits of an event, mirrored by the schema's CHECK constraints.
const (
	MinDurationMinutes = 30
	MaxDurationMinutes = 720
	MaxCapacity        = 500
	// MaxPrice bounds a price per person in centavos: far above any real event
	// and far below int32, so a typo cannot reach MercadoPago.
	MaxPrice = 100_000_000
	// HorizonDays is how far ahead an event may be published, the booking horizon.
	HorizonDays = 365
	// MaxNameLength is the longest event name.
	MaxNameLength = 100
)

var sports = []string{"padel", "tennis", "soccer", "basketball", "volleyball", "hockey", "pickleball"}

// CreateInput is an owner publishing an event.
type CreateInput struct {
	Name  string
	Sport string
	// Date is the day the event starts on, midnight UTC like a parsed request date.
	Date      time.Time
	StartTime string
	// EndTime is when the event ends; earlier than or equal to StartTime means the
	// event runs past midnight.
	EndTime string
	// CourtIDs are the courts the event holds; empty means every active court.
	CourtIDs []uuid.UUID
	Capacity int
	// Price is centavos per person.
	Price int
}

// durationOf is the minutes from start to end on the wall-clock, past midnight
// when end is not after start.
func durationOf(start, end string) int {
	s, _ := time.Parse("15:04", start) //nolint:errcheck // both validated by slots.ValidFormat before
	e, _ := time.Parse("15:04", end)   //nolint:errcheck // see above
	if !e.After(s) {
		e = e.Add(24 * time.Hour)
	}
	return int(e.Sub(s) / time.Minute)
}

// validate checks the shape of an event, field by field.
func (in *CreateInput) validate(now time.Time) map[string]string {
	errs := map[string]string{}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > MaxNameLength {
		errs["name"] = fmt.Sprintf("must be between 1 and %d characters", MaxNameLength)
	}
	if in.Sport == "" {
		in.Sport = "padel"
	}
	if !slices.Contains(sports, in.Sport) {
		errs["sport"] = "must be a supported sport"
	}
	if in.Capacity < 1 || in.Capacity > MaxCapacity {
		errs["capacity"] = fmt.Sprintf("must be between 1 and %d", MaxCapacity)
	}
	if in.Price <= 0 || in.Price > MaxPrice {
		errs["price"] = "must be a positive amount in centavos"
	}
	if !slots.ValidFormat(in.StartTime) {
		errs["start_time"] = "must be in HH:MM format (00:00-23:59)"
	}
	if !slots.ValidFormat(in.EndTime) {
		errs["end_time"] = "must be in HH:MM format (00:00-23:59)"
	}
	if len(errs) > 0 && (errs["start_time"] != "" || errs["end_time"] != "") {
		return errs
	}
	if d := durationOf(in.StartTime, in.EndTime); d < MinDurationMinutes || d > MaxDurationMinutes {
		errs["end_time"] = fmt.Sprintf("the event must last between %d minutes and %d hours", MinDurationMinutes, MaxDurationMinutes/60)
	}
	today := timezone.Day(now)
	date := timezone.Day(in.Date)
	switch {
	case date.Before(today):
		errs["date"] = "must not be in the past"
	case date.After(today.AddDate(0, 0, HorizonDays)):
		errs["date"] = fmt.Sprintf("must be within %d days", HorizonDays)
	case !slots.At(date, in.StartTime).After(now):
		errs["start_time"] = "must be in the future"
	}
	return errs
}

// courtsFor resolves the courts an event holds: the ones asked for, which must
// be this complex's active courts, or every active court when none is named.
func (s *Service) courtsFor(ctx context.Context, complexID uuid.UUID, asked []uuid.UUID) (ids []uuid.UUID, names map[uuid.UUID]string, err error) {
	all, err := s.courts.GetByComplex(ctx, complexID)
	if err != nil {
		return nil, nil, err
	}
	names = map[uuid.UUID]string{}
	var active []*courtstore.Court
	for _, c := range all {
		if c.IsActive {
			active = append(active, c)
			names[c.ID] = c.Name
		}
	}
	if len(active) == 0 {
		return nil, nil, &ValidationError{Errors: map[string]string{"court_ids": "the complex has no active courts"}}
	}
	if len(asked) == 0 {
		for _, c := range active {
			ids = append(ids, c.ID)
		}
		return ids, names, nil
	}
	for _, id := range asked {
		if _, ok := names[id]; !ok {
			return nil, nil, &ValidationError{Errors: map[string]string{"court_ids": "every court must be an active court of this complex"}}
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids, names, nil
}

// Create publishes an event: it validates the shape, resolves the courts, and
// refuses with a ConflictError naming what already holds the hours when anything
// does. Nothing is written in that case.
func (s *Service) Create(ctx context.Context, complex *complexstore.Complex, actor Actor, in CreateInput) (*eventstore.Event, error) {
	if actor.UserID == nil {
		return nil, ErrNoActor
	}
	if errs := in.validate(time.Now()); len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	courtIDs, names, err := s.courtsFor(ctx, complex.ID, in.CourtIDs)
	if err != nil {
		return nil, err
	}

	ev := &eventstore.Event{
		ComplexID: complex.ID, Name: in.Name, Sport: in.Sport, Date: timezone.Day(in.Date),
		StartTime: in.StartTime, DurationMinutes: durationOf(in.StartTime, in.EndTime),
		Capacity: in.Capacity, Price: in.Price, CreatedBy: *actor.UserID,
	}
	if err := s.store.CreateEvent(ctx, ev, courtIDs); err != nil {
		var held *eventstore.ConflictError
		if errors.As(err, &held) {
			return nil, &ConflictError{Message: conflictMessage(held, names)}
		}
		return nil, err
	}
	created, err := s.store.GetEvent(ctx, ev.ID)
	if err != nil {
		return nil, err
	}
	s.realtime.PublishBookingChanged(complex.ID)
	s.record(actor, complex.ID, "create", entityEvent, created.ID, created)
	return created, nil
}

const entityEvent = "open_play_event"

// conflictMessage names what holds the event's hours, and where.
func conflictMessage(c *eventstore.ConflictError, names map[uuid.UUID]string) string {
	court := names[c.CourtID]
	switch c.Reason {
	case eventstore.ConflictBooking:
		return fmt.Sprintf("a booking already holds those hours on %s", court)
	case eventstore.ConflictBlocked:
		return fmt.Sprintf("those hours are blocked on %s", court)
	case eventstore.ConflictFixed:
		return fmt.Sprintf("a fixed weekly booking (turno fijo) holds those hours on %s", court)
	default:
		return fmt.Sprintf("another event already holds those hours on %s", court)
	}
}

// List returns the events of the complex whose date is in [from, to], cancelled
// ones included, soonest first.
func (s *Service) List(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]*eventstore.Event, error) {
	return s.store.ListEvents(ctx, complexID, from, to, true)
}

// Detail is an event with its participants.
type Detail struct {
	Event        *eventstore.Event
	Participants []*eventstore.RegistrationRow
}

// Get returns one event of the complex with every registration that held a spot
// or carries money. An event of another complex is not found.
func (s *Service) Get(ctx context.Context, complexID, eventID uuid.UUID) (*Detail, error) {
	ev, err := s.ownedEvent(ctx, complexID, eventID)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListRegistrations(ctx, ev.ID, true)
	if err != nil {
		return nil, err
	}
	return &Detail{Event: ev, Participants: rows}, nil
}

func (s *Service) ownedEvent(ctx context.Context, complexID, eventID uuid.UUID) (*eventstore.Event, error) {
	ev, err := s.store.GetEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if ev.ComplexID != complexID {
		return nil, data.ErrRecordNotFound
	}
	return ev, nil
}

// Refund is what became of one paid participant's money when an event was
// cancelled.
type Refund struct {
	RegistrationID uuid.UUID
	Outcome        paymentstore.RefundOutcome
}

// CancelResult is a cancelled event and the refund of each paid participant.
type CancelResult struct {
	Event   *eventstore.Event
	Refunds []Refund
}

// Cancel cancels an event: its courts go back on sale and every registration
// ends at once, then each paid participant is refunded through MercadoPago and
// told. Participants still paying lose their hold; a payment that lands for one
// is refunded when it arrives (see payments.processRegistrationPayment).
//
// A refund that cannot finish now stays queued for the retry job, and a refund
// the request never got to start (a crash after the cancellation committed) is
// found by SweepUnrefunded.
func (s *Service) Cancel(ctx context.Context, complex *complexstore.Complex, actor Actor, eventID uuid.UUID, reason *string) (*CancelResult, error) {
	if actor.UserID == nil {
		return nil, ErrNoActor
	}
	if _, err := s.ownedEvent(ctx, complex.ID, eventID); err != nil {
		return nil, err
	}
	cancelled, owed, err := s.store.CancelEvent(ctx, eventID, *actor.UserID, reason)
	if err != nil {
		if errors.Is(err, eventstore.ErrEventNotActive) {
			return nil, &StateError{Message: "the event is already cancelled"}
		}
		return nil, err
	}
	s.realtime.PublishBookingChanged(complex.ID)
	s.record(actor, complex.ID, "cancel", entityEvent, cancelled.ID, cancelled)

	// The refunds run on their own context: the owner closing the tab must not
	// leave half the participants unrefunded.
	rctx := context.WithoutCancel(ctx)
	result := &CancelResult{Event: cancelled}
	for _, regID := range owed {
		result.Refunds = append(result.Refunds, s.refundCancelledRegistration(rctx, complex, cancelled, regID))
	}
	return result, nil
}

// refundCancelledRegistration refunds one paid participant of a cancelled event
// and sends them the notice.
func (s *Service) refundCancelledRegistration(ctx context.Context, complex *complexstore.Complex, ev *eventstore.Event, regID uuid.UUID) Refund {
	outcome := s.payments.RefundRegistration(ctx, regID)
	reg, err := s.store.GetRegistration(ctx, regID)
	if err != nil {
		s.logger.Error("event cancel: cannot read the registration to notify it", "error", err, "registration_id", regID)
		return Refund{RegistrationID: regID, Outcome: outcome}
	}
	s.notifyCancelled(reg, ev, complex, true, outcome)
	return Refund{RegistrationID: regID, Outcome: outcome}
}
