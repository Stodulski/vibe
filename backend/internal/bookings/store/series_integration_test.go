//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// newSeries stores a weekly series starting on the date NewBooking uses.
func newSeries(t *testing.T, f *datatest.Fixture, startsOn time.Time) *bookingstore.Series {
	t.Helper()
	s := &bookingstore.Series{
		ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID,
		Weekday: int(startsOn.Weekday()), StartTime: "18:00", DurationMinutes: 90,
		StartsOn: startsOn, Status: bookingstore.SeriesStatusActive, CreatedBy: f.UserID,
	}
	if err := f.Stores.BookingSeries.InsertSeries(f.Scoped(context.Background()), s, nil); err != nil {
		t.Fatalf("InsertSeries: %v", err)
	}
	return s
}

func occurrence(f *datatest.Fixture, s *bookingstore.Series, start, end string) *bookingstore.Booking {
	b := f.NewBooking(datatest.BookingOptions{StartTime: start, EndTime: end})
	b.SeriesID = &s.ID
	b.Date = s.StartsOn
	return b
}

// Migration 027: an occurrence claims its date in its own transaction, so a
// second booking for the same (series, date) is refused and leaves no row behind.
func TestAnOccurrenceClaimsItsDateAtMostOnce(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	first := occurrence(f, newSeries(t, f, time.Now().AddDate(0, 0, 7).Truncate(24*time.Hour)), "18:00", "19:30")
	series, err := f.Stores.BookingSeries.GetSeries(ctx, *first.SeriesID)
	if err != nil {
		t.Fatalf("GetSeries: %v", err)
	}

	if err := f.Stores.Bookings.InsertSafe(ctx, first); err != nil {
		t.Fatalf("InsertSafe: %v", err)
	}
	dates, err := f.Stores.BookingSeries.SeriesDates(ctx, series.ID, series.StartsOn, series.StartsOn)
	if err != nil || len(dates) != 1 || dates[0].Outcome != bookingstore.SeriesDateBooked {
		t.Fatalf("the date must be recorded as booked; got %+v, %v", dates, err)
	}

	// A cancelled occurrence keeps its date: nothing may book it again.
	f.MarkStatus(t, first.ID, "cancelled")
	again := occurrence(f, series, "20:00", "21:30")
	if err := f.Stores.Bookings.InsertSafe(ctx, again); !errors.Is(err, bookingstore.ErrSeriesDateGenerated) {
		t.Fatalf("a second generation of the date must be refused with ErrSeriesDateGenerated; got %v", err)
	}
	var n int
	if err := f.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM bookings WHERE series_id = $1`, series.ID).Scan(&n); err != nil || n != 1 {
		t.Errorf("the refused insert must roll its booking back; %d rows for the series (%v)", n, err)
	}
}

func TestAnExceptionDateIsNeverGenerated(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	series := newSeries(t, f, time.Now().AddDate(0, 0, 7).Truncate(24*time.Hour))

	if done, err := f.Stores.BookingSeries.RecordSeriesException(ctx, series, series.StartsOn, "overlap"); err != nil || !done {
		t.Fatalf("RecordSeriesException = %v, %v", done, err)
	}
	if done, err := f.Stores.BookingSeries.RecordSeriesException(ctx, series, series.StartsOn, "blocked"); err != nil || done {
		t.Fatalf("a repeated exception is a no-op reported as such: %v, %v", done, err)
	}
	dates, _ := f.Stores.BookingSeries.SeriesDates(ctx, series.ID, series.StartsOn, series.StartsOn)
	if len(dates) != 1 || dates[0].Outcome != bookingstore.SeriesDateException || dates[0].Reason == nil || *dates[0].Reason != "overlap" {
		t.Fatalf("the first answer stands; got %+v", dates)
	}
	if err := f.Stores.Bookings.InsertSafe(ctx, occurrence(f, series, "18:00", "19:30")); !errors.Is(err, bookingstore.ErrSeriesDateGenerated) {
		t.Errorf("an excepted date must not be booked; got %v", err)
	}
}

func TestSeriesBookingsCarryTheirSeriesOnTheOwnersReads(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	series := newSeries(t, f, time.Now().AddDate(0, 0, 7).Truncate(24*time.Hour))
	b := occurrence(f, series, "18:00", "19:30")
	if err := f.Stores.Bookings.InsertSafe(ctx, b); err != nil {
		t.Fatalf("InsertSafe: %v", err)
	}

	got, err := f.Stores.Bookings.GetByID(ctx, b.ID)
	if err != nil || got.SeriesID == nil || *got.SeriesID != series.ID {
		t.Fatalf("GetByID must return series_id; got %+v, %v", got, err)
	}
	list, _, err := f.Stores.Bookings.GetByComplex(ctx, f.ComplexID, b.Date, b.Date, data.Filters{Limit: 20})
	if err != nil || len(list) != 1 || list[0].SeriesID == nil || *list[0].SeriesID != series.ID {
		t.Fatalf("the day's list must return series_id; got %+v, %v", list, err)
	}
	upcoming, err := f.Stores.BookingSeries.SeriesBookingsFrom(ctx, series.ID, b.Date)
	if err != nil || len(upcoming) != 1 {
		t.Fatalf("SeriesBookingsFrom: %v, %v", upcoming, err)
	}

	plain := f.NewBooking(datatest.BookingOptions{StartTime: "09:00", EndTime: "10:30"})
	if err := f.Stores.Bookings.InsertSafe(ctx, plain); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.Stores.Bookings.GetByID(ctx, plain.ID); got == nil || got.SeriesID != nil {
		t.Error("an ordinary booking has no series")
	}
}

func TestAnotherTenantCannotReadASeries(t *testing.T) {
	f := datatest.Isolated(t)
	series := newSeries(t, f, time.Now().AddDate(0, 0, 7).Truncate(24*time.Hour))

	foreign := data.ContextWithTenant(context.Background(), uuid.New())
	if _, err := f.Stores.BookingSeries.GetSeries(foreign, series.ID); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a series of another tenant must read as not found; got %v", err)
	}
}

func TestEndingASeriesStoresTheNewEnd(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	series := newSeries(t, f, time.Now().AddDate(0, 0, 7).Truncate(24*time.Hour))
	before := series.StartsOn.AddDate(0, 0, -1) // the empty series

	ended, err := f.Stores.BookingSeries.EndSeries(ctx, series.ID, &before, bookingstore.SeriesStatusEnded)
	if err != nil || ended.Status != bookingstore.SeriesStatusEnded || ended.EndsOn == nil || !ended.EndsOn.Equal(before) {
		t.Fatalf("EndSeries = %+v, %v", ended, err)
	}
}

func TestEndExpiredSeriesMarksOnlyThoseWhoseLastDayPassed(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	start := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)
	live := newSeries(t, f, start)
	over := newSeries(t, f, start)
	last := start
	if _, err := f.Stores.BookingSeries.EndSeries(ctx, over.ID, &last, bookingstore.SeriesStatusActive); err != nil {
		t.Fatal(err)
	}

	n, err := f.Stores.BookingSeries.EndExpiredSeries(data.ContextWithTenantBypass(context.Background()), start.AddDate(0, 0, 1))
	if err != nil || n != 1 {
		t.Fatalf("EndExpiredSeries = %d, %v; want 1", n, err)
	}
	if got, _ := f.Stores.BookingSeries.GetSeries(ctx, over.ID); got.Status != bookingstore.SeriesStatusEnded {
		t.Errorf("expired series status = %s", got.Status)
	}
	if got, _ := f.Stores.BookingSeries.GetSeries(ctx, live.ID); got.Status != bookingstore.SeriesStatusActive {
		t.Errorf("a series without an end must stay active; got %s", got.Status)
	}
}
