//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/data/datatest"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// eventDay is a date a few days out, as midnight UTC like a parsed request date.
func eventDay() time.Time {
	d := time.Now().AddDate(0, 0, 4)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

func newEvent(f *datatest.Fixture, start string, minutes int) *eventstore.Event {
	return &eventstore.Event{
		ComplexID: f.ComplexID, Name: "Canchas libres", Sport: "padel",
		Date: eventDay(), StartTime: start, DurationMinutes: minutes,
		Capacity: 20, Price: 800_000, CreatedBy: f.UserID,
	}
}

func createEvent(t *testing.T, f *datatest.Fixture, start string, minutes int, courts ...uuid.UUID) *eventstore.Event {
	t.Helper()
	e := newEvent(f, start, minutes)
	if len(courts) == 0 {
		courts = []uuid.UUID{f.CourtID}
	}
	if err := f.Stores.Events.CreateEvent(f.Scoped(context.Background()), e, courts); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	return e
}

func secondCourt(t *testing.T, f *datatest.Fixture) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.DB.QueryRow(context.Background(),
		`INSERT INTO courts (complex_id, name) VALUES ($1, 'Court 2') RETURNING id`, f.ComplexID).Scan(&id); err != nil {
		t.Fatalf("creating the second court: %v", err)
	}
	return id
}

func requireConflict(t *testing.T, err error, reason string) {
	t.Helper()
	var conflict *eventstore.ConflictError
	if !errors.As(err, &conflict) || conflict.Reason != reason {
		t.Fatalf("want a %q conflict; got %v", reason, err)
	}
}

