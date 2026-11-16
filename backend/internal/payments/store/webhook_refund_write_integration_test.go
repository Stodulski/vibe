//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// The webhook reads a payment row, asks MercadoPago about it, and only then
// records the refund MercadoPago reports. A ClaimRefund that commits in between
// moves the row to refund_pending and owns the refund from then on, so the
// webhook's write from its stale copy must not land: it would overwrite the
// claim's status, and the claim's recorder would find nothing left to record.
func TestTheWebhookRefundWriteLeavesAClaimedPaymentAlone(t *testing.T) {
	f := datatest.Shared(t)
	ctx := f.Scoped(context.Background())

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	mpPaymentID := "mp-" + uuid.NewString()
	payment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)

	// The webhook's copy, read before the claim.
	stale := *payment

	if _, err := f.Stores.Payments.ClaimRefund(ctx, payment.ID); err != nil {
		t.Fatalf("ClaimRefund: %v", err)
	}

	stale.RefundAmount = 50_000
	written, err := f.Stores.Payments.UpdateUnlessRefundPending(ctx, &stale)
	if err != nil {
		t.Fatalf("UpdateUnlessRefundPending: %v", err)
	}
	if written {
		t.Error("the write must report that it wrote nothing")
	}

	stale.Status = "refunded"
	stale.RefundAmount = 157_500
	cancelled := *booking
	cancelled.Status = bookingstore.BookingStatusCancelled
	written, err = f.Stores.Payments.RecordWebhookRefund(ctx, &stale, &cancelled)
	if err != nil {
		t.Fatalf("RecordWebhookRefund: %v", err)
	}
	if written {
		t.Error("the write must report that it wrote nothing")
	}

	status, refunded := f.ReadPaymentState(t, payment.ID)
	if status != "refund_pending" || refunded != 0 {
		t.Errorf("the claim's row must be untouched; got status %q refund_amount %d", status, refunded)
	}
	if bookingStatus, _, _ := f.ReadBookingState(t, booking.ID); bookingStatus != string(booking.Status) {
		t.Errorf("the booking must not be written when the payment was not; got %q", bookingStatus)
	}
}

// With no claim in the way both writes land as Update and ConfirmWebhookPayment do.
func TestTheWebhookRefundWriteRecordsAnUnclaimedPayment(t *testing.T) {
	f := datatest.Shared(t)
	ctx := f.Scoped(context.Background())

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	mpPaymentID := "mp-" + uuid.NewString()
	payment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)

	payment.RefundAmount = 50_000
	written, err := f.Stores.Payments.UpdateUnlessRefundPending(ctx, payment)
	if err != nil || !written {
		t.Fatalf("want the partial refund written; written=%v err=%v", written, err)
	}
	if status, refunded := f.ReadPaymentState(t, payment.ID); status != "deposit_paid" || refunded != 50_000 {
		t.Errorf("want deposit_paid with 50_000 refunded; got %q %d", status, refunded)
	}

	payment.Status = "refunded"
	payment.RefundAmount = 157_500
	booking.Status = bookingstore.BookingStatusCancelled
	written, err = f.Stores.Payments.RecordWebhookRefund(ctx, payment, booking)
	if err != nil || !written {
		t.Fatalf("want the full refund written; written=%v err=%v", written, err)
	}
	if status, refunded := f.ReadPaymentState(t, payment.ID); status != "refunded" || refunded != 157_500 {
		t.Errorf("want refunded with 157_500; got %q %d", status, refunded)
	}
	if bookingStatus, _, _ := f.ReadBookingState(t, booking.ID); bookingStatus != string(bookingstore.BookingStatusCancelled) {
		t.Errorf("want the booking cancelled with the refund; got %q", bookingStatus)
	}
}
