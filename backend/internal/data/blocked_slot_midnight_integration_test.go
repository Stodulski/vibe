//go:build integration

package data

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stodulski/vibe-server/internal/slots"
)

// TestBlockedSlotMidnight tests the in-transaction blocked-slot guard in
// BookingModel.InsertSafe (internal/data/bookings.go:251-266), not the handler
// pre-check slotIsBlocked (internal/bookings/grid.go:93-110).
//
// The distinction is the whole point. The handler pre-check compares instants
// and is midnight-safe, but it runs outside the booking transaction, so it
// cannot close the window between "no block existed when we looked" and
// COMMIT. The in-transaction guard is the one that closes that window, and it
// compares clock readings:
//
//	WHERE court_id = $1 AND date = $2 AND start_time < $4 AND end_time > $3
//
// with $4 bound to the booking's end_time. end_time comes from slots.Add,
// which wraps at midnight (internal/slots/slots.go:106-113,
// minutes %= minutesPerDay), so a 120-minute booking starting at 23:00 stores
// end_time = "01:00" rather than "25:00". The predicate then asks
// start_time('23:00') < end_time('01:00'), which is false, and the guard
// reports no collision against a block it overlaps by 59 minutes.
//
// Calling the data layer directly is deliberate: going through the HTTP
// handler would exercise the midnight-safe pre-check and prove nothing about
// the guard under test.
func TestBlockedSlotMidnight(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	// A plain calendar date. pgx encodes a pgtype.Date from the value's own
	// wall-clock Y/M/D, so UTC midnight names the same day the fixture rows do.
	day := func(d int) time.Time {
		return time.Date(2026, time.November, d, 0, 0, 0, 0, time.UTC)
	}

	blockCourt := func(t *testing.T, date time.Time, from, to string) {
		t.Helper()
		_, err := f.Pool.Exec(ctx, `
			INSERT INTO blocked_slots (court_id, date, start_time, end_time, reason)
			VALUES ($1, $2, $3::time, $4::time, 'maintenance')`,
			f.CourtID, date, from, to)
		if err != nil {
			t.Fatalf("inserting blocked slot %s-%s: %v", from, to, err)
		}
	}

	// book builds the booking exactly as the handlers do — a start time and a
	// duration, with the end left to the generated span — and calls the
	// transactional path directly.
	book := func(date time.Time, start string, durationMinutes int) (*Booking, error) {
		b := &Booking{
			ComplexID:        f.ComplexID,
			CourtID:          f.CourtID,
			ClientID:         f.ClientID,
			Date:             date,
			StartTime:        start,
			DurationMinutes:  durationMinutes,
			Price:            500_000,
			DepositAmount:    150_000,
			Status:           "pending",
			CollectionStatus: CollectionStatusUnpaid,
			RefundStatus:     RefundStatusNone,
		}
		return b, f.Models.Bookings.InsertSafe(ctx, b)
	}

	// Control: an ordinary daytime booking that does not cross midnight. Its
	// span stays inside one day, so the guard must see the block. If this case
	// also lets the booking through, the guard is broken for every booking
	// rather than only the midnight ones, and the finding is broader than F01
	// claims.
	t.Run("control_no_midnight_crossing_is_refused", func(t *testing.T) {
		date := day(10)
		blockCourt(t, date, "22:00", "23:00")

		b, err := book(date, "22:00", 60)
		if got := slots.Add(b.StartTime, b.DurationMinutes); got != "23:00" {
			t.Fatalf("control precondition: the booking ends at %q, want %q", got, "23:00")
		}
		if !errors.Is(err, ErrSlotUnavailable) {
			t.Fatalf("booking 22:00 +60m over a 22:00-23:00 block: got err = %v, want ErrSlotUnavailable "+
				"(the guard is blind beyond the midnight case — the defect is broader than F01 describes)", err)
		}
	})

	// The case under test: 23:00 + 120 minutes against a 23:00-23:59 block.
	// The booking covers the whole block. F01 predicts the guard misses it.
	t.Run("midnight_crossing_booking_over_a_block", func(t *testing.T) {
		date := day(11)
		blockCourt(t, date, "23:00", "23:59")

		b, err := book(date, "23:00", 120)

		// Report the clock reading the column used to store, either way: F02
		// claimed it was "01:00", a bare time of day with no date, and that
		// lossy value is what made the guard's predicate false. Dropping bookings.end_time
		// dropped it; slots.Add is the arithmetic that produced it.
		t.Logf("the clock reading 23:00 + 120 minutes used to be stored as = %q (F02 claims %q)",
			slots.Add(b.StartTime, b.DurationMinutes), "01:00")
		if !b.EndsAt.IsZero() {
			t.Logf("generated span upper (the value the exclusion constraint uses) = %s", b.EndsAt.Format(time.RFC3339))
		}

		if !errors.Is(err, ErrSlotUnavailable) {
			t.Fatalf("booking 23:00 +120m over a 23:00-23:59 block: got err = %v, want ErrSlotUnavailable. "+
				"The in-transaction blocked-slot guard did not see the block: the end wrapped to %q, so "+
				"`start_time < end_time` is false and `date = $2` excludes the following day. F01 REPRODUCES.",
				err, slots.Add(b.StartTime, b.DurationMinutes))
		}
	})
}

