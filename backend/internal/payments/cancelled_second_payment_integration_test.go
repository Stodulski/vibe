//go:build integration

package payments

import (
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// Against the real stores: a second approved payment on a cancelled booking
// whose first payment is already refunded commits a refund claim of its own,
// on a row of its own, and leaves the first payment's row as it was. It used
// to overwrite the first row's payment id and have its claim refused as
// already refunded, which left the second payment captured with no claim.
func TestASecondPaymentOnACancelledBookingCommitsItsOwnClaim(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx := f.Scoped(t.Context())
	created := f.CreateBooking(t, datatest.BookingOptions{
		Status: "cancelled", CollectionStatus: bookingstore.CollectionStatusDepositPaid,
	})

	mpFirst := "mp-" + uuid.NewString()
	first := f.CreatePayment(t, created.ID, 150_000, 7_500, &mpFirst)
	firstClaim, err := f.Stores.Payments.ClaimRefund(ctx, first.ID)
	if err != nil {
		t.Fatalf("claiming the first refund: %v", err)
	}
	if _, err := f.Stores.Payments.RecordRefundSuccess(ctx, *firstClaim, 0); err != nil {
		t.Fatalf("recording the first refund: %v", err)
	}

	booking, err := f.Stores.Bookings.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("reading the booking: %v", err)
	}
	mpSecond := "mp-" + uuid.NewString()
	paid := &mp.Payment{ID: 2, Status: "approved", TransactionAmount: 1_575}
	if err := f.service.refundCancelledBookingPayment(ctx, booking, paid, mpSecond, paymentstore.RefundCauseLatePayment); err != nil {
		t.Fatalf("a committed claim hands the money to the refund queue, not an error; got %v", err)
	}

	stillFirst, err := f.Stores.Payments.GetByMPPaymentID(ctx, mpFirst)
	if err != nil || stillFirst.ID != first.ID || stillFirst.Status != "refunded" {
		t.Fatalf("the first payment's row must keep its id and refund; got %+v, %v", stillFirst, err)
	}
	second, err := f.Stores.Payments.GetByMPPaymentID(ctx, mpSecond)
	if err != nil {
		t.Fatalf("the second payment must be recorded on a row of its own; got %v", err)
	}
	if second.ID == first.ID {
		t.Fatal("the second payment must not be written over the first payment's row")
	}
	if second.Status != "refund_pending" {
		t.Errorf("the second payment's refund must be claimed and queued; got %q", second.Status)
	}
}
