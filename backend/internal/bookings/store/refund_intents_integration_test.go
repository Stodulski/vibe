//go:build integration

package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// These tests exercise the reconciliation sweep's store layer
// (refund-intent-durability spec) against a real PostgreSQL: the predicate
// GetRefundIntentOrphans reads, the compare-and-swap ClaimRefundIntent
// depends on, and the bookings_refund_intent_only_when_cancelled CHECK.

// The two claims run as real goroutines against real connections, mirroring
// TestTwoConcurrentClaimsOnlyOneWins above: running them sequentially would
// pass on any implementation that merely re-reads the row, and the property
// under test is specifically what the database does when two UPDATEs
// contend for the same row at once.
func TestTwoConcurrentSweepersOnlyOneClaimsAnOrphan(t *testing.T) {
	f := datatest.Shared(t)
	ctx := context.Background()

	booking := f.CreateBooking(t, datatest.BookingOptions{Status: "cancelled"})
	seen := setRefundIntentAt(f, t, booking.ID, time.Now().Add(-10*time.Minute))

	type result struct{ err error }
	results := make([]result, 2)

	var start sync.WaitGroup
	start.Add(1)
	var done sync.WaitGroup
	done.Add(2)

	for i := range results {
		go func() {
			defer done.Done()
			start.Wait() // release both goroutines as close to together as possible
			err := f.Stores.Bookings.ClaimRefundIntent(ctx, booking.ID, seen)
			results[i] = result{err: err}
		}()
	}
	start.Done()
	done.Wait()

	var won, refused int
	for _, r := range results {
		switch {
		case r.err == nil:
			won++
		case errors.Is(r.err, data.ErrRecordNotFound):
			refused++
		default:
			t.Errorf("a losing claim must be refused with ErrRecordNotFound; got %v", r.err)
		}
	}
	if won != 1 {
		t.Fatalf("exactly one sweeper may claim an orphan, or the client is refunded twice; %d won", won)
	}
	if refused != 1 {
		t.Fatalf("the losing claim must be refused, not silently succeed; %d refused", refused)
	}
}

// GetRefundIntentOrphans' predicate names refund_intent_at and nothing else,
// so it is worth proving against a real query planner and a real column
// value rather than only the Go struct this file otherwise exercises.
func TestGetRefundIntentOrphansFindsAnAgedMarkerAndNothingElse(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	aged := f.CreateBooking(t, datatest.BookingOptions{Status: "cancelled", StartTime: "08:00", EndTime: "09:30"})
	setRefundIntentAt(f, t, aged.ID, time.Now().Add(-10*time.Minute))

	fresh := f.CreateBooking(t, datatest.BookingOptions{Status: "cancelled", StartTime: "10:00", EndTime: "11:30"})
	setRefundIntentAt(f, t, fresh.ID, time.Now())

	unmarked := f.CreateBooking(t, datatest.BookingOptions{Status: "cancelled", StartTime: "12:00", EndTime: "13:30"})

	orphans, err := f.Stores.Bookings.GetRefundIntentOrphans(ctx, 5*time.Minute, 50)
	if err != nil {
		t.Fatalf("GetRefundIntentOrphans: %v", err)
	}
	found := make(map[uuid.UUID]bool, len(orphans))
	for _, o := range orphans {
		found[o.ID] = true
	}

	if !found[aged.ID] {
		t.Error("a marker older than the grace period must be found")
	}
	if found[fresh.ID] {
		t.Error("a marker set moments ago is still inside its grace period and must not be swept yet")
	}
	if found[unmarked.ID] {
		t.Error("a cancelled booking with no marker at all must never be found — refund_intent_at is the only column the predicate names")
	}
}

// ClaimRefund clears the marker inside its own committed transaction (Phase
// 12.5), so a booking that reaches the ordinary claim-first refund path
// never lingers as a false orphan behind it.
func TestClaimRefundLeavesTheRefundIntentMarkerCleared(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	booking := f.CreateBooking(t, datatest.BookingOptions{Status: "cancelled"})
	setRefundIntentAt(f, t, booking.ID, time.Now().Add(-10*time.Minute))
	mpPaymentID := "mp-" + uuid.NewString()
	payment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)

	if _, err := f.Stores.Payments.ClaimRefund(ctx, payment.ID); err != nil {
		t.Fatalf("ClaimRefund: %v", err)
	}

	if got := readRefundIntentAt(f, t, booking.ID); got != nil {
		t.Errorf("ClaimRefund must clear the refund-intent marker inside its own transaction; got %v", *got)
	}
}

// bookings_refund_intent_only_when_cancelled is the floor under "an intent marker on a live
// booking is not a state this product has": nothing today can trip it (the
// design's own caveat), but this proves it refuses the value directly rather
// than only by inspection of the migration.
func TestARefundIntentMarkerIsRefusedOnAConfirmedBooking(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	booking := f.CreateBooking(t, datatest.BookingOptions{Status: "confirmed"})

	_, err := f.DB.Exec(ctx,
		`UPDATE bookings SET refund_intent_at = NOW() WHERE id = $1`, booking.ID)
	if err == nil {
		t.Fatal("the bookings_refund_intent_only_when_cancelled CHECK must refuse a marker on a non-cancelled row")
	}
}

