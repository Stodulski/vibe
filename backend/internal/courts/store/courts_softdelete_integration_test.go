//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
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
	f := datatest.NewFixture(t)

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
	f := datatest.NewFixture(t)
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
	f := datatest.NewFixture(t)
	ctx := context.Background()

	f.CreateBooking(t, datatest.BookingOptions{Status: "confirmed"})

	err := f.Stores.Courts.SoftDelete(ctx, f.CourtID)
	if !errors.Is(err, courtstore.ErrCourtHasActiveBookings) {
		t.Errorf("deleting a court with a live booking must be refused with ErrCourtHasActiveBookings; got %v", err)
	}
}
