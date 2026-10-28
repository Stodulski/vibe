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

// lockHolder reads the lock on a slot back: how many rows there are, and the
// holder token the row names. A slot with no row reads as an empty holder.
func lockHolder(t *testing.T, f *datatest.Fixture, date time.Time, startTime string) (rows int, holder string) {
	t.Helper()

	if err := f.DB.QueryRow(context.Background(),
		`SELECT count(*), COALESCE(max(locked_by), '') FROM slot_locks WHERE court_id = $1 AND date = $2 AND start_time = $3`,
		f.CourtID, date, startTime,
	).Scan(&rows, &holder); err != nil {
		t.Fatalf("reading the lock back: %v", err)
	}
	return rows, holder
}

// The ordinary case, unchanged: a live lock is a live lock.
func TestAcquireLockRefusesASlotThatIsStillHeld(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	locks := f.Stores.SlotLocks
	date := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)

	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, "checkout-a", 15*time.Minute); err != nil {
		t.Fatalf("first acquisition: %v", err)
	}

	err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, "checkout-b", 15*time.Minute)
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
	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, "checkout-a", -time.Minute); err != nil {
		t.Fatalf("seeding the expired lock: %v", err)
	}

	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, "checkout-b", 15*time.Minute); err != nil {
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
	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, "checkout-c", 15*time.Minute); !errors.Is(err, bookingstore.ErrSlotLocked) {
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

	if err := locks.AcquireLock(ctx, f.CourtID, date, "20:00", "21:30", nil, "checkout-a", -time.Minute); err != nil {
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
			results[i] = locks.AcquireLock(context.Background(), f.CourtID, date, "20:00", "21:30", nil, uuid.NewString(), 15*time.Minute)
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
	if err := locks.AcquireLock(ctx, f.CourtID, date, "21:00", "22:30", &first.ID, "checkout-a", -time.Minute); err != nil {
		t.Fatalf("seeding the expired lock: %v", err)
	}

	second := f.CreateBooking(t, datatest.BookingOptions{StartTime: "09:00", EndTime: "10:30"})
	if err := locks.AcquireLock(ctx, f.CourtID, date, "21:00", "22:30", &second.ID, "checkout-b", 15*time.Minute); err != nil {
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

// The checkout that took a lock records its holder token on it, and frees the
// lock with that same token. The slot is takeable again afterwards.
func TestReleaseHeldLockFreesTheLockForItsHolder(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	locks := f.Stores.SlotLocks
	date := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)

	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, "checkout-a", 15*time.Minute); err != nil {
		t.Fatalf("taking the lock: %v", err)
	}
	if _, holder := lockHolder(t, f, date, "18:00"); holder != "checkout-a" {
		t.Fatalf("the acquired lock must record its holder token; got %q", holder)
	}

	if err := locks.ReleaseHeldLock(ctx, f.CourtID, date, "18:00", "checkout-a"); err != nil {
		t.Fatalf("releasing with the holder token: %v", err)
	}
	if rows, _ := lockHolder(t, f, date, "18:00"); rows != 0 {
		t.Errorf("the holder must free its own lock; %d row(s) remain", rows)
	}
	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, "checkout-b", 15*time.Minute); err != nil {
		t.Errorf("a freed slot must be takeable again: %v", err)
	}
}

// A token that does not hold the lock frees nothing, and the lock keeps the holder
// it was taken with. Nothing to free is not an error.
func TestReleaseHeldLockLeavesALockItDoesNotHold(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	locks := f.Stores.SlotLocks
	date := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)

	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, "checkout-a", 15*time.Minute); err != nil {
		t.Fatalf("taking the lock: %v", err)
	}

	if err := locks.ReleaseHeldLock(ctx, f.CourtID, date, "18:00", "checkout-b"); err != nil {
		t.Fatalf("a release by a non-holder must not fail: %v", err)
	}
	if rows, holder := lockHolder(t, f, date, "18:00"); rows != 1 || holder != "checkout-a" {
		t.Errorf("a release by a non-holder must leave the lock as it was; got %d row(s) held by %q", rows, holder)
	}
	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, "checkout-c", 15*time.Minute); !errors.Is(err, bookingstore.ErrSlotLocked) {
		t.Errorf("the lock must still refuse other checkouts after a non-holder release; got %v", err)
	}
}

// The takeover writes the new holder's token onto the row. If it kept the old
// one, the checkout whose TTL ran out would still be the holder on record.
func TestTakeoverRewritesTheHolderToken(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	locks := f.Stores.SlotLocks
	date := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)

	// A lock that expired a minute ago, still naming the checkout that abandoned it.
	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, "checkout-a", -time.Minute); err != nil {
		t.Fatalf("seeding the expired lock: %v", err)
	}
	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, "checkout-b", 15*time.Minute); err != nil {
		t.Fatalf("taking over the expired lock: %v", err)
	}

	if rows, holder := lockHolder(t, f, date, "18:00"); rows != 1 || holder != "checkout-b" {
		t.Errorf("the takeover must rewrite the holder token; got %d row(s) held by %q", rows, holder)
	}
}

// The case the holder check exists for. A checkout's TTL ran out, another checkout
// took the slot over, and then the first checkout releases late. That late release
// must not free the other checkout's lock. The new holder's own release still does.
func TestAStaleHolderCannotFreeALockTakenOverFromIt(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	locks := f.Stores.SlotLocks
	date := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)

	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, "checkout-a", -time.Minute); err != nil {
		t.Fatalf("seeding the expired lock: %v", err)
	}
	if err := locks.AcquireLock(ctx, f.CourtID, date, "18:00", "19:30", nil, "checkout-b", 15*time.Minute); err != nil {
		t.Fatalf("taking over the expired lock: %v", err)
	}

	if err := locks.ReleaseHeldLock(ctx, f.CourtID, date, "18:00", "checkout-a"); err != nil {
		t.Fatalf("the stale holder's late release must not fail: %v", err)
	}
	if rows, holder := lockHolder(t, f, date, "18:00"); rows != 1 || holder != "checkout-b" {
		t.Fatalf("the stale release must leave the new holder's lock in place; got %d row(s) held by %q", rows, holder)
	}

	if err := locks.ReleaseHeldLock(ctx, f.CourtID, date, "18:00", "checkout-b"); err != nil {
		t.Fatalf("the new holder releasing its own lock: %v", err)
	}
	if rows, _ := lockHolder(t, f, date, "18:00"); rows != 0 {
		t.Errorf("the new holder must free its own lock; %d row(s) remain", rows)
	}
}