// The cause stored next to a standing marker is what the sweep reads to tell
// the client why the money came back. It belongs to that marker alone: every
// write that clears the marker drops it, so a later cancellation of the same
// booking can never inherit a reason that was not its own.
func TestARefundIntentCauseLivesAndDiesWithItsMarker(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	withCause := func(start, end string) uuid.UUID {
		t.Helper()
		b := f.CreateBooking(t, datatest.BookingOptions{Status: "cancelled", StartTime: start, EndTime: end})
		setRefundIntentAt(f, t, b.ID, time.Now().Add(-10*time.Minute))
		if err := f.Stores.Bookings.SetRefundIntentCause(ctx, b.ID, "late_payment"); err != nil {
			t.Fatalf("SetRefundIntentCause: %v", err)
		}
		return b.ID
	}

	swept := withCause("08:00", "09:00")
	orphans, err := f.Stores.Bookings.GetRefundIntentOrphans(ctx, 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("GetRefundIntentOrphans: %v", err)
	}
	if len(orphans) != 1 || orphans[0].ID != swept || orphans[0].RefundIntentCause != "late_payment" {
		t.Fatalf("the sweep must read the cause next to the marker; got %+v", orphans)
	}

	claimed := withCause("10:00", "11:00")
	mpPaymentID := "mp-" + uuid.NewString()
	payment := f.CreatePayment(t, claimed, 150_000, 7_500, &mpPaymentID)
	if _, err := f.Stores.Payments.ClaimRefund(ctx, payment.ID); err != nil {
		t.Fatalf("ClaimRefund: %v", err)
	}

	cleared := withCause("12:00", "13:00")
	if err := f.Stores.Bookings.ClearRefundIntent(ctx, cleared); err != nil {
		t.Fatalf("ClearRefundIntent: %v", err)
	}

	updated := withCause("14:00", "15:00")
	booking, err := f.Stores.Bookings.GetByID(f.Scoped(ctx), updated)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	booking.RefundIntentAt = nil
	if err := f.Stores.Bookings.Update(f.Scoped(ctx), booking); err != nil {
		t.Fatalf("Update: %v", err)
	}

	for name, id := range map[string]uuid.UUID{"ClaimRefund": claimed, "ClearRefundIntent": cleared, "Update": updated} {
		var cause *string
		if err := f.DB.QueryRow(ctx, `SELECT refund_intent_cause FROM bookings WHERE id = $1`, id).Scan(&cause); err != nil {
			t.Fatalf("reading the cause: %v", err)
		}
		if cause != nil {
			t.Errorf("%s cleared the marker but left its cause %q behind", name, *cause)
		}
	}

	if err := f.Stores.Bookings.SetRefundIntentCause(ctx, cleared, "late_payment"); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a cause with no marker to stand next to must be refused; got %v", err)
	}
	if err := f.Stores.Bookings.MarkRefundIntent(ctx, cleared); err != nil {
		t.Fatalf("MarkRefundIntent: %v", err)
	}
	if got := readRefundIntentAt(f, t, cleared); got == nil {
		t.Error("MarkRefundIntent must stand the marker up again for the sweep")
	}
}

// setRefundIntentAt writes a specific refund_intent_at value directly, bypassing
// bookingstore.Store.Update so tests can pin the exact "seen" value ClaimRefundIntent's
// compare-and-swap needs, and returns it for the caller to pass straight through.
func setRefundIntentAt(f *datatest.Fixture, t *testing.T, id uuid.UUID, at time.Time) time.Time {
	t.Helper()

	tag, err := f.DB.Exec(context.Background(),
		`UPDATE bookings SET refund_intent_at = $2 WHERE id = $1`, id, at)
	if err != nil {
		t.Fatalf("setting refund_intent_at for booking %s: %v", id, err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("setting refund_intent_at for booking %s: want 1 row affected, got %d", id, tag.RowsAffected())
	}

	var stored time.Time
	if err := f.DB.QueryRow(context.Background(),
		`SELECT refund_intent_at FROM bookings WHERE id = $1`, id,
	).Scan(&stored); err != nil {
		t.Fatalf("reading back refund_intent_at for booking %s: %v", id, err)
	}
	return stored
}

// readRefundIntentAt re-reads a booking's marker straight from the database,
// bypassing any struct a store method may have mutated in memory.
func readRefundIntentAt(f *datatest.Fixture, t *testing.T, id uuid.UUID) *time.Time {
	t.Helper()

	var result *time.Time
	err := f.DB.QueryRow(context.Background(),
		`SELECT refund_intent_at FROM bookings WHERE id = $1`, id,
	).Scan(&result)
	if err != nil {
		t.Fatalf("reading refund_intent_at for booking %s: %v", id, err)
	}
	return result
}
