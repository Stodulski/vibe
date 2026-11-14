//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// seriesOn stores a weekly series on the fixture's court, ending on endsOn when
// it is not nil.
func seriesOn(t *testing.T, f *datatest.Fixture, endsOn *time.Time) *bookingstore.Series {
	t.Helper()
	start := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
	s := &bookingstore.Series{
		ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID,
		Weekday: int(start.Weekday()), StartTime: "18:00", DurationMinutes: 90,
		StartsOn: start, EndsOn: endsOn, Status: bookingstore.SeriesStatusActive, CreatedBy: f.UserID,
	}
	if err := f.Stores.BookingSeries.InsertSeries(f.Scoped(context.Background()), s, nil); err != nil {
		t.Fatalf("InsertSeries: %v", err)
	}
	return s
}

// An active series has no booking row for dates not booked yet; deleting the
// complex must still be refused, and the refusal must roll the delete back.
func TestSoftDeleteCascadeRefusesAComplexWithAnActiveSeries(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	seriesOn(t, f, nil)

	_, err := f.Stores.Complexes.SoftDeleteCascade(ctx, f.ComplexID)
	if !errors.Is(err, complexstore.ErrActiveSeries) {
		t.Fatalf("want ErrActiveSeries; got %v", err)
	}
	if _, err := f.Stores.Complexes.GetByID(ctx, f.ComplexID); err != nil {
		t.Errorf("a refused delete must leave the complex live; got %v", err)
	}
}

func TestSoftDeleteCascadeRefusesASeriesThatEndsInTheFuture(t *testing.T) {
	f := datatest.Isolated(t)
	end := time.Now().AddDate(0, 0, 30).Truncate(24 * time.Hour)
	seriesOn(t, f, &end)

	_, err := f.Stores.Complexes.SoftDeleteCascade(context.Background(), f.ComplexID)
	if !errors.Is(err, complexstore.ErrActiveSeries) {
		t.Errorf("want ErrActiveSeries; got %v", err)
	}
}

func TestSoftDeleteCascadeIgnoresAnEndedSeries(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	end := time.Now().AddDate(0, 0, 30).Truncate(24 * time.Hour)
	ser := seriesOn(t, f, nil)
	if _, err := f.Stores.BookingSeries.EndSeries(f.Scoped(ctx), ser.ID, &end, bookingstore.SeriesStatusEnded); err != nil {
		t.Fatalf("EndSeries: %v", err)
	}

	if _, err := f.Stores.Complexes.SoftDeleteCascade(ctx, f.ComplexID); err != nil {
		t.Errorf("an ended series holds nothing; got %v", err)
	}
}
