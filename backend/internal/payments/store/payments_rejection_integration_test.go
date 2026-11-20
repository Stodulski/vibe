//go:build integration

package store_test

import (
	"context"
	"testing"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// A rejected attempt's webhook reads the payment row, then writes MercadoPago's
// decline onto it. When the booking's approved payment settled that same row in
// between, the write used to land anyway: the row kept its status but its
// mp_payment_id now named the rejected attempt, so the refund of that booking
// would be sent against a payment that never captured anything. The rejection
// is recorded only on a row that is still unpaid.
func TestRecordRejectionLeavesASettledPaymentAlone(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	approved := "mp-approved"
	payment := f.CreatePayment(t, booking.ID, booking.DepositAmount, 7_500, &approved)

	// The webhook's snapshot of the row: read before the approval settled it.
	detail := "cc_rejected_insufficient_amount"
	rejected := "mp-rejected"
	payment.Status = "unpaid"
	payment.MPPaymentID = &rejected
	payment.StatusDetail = &detail
	written, err := f.Stores.Payments.RecordRejection(ctx, payment)
	if err != nil {
		t.Fatalf("RecordRejection: %v", err)
	}
	if written {
		t.Error("a settled payment must not take a rejection")
	}

	got, err := f.Stores.Payments.GetByBookingID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the payment back: %v", err)
	}
	if got.MPPaymentID == nil || *got.MPPaymentID != approved {
		t.Errorf("the settled row must keep the approved payment's id; got %v", got.MPPaymentID)
	}
	if got.Status != "deposit_paid" {
		t.Errorf("the settled row must keep its status; got %q", got.Status)
	}
	if got.StatusDetail != nil {
		t.Errorf("the settled row must not take the decline reason; got %q", *got.StatusDetail)
	}
}

// The checkout row a pending booking is still waiting on does take it.
func TestRecordRejectionRecordsTheReasonOnAnUnpaidCheckoutRow(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	booking := f.CreateBooking(t, datatest.BookingOptions{
		Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid, Public: true,
	})
	payment := f.CreatePayment(t, booking.ID, booking.DepositAmount, 7_500, nil)
	if _, err := f.DB.Exec(ctx, `UPDATE payments SET status = 'unpaid' WHERE id = $1`, payment.ID); err != nil {
		t.Fatalf("marking the checkout row unpaid: %v", err)
	}

	detail := "cc_rejected_insufficient_amount"
	rejected := "mp-rejected"
	payment.Status = "unpaid"
	payment.MPPaymentID = &rejected
	payment.StatusDetail = &detail
	written, err := f.Stores.Payments.RecordRejection(ctx, payment)
	if err != nil {
		t.Fatalf("RecordRejection: %v", err)
	}
	if !written {
		t.Fatal("an unpaid checkout row must take the rejection")
	}

	got, err := f.Stores.Payments.GetByBookingID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reading the payment back: %v", err)
	}
	if got.Status != "unpaid" {
		t.Errorf("a rejection must not move the row's status; got %q", got.Status)
	}
	if got.MPPaymentID == nil || *got.MPPaymentID != "mp-rejected" {
		t.Errorf("want the rejected attempt's id recorded; got %v", got.MPPaymentID)
	}
	if got.StatusDetail == nil || *got.StatusDetail != detail {
		t.Errorf("want the decline reason recorded; got %v", got.StatusDetail)
	}
}
