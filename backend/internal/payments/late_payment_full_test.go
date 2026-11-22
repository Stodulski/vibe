package payments

import (
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// L19: a payment that lands on a cancelled booking is recorded before it is
// refunded, and what it collected is what an approved payment on the same
// booking collects (collectedOnline): a booking whose checkout charged the
// whole price was paid in full, not a deposit. Recording it as deposit_paid
// left a refunded full payment reading "Seña pagada" to the owner.
func TestALatePaymentOwedARefundRecordsWhatItCollected(t *testing.T) {
	cases := []struct {
		name    string
		deposit int
		want    bookingstore.CollectionStatus
	}{
		{name: "the whole price", deposit: 500_000, want: bookingstore.CollectionStatusFullyPaid},
		{name: "a deposit", deposit: 150_000, want: bookingstore.CollectionStatusDepositPaid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, withCheckoutRow := range []bool{true, false} {
				f := newFixture(t)
				booking, payment := paidBooking(uuid.New())
				booking.Status = "cancelled"
				booking.CollectionStatus = bookingstore.CollectionStatusUnpaid
				booking.DepositAmount = tc.deposit
				payment.Status = "unpaid"
				payment.MPPaymentID = nil
				if withCheckoutRow {
					f.payments.byBooking = payment
				}
				f.payments.claimErr = errDatabase // stop after the payment is recorded

				mpPayment := &mp.Payment{ID: 123, Status: "approved", TransactionAmount: float64(tc.deposit) / 100}
				_ = f.service.refundCancelledBookingPayment(t.Context(), booking, mpPayment, "mp-123", latePaymentReason)

				var row *paymentstore.Payment
				committed := f.payments.confirmedBooking
				if withCheckoutRow {
					row = f.payments.confirmed
				} else {
					row = f.payments.inserted
					committed = f.payments.insertedBooking
				}
				if committed == nil || row == nil {
					t.Fatalf("checkout row %v: the payment must be recorded before any claim", withCheckoutRow)
				}
				if committed.CollectionStatus != tc.want {
					t.Errorf("checkout row %v: booking collection_status = %q, want %q", withCheckoutRow, committed.CollectionStatus, tc.want)
				}
				if row.Status != string(tc.want) {
					t.Errorf("checkout row %v: payment status = %q, want %q", withCheckoutRow, row.Status, tc.want)
				}
			}
		})
	}
}
