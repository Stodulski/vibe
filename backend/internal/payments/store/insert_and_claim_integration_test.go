//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// A duplicate payment is recorded together with its refund claim: the row reads
// refund_pending and a pending attempt row is committed with it, which is what the
// retry job brings the money back from.
func TestInsertAndClaimRefundCommitsTheRowWithItsClaim(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	booking := f.CreateBooking(t, datatest.BookingOptions{Status: "confirmed"})
	mpPaymentID := "mp-" + uuid.NewString()

	payment := &paymentstore.Payment{
		BookingID: booking.ID, ComplexID: f.ComplexID,
		Amount: 150_000, ServiceFee: 7_500,
		Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &mpPaymentID,
	}
	claim, err := f.Stores.Payments.InsertAndClaimRefund(ctx, payment)
	if err != nil {
		t.Fatalf("InsertAndClaimRefund: %v", err)
	}
	if claim.PaymentID != payment.ID || claim.RefundCentavos != 157_500 || claim.MPPaymentID != mpPaymentID {
		t.Errorf("the claim must be for the whole of the new row; got %+v for row %s", claim, payment.ID)
	}
	if status, _ := f.ReadPaymentState(t, payment.ID); status != "refund_pending" {
		t.Errorf("the new row must read refund_pending; got %q", status)
	}
	var attempts int
	if err := f.DB.QueryRow(context.Background(),
		`SELECT count(*) FROM failed_refunds WHERE payment_id = $1 AND status = 'pending'`, payment.ID,
	).Scan(&attempts); err != nil {
		t.Fatalf("reading the attempt rows: %v", err)
	}
	if attempts != 1 {
		t.Errorf("one pending attempt row must be committed with the claim; got %d", attempts)
	}
}

// When the claim is refused, the insert is rolled back with it, so MercadoPago's
// retry of the event finds no row for the payment and starts over.
func TestInsertAndClaimRefundLeavesNoRowWhenTheClaimIsRefused(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	booking := f.CreateBooking(t, datatest.BookingOptions{Status: "confirmed"})

	// No MercadoPago id, so the claim refuses: there is nothing to refund against.
	payment := &paymentstore.Payment{
		BookingID: booking.ID, ComplexID: f.ComplexID,
		Amount: 150_000, ServiceFee: 7_500,
		Method: "mercadopago", Status: "deposit_paid",
	}
	if _, err := f.Stores.Payments.InsertAndClaimRefund(ctx, payment); err == nil {
		t.Fatal("a payment with no MercadoPago id cannot be claimed for refund")
	}
	var rows int
	if err := f.DB.QueryRow(context.Background(),
		`SELECT count(*) FROM payments WHERE booking_id = $1`, booking.ID,
	).Scan(&rows); err != nil {
		t.Fatalf("reading the payment rows: %v", err)
	}
	if rows != 0 {
		t.Errorf("a refused claim must leave no payment row behind; got %d", rows)
	}
}
