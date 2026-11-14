package events

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

func validInput() CreateInput {
	return CreateInput{
		Name: "Canchas libres", Date: timezone.Day(time.Now()).AddDate(0, 0, 3),
		StartTime: "19:00", EndTime: "21:00", Capacity: 20, Price: 800_000,
	}
}

// S1: the shape of an event is checked field by field.
func TestCreateRejectsAnInvalidEventFieldByField(t *testing.T) {
	cases := map[string]func(*CreateInput){
		"name":       func(in *CreateInput) { in.Name = "  " },
		"sport":      func(in *CreateInput) { in.Sport = "curling" },
		"capacity":   func(in *CreateInput) { in.Capacity = 0 },
		"price":      func(in *CreateInput) { in.Price = 0 },
		"date":       func(in *CreateInput) { in.Date = timezone.Day(time.Now()).AddDate(0, 0, -2) },
		"start_time": func(in *CreateInput) { in.StartTime = "25:00" },
		"end_time":   func(in *CreateInput) { in.EndTime = "19:10" },
	}
	for field, mutate := range cases {
		t.Run(field, func(t *testing.T) {
			f := newFixture(t)
			in := validInput()
			mutate(&in)
			_, err := f.svc.Create(t.Context(), f.complex, f.ownerActor(), in)
			var invalid *ValidationError
			if !errors.As(err, &invalid) || invalid.Errors[field] == "" {
				t.Fatalf("want a validation error on %q; got %v", field, err)
			}
			if f.store.created != nil {
				t.Error("an invalid event must write nothing")
			}
		})
	}
}

// The event holds every active court when none is named, and only courts of the
// complex when some are.
func TestCreateHoldsEveryActiveCourtByDefaultAndOnlyTheComplexOwnWhenNamed(t *testing.T) {
	f := newFixture(t)
	ev, err := f.svc.Create(t.Context(), f.complex, f.ownerActor(), validInput())
	if err != nil || len(f.store.createdCrts) != 2 {
		t.Fatalf("courts = %d (%v); want both active courts and not the inactive one", len(f.store.createdCrts), err)
	}
	if ev.DurationMinutes != 120 {
		t.Errorf("duration = %d", ev.DurationMinutes)
	}

	f = newFixture(t)
	in := validInput()
	in.CourtIDs = []uuid.UUID{f.courts.courts[0].ID}
	if _, err := f.svc.Create(t.Context(), f.complex, f.ownerActor(), in); err != nil || len(f.store.createdCrts) != 1 {
		t.Fatalf("a subset must be honoured; got %d (%v)", len(f.store.createdCrts), err)
	}

	for _, foreign := range []uuid.UUID{uuid.New(), f.courts.courts[2].ID} {
		in.CourtIDs = []uuid.UUID{foreign}
		var invalid *ValidationError
		if _, err := f.svc.Create(t.Context(), f.complex, f.ownerActor(), in); !errors.As(err, &invalid) || invalid.Errors["court_ids"] == "" {
			t.Fatalf("a court that is not an active court of the complex must be refused; got %v", err)
		}
	}
}

func TestAnEventMayRunPastMidnight(t *testing.T) {
	f := newFixture(t)
	in := validInput()
	in.StartTime, in.EndTime = "23:00", "01:00"
	ev, err := f.svc.Create(t.Context(), f.complex, f.ownerActor(), in)
	if err != nil || ev.DurationMinutes != 120 {
		t.Fatalf("duration = %v (%v); want two hours across midnight", ev, err)
	}
}

