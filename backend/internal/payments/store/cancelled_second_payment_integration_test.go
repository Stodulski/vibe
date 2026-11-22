//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// A cancelled booking whose first MercadoPago payment is already refunded takes
// a second approved payment the way the webhook records it: a row of its own,
// written with the refund-intent marker, and claimed per payment. The claim on
// that row goes through and refunds it in full, the first payment's row keeps
// its own id and refund, a claim against the first row is still refused as
// already refunded, and the same MercadoPago payment cannot be recorded twice.
func TestASecondPaymentOnACancelledBookingIsClaimedOnItsOwnRow(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	booking := f.CreateBooking(t, datatest.BookingOptions{
		Status: "cancelled", CollectionStatus: bookingstore.CollectionStatusDepositPaid,
	})

	mpFirst := "mp-" + uuid.NewString()
	first := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpFirst)
	firstClaim, err := f.Stores.Payments.ClaimRefundWithCause(ctx, first.ID, paymentstore.RefundCauseLatePayment)
	if err != nil {
		t.Fatalf("claiming the first refund: %v", err)
	}
	if _, err := f.Stores.Payments.RecordRefundSuccess(ctx, *firstClaim, 0); err != nil {
		t.Fatalf("recording the first refund: %v", err)
	}

	current, err := f.Stores.Bookings.GetByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the booking: %v", err)
	}
	now := time.Now()
	current.RefundIntentAt = &now
	mpSecond := "mp-" + uuid.NewString()
	second := &paymentstore.Payment{
		BookingID: booking.ID, ComplexID: f.ComplexID, Amount: 150_000, ServiceFee: 7_500,
		Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &mpSecond,
	}
	if err := f.Stores.Payments.InsertAndConfirmBooking(ctx, second, current); err != nil {
		t.Fatalf("recording the second payment: %v", err)
	}

	claim, err := f.Stores.Payments.ClaimRefundWithCause(ctx, second.ID, paymentstore.RefundCauseLatePayment)
	if err != nil {
		t.Fatalf("the second payment's own claim must go through; got %v", err)
	}
	if claim.MPPaymentID != mpSecond || claim.RefundCentavos != 157_500 {
		t.Errorf("the claim must refund the second payment in full; got mp=%s amount=%d", claim.MPPaymentID, claim.RefundCentavos)
	}
	if _, err := f.Stores.Payments.RecordRefundSuccess(ctx, *claim, 0); err != nil {
		t.Fatalf("recording the second refund: %v", err)
	}

	if _, err := f.Stores.Payments.ClaimRefund(ctx, first.ID); !errors.Is(err, paymentstore.ErrAlreadyRefunded) {
		t.Errorf("the first payment stays refunded once; got %v", err)
	}
	rows, err := f.Stores.Payments.ListByBookingID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("listing the payments: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want one row per payment; got %d", len(rows))
	}
	for _, r := range rows {
		if r.Status != "refunded" || r.RefundAmount != 157_500 {
			t.Errorf("row %s must read refunded in full; got status=%s refund=%d", *r.MPPaymentID, r.Status, r.RefundAmount)
		}
	}
	if *rows[0].MPPaymentID != mpFirst {
		t.Errorf("the first row must keep its own payment id; got %s", *rows[0].MPPaymentID)
	}

	again := &paymentstore.Payment{
		BookingID: booking.ID, ComplexID: f.ComplexID, Amount: 150_000, ServiceFee: 7_500,
		Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &mpSecond,
	}
	if _, err := f.Stores.Payments.InsertAndClaimRefundWithCause(ctx, again, paymentstore.RefundCauseLatePayment); err == nil {
		t.Error("the same MercadoPago payment must not be recorded, nor refunded, twice")
	}
}
