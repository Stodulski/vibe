//go:build integration

package data_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stodulski/vibe-server/internal/data"
)

// R1-cancelled-payment-blocked / R3-confirm-guard-blocks-autorefund /
// R2-confirmable-guard-contradicts-refund-note: an earlier version of
// guardBookingConfirmable (payments.go) ran unconditionally and refused every
// booking whose re-read status was cancelled, completed or no_show. That
// caught the H-15 race below, but it also caught the auto-refund path's own
// write of a cancelled booking's payment — recordPaymentOwedARefund
// (internal/payments/process.go) depends on this exact transaction accepting
// a booking whose target status is "cancelled", because the write it needs is
// recording the money already captured for it, not confirming it. Scoping the
// guard to fire only when b.Status == "confirmed" is what lets both of these
// hold at once; these two tests are one behaviour each.

// TestGuardBookingConfirmableRefusesConfirmingAConcurrentlyCancelledBooking is
// the H-15 race itself: ConfirmPayment reads booking.Status well before this
// transaction opens, so a client cancellation committing in that gap must
// still be caught by the re-read here rather than reaching the UPDATE and
// falling through to bookings_forbid_status_reversal as a raw 500.
func TestGuardBookingConfirmableRefusesConfirmingAConcurrentlyCancelledBooking(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	b := f.createBooking(t, bookingOptions{Status: "pending", CollectionStatus: data.CollectionStatusUnpaid})

	// The cancellation committing in the gap between ConfirmPayment's read and
	// this transaction — markStatus is the same raw UPDATE the sibling
	// occupancy tests use to move a booking straight to a terminal status.
	markStatus(t, f, b.ID, "cancelled")

	// ConfirmPayment's own "if pending then confirmed" runs against the stale
	// in-memory copy it read before the race, so the booking this call carries
	// still says "confirmed" — exactly what it would say in production at the
	// moment the cancellation lands.
	b.Status = "confirmed"
	payment := &data.Payment{
		BookingID:  b.ID,
		ComplexID:  f.ComplexID,
		Amount:     b.DepositAmount,
		ServiceFee: 10_500,
		Method:     "cash",
		Status:     "fully_paid",
	}

	// ErrBookingCancelled, not ErrBookingNotConfirmable. H-23 split the
	// cancelled case out of that sentinel because the two halves have opposite
	// consequences for the client's money on the OTHER caller of this
	// transaction: the webhook has a captured payment to send back for a
	// cancelled booking, and nothing to send back for a completed or no_show
	// one. This path — an owner confirming cash at the counter while the client
	// cancels — is unaffected by that split and must keep behaving exactly as
	// it did: refused, nothing written, and answered as a 409 by
	// internal/bookings/actions.go, which names both sentinels.
	err := f.Models.Payments.InsertAndConfirmBooking(ctx, payment, b)
	if !errors.Is(err, data.ErrBookingCancelled) {
		t.Errorf("confirming a concurrently cancelled booking must be refused with ErrBookingCancelled; got %v", err)
	}

	status, _, _ := f.readBookingState(t, b.ID)
	if status != "cancelled" {
		t.Errorf("a refused confirmation must leave the booking cancelled; got status=%q", status)
	}

	var payments int
	if err := f.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments WHERE booking_id = $1`, b.ID).Scan(&payments); err != nil {
		t.Fatalf("counting payments: %v", err)
	}
	if payments != 0 {
		t.Errorf("a refused confirmation must write no payment row; found %d", payments)
	}
}

// TestGuardBookingConfirmableAllowsRecordingAPaymentForAnAlreadyCancelledBooking
// is the auto-refund path the guard must not break. Before this fix,
// guardBookingConfirmable refused every cancelled booking unconditionally, so
// this exact write — money already captured, a refund about to be claimed
// against the row this call inserts — never reached the database, and the
// money stayed captured with nothing to refund it against.
func TestGuardBookingConfirmableAllowsRecordingAPaymentForAnAlreadyCancelledBooking(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	b := f.createBooking(t, bookingOptions{Status: "confirmed"})
	markStatus(t, f, b.ID, "cancelled")

	// refundBookingWhoseSlotIsGone sets booking.Status = "cancelled" before
	// calling InsertAndConfirmBooking, precisely so the guard has nothing to
	// object to on this path.
	b.Status = "cancelled"
	payment := &data.Payment{
		BookingID:  b.ID,
		ComplexID:  f.ComplexID,
		Amount:     b.DepositAmount,
		ServiceFee: 10_500,
		Method:     "mercadopago",
		Status:     "deposit_paid",
	}

	if err := f.Models.Payments.InsertAndConfirmBooking(ctx, payment, b); err != nil {
		t.Fatalf("recording a payment for an already-cancelled booking must succeed: %v", err)
	}

	status, _, _ := f.readBookingState(t, b.ID)
	if status != "cancelled" {
		t.Errorf("the booking must stay cancelled; got status=%q", status)
	}

	var payments int
	if err := f.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments WHERE booking_id = $1`, b.ID).Scan(&payments); err != nil {
		t.Fatalf("counting payments: %v", err)
	}
	if payments != 1 {
		t.Errorf("the payment behind the refund must be recorded; found %d rows", payments)
	}
}
