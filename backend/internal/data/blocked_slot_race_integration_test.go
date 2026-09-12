//go:build integration

package data_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/data"
)

// These tests cover the other direction of the blocked-slot guard: the write
// that takes hours off sale, rather than the one that sells them.
//
// blocked_slots.span made booking-versus-block overlap visible to the booking
// transaction and made block-versus-block overlap unrepresentable. Neither
// closes the window this file is about. InsertBlockedSlot took no advisory
// lock and asked nothing about bookings, so under READ COMMITTED a block could
// commit in the middle of a booking transaction that had already looked: the
// booking's SELECT saw no block, the block's INSERT saw no booking, and
// bookings_no_overlapping_span cannot help because EXCLUDE is single-table.
// Both rows committed and the court was sold while it was closed.
//
// The interleaving is replayed rather than raced for. blockCourtDay (see
// slot_guard_integration_test.go) holds the very lock the write paths take, so
// a writer that reaches it queues instead of finishing, and waitForLockWaiters
// proves it really queued rather than merely having been started — a writer
// that takes no lock never appears in that queue and fails the test there.

// bookedDay is the calendar date helper these tests hang their rows on. pgx
// encodes a pgtype.Date from the value's own wall-clock Y/M/D, so UTC midnight
// names the same day the fixture rows do.
func bookedDay(d int) time.Time {
	return time.Date(2026, time.December, d, 0, 0, 0, 0, time.UTC)
}

// insertLiveBooking commits a confirmed booking straight through the pool,
// without the court-day lock. That is the point: it stands for the writer that
// is not participating in the blocked-slot transaction's serialization.
func insertLiveBooking(t *testing.T, f *testFixture, date time.Time, start string, durationMinutes int) {
	t.Helper()

	_, err := f.Pool.Exec(context.Background(), `
		INSERT INTO bookings (
			complex_id, court_id, client_id, date,
			start_time, duration_minutes,
			price, deposit_amount, status, collection_status, created_by
		)
		VALUES ($1, $2, $3, $4, $5::time, $6,
		        500000, 150000, 'confirmed', 'deposit_paid', $7)`,
		f.ComplexID, f.CourtID, f.ClientID, date, start, durationMinutes, f.UserID)
	if err != nil {
		t.Fatalf("inserting the booking %s +%dm: %v", start, durationMinutes, err)
	}
}

