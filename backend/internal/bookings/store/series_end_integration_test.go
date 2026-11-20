//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// seriesOccurrenceOn stores an occurrence of series on date with the given
// collection status.
func seriesOccurrenceOn(t *testing.T, f *datatest.Fixture, s *bookingstore.Series, date time.Time, collection bookingstore.CollectionStatus) *bookingstore.Booking {
	t.Helper()
	b := f.NewBooking(datatest.BookingOptions{StartTime: "18:00", EndTime: "19:30", CollectionStatus: collection})
	b.SeriesID = &s.ID
	b.Date = date
	if collection == bookingstore.CollectionStatusUnpaid {
		b.DepositAmount = 0
	}
	if err := f.Stores.Bookings.InsertSafe(f.Scoped(context.Background()), b); err != nil {
		t.Fatalf("InsertSafe: %v", err)
	}
	return b
}

// Ending a series cancels, in the same transaction that moves its end, only
// the occurrences still unpaid and not started: one a counter payment reached
// first stands, and the new end is written either way.
func TestEndingASeriesCancelsOnlyUnpaidOccurrencesWithItsEnd(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	start := seriesStart()
	series := newSeries(t, f, start)
	unpaid := seriesOccurrenceOn(t, f, series, start, bookingstore.CollectionStatusUnpaid)
	paidMeanwhile := seriesOccurrenceOn(t, f, series, start.AddDate(0, 0, 7), bookingstore.CollectionStatusDepositPaid)

	endsOn := start.AddDate(0, 0, -1)
	got, err := f.Stores.BookingSeries.EndSeriesCancelling(ctx, f.ComplexID, series.ID,
		[]uuid.UUID{unpaid.ID, paidMeanwhile.ID}, "Cancelación: baja del turno fijo", time.Now(),
		&endsOn, bookingstore.SeriesStatusEnded)
	if err != nil {
		t.Fatalf("EndSeriesCancelling: %v", err)
	}
	if len(got.Cancelled) != 1 || got.Cancelled[0].ID != unpaid.ID {
		t.Fatalf("want only the unpaid occurrence cancelled; got %+v", got.Cancelled)
	}
	if status, _, _ := f.ReadBookingState(t, unpaid.ID); status != "cancelled" {
		t.Errorf("unpaid occurrence status = %s", status)
	}
	if status, collection, _ := f.ReadBookingState(t, paidMeanwhile.ID); status != "confirmed" || collection != "deposit_paid" {
		t.Errorf("the paid occurrence must stand; got %s/%s", status, collection)
	}
	if got.Series.Status != bookingstore.SeriesStatusEnded || got.Series.EndsOn == nil || !got.Series.EndsOn.Equal(endsOn) {
		t.Errorf("series = %+v, want ended on %s", got.Series, endsOn.Format(time.DateOnly))
	}
}

// Creating a series rechecks, under the court lock, the other series on the
// court as well as its bookings: two owners' tabs creating the same weekly
// hours both passed the service's read-only pre-check.
func TestInsertSeriesRechecksOtherSeriesInItsTransaction(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	start := seriesStart()
	newSeries(t, f, start)

	// The same hours, starting a week later: every date of it is held already.
	clash := &bookingstore.Series{
		ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID,
		Weekday: int(start.Weekday()), StartTime: "19:00", DurationMinutes: 60,
		StartsOn: start.AddDate(0, 0, 7), Status: bookingstore.SeriesStatusActive, CreatedBy: f.UserID,
	}
	if err := f.Stores.BookingSeries.InsertSeries(ctx, clash, nil); !errors.Is(err, bookingstore.ErrSlotUnavailable) {
		t.Fatalf("want ErrSlotUnavailable for a series over another; got %v", err)
	}

	// Other hours the same day are free.
	free := *clash
	free.ID = uuid.Nil
	free.StartTime = "21:00"
	if err := f.Stores.BookingSeries.InsertSeries(ctx, &free, nil); err != nil {
		t.Fatalf("a series on free hours must be created: %v", err)
	}
}

// An occurrence whose hours have started is not cancelled by ending its series.
func TestEndingASeriesLeavesAStartedOccurrence(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	start := seriesStart()
	series := newSeries(t, f, start)
	occ := seriesOccurrenceOn(t, f, series, start, bookingstore.CollectionStatusUnpaid)

	endsOn := start.AddDate(0, 0, -1)
	got, err := f.Stores.BookingSeries.EndSeriesCancelling(ctx, f.ComplexID, series.ID,
		[]uuid.UUID{occ.ID}, "baja", occ.StartsAt.Add(time.Minute), &endsOn, bookingstore.SeriesStatusEnded)
	if err != nil {
		t.Fatalf("EndSeriesCancelling: %v", err)
	}
	if len(got.Cancelled) != 0 {
		t.Errorf("a started occurrence was cancelled: %+v", got.Cancelled)
	}
	if status, _, _ := f.ReadBookingState(t, occ.ID); status != "confirmed" {
		t.Errorf("status = %s, want confirmed", status)
	}
}
