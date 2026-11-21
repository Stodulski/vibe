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

// A booking the expiry cancelled whose first late payment was already refunded
// (or is owed a refund) is not reopened by a second payment. The reversal
// trigger refuses that write, and a guard looser than the trigger answered it
// with a raw 23514 the webhook retried forever instead of refunding; the guard
// and GetByID now refuse exactly what the trigger refuses, as a cancellation.
func TestASecondLatePaymentOnARefundedExpiredBookingIsRefusedAsCancelled(t *testing.T) {
	cases := map[string]string{
		"refunded":       `UPDATE bookings SET collection_status = 'deposit_paid', refund_status = 'full' WHERE id = $1`,
		"refund pending": `UPDATE bookings SET collection_status = 'deposit_paid', refund_status = 'pending' WHERE id = $1`,
		"refund owed":    `UPDATE bookings SET refund_intent_at = NOW() WHERE id = $1`,
	}
	for name, write := range cases {
		t.Run(name, func(t *testing.T) {
			f := datatest.Isolated(t)
			ctx := f.Scoped(context.Background())
			b, checkout := expiredStorefrontBooking(t, f)
			if _, err := f.DB.Exec(ctx, write, b.ID); err != nil {
				t.Fatalf("settling the first late payment: %v", err)
			}
			got, err := f.Stores.Bookings.GetByID(ctx, b.ID)
			if err != nil {
				t.Fatalf("GetByID: %v", err)
			}
			if got.CancelledByHoldExpiry {
				t.Error("GetByID must not offer a booking the trigger refuses to reopen")
			}
			confirmedByLatePayment(b, checkout)
			if err := f.Stores.Payments.ConfirmWebhookPayment(ctx, checkout, b); !errors.Is(err, bookingstore.ErrBookingCancelled) {
				t.Fatalf("want ErrBookingCancelled, which the webhook refunds; got %v", err)
			}
			if status, _, _ := f.ReadBookingState(t, b.ID); status != "cancelled" {
				t.Errorf("want the booking still cancelled; got %s", status)
			}
		})
	}
}

// A late hand-over (the reconciliation can run hours later) must not confirm
// a booking whose hours already began: the payment is refunded as late instead.
func TestALatePaymentDoesNotReopenABookingThatAlreadyStarted(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	b, checkout := expiredStorefrontBooking(t, f)
	if _, err := f.DB.Exec(ctx, `UPDATE bookings SET date = CURRENT_DATE - 1 WHERE id = $1`, b.ID); err != nil {
		t.Fatalf("moving the booking into the past: %v", err)
	}
	b.Date = time.Now().AddDate(0, 0, -1)
	got, err := f.Stores.Bookings.GetByID(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.CancelledByHoldExpiry {
		t.Error("GetByID must not offer a booking that already started for reopening")
	}
	confirmedByLatePayment(b, checkout)
	if err := f.Stores.Payments.ConfirmWebhookPayment(ctx, checkout, b); !errors.Is(err, bookingstore.ErrBookingCancelled) {
		t.Fatalf("want ErrBookingCancelled, which the webhook refunds as a late payment; got %v", err)
	}
	if status, _, _ := f.ReadBookingState(t, b.ID); status != "cancelled" {
		t.Errorf("want the booking still cancelled; got %s", status)
	}
}

// Another visitor's checkout on the same hours, itself past its payment
// window, does not block the court (SlotTaken's stale carve-out) but did still
// count for the exclusion constraint, so the reopening failed with a raw
// 23P01 and the event was requeued. The stale row is released first, as an
// insert releases it, and recorded as expired for the sweep.
func TestALatePaymentReopensOverAStaleCheckoutOnTheSameHours(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	b, checkout := expiredStorefrontBooking(t, f)
	stale := f.CreateBooking(t, datatest.BookingOptions{
		Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid, Public: true,
	})
	f.BackdateBookingCreatedAt(t, stale.ID, time.Hour)
	confirmedByLatePayment(b, checkout)

	if err := f.Stores.Payments.ConfirmWebhookPayment(ctx, checkout, b); err != nil {
		t.Fatalf("a late payment into hours only a stale checkout holds must reopen the booking: %v", err)
	}
	if status, _, _ := f.ReadBookingState(t, b.ID); status != "confirmed" {
		t.Errorf("want the booking confirmed; got %s", status)
	}
	if status, _, _ := f.ReadBookingState(t, stale.ID); status != "cancelled" {
		t.Errorf("want the stale checkout released; got %s", status)
	}
	if n := holdExpiryRows(t, f, stale); n != 1 {
		t.Errorf("want the released checkout recorded as expired for the sweep; got %d rows", n)
	}
}

// A reopened booking no longer reads as cancelled by the expiry: its note is
// taken off, whichever expiry path wrote it, and anything else the note said
// is kept.
func TestAReopenedBookingLosesTheExpiryNote(t *testing.T) {
	cases := map[string]struct{ note, want string }{
		"sweep note":      {"Cancelado automáticamente: tiempo de pago expirado (10m0s)", ""},
		"inline note":     {"Cancelado automáticamente: tiempo de pago expirado", ""},
		"after a comment": {"Llega tarde | Cancelado automáticamente: tiempo de pago expirado (15m0s)", "Llega tarde"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := datatest.Isolated(t)
			ctx := f.Scoped(context.Background())
			b, checkout := expiredStorefrontBooking(t, f)
			note := tc.note
			b.Notes = &note
			confirmedByLatePayment(b, checkout)

			if err := f.Stores.Payments.ConfirmWebhookPayment(ctx, checkout, b); err != nil {
				t.Fatalf("ConfirmWebhookPayment: %v", err)
			}
			got, err := f.Stores.Bookings.GetByID(ctx, b.ID)
			if err != nil {
				t.Fatalf("GetByID: %v", err)
			}
			gotNote := ""
			if got.Notes != nil {
				gotNote = *got.Notes
			}
			if gotNote != tc.want {
				t.Errorf("want note %q; got %q", tc.want, gotNote)
			}
		})
	}
}