// TestBlockedSlotsCannotOverlap pins blocked_slots_no_overlapping_span.
//
// Before it, InsertBlockedSlot did SELECT EXISTS(...overlapping...) and then
// INSERT on the same transaction. Under READ COMMITTED two concurrent requests
// both see no overlap and both insert, and nothing in the schema refuses the
// second one: the only constraint blocked_slots carried was
// blocked_slots_check (start_time < end_time). The owner ends up with
// duplicate maintenance blocks the availability grid subtracts twice, and no
// way to tell which row to delete.
//
// The first subtest goes straight to SQL rather than through the store, so it
// pins the database refusing the overlap rather than the application noticing
// it. That is the difference the migration is for.
func TestBlockedSlotsCannotOverlap(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()
	date := time.Date(2026, time.November, 20, 0, 0, 0, 0, time.UTC)

	insert := func(from, to string) error {
		_, err := f.Pool.Exec(ctx, `
			INSERT INTO blocked_slots (court_id, date, start_time, end_time, reason)
			VALUES ($1, $2, $3::time, $4::time, 'maintenance')`,
			f.CourtID, date, from, to)
		return err
	}

	t.Run("the_database_refuses_the_overlap", func(t *testing.T) {
		if err := insert("10:00", "12:00"); err != nil {
			t.Fatalf("first block 10:00-12:00 should be accepted: %v", err)
		}

		err := insert("11:00", "13:00")
		if err == nil {
			t.Fatal("second block 11:00-13:00 overlaps the first by an hour and was accepted: " +
				"blocked_slots has no exclusion constraint")
		}

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) {
			t.Fatalf("expected a *pgconn.PgError, got %T: %v", err, err)
		}
		if pgErr.Code != sqlStateExclusionViolation {
			t.Fatalf("SQLSTATE = %q (%s), want %q (exclusion_violation)",
				pgErr.Code, pgErr.ConstraintName, sqlStateExclusionViolation)
		}
		if pgErr.ConstraintName != "blocked_slots_no_overlapping_span" {
			t.Errorf("constraint = %q, want blocked_slots_no_overlapping_span", pgErr.ConstraintName)
		}
		t.Logf("refused by %s with SQLSTATE %s", pgErr.ConstraintName, pgErr.Code)
	})

	t.Run("touching_blocks_are_not_overlapping", func(t *testing.T) {
		// Half-open '[)', the same convention bookings.span uses: a block that
		// starts exactly when the previous one ends is adjacent, not overlapping.
		if err := insert("14:00", "15:00"); err != nil {
			t.Fatalf("block 14:00-15:00: %v", err)
		}
		if err := insert("15:00", "16:00"); err != nil {
			t.Fatalf("block 15:00-16:00 only touches the previous one and must be accepted: %v", err)
		}
	})

	t.Run("the_store_still_maps_the_refusal_to_ErrSlotAlreadyBlocked", func(t *testing.T) {
		reason := "maintenance"
		first := &BlockedSlot{
			CourtID: f.CourtID, Date: date,
			StartTime: "18:00", EndTime: "19:00", Reason: &reason,
		}
		if err := f.Models.Courts.InsertBlockedSlot(ctx, first); err != nil {
			t.Fatalf("first store insert: %v", err)
		}

		second := &BlockedSlot{
			CourtID: f.CourtID, Date: date,
			StartTime: "18:30", EndTime: "19:30", Reason: &reason,
		}
		err := f.Models.Courts.InsertBlockedSlot(ctx, second)
		if !errors.Is(err, ErrSlotAlreadyBlocked) {
			t.Fatalf("overlapping store insert: got err = %v, want ErrSlotAlreadyBlocked", err)
		}
	})
}
