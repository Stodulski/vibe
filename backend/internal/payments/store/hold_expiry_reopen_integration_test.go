//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// expiredStorefrontBooking is a storefront booking whose payment window ran
// out and that the release sweep cancelled (ExpireHold), with the checkout
// row its MercadoPago link was created with.
func expiredStorefrontBooking(t *testing.T, f *datatest.Fixture) (*bookingstore.Booking, *paymentstore.Payment) {
	t.Helper()
	ctx := f.Scoped(context.Background())
	b := f.CreateBooking(t, datatest.BookingOptions{
		Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid, Public: true,
	})
	f.BackdateBookingCreatedAt(t, b.ID, time.Hour)
	preferenceID := "pref-" + b.ID.String()
	checkout := &paymentstore.Payment{
		BookingID: b.ID, ComplexID: f.ComplexID, Amount: b.DepositAmount, ServiceFee: 10_500,
		Method: "mercadopago", Status: "unpaid", MPPreferenceID: &preferenceID,
	}
	if err := f.Stores.Payments.Insert(ctx, checkout); err != nil {
		t.Fatalf("creating the checkout row: %v", err)
	}
	b.Status = "cancelled"
	note := "Cancelado automáticamente: tiempo de pago expirado"
	b.Notes = &note
	if err := f.Stores.Bookings.ExpireHold(ctx, b); err != nil {
		t.Fatalf("expiring the booking: %v", err)
	}
	return b, checkout
}

// confirmedByLatePayment moves b and its checkout to what the webhook writes
// for an approved payment.
func confirmedByLatePayment(b *bookingstore.Booking, checkout *paymentstore.Payment) {
	b.Status = "confirmed"
	b.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	checkout.Status = "deposit_paid"
	mpPaymentID := "mp-late-" + b.ID.String()
	checkout.MPPaymentID = &mpPaymentID
}

func holdExpiryRows(t *testing.T, f *datatest.Fixture, b *bookingstore.Booking) int {
	t.Helper()
	var n int
	if err := f.DB.QueryRow(f.Scoped(context.Background()),
		`SELECT count(*) FROM booking_hold_expiries WHERE booking_id = $1`, b.ID).Scan(&n); err != nil {
		t.Fatalf("counting hold expiry rows: %v", err)
	}
	return n
}

// S5: a late payment on a booking only the expiry cancelled confirms it while
// its hours are free, through both confirmation writes, and the expiry record
// goes with it so a later cancellation is final.
func TestALatePaymentReopensABookingCancelledByHoldExpiry(t *testing.T) {
	t.Run("the checkout row the webhook settles", func(t *testing.T) {
		f := datatest.Isolated(t)
		b, checkout := expiredStorefrontBooking(t, f)
		got, err := f.Stores.Bookings.GetByID(f.Scoped(context.Background()), b.ID)
		if err != nil || !got.CancelledByHoldExpiry {
			t.Fatalf("want GetByID to read the booking as cancelled by hold expiry; got %+v, %v", got, err)
		}
		confirmedByLatePayment(b, checkout)

		if err := f.Stores.Payments.ConfirmWebhookPayment(f.Scoped(context.Background()), checkout, b); err != nil {
			t.Fatalf("a late payment into free hours must reopen the booking: %v", err)
		}
		status, collection, _ := f.ReadBookingState(t, b.ID)
		if status != "confirmed" || collection != "deposit_paid" {
			t.Errorf("want the booking confirmed and collected; got %s/%s", status, collection)
		}
		if n := holdExpiryRows(t, f, b); n != 0 {
			t.Errorf("want the expiry record deleted with the reopening; got %d rows", n)
		}
	})

	t.Run("a new payment row", func(t *testing.T) {
		f := datatest.Isolated(t)
		b, _ := expiredStorefrontBooking(t, f)
		b.Status = "confirmed"
		b.CollectionStatus = bookingstore.CollectionStatusDepositPaid
		mpPaymentID := "mp-late-new-" + b.ID.String()
		payment := &paymentstore.Payment{
			BookingID: b.ID, ComplexID: f.ComplexID, Amount: b.DepositAmount, ServiceFee: 10_500,
			Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &mpPaymentID,
		}

		if err := f.Stores.Payments.InsertAndConfirmBooking(f.Scoped(context.Background()), payment, b); err != nil {
			t.Fatalf("a late payment into free hours must reopen the booking: %v", err)
		}
		if status, _, _ := f.ReadBookingState(t, b.ID); status != "confirmed" {
			t.Errorf("want the booking confirmed; got %s", status)
		}
		if n := holdExpiryRows(t, f, b); n != 0 {
			t.Errorf("want the expiry record deleted with the reopening; got %d rows", n)
		}
	})
}

