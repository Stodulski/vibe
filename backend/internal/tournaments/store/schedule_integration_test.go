//go:build integration

package store_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/data/datatest"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// drawn makes a tournament of four pairs with its draw and returns the first
// round's matches.
func drawn(t *testing.T, f *fixture) (*tournamentstore.Tournament, *tournamentstore.Category, []*tournamentstore.Match) {
	t.Helper()
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	f.pairs(t, cat, 4)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	return tr, cat, matchesOf(f.detail(t, tr), tournamentstore.StageBracket, 1)
}

func (f *fixture) schedule(m *tournamentstore.Match, court uuid.UUID, startTime string) error {
	_, err := f.Stores.Tournaments.Schedule(f.ctx, tournamentstore.ScheduleInput{
		MatchID: m.ID, CourtID: court, Date: start(), StartTime: startTime, Minutes: 90,
	})
	return err
}

// S5: a scheduled match holds its court for its duration everywhere
// availability is decided, like an event does.
func TestAScheduledMatchHoldsItsCourt(t *testing.T) {
	f := newFixture(t)
	tr, _, first := drawn(t, f)
	if err := f.schedule(first[0], f.CourtID, "18:00"); err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	spans, err := f.Stores.Bookings.GetBookedSlotsByCourtIDs(f.ctx, []uuid.UUID{f.CourtID}, start())
	if err != nil || len(spans) != 1 || !spans[0].Event {
		t.Fatalf("the storefront must see the match's hours as held: %+v (%v)", spans, err)
	}
	over := f.NewBooking(datatest.BookingOptions{StartTime: "19:00", EndTime: "20:30"})
	over.Date = start()
	if err := f.Stores.Bookings.InsertSafe(f.ctx, over); !errors.Is(err, bookingstore.ErrSlotUnavailable) {
		t.Fatalf("a booking over the match must be refused: %v", err)
	}
	block := &courtstore.BlockedSlot{CourtID: f.CourtID, Date: start(), StartTime: "17:00", EndTime: "18:30", CreatedBy: &f.UserID}
	if err := f.Stores.Courts.InsertBlockedSlot(f.ctx, block); !errors.Is(err, courtstore.ErrSlotHasBooking) {
		t.Fatalf("blocking the match's hours must be refused: %v", err)
	}
	ev := &eventstore.Event{
		ComplexID: f.ComplexID, Name: "Libres", Sport: "padel", Date: start(), StartTime: "19:00", DurationMinutes: 60,
		Capacity: 10, Price: 100_000, CreatedBy: f.UserID,
	}
	requireConflict(t, f.Stores.Events.CreateEvent(f.ctx, ev, []uuid.UUID{f.CourtID}), eventstore.ConflictMatch)
	if err := f.Stores.Courts.SoftDelete(f.ctx, f.CourtID); !errors.Is(err, courtstore.ErrCourtHasActiveEvent) {
		t.Fatalf("a court with a scheduled match cannot be deleted: %v", err)
	}

	// The owner's calendar reads it as a match, labelled with the tournament.
	occs, err := f.Stores.Bookings.EventOccupancies(f.ctx, &f.ComplexID, nil, start(), start())
	if err != nil || len(occs) != 1 || occs[0].Kind != bookingstore.OccupancyMatch || occs[0].Name != "Torneo · 6ta" || occs[0].ParentID == nil {
		t.Fatalf("calendar read = %+v (%v)", occs, err)
	}
	// ...and names its two pairs by surname, payer first.
	want := map[uuid.UUID]string{}
	for _, p := range f.detail(t, tr).Pairs {
		want[p.ID] = p.Payer.LastName + " / " + p.Partner.LastName
	}
	if occs[0].Pair1 == nil || *occs[0].Pair1 != want[*first[0].Pair1ID] ||
		occs[0].Pair2 == nil || *occs[0].Pair2 != want[*first[0].Pair2ID] {
		t.Errorf("pairs = %v / %v; want %q / %q", occs[0].Pair1, occs[0].Pair2, want[*first[0].Pair1ID], want[*first[0].Pair2ID])
	}

	// Unscheduling releases the hours.
	if _, err := f.Stores.Tournaments.Unschedule(f.ctx, first[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := f.Stores.Bookings.InsertSafe(f.ctx, over); err != nil {
		t.Fatalf("the hours are free again: %v", err)
	}
}

// S5: scheduling refuses conflicts with bookings, blocked slots, fixed
// bookings, events and other matches, and writes nothing.
func TestSchedulingRefusesWhatAlreadyHoldsTheHours(t *testing.T) {
	f := newFixture(t)
	_, _, first := drawn(t, f)

	booking := f.NewBooking(datatest.BookingOptions{StartTime: "09:00", EndTime: "10:30"})
	booking.Date = start()
	if err := f.Stores.Bookings.InsertSafe(f.ctx, booking); err != nil {
		t.Fatal(err)
	}
	requireConflict(t, f.schedule(first[0], f.CourtID, "10:00"), eventstore.ConflictBooking)

	block := &courtstore.BlockedSlot{CourtID: f.CourtID, Date: start(), StartTime: "12:00", EndTime: "13:00", CreatedBy: &f.UserID}
	if err := f.Stores.Courts.InsertBlockedSlot(f.ctx, block); err != nil {
		t.Fatal(err)
	}
	requireConflict(t, f.schedule(first[0], f.CourtID, "11:30"), eventstore.ConflictBlocked)

	series := &bookingstore.Series{
		ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID, Weekday: int(start().Weekday()),
		StartTime: "15:00", DurationMinutes: 90, StartsOn: start(), Status: bookingstore.SeriesStatusActive, CreatedBy: f.UserID,
	}
	if err := f.Stores.BookingSeries.InsertSeries(f.ctx, series, nil); err != nil {
		t.Fatal(err)
	}
	requireConflict(t, f.schedule(first[0], f.CourtID, "14:30"), eventstore.ConflictFixed)

	ev := &eventstore.Event{
		ComplexID: f.ComplexID, Name: "Libres", Sport: "padel", Date: start(), StartTime: "20:00", DurationMinutes: 120,
		Capacity: 10, Price: 100_000, CreatedBy: f.UserID,
	}
	if err := f.Stores.Events.CreateEvent(f.ctx, ev, []uuid.UUID{f.CourtID}); err != nil {
		t.Fatal(err)
	}
	requireConflict(t, f.schedule(first[0], f.CourtID, "19:00"), eventstore.ConflictEvent)

	if err := f.schedule(first[0], f.CourtID, "17:00"); err != nil {
		t.Fatalf("free hours: %v", err)
	}
	requireConflict(t, f.schedule(first[1], f.CourtID, "18:00"), eventstore.ConflictMatch)
	// A match moved onto its own hours does not conflict with itself.
	if err := f.schedule(first[0], f.CourtID, "17:30"); err != nil {
		t.Fatalf("rescheduling over its own hours: %v", err)
	}
}

func TestSchedulingRefusesByesOutsideTheDatesAndUnknownCourts(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	f.pairs(t, cat, 3)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	first := matchesOf(f.detail(t, tr), tournamentstore.StageBracket, 1)

	for _, m := range first {
		if m.Bye {
			requireErr(t, f.schedule(m, f.CourtID, "10:00"), tournamentstore.ErrMatchNotReady)
		}
	}
	_, err := f.Stores.Tournaments.Schedule(f.ctx, tournamentstore.ScheduleInput{
		MatchID: first[1].ID, CourtID: f.CourtID, Date: start().AddDate(0, 0, 5), StartTime: "10:00", Minutes: 90,
	})
	requireErr(t, err, tournamentstore.ErrOutOfDates)
	requireErr(t, f.schedule(first[1], uuid.New(), "10:00"), data.ErrRecordNotFound)
}

// Cancelling a tournament releases every court its matches held.
func TestCancellingTheTournamentReleasesItsCourts(t *testing.T) {
	f := newFixture(t)
	tr, _, first := drawn(t, f)
	if err := f.schedule(first[0], f.CourtID, "18:00"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Stores.Tournaments.Cancel(f.ctx, tr.ID, f.UserID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Stores.Tournaments.Cancel(f.ctx, tr.ID, f.UserID, nil); !errors.Is(err, tournamentstore.ErrNotActive) {
		t.Fatalf("a second cancel: %v", err)
	}
	over := f.NewBooking(datatest.BookingOptions{StartTime: "18:30", EndTime: "20:00"})
	over.Date = start()
	if err := f.Stores.Bookings.InsertSafe(f.ctx, over); err != nil {
		t.Fatalf("the court is free after the cancellation: %v", err)
	}
	if err := f.schedule(first[1], f.CourtID, "10:00"); !errors.Is(err, tournamentstore.ErrCancelled) {
		t.Fatalf("a cancelled tournament schedules nothing: %v", err)
	}
}