func countBlocks(t *testing.T, f *testFixture) int {
	t.Helper()

	var n int
	err := f.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM blocked_slots WHERE court_id = $1`, f.CourtID).Scan(&n)
	if err != nil {
		t.Fatalf("counting blocked slots: %v", err)
	}
	return n
}

// TestBlockOverALiveBookingIsRefused is the deterministic half: the booking is
// already committed when the block is filed, so no interleaving is involved
// and the only question is whether InsertBlockedSlot asks about bookings at
// all. It did not — the sole booking-versus-block check on this side lived in
// the handler, outside any transaction, which is what made the race below
// possible in the first place.
func TestBlockOverALiveBookingIsRefused(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()
	date := bookedDay(1)

	insertLiveBooking(t, f, date, "20:00", 60)

	reason := "maintenance"
	block := &data.BlockedSlot{
		CourtID: f.CourtID, Date: date,
		StartTime: "20:30", EndTime: "21:30", Reason: &reason,
	}
	err := f.Models.Courts.InsertBlockedSlot(ctx, block)
	if !errors.Is(err, data.ErrSlotHasBooking) {
		t.Fatalf("blocking 20:30-21:30 over a confirmed 20:00-21:00 booking: got err = %v, want ErrSlotHasBooking", err)
	}
	if n := countBlocks(t, f); n != 0 {
		t.Errorf("the refused block must leave no row behind; found %d", n)
	}
}

// TestBlockCommittingMidBookingCannotSlipPast replays the interleaving F01
// opens with: an owner files a maintenance block at the same moment a customer
// checks out the same hours.
//
// The gate holds the court-day lock, so InsertBlockedSlot queues behind it
// before it has inserted anything or looked at bookings. The booking is then
// committed from outside the lock — standing for the booking transaction that
// had already run its own checks — and only then is the gate released. A block
// that takes no lock does not queue (waitForLockWaiters says so), runs its
// checks before the booking exists, and commits on top of it.
func TestBlockCommittingMidBookingCannotSlipPast(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()
	date := bookedDay(2)

	release := blockCourtDay(t, f, date)

	reason := "maintenance"
	block := &data.BlockedSlot{
		CourtID: f.CourtID, Date: date,
		StartTime: "19:00", EndTime: "20:00", Reason: &reason,
	}
	result := make(chan error, 1)
	go func() { result <- f.Models.Courts.InsertBlockedSlot(ctx, block) }()

	// The block must be inside a transaction, queued on the court-day lock,
	// before the booking commits. Without the lock it is already finished here.
	waitForLockWaiters(t, f, 1)

	insertLiveBooking(t, f, date, "19:00", 60)
	release()

	err := <-result
	if !errors.Is(err, data.ErrSlotHasBooking) {
		t.Fatalf("a block filed while a booking for the same hours was committing: got err = %v, "+
			"want ErrSlotHasBooking. The court is both sold and closed for maintenance.", err)
	}
	if n := countBlocks(t, f); n != 0 {
		t.Errorf("the refused block must leave no row behind; found %d", n)
	}
}

// TestBookingCrossingMidnightWaitsForTheNextDaysLock covers the asymmetry the
// court-day lock has when a booking outlives its own date.
//
// A booking on D running 23:00 + 120 minutes occupies D and D+1. A block on
// D+1 keys its lock on D+1. Locking only the booking's own date therefore
// serializes the two against nothing, and the span comparison blocked_slots.span
// added — which is date-free and would have seen the block — never gets the
// chance to run after it exists.
//
// The gate holds D+1. A booking that locks both of its local days queues
// there; one that locks only D sails past, checks blocked_slots before the
// block is committed, and sells the court.
func TestBookingCrossingMidnightWaitsForTheNextDaysLock(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()
	date := bookedDay(10)
	nextDay := date.AddDate(0, 0, 1)

	release := blockCourtDay(t, f, nextDay)

	b := &data.Booking{
		ComplexID: f.ComplexID, CourtID: f.CourtID, ClientID: f.ClientID,
		Date: date, StartTime: "23:00", DurationMinutes: 120,
		Price: 500_000, DepositAmount: 150_000,
		Status: "pending", CollectionStatus: data.CollectionStatusUnpaid,
		RefundStatus: data.RefundStatusNone,
	}
	result := make(chan error, 1)
	go func() { result <- f.Models.Bookings.InsertSafe(ctx, b) }()

	waitForLockWaiters(t, f, 1)

	// 00:00-01:00 on the following day: the hours the booking runs into, and a
	// row the booking's own date filter never had anything to do with.
	_, err := f.Pool.Exec(ctx, `
		INSERT INTO blocked_slots (court_id, date, start_time, end_time, reason)
		VALUES ($1, $2, '00:00'::time, '01:00'::time, 'maintenance')`,
		f.CourtID, nextDay)
	if err != nil {
		t.Fatalf("inserting the next-day block: %v", err)
	}
	release()

	if err := <-result; !errors.Is(err, data.ErrSlotUnavailable) {
		t.Fatalf("a 23:00 +120m booking against a 00:00-01:00 block committed on the following day: "+
			"got err = %v, want ErrSlotUnavailable. The booking locked only its own date, so the block "+
			"landed between its check and its COMMIT.", err)
	}

	var booked int
	if err := f.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM bookings WHERE court_id = $1`, f.CourtID).Scan(&booked); err != nil {
		t.Fatalf("counting bookings: %v", err)
	}
	if booked != 0 {
		t.Errorf("the refused booking must leave no row behind; found %d", booked)
	}
}
