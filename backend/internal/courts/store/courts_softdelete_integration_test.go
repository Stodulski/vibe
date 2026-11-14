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
)

// R2-softdelete-error-name-overreaches / R3-softdelete-conflates-notfound:
// courtstore.Store.SoftDelete's WHERE clause has three independent ways to match
// zero rows, and an earlier version mapped all three to
// ErrCourtHasActiveBookings. These three tests are the three causes, each
// pinned against the real query rather than the handler stub — the stub can
// only model the has-bookings branch, not the SQL's own zero-row cases.

// TestSoftDeleteAnUnknownCourtIsNotFound is the first cause: an id nothing
// owns. It must answer ErrRecordNotFound, not a claim about bookings that
// cannot exist for a court that was never created.
func TestSoftDeleteAnUnknownCourtIsNotFound(t *testing.T) {
	f := datatest.Isolated(t)

	err := f.Stores.Courts.SoftDelete(context.Background(), uuid.New())
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("deleting an unknown court must answer ErrRecordNotFound; got %v", err)
	}
}

// TestSoftDeleteIsIdempotentOnAnAlreadyDeletedCourt is the second cause: a
// retried or concurrent delete of a court already soft-deleted. It used to
// succeed silently before the has-bookings mapping swallowed it into a false
// 409; it must go back to succeeding, since the caller's state is already
// what it asked for.
func TestSoftDeleteIsIdempotentOnAnAlreadyDeletedCourt(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	if err := f.Stores.Courts.SoftDelete(ctx, f.CourtID); err != nil {
		t.Fatalf("first delete must succeed: %v", err)
	}
	if err := f.Stores.Courts.SoftDelete(ctx, f.CourtID); err != nil {
		t.Errorf("a retried delete of an already-deleted court must be idempotent success, not an error; got %v", err)
	}
}

// TestSoftDeleteRefusesACourtWithLiveBookings is the third cause, and the
// only one ErrCourtHasActiveBookings actually describes: a live booking still
// owes someone those hours.
func TestSoftDeleteRefusesACourtWithLiveBookings(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	f.CreateBooking(t, datatest.BookingOptions{Status: "confirmed"})

	err := f.Stores.Courts.SoftDelete(ctx, f.CourtID)
	if !errors.Is(err, courtstore.ErrCourtHasActiveBookings) {
		t.Errorf("deleting a court with a live booking must be refused with ErrCourtHasActiveBookings; got %v", err)
	}
}

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

// A fixed weekly booking has no booking row until its day, so the live-booking
// question cannot see it; the court must still refuse to be deleted.
func TestSoftDeleteRefusesACourtWithAnActiveSeries(t *testing.T) {
	f := datatest.Isolated(t)
	seriesOn(t, f, nil)

	err := f.Stores.Courts.SoftDelete(context.Background(), f.CourtID)
	if !errors.Is(err, courtstore.ErrCourtHasActiveSeries) {
		t.Errorf("want ErrCourtHasActiveSeries; got %v", err)
	}
}

func TestSoftDeleteRefusesASeriesThatEndsInTheFuture(t *testing.T) {
	f := datatest.Isolated(t)
	end := time.Now().AddDate(0, 0, 30).Truncate(24 * time.Hour)
	seriesOn(t, f, &end)

	if err := f.Stores.Courts.SoftDelete(context.Background(), f.CourtID); !errors.Is(err, courtstore.ErrCourtHasActiveSeries) {
		t.Errorf("want ErrCourtHasActiveSeries; got %v", err)
	}
}

func TestSoftDeleteIgnoresAnEndedSeries(t *testing.T) {
	f := datatest.Isolated(t)
	end := time.Now().AddDate(0, 0, 30).Truncate(24 * time.Hour)
	ser := seriesOn(t, f, nil)
	if _, err := f.Stores.BookingSeries.EndSeries(f.Scoped(context.Background()), ser.ID, &end, bookingstore.SeriesStatusEnded); err != nil {
		t.Fatalf("EndSeries: %v", err)
	}

	if err := f.Stores.Courts.SoftDelete(context.Background(), f.CourtID); err != nil {
		t.Errorf("an ended series holds nothing; got %v", err)
	}
}