// S1: creating an event checks what already holds the hours on each of its
// courts, and writes nothing when anything does.
func TestCreateEventRefusesWhatAlreadyHoldsTheHours(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	day := eventDay()

	booking := f.NewBooking(datatest.BookingOptions{StartTime: "18:00", EndTime: "19:30"})
	booking.Date = day
	if err := f.Stores.Bookings.InsertSafe(ctx, booking); err != nil {
		t.Fatal(err)
	}
	requireConflict(t, f.Stores.Events.CreateEvent(ctx, newEvent(f, "19:00", 120), []uuid.UUID{f.CourtID}), eventstore.ConflictBooking)

	block := &courtstore.BlockedSlot{CourtID: f.CourtID, Date: day, StartTime: "21:00", EndTime: "22:00", CreatedBy: &f.UserID}
	if err := f.Stores.Courts.InsertBlockedSlot(ctx, block); err != nil {
		t.Fatal(err)
	}
	requireConflict(t, f.Stores.Events.CreateEvent(ctx, newEvent(f, "20:30", 60), []uuid.UUID{f.CourtID}), eventstore.ConflictBlocked)

	series := &bookingstore.Series{
		ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID,
		Weekday: int(day.Weekday()), StartTime: "10:00", DurationMinutes: 90,
		StartsOn: day, Status: bookingstore.SeriesStatusActive, CreatedBy: f.UserID,
	}
	if err := f.Stores.BookingSeries.InsertSeries(ctx, series, nil); err != nil {
		t.Fatal(err)
	}
	requireConflict(t, f.Stores.Events.CreateEvent(ctx, newEvent(f, "10:30", 60), []uuid.UUID{f.CourtID}), eventstore.ConflictFixed)

	createEvent(t, f, "12:00", 120)
	requireConflict(t, f.Stores.Events.CreateEvent(ctx, newEvent(f, "13:00", 120), []uuid.UUID{f.CourtID}), eventstore.ConflictEvent)

	// Nothing was written by any refused attempt: only the one event exists.
	var n int
	if err := f.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM open_play_events WHERE complex_id = $1`, f.ComplexID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("refused events must leave no row; %d (%v)", n, err)
	}
}

// Hours that end exactly when the event starts, or a booking that was
// cancelled, hold nothing; and one clash on any court refuses the whole event.
func TestCreateEventAcceptsFreeHoursAndRefusesOnAnyCourt(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	second := secondCourt(t, f)

	cancelled := f.NewBooking(datatest.BookingOptions{StartTime: "18:00", EndTime: "19:30", Status: "cancelled"})
	cancelled.Date = eventDay()
	if err := f.Stores.Bookings.InsertSafe(ctx, cancelled); err != nil {
		t.Fatal(err)
	}
	adjacent := f.NewBooking(datatest.BookingOptions{StartTime: "16:30", EndTime: "18:00"})
	adjacent.Date = eventDay()
	if err := f.Stores.Bookings.InsertSafe(ctx, adjacent); err != nil {
		t.Fatal(err)
	}
	e := createEvent(t, f, "18:00", 120, f.CourtID, second)
	if len(e.Courts) != 0 && len(e.Courts) != 2 {
		t.Fatalf("unexpected courts %+v", e.Courts)
	}

	// A single clash on the second court only refuses a second event that asks for both.
	third := newEvent(f, "19:00", 60)
	requireConflict(t, f.Stores.Events.CreateEvent(ctx, third, []uuid.UUID{second}), eventstore.ConflictEvent)
}

// S2: an event blocks its courts for its window everywhere availability is
// decided, and a cancelled event blocks nothing.
func TestAnEventHoldsItsCourtsForItsWindow(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	e := createEvent(t, f, "18:00", 120)

	spans, err := f.Stores.Bookings.GetBookedSlotsByCourtIDs(ctx, []uuid.UUID{f.CourtID}, e.Date)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 1 || !spans[0].Event || !spans[0].StartsAt.Equal(slots.At(timezone.Day(e.Date), "18:00")) ||
		!spans[0].EndsAt.Equal(slots.At(timezone.Day(e.Date), "20:00")) {
		t.Fatalf("the storefront's read must show the event's 18:00-20:00 as held; got %+v", spans)
	}

	over := f.NewBooking(datatest.BookingOptions{StartTime: "19:00", EndTime: "20:30"})
	over.Date = e.Date
	if err := f.Stores.Bookings.InsertSafe(ctx, over); !errors.Is(err, bookingstore.ErrSlotUnavailable) {
		t.Fatalf("a booking over the event must be refused; got %v", err)
	}
	next := f.NewBooking(datatest.BookingOptions{StartTime: "20:00", EndTime: "21:30"})
	next.Date = e.Date
	if err := f.Stores.Bookings.InsertSafe(ctx, next); err != nil {
		t.Fatalf("hours right after the event are free: %v", err)
	}

	block := &courtstore.BlockedSlot{CourtID: f.CourtID, Date: e.Date, StartTime: "17:00", EndTime: "18:30", CreatedBy: &f.UserID}
	if err := f.Stores.Courts.InsertBlockedSlot(ctx, block); !errors.Is(err, courtstore.ErrSlotHasBooking) {
		t.Fatalf("blocking hours an event holds must be refused; got %v", err)
	}

	occs, err := f.Stores.Bookings.EventOccupancies(ctx, &f.ComplexID, nil, e.Date, e.Date)
	if err != nil || len(occs) != 1 || occs[0].Name != "Canchas libres" || occs[0].EventID != e.ID {
		t.Fatalf("the owner's calendar read = %+v (%v)", occs, err)
	}

	if _, _, err := f.Stores.Events.CancelEvent(ctx, e.ID, f.UserID, nil); err != nil {
		t.Fatal(err)
	}
	if got, err := f.Stores.Bookings.GetBookedSlotsByCourtIDs(ctx, []uuid.UUID{f.CourtID}, e.Date); err != nil || len(got) != 1 {
		t.Fatalf("a cancelled event holds nothing (only the 20:00 booking remains); got %+v (%v)", got, err)
	}
	if err := f.Stores.Courts.InsertBlockedSlot(ctx, block); err != nil {
		t.Fatalf("with the event cancelled the hours are blockable: %v", err)
	}
}

// An event that runs past midnight holds the following morning too.
func TestAnOvernightEventHoldsTheNextMorning(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	e := createEvent(t, f, "23:00", 120)

	got, err := f.Stores.Bookings.GetBookedSlotsByCourtIDs(ctx, []uuid.UUID{f.CourtID}, e.Date.AddDate(0, 0, 1))
	if err != nil || len(got) != 1 || !got[0].Event {
		t.Fatalf("the morning after an event at 23:00 is held until 01:00: %+v (%v)", got, err)
	}
}

// A court cannot be deleted while a live event still holds it.
func TestACourtWithALiveEventCannotBeDeleted(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	e := createEvent(t, f, "18:00", 120)

	if err := f.Stores.Courts.SoftDelete(ctx, f.CourtID); !errors.Is(err, courtstore.ErrCourtHasActiveEvent) {
		t.Fatalf("want ErrCourtHasActiveEvent; got %v", err)
	}
	if _, _, err := f.Stores.Events.CancelEvent(ctx, e.ID, f.UserID, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.Stores.Courts.SoftDelete(ctx, f.CourtID); err != nil {
		t.Fatalf("with the event cancelled the court can go: %v", err)
	}
}

// Events are tenant data: another complex's scope neither reads an event nor
// cancels it, and cannot sign people up under it.
func TestEventsAreScopedToTheirComplex(t *testing.T) {
	f := datatest.Isolated(t)
	e := createEvent(t, f, "18:00", 120)
	foreign := data.ContextWithTenant(context.Background(), uuid.New())

	if _, err := f.Stores.Events.GetEvent(foreign, e.ID); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("another tenant must not read the event; got %v", err)
	}
	if _, _, err := f.Stores.Events.CancelEvent(foreign, e.ID, f.UserID, nil); err == nil {
		t.Error("another tenant must not cancel the event")
	}
	if _, err := signUpAs(foreign, f, e); err == nil {
		t.Error("another tenant must not sign people up under the event")
	}
	if got, err := f.Stores.Events.GetEvent(f.Scoped(context.Background()), e.ID); err != nil || got.Status != eventstore.EventActive {
		t.Errorf("the event must be untouched; got %+v (%v)", got, err)
	}
}

func signUpAs(ctx context.Context, f *datatest.Fixture, e *eventstore.Event) (*eventstore.Registration, error) {
	reg, _, err := f.Stores.Events.SignUp(ctx, eventstore.SignUpInput{
		EventID: e.ID, FirstName: "A", LastName: "B", Phone: "+5491100000042", IP: "203.0.113.50", MaxHoldsPerIP: 10,
	})
	return reg, err
}
