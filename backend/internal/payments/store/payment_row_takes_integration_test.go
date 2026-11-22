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

// Two approved MercadoPago payments both read a booking's untouched checkout
// row and both decide it may take them. Only the first write lands: the second
// is refused with ErrPaymentRowTaken, so its caller records it on a row of its
// own, and the row keeps the first payment's id. The write used to be
// unconditional, so the second payment's id replaced the first's on the row
// and one of the two payments was left with no record. Writing the same
// payment again is still accepted.
func TestAnApprovedPaymentDoesNotTakeACheckoutRowAnotherPaymentTook(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	created := f.CreateBooking(t, datatest.BookingOptions{
		Status: "cancelled", CollectionStatus: bookingstore.CollectionStatusUnpaid,
	})
	preference := "pref-" + uuid.NewString()
	checkout := &paymentstore.Payment{
		BookingID: created.ID, ComplexID: f.ComplexID, Amount: 150_000, ServiceFee: 7_500,
		Method: "mercadopago", Status: "unpaid", MPPreferenceID: &preference,
	}
	if err := f.Stores.Payments.Insert(ctx, checkout); err != nil {
		t.Fatalf("creating the checkout row: %v", err)
	}

	write := func(mpPaymentID string) error {
		booking, err := f.Stores.Bookings.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("reading the booking: %v", err)
		}
		booking.CollectionStatus = bookingstore.CollectionStatusDepositPaid
		now := time.Now()
		booking.RefundIntentAt = &now
		// Both payments start from the row as it was read before either wrote.
		row := *checkout
		row.Status = "deposit_paid"
		row.MPPaymentID = &mpPaymentID
		return f.Stores.Payments.ConfirmWebhookPayment(ctx, &row, booking)
	}

	mpFirst, mpSecond := "mp-"+uuid.NewString(), "mp-"+uuid.NewString()
	if err := write(mpFirst); err != nil {
		t.Fatalf("the first payment takes the untouched checkout row; got %v", err)
	}
	if err := write(mpSecond); !errors.Is(err, paymentstore.ErrPaymentRowTaken) {
		t.Fatalf("the second payment must be refused the row the first took; got %v", err)
	}
	stored, err := f.Stores.Payments.GetByMPPaymentID(ctx, mpFirst)
	if err != nil || stored.ID != checkout.ID {
		t.Fatalf("the checkout row must keep the first payment's id; got %+v, %v", stored, err)
	}
	if err := write(mpFirst); err != nil {
		t.Errorf("the same payment written again must still be accepted; got %v", err)
	}
}
