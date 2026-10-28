//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// The owner's edit is built from a read taken outside any lock, so a write that
// rebuilds the row from it must land only while the row is still the one read.
// The read and the conditional write are two statements; only the real database
// can show that the updated_at predicate holds across them.
func TestUpdateIfUnchangedAppliesToTheRowTheOwnerRead(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	booking := f.NewBooking(datatest.BookingOptions{Status: "confirmed", CollectionStatus: bookingstore.CollectionStatusUnpaid})
	if err := f.Stores.Bookings.InsertSafe(ctx, booking); err != nil {
		t.Fatalf("inserting the booking: %v", err)
	}

	snapshot, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the booking: %v", err)
	}

	note := "arrived early"
	snapshot.Notes = &note
	if err := f.Stores.Bookings.UpdateIfUnchanged(ctx, snapshot); err != nil {
		t.Fatalf("an edit of an unchanged row must land: %v", err)
	}

	stored, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the booking back: %v", err)
	}
	if stored.Notes == nil || *stored.Notes != note {
		t.Errorf("want notes %q stored; got %v", note, stored.Notes)
	}
}

// A cancellation that commits after the owner read the booking must not be
// reverted by the owner's stale write. The conditional write matches no row, and
// the cancellation, with its refund intent, stands.
//
// This test needs Shared, not Isolated: the cancellation has to commit in its own
// transaction, as a second request's does. The set_updated_at trigger stamps
// NOW(), the transaction's start time, so inside one transaction both writes
// carry the same stamp and the conditional write could not tell them apart.
func TestUpdateIfUnchangedRefusesARowThatMovedAfterTheRead(t *testing.T) {
	f := datatest.Shared(t)
	ctx := f.Scoped(context.Background())

	booking := f.NewBooking(datatest.BookingOptions{Status: "confirmed", CollectionStatus: bookingstore.CollectionStatusUnpaid})
	if err := f.Stores.Bookings.InsertSafe(ctx, booking); err != nil {
		t.Fatalf("inserting the booking: %v", err)
	}

	snapshot, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the booking: %v", err)
	}

	// The client cancels after the owner's read and records a refund intent.
	current, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the booking for the cancellation: %v", err)
	}
	intent := time.Now()
	current.Status = bookingstore.BookingStatus("cancelled")
	current.RefundIntentAt = &intent
	if err := f.Stores.Bookings.Update(ctx, current); err != nil {
		t.Fatalf("cancelling the booking: %v", err)
	}

	snapshot.Status = bookingstore.BookingStatus("no_show")
	snapshot.RefundIntentAt = nil
	err = f.Stores.Bookings.UpdateIfUnchanged(ctx, snapshot)
	if !errors.Is(err, data.ErrEditConflict) {
		t.Fatalf("a stale edit must answer data.ErrEditConflict; got %v", err)
	}

	stored, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the booking back: %v", err)
	}
	if stored.Status != "cancelled" || stored.RefundIntentAt == nil {
		t.Errorf("the cancellation was reverted: status %q, refund intent %v", stored.Status, stored.RefundIntentAt)
	}
}