// S5: hours that went to somebody else are not sold twice: the reopening is
// refused with ErrSlotUnavailable (refunded by the webhook) and the booking
// stays cancelled with its record.
func TestALatePaymentDoesNotReopenAnExpiredBookingWhoseHoursAreTaken(t *testing.T) {
	f := datatest.Isolated(t)
	b, checkout := expiredStorefrontBooking(t, f)
	f.CreateBooking(t, datatest.BookingOptions{Status: "confirmed"})
	confirmedByLatePayment(b, checkout)

	err := f.Stores.Payments.ConfirmWebhookPayment(f.Scoped(context.Background()), checkout, b)
	if !errors.Is(err, bookingstore.ErrSlotUnavailable) {
		t.Fatalf("want ErrSlotUnavailable; got %v", err)
	}
	if status, _, _ := f.ReadBookingState(t, b.ID); status != "cancelled" {
		t.Errorf("want the booking still cancelled; got %s", status)
	}
	if n := holdExpiryRows(t, f, b); n != 1 {
		t.Errorf("want the expiry record kept; got %d rows", n)
	}
}

// The previous behavior holds for every other cancellation: a booking the owner
// cancelled is never reopened by a payment, whichever write carries it.
func TestALatePaymentDoesNotReopenABookingTheOwnerCancelled(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	b := f.CreateBooking(t, datatest.BookingOptions{
		Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid, Public: true,
	})
	preferenceID := "pref-owner"
	checkout := &paymentstore.Payment{
		BookingID: b.ID, ComplexID: f.ComplexID, Amount: b.DepositAmount, ServiceFee: 10_500,
		Method: "mercadopago", Status: "unpaid", MPPreferenceID: &preferenceID,
	}
	if err := f.Stores.Payments.Insert(ctx, checkout); err != nil {
		t.Fatalf("creating the checkout row: %v", err)
	}
	b.Status = "cancelled"
	if err := f.Stores.Bookings.Update(ctx, b); err != nil {
		t.Fatalf("owner cancellation: %v", err)
	}
	confirmedByLatePayment(b, checkout)

	err := f.Stores.Payments.ConfirmWebhookPayment(ctx, checkout, b)
	if !errors.Is(err, bookingstore.ErrBookingCancelled) {
		t.Fatalf("want ErrBookingCancelled; got %v", err)
	}
	if status, _, _ := f.ReadBookingState(t, b.ID); status != "cancelled" {
		t.Errorf("want the booking still cancelled; got %s", status)
	}
}

// A counter payment is not the payment the booking was waiting for: the
// owner's paths never reopen an expired booking.
func TestACounterPaymentDoesNotReopenAnExpiredBooking(t *testing.T) {
	f := datatest.Isolated(t)
	b, _ := expiredStorefrontBooking(t, f)
	b.Status = "confirmed"
	b.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	cash := &paymentstore.Payment{
		BookingID: b.ID, ComplexID: f.ComplexID, Amount: b.DepositAmount, Method: "cash", Status: "deposit_paid",
	}

	err := f.Stores.Payments.InsertAndConfirmBooking(f.Scoped(context.Background()), cash, b)
	if !errors.Is(err, bookingstore.ErrBookingCancelled) {
		t.Fatalf("want ErrBookingCancelled; got %v", err)
	}
}

// The reversal trigger still refuses any other way back: without the expiry
// record, or without money arriving, a cancelled booking cannot go live.
func TestTheReversalTriggerStillRefusesReopeningWithoutAnExpiryAndMoney(t *testing.T) {
	t.Run("no expiry record", func(t *testing.T) {
		f := datatest.Isolated(t)
		b := f.CreateBooking(t, datatest.BookingOptions{
			Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid, Public: true,
		})
		f.MarkStatus(t, b.ID, "cancelled")
		_, err := f.DB.Exec(f.Scoped(context.Background()),
			`UPDATE bookings SET status = 'confirmed', collection_status = 'deposit_paid' WHERE id = $1`, b.ID)
		if err == nil {
			t.Fatal("a cancelled booking without an expiry record must not go live")
		}
	})

	t.Run("expiry record but no money", func(t *testing.T) {
		f := datatest.Isolated(t)
		b, _ := expiredStorefrontBooking(t, f)
		_, err := f.DB.Exec(f.Scoped(context.Background()),
			`UPDATE bookings SET status = 'confirmed' WHERE id = $1`, b.ID)
		if err == nil {
			t.Fatal("an expired booking must not go live without its payment")
		}
	})
}
