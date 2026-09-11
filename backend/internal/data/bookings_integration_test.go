//go:build integration

package data

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// Two clients paying for the same court at the same hour is the failure this system
// exists to prevent, and it is the one invariant no mock can check: the advisory lock,
// the overlap query and the insert have to hold together inside one real transaction
// against one real database, with a second transaction genuinely racing it.
//
// The bookings here are freshly created and owner-attributed, so the stale-pending
// exclusion inside InsertSafe plays no part — this is the plain collision.
func TestConcurrentInsertSafeLetsExactlyOneBookingThrough(t *testing.T) {
	f := newTestFixture(t)

	const attempts = 2

	start := make(chan struct{})
	results := make([]error, attempts)

	var wg sync.WaitGroup
	for i := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()

			booking := f.newBooking(bookingOptions{Status: "pending", CollectionStatus: CollectionStatusUnpaid})

			<-start
			results[i] = f.Models.Bookings.InsertSafe(context.Background(), booking)
		}()
	}

	close(start)
	wg.Wait()

	var accepted int
	var rejected int
	for i, err := range results {
		switch {
		case err == nil:
			accepted++
		case errors.Is(err, ErrSlotUnavailable):
			rejected++
		default:
			t.Errorf("attempt %d failed for an unexpected reason: %v", i, err)
		}
	}

	if accepted != 1 {
		t.Errorf("exactly one of the two racing bookings must be accepted; %d were", accepted)
	}
	if rejected != 1 {
		t.Errorf("the loser must be turned away with ErrSlotUnavailable; %d were", rejected)
	}

	var stored int
	err := f.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM bookings
		WHERE court_id = $1 AND status NOT IN ('cancelled', 'no_show')`,
		f.CourtID,
	).Scan(&stored)
	if err != nil {
		t.Fatalf("counting stored bookings: %v", err)
	}
	if stored != 1 {
		t.Errorf("the court must hold exactly one live booking for that slot; found %d", stored)
	}
}

// The overlap check is a time-range test, not a start-time equality test: a booking
// starting half an hour into an existing one is just as double-sold, and the partial
// unique index on (court_id, date, start_time) does not see it. Running these
// sequentially isolates the query from the locking above.
func TestInsertSafeRejectsAnOverlappingBooking(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	first := f.newBooking(bookingOptions{StartTime: "18:00", EndTime: "19:30"})
	if err := f.Models.Bookings.InsertSafe(ctx, first); err != nil {
		t.Fatalf("the first booking must be accepted: %v", err)
	}

	overlapping := f.newBooking(bookingOptions{StartTime: "18:30", EndTime: "20:00"})
	err := f.Models.Bookings.InsertSafe(ctx, overlapping)
	if !errors.Is(err, ErrSlotUnavailable) {
		t.Errorf("a booking overlapping a live one must be refused with ErrSlotUnavailable; got %v", err)
	}

	adjacent := f.newBooking(bookingOptions{StartTime: "19:30", EndTime: "21:00"})
	if err := f.Models.Bookings.InsertSafe(ctx, adjacent); err != nil {
		t.Errorf("a booking starting exactly when the previous one ends must be accepted: %v", err)
	}
}
