//go:build integration

package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// The lock's TTL only means something if acquiring enforces it. These run against
// a real PostgreSQL because the whole fix is one ON CONFLICT clause: the takeover
// and the refusal are the same statement, and only the database decides which of
// them a given row gets.

// The ordinary case, unchanged: a live lock is a live lock.
func TestAcquireLockRefusesASlotThatIsStillHeld(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	locks := f.Stores.SlotLocks
	date := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)

	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, 15*time.Minute); err != nil {
		t.Fatalf("first acquisition: %v", err)
	}

	err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, 15*time.Minute)
	if !errors.Is(err, bookingstore.ErrSlotLocked) {
		t.Errorf("a slot held by a live lock must be refused; got %v", err)
	}
}

// The defect: expires_at was written and never read on the way in. A lock whose
// TTL had passed still refused every acquisition, so a checkout somebody
// abandoned kept the court unsellable from the public booking page until the
// five-minute sweeper happened to delete the row — and forever if it was not
// running.
func TestAcquireLockTakesOverAnExpiredLock(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	locks := f.Stores.SlotLocks
	date := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)

	// A lock that expired a minute ago: exactly what an abandoned checkout leaves.
	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, -time.Minute); err != nil {
		t.Fatalf("seeding the expired lock: %v", err)
	}

	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, 15*time.Minute); err != nil {
		t.Fatalf("an expired lock must not hold the slot: %v", err)
	}

	// And the takeover has to be a real lock, not a no-op that reported success.
	var expiresAt time.Time
	var rows int
	if err := f.DB.QueryRow(ctx,
		`SELECT count(*), max(expires_at) FROM slot_locks WHERE court_id = $1 AND date = $2 AND start_time = $3`,
		f.CourtID, date, "18:00",
	).Scan(&rows, &expiresAt); err != nil {
		t.Fatalf("reading the lock back: %v", err)
	}
	if rows != 1 {
		t.Fatalf("the takeover must reuse the row rather than duplicate it; got %d rows", rows)
	}
	if !expiresAt.After(time.Now()) {
		t.Errorf("the taken-over lock must carry the new TTL; expires_at is %s", expiresAt)
	}
	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, 15*time.Minute); !errors.Is(err, bookingstore.ErrSlotLocked) {
		t.Errorf("the slot must now be held by the caller that took it over; got %v", err)
	}
}

// Two clients reaching an abandoned slot at the same moment must not both be sent
// to MercadoPago for it. The takeover is one statement, so PostgreSQL serialises
// them on the row and exactly one wins.
func TestTwoCallersRacingForAnExpiredLockYieldOneWinner(t *testing.T) {
	f := datatest.Shared(t)
	ctx := context.Background()
	locks := f.Stores.SlotLocks
	date := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)

	if err := locks.AcquireLock(ctx, f.CourtID, date, "20:00", "21:30", nil, -time.Minute); err != nil {
		t.Fatalf("seeding the expired lock: %v", err)
	}

	var wg sync.WaitGroup
	results := make([]error, 2)
	start := make(chan struct{})
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i] = locks.AcquireLock(context.Background(), f.CourtID, date, "20:00", "21:30", nil, 15*time.Minute)
		}()
	}
	close(start)
	wg.Wait()

	winners := 0
	for _, err := range results {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, bookingstore.ErrSlotLocked):
		default:
			t.Fatalf("unexpected acquisition error: %v", err)
		}
	}
	if winners != 1 {
		t.Errorf("exactly one caller may take over an expired lock; %d did", winners)
	}
}

// A lock taken over carries the new holder's booking, so nothing inherits the
// previous one's identity.
func TestATakenOverLockCarriesTheNewHoldersBooking(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	locks := f.Stores.SlotLocks
	date := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)

	first := f.CreateBooking(t, datatest.BookingOptions{StartTime: "07:00", EndTime: "08:30"})
	if err := locks.AcquireLock(ctx, f.CourtID, date, "21:00", "22:30", &first.ID, -time.Minute); err != nil {
		t.Fatalf("seeding the expired lock: %v", err)
	}

	second := f.CreateBooking(t, datatest.BookingOptions{StartTime: "09:00", EndTime: "10:30"})
	if err := locks.AcquireLock(ctx, f.CourtID, date, "21:00", "22:30", &second.ID, 15*time.Minute); err != nil {
		t.Fatalf("taking over the expired lock: %v", err)
	}

	var held uuid.UUID
	if err := f.DB.QueryRow(ctx,
		`SELECT booking_id FROM slot_locks WHERE court_id = $1 AND date = $2 AND start_time = $3`,
		f.CourtID, date, "21:00",
	).Scan(&held); err != nil {
		t.Fatalf("reading the lock back: %v", err)
	}
	if held != second.ID {
		t.Errorf("the lock must name whoever holds it now; want %s, got %s", second.ID, held)
	}
}
