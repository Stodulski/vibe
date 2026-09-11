//go:build integration

package data

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// This file replaces version_bypass_integration_test.go.
//
// That test asked an open question: when a bulk cancel and a concurrent status
// write race, is it `bookings.version` or the bookings_forbid_status_reversal
// trigger that refuses the write? It classified whatever came back and only
// failed if nothing did. The answer it produced was the trigger, every time —
// CancelFutureByComplex never touched `version`, so the stale version still
// matched and the row-count check could not fire. The schema removed the
// column on the strength of that measurement.
//
// So the question is closed and these tests assert the answer instead: the
// trigger, alone, with no optimistic-concurrency counter anywhere in the
// picture, refuses the write and names the rule that refused it. If the trigger
// is ever dropped or its WHEN clause narrowed, these fail rather than quietly
// reporting that "something else" caught it — which is the whole reason the
// column could be removed.

// TestTerminalStatusReentryIsRefusedByTheTrigger drives the exact scenario the
// removed version column was supposed to guard.
//
// An owner bulk-cancels the complex's future bookings (CancelFutureByComplex,
// raw SQL, no per-row read) while a payment confirmation is in flight on one of
// them holding a Booking struct read before the cancel. The confirmation writes
// `confirmed` over `cancelled`. Nothing in Go compares anything: the refusal
// comes from inside the database, as SQLSTATE 23514 carrying the rule name.
func TestTerminalStatusReentryIsRefusedByTheTrigger(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	booking := seedFutureBooking(t, f, "09:00", "10:00")

	// The in-flight writer reads the booking before the bulk cancel commits.
	inFlight, err := f.Models.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the booking before the cancel: %v", err)
	}
	if inFlight.Status != "pending" {
		t.Fatalf("seeded status = %q, want pending", inFlight.Status)
	}

	if err := f.Models.Bookings.CancelFutureByComplex(ctx, f.ComplexID); err != nil {
		t.Fatalf("CancelFutureByComplex: %v", err)
	}

	// The stale writer commits its decision.
	inFlight.Status = "confirmed"
	inFlight.CollectionStatus = CollectionStatusFullyPaid
	err = f.Models.Bookings.Update(ctx, inFlight)

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("Update after the bulk cancel returned %#v, want a *pgconn.PgError "+
			"from bookings_forbid_status_reversal. A nil error here means a cancelled "+
			"booking was written back to confirmed and nothing stopped it.", err)
	}
	if pgErr.Code != "23514" {
		t.Errorf("SQLSTATE = %q, want 23514 (check_violation)", pgErr.Code)
	}
	if pgErr.ConstraintName != "bookings_status_no_terminal_reentry" {
		t.Errorf("constraint = %q, want bookings_status_no_terminal_reentry", pgErr.ConstraintName)
	}

	final, err := f.Models.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("final read: %v", err)
	}
	if final.Status != "cancelled" {
		t.Errorf("final status = %q, want cancelled", final.Status)
	}
	if final.CollectionStatus != CollectionStatusUnpaid {
		t.Errorf("final collection_status = %q, want unpaid — the refused write must "+
			"have left nothing behind", final.CollectionStatus)
	}
	if final.RefundStatus != RefundStatusNone {
		t.Errorf("final refund_status = %q, want none", final.RefundStatus)
	}
}

// TestCollectionStatusCannotReturnToUnpaid pins the trigger's other rule, for
// the same reason: it is the second half of what is left holding this table now
// that the version counter is gone, and it is enforced by the same function
// under the same WHEN clause, so a change that silences one silences both.
//
// The payment_status split moved this rule from payment_status onto collection_status.
// The translation is exact rather than merely similar, and the second half of
// the test is what says so: the row keeps its refund status through the refused
// write, so the rule is refusing the erasure of collected money and not
// incidentally refusing anything on the refund axis. The other direction — a
// (unpaid, full) row being written back to (unpaid, none), which the old enum
// refused as refunded -> unpaid — is unreachable rather than unguarded, and
// TestARefundCannotExistWithoutMoneyHavingBeenCollected in
// schema_constraints_integration_test.go is what makes it so.
func TestCollectionStatusCannotReturnToUnpaid(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	booking := seedFutureBooking(t, f, "11:00", "12:00")

	paid, err := f.Models.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	paid.CollectionStatus = CollectionStatusDepositPaid
	paid.DepositAmount = 100_000
	if err := f.Models.Bookings.Update(ctx, paid); err != nil {
		t.Fatalf("taking the deposit: %v", err)
	}
	// A refund claim goes out on that deposit. The row now carries something on
	// both axes, which is the state the single enum could not hold at all.
	paid.RefundStatus = RefundStatusPending
	if err := f.Models.Bookings.Update(ctx, paid); err != nil {
		t.Fatalf("claiming the refund: %v", err)
	}

	paid.CollectionStatus = CollectionStatusUnpaid
	err = f.Models.Bookings.Update(ctx, paid)

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("Update back to unpaid returned %#v, want a *pgconn.PgError. A nil "+
			"error here means the record of money already taken was erased.", err)
	}
	if pgErr.Code != "23514" {
		t.Errorf("SQLSTATE = %q, want 23514 (check_violation)", pgErr.Code)
	}
	if pgErr.ConstraintName != "bookings_collection_status_no_return_to_unpaid" {
		t.Errorf("constraint = %q, want bookings_collection_status_no_return_to_unpaid", pgErr.ConstraintName)
	}

	final, err := f.Models.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("final read: %v", err)
	}
	if final.CollectionStatus != CollectionStatusDepositPaid {
		t.Errorf("final collection_status = %q, want deposit_paid", final.CollectionStatus)
	}
	if final.RefundStatus != RefundStatusPending {
		t.Errorf("final refund_status = %q, want pending — the refused write must have "+
			"left the refund axis exactly where it was", final.RefundStatus)
	}
}

// seedFutureBooking commits one pending, unpaid booking a week out, which is
// what CancelFutureByComplex's `date >= CURRENT_DATE` predicate selects. The
// date is built in UTC and pinned to midnight so the row lands on exactly one
// local day whatever hour the suite runs at.
func seedFutureBooking(t *testing.T, f *testFixture, startTime, endTime string) *Booking {
	t.Helper()

	d := time.Now().In(time.UTC).AddDate(0, 0, 7)
	date := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)

	b := &Booking{
		ComplexID:        f.ComplexID,
		CourtID:          f.CourtID,
		ClientID:         f.ClientID,
		Date:             date,
		StartTime:        startTime,
		DurationMinutes:  60,
		Price:            400_000,
		DepositAmount:    0,
		Status:           "pending",
		CollectionStatus: CollectionStatusUnpaid,
		RefundStatus:     RefundStatusNone,
	}
	if err := f.Models.Bookings.InsertSafe(context.Background(), b); err != nil {
		t.Fatalf("seeding the booking: %v", err)
	}
	return b
}