// The counter payment taken when an owner creates a booking commits with the
// booking. A payment that cannot be written must leave no booking behind, so the
// owner who was refused is not left with a confirmed booking that holds the slot.
func TestInsertSafeWithPaymentRollsBackTheBookingWhenThePaymentFails(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	booking := f.NewBooking(datatest.BookingOptions{Status: "confirmed", CollectionStatus: bookingstore.CollectionStatusFullyPaid})
	// "bogus" is not a payment method the enum holds. The booking row is written
	// first inside the transaction, so the failure has to roll it back.
	err := f.Stores.Bookings.InsertSafeWithPayment(ctx, booking, &bookingstore.CounterPayment{
		Amount: booking.Price, Method: "bogus", Status: "fully_paid",
	})
	if err == nil {
		t.Fatal("a payment with an unknown method must fail the write")
	}

	var stored int
	if err := f.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM bookings WHERE id = $1`, booking.ID,
	).Scan(&stored); err != nil {
		t.Fatalf("counting the booking: %v", err)
	}
	if stored != 0 {
		t.Errorf("the booking must not survive its failed payment; found %d row(s)", stored)
	}

	retry := f.NewBooking(datatest.BookingOptions{Status: "confirmed", CollectionStatus: bookingstore.CollectionStatusFullyPaid})
	if err := f.Stores.Bookings.InsertSafe(ctx, retry); err != nil {
		t.Errorf("the slot must be free after the rolled-back write: %v", err)
	}
}

// A cancellation that still owes a refund carries refund_intent_at until the refund
// pipeline claims it. An owner's edit that touches nothing about the refund must
// not clear it: the edit is written from the booking the owner read, so that read
// has to carry the intent.
func TestOwnerEditKeepsARefundIntentTheBookingStillOwes(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	booking := f.NewBooking(datatest.BookingOptions{Status: "confirmed", CollectionStatus: bookingstore.CollectionStatusDepositPaid})
	if err := f.Stores.Bookings.InsertSafe(ctx, booking); err != nil {
		t.Fatalf("inserting the booking: %v", err)
	}

	cancelled, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the booking: %v", err)
	}
	intent := time.Now()
	cancelled.Status = bookingstore.BookingStatus("cancelled")
	cancelled.RefundIntentAt = &intent
	if err := f.Stores.Bookings.Update(ctx, cancelled); err != nil {
		t.Fatalf("cancelling the booking: %v", err)
	}

	snapshot, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the cancelled booking: %v", err)
	}
	note := "devolver por transferencia"
	snapshot.Notes = &note
	if err := f.Stores.Bookings.UpdateIfUnchanged(ctx, snapshot); err != nil {
		t.Fatalf("a notes edit must land: %v", err)
	}

	stored, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the booking back: %v", err)
	}
	if stored.RefundIntentAt == nil {
		t.Error("an owner's notes edit cleared the refund intent the cancellation recorded")
	}
}

// The happy path: the booking and its counter payment are both written, and the
// payment is tied to the booking it paid for.
func TestInsertSafeWithPaymentWritesTheBookingAndItsPayment(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	booking := f.NewBooking(datatest.BookingOptions{Status: "confirmed", CollectionStatus: bookingstore.CollectionStatusFullyPaid})
	if err := f.Stores.Bookings.InsertSafeWithPayment(ctx, booking, &bookingstore.CounterPayment{
		Amount: booking.Price, Method: "cash", Status: "fully_paid",
	}); err != nil {
		t.Fatalf("a booking with its counter payment must be written: %v", err)
	}

	var payments int
	if err := f.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM payments WHERE booking_id = $1 AND amount = $2 AND method = 'cash'`,
		booking.ID, booking.Price,
	).Scan(&payments); err != nil {
		t.Fatalf("counting the payment: %v", err)
	}
	if payments != 1 {
		t.Errorf("want exactly one counter payment for the booking; found %d", payments)
	}
}

// Marking a cancelled booking's cash balance as owed moves refund_status and
// nothing else, so an owner edit that landed after the refund path read the
// booking survives it. A row already past 'none' is left where it is.
func TestMarkManualBalanceOwedMovesOnlyTheRefundStatus(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	booking := f.NewBooking(datatest.BookingOptions{Status: "cancelled", CollectionStatus: bookingstore.CollectionStatusDepositPaid})
	if err := f.Stores.Bookings.InsertSafe(ctx, booking); err != nil {
		t.Fatalf("inserting the booking: %v", err)
	}

	// The refund path's snapshot, taken before the owner's edit below.
	snapshot, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the booking: %v", err)
	}

	edited, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the booking for the edit: %v", err)
	}
	note := "client called to cancel"
	edited.Notes = &note
	if err := f.Stores.Bookings.Update(ctx, edited); err != nil {
		t.Fatalf("the owner's edit: %v", err)
	}

	if err := f.Stores.Bookings.MarkManualBalanceOwed(ctx, snapshot); err != nil {
		t.Fatalf("marking the balance owed: %v", err)
	}

	stored, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the booking back: %v", err)
	}
	if stored.RefundStatus != bookingstore.RefundStatusPartial {
		t.Errorf("want refund_status %q; got %q", bookingstore.RefundStatusPartial, stored.RefundStatus)
	}
	if stored.Notes == nil || *stored.Notes != note {
		t.Errorf("the owner's edit must survive; want notes %q, got %v", note, stored.Notes)
	}

	stored.RefundStatus = bookingstore.RefundStatusFull
	if err := f.Stores.Bookings.Update(ctx, stored); err != nil {
		t.Fatalf("settling the refund: %v", err)
	}
	if err := f.Stores.Bookings.MarkManualBalanceOwed(ctx, snapshot); err != nil {
		t.Fatalf("marking a settled booking: %v", err)
	}
	again, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the booking again: %v", err)
	}
	if again.RefundStatus != bookingstore.RefundStatusFull {
		t.Errorf("a row past 'none' must be left alone; got %q", again.RefundStatus)
	}
}
