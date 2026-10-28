//go:build integration

package store_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// TestConfirmInTxRefusesACollectionAboveTheBalanceUnderTheLock is the counter
// double-collection race. ConfirmPayment checks the amount against the booking
// it read before any lock is taken, so two owners collecting at once can both
// pass that check. The second collection reaches ConfirmInTx after the first
// has committed, and ConfirmInTx has to judge it on the row as it now stands:
// refused, with nothing written.
func TestConfirmInTxRefusesACollectionAboveTheBalanceUnderTheLock(t *testing.T) {
	for _, status := range []bookingstore.BookingStatus{"confirmed", "completed"} {
		for _, amount := range []int{1, 500_000} {
			t.Run(fmt.Sprintf("%s/amount=%d", status, amount), func(t *testing.T) {
				f := datatest.Isolated(t)
				ctx := context.Background()

				b := f.CreateBooking(t, datatest.BookingOptions{
					Status:           status,
					CollectionStatus: bookingstore.CollectionStatusUnpaid,
					Price:            500_000,
					DepositAmount:    150_000,
				})

				// The copy the racing ConfirmPayment read before the first collection
				// committed. In memory it still says the booking owes the full price.
				stale := *b

				// The first collection takes the full balance, the way ConfirmPayment
				// does: the in-memory booking is moved first, then written through the store.
				b.RecordCounterPayment(b.Price)
				first := &paymentstore.Payment{
					BookingID: b.ID,
					ComplexID: f.ComplexID,
					Amount:    b.Price,
					Method:    "cash",
					Status:    "fully_paid",
				}
				if err := f.Stores.Payments.InsertAndConfirmBooking(f.Scoped(ctx), first, b); err != nil {
					t.Fatalf("the full-balance collection must be recorded: %v", err)
				}

				// The second collection, from the stale copy.
				stale.RecordCounterPayment(amount)
				second := &paymentstore.Payment{
					BookingID: b.ID,
					ComplexID: f.ComplexID,
					Amount:    amount,
					Method:    "cash",
					Status:    "fully_paid",
				}
				err := f.Stores.Payments.InsertAndConfirmBooking(f.Scoped(ctx), second, &stale)
				if !errors.Is(err, paymentstore.ErrAmountExceedsBalance) {
					t.Fatalf("a collection above the balance under the lock must be refused with ErrAmountExceedsBalance; got %v", err)
				}

				var over *paymentstore.AmountExceedsBalanceError
				if !errors.As(err, &over) || over.Balance != 0 {
					t.Errorf("the refusal must name the balance the locked row still owed (0); got %v", err)
				}

				var payments int
				if err := f.DB.QueryRow(ctx,
					`SELECT COUNT(*) FROM payments WHERE booking_id = $1`, b.ID).Scan(&payments); err != nil {
					t.Fatalf("counting payments: %v", err)
				}
				if payments != 1 {
					t.Errorf("a refused collection must write no payment row; found %d, want only the first", payments)
				}

				_, collection, _ := f.ReadBookingState(t, b.ID)
				if collection != string(bookingstore.CollectionStatusFullyPaid) {
					t.Errorf("a refused collection must leave the booking fully paid; got collection_status=%q", collection)
				}
			})
		}
	}
}

// TestConfirmInTxAcceptsACollectionThatFitsTheLockedBalance is the other side of
// the check: a payment that exactly covers what the row still owes goes through,
// so the refusal above is for what is over the balance and nothing else.
func TestConfirmInTxAcceptsACollectionThatFitsTheLockedBalance(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	// A deposit already on the row: 150_000 of a 500_000 price leaves 350_000 owed.
	b := f.CreateBooking(t, datatest.BookingOptions{
		Status:           "confirmed",
		CollectionStatus: bookingstore.CollectionStatusDepositPaid,
		Price:            500_000,
		DepositAmount:    150_000,
	})

	b.RecordCounterPayment(350_000)
	payment := &paymentstore.Payment{
		BookingID: b.ID,
		ComplexID: f.ComplexID,
		Amount:    350_000,
		Method:    "transfer",
		Status:    "fully_paid",
	}
	if err := f.Stores.Payments.InsertAndConfirmBooking(f.Scoped(ctx), payment, b); err != nil {
		t.Fatalf("a collection equal to the locked balance must be recorded: %v", err)
	}

	_, collection, _ := f.ReadBookingState(t, b.ID)
	if collection != string(bookingstore.CollectionStatusFullyPaid) {
		t.Errorf("the booking must read fully paid; got collection_status=%q", collection)
	}
}

// TestRecordingAPaymentForACancelledBookingIgnoresItsBalance pins the auto-refund
// path the balance check has to leave alone. A booking cancelled after it was
// fully paid owes nothing, yet the late payment for it still has to be recorded so
// the money can be sent back against the row it belongs to. The check only
// measures confirmations and completed-booking collections.
func TestRecordingAPaymentForACancelledBookingIgnoresItsBalance(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	b := f.CreateBooking(t, datatest.BookingOptions{
		Status:           "confirmed",
		CollectionStatus: bookingstore.CollectionStatusFullyPaid,
		Price:            500_000,
		DepositAmount:    150_000,
	})
	f.MarkStatus(t, b.ID, "cancelled")

	// refundBookingWhoseSlotIsGone and recordPaymentOwedARefund set the in-memory
	// status to cancelled before writing the payment.
	b.Status = "cancelled"
	payment := &paymentstore.Payment{
		BookingID:  b.ID,
		ComplexID:  f.ComplexID,
		Amount:     500_000,
		ServiceFee: 10_500,
		Method:     "mercadopago",
		Status:     "deposit_paid",
	}
	if err := f.Stores.Payments.InsertAndConfirmBooking(f.Scoped(ctx), payment, b); err != nil {
		t.Fatalf("recording the payment owed back on a cancelled booking must succeed: %v", err)
	}

	var payments int
	if err := f.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments WHERE booking_id = $1`, b.ID).Scan(&payments); err != nil {
		t.Fatalf("counting payments: %v", err)
	}
	if payments != 1 {
		t.Errorf("the payment owed back must be recorded; found %d rows", payments)
	}
}
