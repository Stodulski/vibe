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
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// seriesStart is a date a few days out; its weekday is the series'.
func seriesStart() time.Time {
	d := time.Now().AddDate(0, 0, 3)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

func fixedSpansOn(t *testing.T, f *datatest.Fixture, day time.Time) []bookingstore.BookedSpan {
	t.Helper()
	spans, err := f.Stores.Bookings.GetBookedSlotsByCourtIDs(f.Scoped(context.Background()), []uuid.UUID{f.CourtID}, day)
	if err != nil {
		t.Fatalf("GetBookedSlotsByCourtIDs: %v", err)
	}
	var fixed []bookingstore.BookedSpan
	for _, sp := range spans {
		if sp.Fixed {
			fixed = append(fixed, sp)
		}
	}
	return fixed
}

// The rule holds its date for as far ahead as anyone looks: months out, with
// no booking row anywhere, the storefront's read of the day shows the hours
// taken — and not on an excepted date, an unrelated weekday, or after ends_on.
func TestASeriesHoldsItsDatesMonthsAheadUntilExceptedOrEnded(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	start := seriesStart()
	series := newSeries(t, f, start)

	for _, weeks := range []int{0, 1, 20, 51} {
		day := start.AddDate(0, 0, 7*weeks)
		fixed := fixedSpansOn(t, f, day)
		if len(fixed) != 1 || !fixed[0].StartsAt.Equal(slots.At(timezone.Day(day), "18:00")) ||
			!fixed[0].EndsAt.Equal(slots.At(timezone.Day(day), "19:30")) {
			t.Fatalf("week %d: want the series' 18:00-19:30 held, got %+v", weeks, fixed)
		}
	}
	if got := fixedSpansOn(t, f, start.AddDate(0, 0, 1)); len(got) != 0 {
		t.Errorf("another weekday must not be held: %+v", got)
	}
	if got := fixedSpansOn(t, f, start.AddDate(0, 0, -7)); len(got) != 0 {
		t.Errorf("never before starts_on: %+v", got)
	}

	excepted := start.AddDate(0, 0, 20*7)
	if _, err := f.Stores.BookingSeries.RecordSeriesException(ctx, series, excepted, bookingstore.ReasonCancelledByOwner); err != nil {
		t.Fatal(err)
	}
	if got := fixedSpansOn(t, f, excepted); len(got) != 0 {
		t.Errorf("an excepted date is free: %+v", got)
	}

	last := start.AddDate(0, 0, 21*7)
	if _, err := f.Stores.BookingSeries.EndSeries(ctx, series.ID, &last, bookingstore.SeriesStatusEnded); err != nil {
		t.Fatal(err)
	}
	if got := fixedSpansOn(t, f, start.AddDate(0, 0, 22*7)); len(got) != 0 {
		t.Errorf("an ended series holds nothing after ends_on: %+v", got)
	}
}

// An occurrence that runs past midnight holds the following morning too.
func TestAnOvernightOccurrenceHoldsTheNextMorning(t *testing.T) {
	f := datatest.Isolated(t)
	start := seriesStart()
	s := &bookingstore.Series{
		ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID,
		Weekday: int(start.Weekday()), StartTime: "23:00", DurationMinutes: 120,
		StartsOn: start, Status: bookingstore.SeriesStatusActive, CreatedBy: f.UserID,
	}
	if err := f.Stores.BookingSeries.InsertSeries(f.Scoped(context.Background()), s, nil); err != nil {
		t.Fatal(err)
	}
	if got := fixedSpansOn(t, f, start.AddDate(0, 0, 8)); len(got) != 1 {
		t.Errorf("the morning after an occurrence at 23:00 is held until 01:00: %+v", got)
	}
}

// The insert-time guard sees the rule: a booking on a held date is refused with
// the same sentinel as for any taken hour, a booking on an excepted date or on a
// neighbouring hour is not, and the series' own occurrence is not a clash with
// itself.
func TestTheInsertGuardRefusesBookingsOnHeldDates(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	start := seriesStart()
	series := newSeries(t, f, start)

	onHeld := f.NewBooking(datatest.BookingOptions{StartTime: "18:30", EndTime: "20:00"})
	onHeld.Date = start.AddDate(0, 0, 14)
	if err := f.Stores.Bookings.InsertSafe(ctx, onHeld); !errors.Is(err, bookingstore.ErrSlotUnavailable) {
		t.Fatalf("a booking over a held date must be refused; got %v", err)
	}

	adjacent := f.NewBooking(datatest.BookingOptions{StartTime: "19:30", EndTime: "21:00"})
	adjacent.Date = start.AddDate(0, 0, 14)
	if err := f.Stores.Bookings.InsertSafe(ctx, adjacent); err != nil {
		t.Fatalf("hours right after the series' are free: %v", err)
	}

	if _, err := f.Stores.BookingSeries.RecordSeriesException(ctx, series, start.AddDate(0, 0, 7), bookingstore.ReasonCancelledByOwner); err != nil {
		t.Fatal(err)
	}
	onExcepted := f.NewBooking(datatest.BookingOptions{StartTime: "18:00", EndTime: "19:30"})
	onExcepted.Date = start.AddDate(0, 0, 7)
	if err := f.Stores.Bookings.InsertSafe(ctx, onExcepted); err != nil {
		t.Fatalf("an excepted date is free: %v", err)
	}

	own := occurrence(f, series, "18:00", "19:30")
	own.Date = start
	if err := f.Stores.Bookings.InsertSafe(ctx, own); err != nil {
		t.Fatalf("the series' own occurrence must be accepted on its own date: %v", err)
	}
}

// The owner's blocked-slot write asks the same question under its own lock.
func TestABlockedSlotOverAHeldDateIsRefused(t *testing.T) {
	f := datatest.Isolated(t)
	start := seriesStart()
	newSeries(t, f, start)
	day := start.AddDate(0, 0, 28)

	blocked := &courtstore.BlockedSlot{CourtID: f.CourtID, Date: day, StartTime: "18:30", EndTime: "19:00", CreatedBy: &f.UserID}
	if err := f.Stores.Courts.InsertBlockedSlot(f.Scoped(context.Background()), blocked); !errors.Is(err, courtstore.ErrSlotHasBooking) {
		t.Fatalf("blocking hours a series holds must be refused; got %v", err)
	}
	free := &courtstore.BlockedSlot{CourtID: f.CourtID, Date: day, StartTime: "10:00", EndTime: "11:00", CreatedBy: &f.UserID}
	if err := f.Stores.Courts.InsertBlockedSlot(f.Scoped(context.Background()), free); err != nil {
		t.Fatalf("other hours stay blockable: %v", err)
	}
}

// Creating a series refuses, writing nothing, when a booking was made on a held
// date after the service read; the recheck runs under the court lock.
func TestInsertSeriesRechecksBookingsInItsTransaction(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	start := seriesStart()

	clash := f.NewBooking(datatest.BookingOptions{StartTime: "18:00", EndTime: "19:30"})
	clash.Date = start.AddDate(0, 0, 70)
	if err := f.Stores.Bookings.InsertSafe(ctx, clash); err != nil {
		t.Fatal(err)
	}
	mk := func() *bookingstore.Series {
		return &bookingstore.Series{
			ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID,
			Weekday: int(start.Weekday()), StartTime: "18:00", DurationMinutes: 90,
			StartsOn: start, Status: bookingstore.SeriesStatusActive, CreatedBy: f.UserID,
		}
	}
	if err := f.Stores.BookingSeries.InsertSeries(ctx, mk(), nil); !errors.Is(err, bookingstore.ErrSlotUnavailable) {
		t.Fatalf("want ErrSlotUnavailable; got %v", err)
	}
	var n int
	if err := f.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM booking_series WHERE complex_id = $1`, f.ComplexID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("a refused series must leave no row; %d (%v)", n, err)
	}
	// With the clashing date excepted from the start, the same series is accepted.
	ok := mk()
	if err := f.Stores.BookingSeries.InsertSeries(ctx, ok, []bookingstore.SeriesException{{Date: clash.Date, Reason: bookingstore.ReasonOverlap}}); err != nil {
		t.Fatalf("with the clash excepted the series is created: %v", err)
	}
}

// The owner's calendar read, and tenant isolation of it.
func TestSeriesOccurrencesAreScopedToTheTenantAndTheRange(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	start := seriesStart()
	series := newSeries(t, f, start)

	occs, err := f.Stores.Bookings.SeriesOccurrences(ctx, &f.ComplexID, nil, start, start.AddDate(0, 0, 14))
	if err != nil || len(occs) != 3 {
		t.Fatalf("want 3 dates in a 15-day window, got %d (%v)", len(occs), err)
	}
	if occs[0].SeriesID != series.ID || occs[0].ClientName == "" || !occs[0].Date.Equal(start) {
		t.Errorf("occurrence = %+v", occs[0])
	}

	foreign := data.ContextWithTenant(context.Background(), uuid.New())
	other := uuid.New()
	got, err := f.Stores.Bookings.SeriesOccurrences(foreign, &other, nil, start, start.AddDate(0, 0, 14))
	if err != nil || len(got) != 0 {
		t.Errorf("another tenant must see none of them; got %d (%v)", len(got), err)
	}
}
