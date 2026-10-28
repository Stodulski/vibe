//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// An owner can confirm a booking before the client has paid. When the client then pays
// through the checkout that is still open, the webhook settles that checkout row with
// ConfirmWebhookPayment. The booking is already confirmed, so the write must keep its
// status and must not be refused as a status reversal.
func TestConfirmWebhookPaymentSettlesACheckoutOnAConfirmedBookingWithNoMoney(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	booking := f.CreateBooking(t, datatest.BookingOptions{
		Status:           "confirmed",
		CollectionStatus: bookingstore.CollectionStatusUnpaid,
	})
	checkout := f.CreatePayment(t, booking.ID, 150_000, 7_500, nil)
	if _, err := f.DB.Exec(context.Background(),
		`UPDATE payments SET status = 'unpaid', method = 'mercadopago', mp_preference_id = $2 WHERE id = $1`,
		checkout.ID, "pref-"+uuid.NewString(),
	); err != nil {
		t.Fatalf("setup: marking the checkout row unpaid: %v", err)
	}

	mpPaymentID := "mp-" + uuid.NewString()
	checkout.Status = "deposit_paid"
	checkout.MPPaymentID = &mpPaymentID
	booking.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	if err := f.Stores.Payments.ConfirmWebhookPayment(ctx, checkout, booking); err != nil {
		t.Fatalf("ConfirmWebhookPayment on a confirmed booking with no money: %v", err)
	}

	if status, _ := f.ReadPaymentState(t, checkout.ID); status != "deposit_paid" {
		t.Errorf("the checkout row must read as received; got %q", status)
	}
	bookingStatus, bookingCollection, bookingRefund := f.ReadBookingState(t, booking.ID)
	if bookingStatus != "confirmed" {
		t.Errorf("the booking must stay confirmed; got %q", bookingStatus)
	}
	if bookingCollection != bookingstore.CollectionStatusDepositPaid {
		t.Errorf("the booking must read as deposit paid; got %q", bookingCollection)
	}
	if bookingRefund != bookingstore.RefundStatusNone {
		t.Errorf("a payment must not start a refund; got refund_status %q", bookingRefund)
	}
}

// The same owner-confirmed booking, paid through the path a booking takes when it has
// no checkout row: the payment is inserted and the booking confirmed in one
// transaction. The confirmation guards must accept a booking that is already confirmed.
func TestInsertAndConfirmBookingAcceptsAConfirmedBookingWithNoMoney(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	booking := f.CreateBooking(t, datatest.BookingOptions{
		Status:           "confirmed",
		CollectionStatus: bookingstore.CollectionStatusUnpaid,
	})
	mpPaymentID := "mp-" + uuid.NewString()
	payment := &paymentstore.Payment{
		BookingID:   booking.ID,
		ComplexID:   booking.ComplexID,
		Amount:      150_000,
		ServiceFee:  7_500,
		Method:      "mercadopago",
		Status:      "deposit_paid",
		MPPaymentID: &mpPaymentID,
	}
	booking.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	if err := f.Stores.Payments.InsertAndConfirmBooking(ctx, payment, booking); err != nil {
		t.Fatalf("InsertAndConfirmBooking on a confirmed booking with no money: %v", err)
	}

	bookingStatus, bookingCollection, _ := f.ReadBookingState(t, booking.ID)
	if bookingStatus != "confirmed" {
		t.Errorf("the booking must stay confirmed; got %q", bookingStatus)
	}
	if bookingCollection != bookingstore.CollectionStatusDepositPaid {
		t.Errorf("the booking must read as deposit paid; got %q", bookingCollection)
	}
}
