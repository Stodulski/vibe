//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

func secondCourt(t *testing.T, f *datatest.Fixture) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.DB.QueryRow(context.Background(),
		`INSERT INTO courts (complex_id, name) VALUES ($1, 'Court 2') RETURNING id`, f.ComplexID).Scan(&id); err != nil {
		t.Fatalf("creating the second court: %v", err)
	}
	return id
}

// A date the owner moved to another court is booked there as part of the
// series' own transaction: linked to the series, recorded in the ledger as
// booked, and no longer held on the series' court even though a booking sits
// there that day. No migration is involved: bookings.series_id only names a
// series of the same complex.
func TestAMovedDateIsBookedOnAnotherCourtAndFreesTheSeriesCourt(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	start := seriesStart()
	other := secondCourt(t, f)
	day := start.AddDate(0, 0, 14)

	clash := f.NewBooking(datatest.BookingOptions{StartTime: "18:00", EndTime: "19:30"})
	clash.Date = day
	if err := f.Stores.Bookings.InsertSafe(ctx, clash); err != nil {
		t.Fatal(err)
	}
	moved := f.NewBooking(datatest.BookingOptions{StartTime: "18:00", EndTime: "19:30"})
	moved.CourtID, moved.Date = other, day
	series := &bookingstore.Series{
		ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID,
		Weekday: int(start.Weekday()), StartTime: "18:00", DurationMinutes: 90,
		StartsOn: start, Status: bookingstore.SeriesStatusActive, CreatedBy: f.UserID,
	}

	if err := f.Stores.BookingSeries.InsertSeriesWithBookings(ctx, series, nil, []*bookingstore.Booking{moved}); err != nil {
		t.Fatalf("InsertSeriesWithBookings: %v", err)
	}
	if moved.SeriesID == nil || *moved.SeriesID != series.ID || moved.CourtID != other {
		t.Fatalf("the booking is linked to the series on the other court: %+v", moved)
	}
	dates, err := f.Stores.BookingSeries.SeriesDates(ctx, series.ID, day, day)
	if err != nil || len(dates) != 1 || dates[0].Outcome != bookingstore.SeriesDateBooked {
		t.Fatalf("the moved date is booked in the ledger; got %+v, %v", dates, err)
	}
	occs, err := f.Stores.Bookings.SeriesOccurrences(ctx, &f.ComplexID, []uuid.UUID{f.CourtID}, day, day)
	if err != nil || len(occs) != 0 {
		t.Errorf("the series must not hold its own court on the moved date; got %+v, %v", occs, err)
	}
}

// The picked court is rechecked under the lock: when something already holds
// those hours there, nothing is written, series included.
func TestAMovedDateOnABusyCourtWritesNothing(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	start := seriesStart()
	other := secondCourt(t, f)
	day := start.AddDate(0, 0, 14)

	busy := f.NewBooking(datatest.BookingOptions{StartTime: "18:30", EndTime: "20:00"})
	busy.CourtID, busy.Date = other, day
	if err := f.Stores.Bookings.InsertSafe(ctx, busy); err != nil {
		t.Fatal(err)
	}
	moved := f.NewBooking(datatest.BookingOptions{StartTime: "18:00", EndTime: "19:30"})
	moved.CourtID, moved.Date = other, day
	series := &bookingstore.Series{
		ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID,
		Weekday: int(start.Weekday()), StartTime: "18:00", DurationMinutes: 90,
		StartsOn: start, Status: bookingstore.SeriesStatusActive, CreatedBy: f.UserID,
	}

	err := f.Stores.BookingSeries.InsertSeriesWithBookings(ctx, series, nil, []*bookingstore.Booking{moved})
	if !errors.Is(err, bookingstore.ErrDuplicateBooking) && !errors.Is(err, bookingstore.ErrSlotUnavailable) {
		t.Fatalf("want the slot refused; got %v", err)
	}
	var n int
	if err := f.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM booking_series WHERE complex_id = $1`, f.ComplexID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("a refused series must leave no row; %d (%v)", n, err)
	}
}
