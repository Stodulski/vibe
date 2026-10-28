//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// blockedPendingBooking is a public pending booking that has outlived its payment
// hold, so the owner can block its hours (the courts store lets a stale unpaid public
// booking be overtaken). The payment for it can still arrive after that. The block
// covers 18:00 to 20:00 and the booking is 18:00 to 19:30.
func blockedPendingBooking(t *testing.T, f *datatest.Fixture) *bookingstore.Booking {
	t.Helper()
	b := f.CreateBooking(t, datatest.BookingOptions{
		Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid, Public: true,
	})
	f.BackdateBookingCreatedAt(t, b.ID, time.Hour)
	block := &courtstore.BlockedSlot{CourtID: f.CourtID, Date: b.Date, StartTime: "18:00", EndTime: "20:00"}
	if err := f.Stores.Courts.InsertBlockedSlot(f.Scoped(context.Background()), block); err != nil {
		t.Fatalf("blocking the slot: %v", err)
	}
	return b
}

// An owner can block hours that a pending public booking still holds. Confirming
// that booking used to check other bookings and never the blocked slots, so the
// owner's block was sold over. A confirmation over a blocked slot is refused with
// ErrSlotUnavailable, which sends the client's money back through the refund path
// the same way a slot taken by another booking does.
//
// Both confirmation writes are exercised, because the webhook takes one or the
// other depending on whether a checkout row already exists.
func TestConfirmationRefusesASlotTheOwnerBlockedAfterTheBookingWasMade(t *testing.T) {
	ctx := context.Background()

	t.Run("a new payment row", func(t *testing.T) {
		f := datatest.Isolated(t)
		b := blockedPendingBooking(t, f)
		// The webhook sets the confirmed status before the write, as processApprovedPayment does.
		b.Status = "confirmed"
		payment := &paymentstore.Payment{
			BookingID:  b.ID,
			ComplexID:  f.ComplexID,
			Amount:     b.DepositAmount,
			ServiceFee: 10_500,
			Method:     "mercadopago",
			Status:     "deposit_paid",
		}

		err := f.Stores.Payments.InsertAndConfirmBooking(f.Scoped(ctx), payment, b)
		if !errors.Is(err, bookingstore.ErrSlotUnavailable) {
			t.Fatalf("confirming over an owner's blocked slot must be refused with ErrSlotUnavailable; got %v", err)
		}
		assertBookingStillPending(t, f, b.ID)
	})

	t.Run("the checkout row the webhook settles", func(t *testing.T) {
		f := datatest.Isolated(t)
		b := blockedPendingBooking(t, f)
		checkout := f.CreatePayment(t, b.ID, b.DepositAmount, 10_500, nil)
		b.Status = "confirmed"
		checkout.Status = "deposit_paid"
		mpPaymentID := "mp-blocked-slot"
		checkout.MPPaymentID = &mpPaymentID

		err := f.Stores.Payments.ConfirmWebhookPayment(f.Scoped(ctx), checkout, b)
		if !errors.Is(err, bookingstore.ErrSlotUnavailable) {
			t.Fatalf("settling a checkout over an owner's blocked slot must be refused with ErrSlotUnavailable; got %v", err)
		}
		assertBookingStillPending(t, f, b.ID)
	})
}

// A block that starts exactly when the booking ends shares no moment with it, so the
// slot is still free. This pins the half-open range the overlap check uses, the same
// one the bookings exclusion constraint uses.
func TestConfirmationAllowsABlockThatStartsWhenTheBookingEnds(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	b := f.CreateBooking(t, datatest.BookingOptions{Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid})
	block := &courtstore.BlockedSlot{CourtID: f.CourtID, Date: b.Date, StartTime: "19:30", EndTime: "21:00"}
	if err := f.Stores.Courts.InsertBlockedSlot(f.Scoped(ctx), block); err != nil {
		t.Fatalf("blocking the slot: %v", err)
	}

	b.Status = "confirmed"
	payment := &paymentstore.Payment{
		BookingID:  b.ID,
		ComplexID:  f.ComplexID,
		Amount:     b.DepositAmount,
		ServiceFee: 10_500,
		Method:     "mercadopago",
		Status:     "deposit_paid",
	}
	if err := f.Stores.Payments.InsertAndConfirmBooking(f.Scoped(ctx), payment, b); err != nil {
		t.Fatalf("a block that starts when the booking ends must not refuse it: %v", err)
	}
}

func assertBookingStillPending(t *testing.T, f *datatest.Fixture, bookingID uuid.UUID) {
	t.Helper()
	status, _, _ := f.ReadBookingState(t, bookingID)
	if status != "pending" {
		t.Errorf("a refused confirmation must leave the booking pending; got status=%q", status)
	}
}
