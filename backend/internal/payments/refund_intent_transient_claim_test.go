package payments

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/pricing"
)

// standingRefundIntents answers what GetRefundIntentOrphans would read once the
// grace period has passed: the booking as it was committed with the payment,
// for as long as its refund-intent marker stands. The marker stands from the
// commit that records the payment until either ClearRefundIntent runs or a
// claim commits (the real ClaimRefund clears it inside its own transaction).
func standingRefundIntents(f *fixture) []*bookingstore.Booking {
	committed := f.payments.confirmedBooking
	if committed == nil || committed.RefundIntentAt == nil {
		return nil
	}
	if len(f.refundIntents.cleared) > 0 || len(f.payments.claimed) > 0 {
		return nil
	}
	orphan := *committed
	return []*bookingstore.Booking{&orphan}
}

// The first approved payment on an owner- or client-cancelled booking is
// written onto the checkout row together with a refund-intent marker, and only
// then is its refund claimed. When that claim fails for a moment, the money is
// captured and nothing is queued. The webhook's redelivery cannot help: the row
// now carries the MercadoPago id, so it stops at "payment already processed".
// The refund-intent sweep is what is left, and it only sees the booking while
// the marker stands. Clearing it on the failed claim left the money captured
// with nothing that would ever come back for it.
func TestAFirstPaymentWhoseClaimFailsForAMomentIsStillRefunded(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	booking.Status = bookingstore.BookingStatusCancelled
	mpPayment.CollectorID = 111111111
	mpPayment.ExternalReference = booking.ID.String()

	fee := pricing.ServiceFee(booking.DepositAmount)
	checkout := &paymentstore.Payment{
		ID: uuid.New(), BookingID: booking.ID, ComplexID: complexID,
		Amount: booking.DepositAmount, ServiceFee: fee, Method: "mercadopago", Status: "unpaid",
	}
	f.payments.byBooking = checkout
	f.payments.byBookingAll = []*paymentstore.Payment{checkout}
	f.payments.claimAmount = booking.DepositAmount + fee
	f.provider.refundAmount = float64(booking.DepositAmount+fee) / 100
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	f.provider.payment = mpPayment
	readableNotice(f, booking)

	f.payments.claimErr = errors.New("connection reset")
	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err == nil {
		t.Fatal("a claim that failed for a moment must fail the event")
	}
	f.payments.claimErr = nil

	// MercadoPago redelivers the event, and the refund-intent sweep runs once
	// the marker's grace period has passed. Between them the money must go back.
	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatalf("the redelivered event: %v", err)
	}
	f.refundIntents.orphans = standingRefundIntents(f)
	f.service.SweepOrphanedRefundIntents(t.Context())

	if len(f.provider.refunds) != 1 {
		t.Fatalf("the payment must be refunded exactly once after the failed claim; got %v", f.provider.refunds)
	}
	if len(f.payments.recordedSuccess) != 1 {
		t.Fatalf("the refund must be recorded once; got %d", len(f.payments.recordedSuccess))
	}
	if row := f.payments.byMPID; row == nil || row.Status != "refunded" {
		t.Fatalf("the payment row must end refunded; got %+v", row)
	}

	// Neither a later redelivery nor a later sweep may send the money twice.
	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatalf("a later redelivery: %v", err)
	}
	f.refundIntents.orphans = standingRefundIntents(f)
	f.service.SweepOrphanedRefundIntents(t.Context())
	if len(f.provider.refunds) != 1 {
		t.Errorf("a refunded payment must never be refunded again; got %v", f.provider.refunds)
	}
}