// S1: when something already holds the hours the owner is told what and where.
func TestCreateAnswersWhatHoldsTheHoursAndWhere(t *testing.T) {
	for reason, want := range map[string]string{
		eventstore.ConflictBooking: "a booking already holds those hours on Cancha 1",
		eventstore.ConflictBlocked: "those hours are blocked on Cancha 1",
		eventstore.ConflictFixed:   "a fixed weekly booking (turno fijo) holds those hours on Cancha 1",
		eventstore.ConflictEvent:   "another event already holds those hours on Cancha 1",
	} {
		t.Run(reason, func(t *testing.T) {
			f := newFixture(t)
			f.store.createErr = &eventstore.ConflictError{Reason: reason, CourtID: f.courts.courts[0].ID}
			_, err := f.svc.Create(t.Context(), f.complex, f.ownerActor(), validInput())
			var conflict *ConflictError
			if !errors.As(err, &conflict) || conflict.Message != want {
				t.Fatalf("err = %v; want conflict %q", err, want)
			}
		})
	}
}

func TestCreateNeedsAnAuthenticatedOwner(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Create(t.Context(), f.complex, Actor{}, validInput()); !errors.Is(err, ErrNoActor) {
		t.Fatalf("want ErrNoActor; got %v", err)
	}
}

func TestGetOfAnotherComplexsEventIsNotFound(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Get(t.Context(), uuid.New(), f.store.event.ID); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("want not found; got %v", err)
	}
}

// S5: cancelling refunds every paid participant through MercadoPago and tells
// each of them; the courts and the spots are released in the store's one
// transaction.
func TestOwnerCancelRefundsEveryPaidParticipantAndNotifiesThem(t *testing.T) {
	f := newFixture(t)
	email := "ana@example.com"
	var paid []uuid.UUID
	for _, e := range []*string{&email, nil} {
		reg := &eventstore.Registration{
			ID: uuid.New(), EventID: f.store.event.ID, ComplexID: f.complex.ID, FirstName: "Ana", Phone: "+5491100000000",
			Email: e, Status: eventstore.RegistrationConfirmed, Price: 800_000, ServiceFee: 20_000,
		}
		f.store.remember(reg, reg.ID.String())
		paid = append(paid, reg.ID)
	}
	f.store.paid = paid
	f.payments.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 820_000}

	result, err := f.svc.Cancel(t.Context(), f.complex, f.ownerActor(), f.store.event.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.store.cancelled) != 1 || result.Event.Status != eventstore.EventCancelled {
		t.Errorf("the event must be cancelled once; got %v", f.store.cancelled)
	}
	if len(f.payments.refunded) != 2 || f.payments.refunded[0] != paid[0] || f.payments.refunded[1] != paid[1] {
		t.Errorf("every paid participant is refunded; got %v", f.payments.refunded)
	}
	if len(f.notify.cancelled) != 2 || !f.notify.cancelled[0].ByOwner || f.notify.cancelled[0].Email != email ||
		f.notify.cancelled[0].RefundAmount != "$8.200" || f.notify.cancelled[1].Email != "" {
		t.Errorf("notices = %+v", f.notify.cancelled)
	}
	if len(result.Refunds) != 2 || result.Refunds[0].Outcome.Result != paymentstore.RefundIssued {
		t.Errorf("refunds = %+v", result.Refunds)
	}
	if f.realtime.published == 0 || len(f.audit.entries) != 1 || f.audit.entries[0].Action != "cancel" {
		t.Errorf("the dashboard must hear of it and the trail record it; published=%d entries=%d", f.realtime.published, len(f.audit.entries))
	}
}

func TestOwnerCancelOfAnEventWithNobodyPaidRefundsNothing(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Cancel(t.Context(), f.complex, f.ownerActor(), f.store.event.ID, nil); err != nil {
		t.Fatal(err)
	}
	if len(f.payments.refunded) != 0 || len(f.notify.cancelled) != 0 {
		t.Error("nobody paid, so nobody is refunded or told")
	}
}

func TestAnEventCannotBeCancelledTwice(t *testing.T) {
	f := newFixture(t)
	f.store.cancelErr = eventstore.ErrEventNotActive
	_, err := f.svc.Cancel(t.Context(), f.complex, f.ownerActor(), f.store.event.ID, nil)
	var state *StateError
	if !errors.As(err, &state) {
		t.Fatalf("want a state error; got %v", err)
	}
	if len(f.payments.refunded) != 0 {
		t.Error("a refused cancel must refund nothing")
	}
}
