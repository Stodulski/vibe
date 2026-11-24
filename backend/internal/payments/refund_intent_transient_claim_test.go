package payments

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"

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
	orphan.RefundIntentCause = f.refundIntents.causes[orphan.ID]
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
	// The sweep finished the refund the webhook started, so the client is told
	// why the money came back, as the webhook would have told them.
	if len(f.notify.refunds) != 1 || f.notify.refunds[0].Reason != latePaymentReason {
		t.Errorf("the client must be told the payment came back because it arrived late; got %+v", f.notify.refunds)
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

// A cancellation whose refund claim fails for a moment used to owe the money by
// hand, with an alert, although nothing was wrong with the refund itself. The
// marker the cancellation wrote is what brings the sweep back, so the claim is
// retried there and the money goes back on its own, exactly once.
func TestACancellationWhoseClaimFailsForAMomentIsRefundedByTheSweep(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	booking.Status = bookingstore.BookingStatusCancelled
	f.payments.byBooking = payment
	f.complexes.complex = linkedComplex(complexID, "")
	readableNotice(f, booking)

	f.payments.claimErr = errors.New("connection reset")
	got := f.service.AutoRefundIfPaid(t.Context(), booking)
	f.payments.claimErr = nil

	if got.NeedsAHuman() {
		t.Fatalf("a claim that failed for a moment must be retried, not owed by hand; got %+v", got)
	}
	if len(f.refundIntents.marked) != 1 || len(f.refundIntents.cleared) != 0 {
		t.Fatalf("the marker must stand for the sweep; marked %v, cleared %v", f.refundIntents.marked, f.refundIntents.cleared)
	}

	standing := time.Now().Add(-refundIntentGrace)
	orphan := *booking
	orphan.RefundIntentAt = &standing
	f.refundIntents.orphans = []*bookingstore.Booking{&orphan}
	f.service.SweepOrphanedRefundIntents(t.Context())
	f.refundIntents.orphans = nil
	f.service.SweepOrphanedRefundIntents(t.Context())

	if len(f.provider.refunds) != 1 {
		t.Fatalf("the sweep must refund the payment exactly once; got %v", f.provider.refunds)
	}
	if len(f.payments.recordedSuccess) != 1 {
		t.Fatalf("the refund must be recorded once; got %d", len(f.payments.recordedSuccess))
	}
	if len(f.notify.refunds) != 1 || f.notify.refunds[0].Reason != "" {
		t.Errorf("a cancellation's refund is told plainly, once; got %+v", f.notify.refunds)
	}
}

// The sweep's own claim failing is owed by hand rather than marked for another
// sweep, so a claim that keeps failing raises an alert instead of being retried
// in silence every few minutes.
func TestTheSweepsOwnFailedClaimIsOwedByHand(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	booking.Status = bookingstore.BookingStatusCancelled
	f.payments.byBooking = payment
	f.complexes.complex = linkedComplex(complexID, "")

	f.payments.claimErr = errors.New("connection reset")
	got := f.service.refundCancelledBooking(t.Context(), booking, true)

	if got.Result != paymentstore.RefundManual {
		t.Errorf("the sweep's failed claim must be owed by hand; got %+v", got)
	}
	if len(f.refundIntents.marked) != 0 {
		t.Errorf("the sweep must not stand its own marker up again; marked %v", f.refundIntents.marked)
	}
	if len(f.refundIntents.cleared) != 1 {
		t.Errorf("the marker must be cleared once the refund is owed by hand; cleared %v", f.refundIntents.cleared)
	}
}

// A claim that fails for good (the payment row is gone, or carries no
// MercadoPago id) cannot be fixed by a retry. Marking it for the sweep only
// delayed the alert by one sweep; it is owed by hand at once.
func TestACancellationWhoseClaimFailsForGoodIsOwedByHandAtOnce(t *testing.T) {
	for name, claimErr := range map[string]error{
		"the payment row is gone":           data.ErrRecordNotFound,
		"the payment has no MercadoPago id": fmt.Errorf("payment x: %w", paymentstore.ErrNoMercadoPagoID),
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, payment := paidBooking(complexID)
			booking.Status = bookingstore.BookingStatusCancelled
			f.payments.byBooking = payment
			f.complexes.complex = linkedComplex(complexID, "")

			f.payments.claimErr = claimErr
			got := f.service.AutoRefundIfPaid(t.Context(), booking)

			if got.Result != paymentstore.RefundManual {
				t.Errorf("a claim no retry can fix must be owed by hand; got %+v", got)
			}
			if len(f.refundIntents.marked) != 0 {
				t.Errorf("nothing must be marked for a sweep that would fail the same way; marked %v", f.refundIntents.marked)
			}
		})
	}
}

// The webhook's refund of a payment on a cancelled booking keeps its marker
// for a claim that failed for a moment. One that failed for good is owed by
// hand at once, with its amount on the trail: neither the redelivery nor the
// sweep could ever claim it.
func TestALatePaymentWhoseClaimFailsForGoodIsOwedByHand(t *testing.T) {
	for name, claimErr := range map[string]error{
		"the payment row is gone":           data.ErrRecordNotFound,
		"the payment has no MercadoPago id": fmt.Errorf("payment x: %w", paymentstore.ErrNoMercadoPagoID),
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, payment := paidBooking(complexID)
			booking.Status = bookingstore.BookingStatusCancelled
			booking.CollectionStatus = bookingstore.CollectionStatusUnpaid
			f.payments.byBooking = payment
			f.payments.claimErr = claimErr

			mpPayment := &mp.Payment{ID: 123, Status: "approved", TransactionAmount: 1_500.00}
			if err := f.service.refundCancelledBookingPayment(t.Context(), booking, mpPayment, "mp-123", paymentstore.RefundCauseLatePayment); err != nil {
				t.Errorf("a claim no retry can fix must not fail the event for a redelivery; got %v", err)
			}
			if len(f.refundIntents.cleared) != 1 || len(f.refundIntents.causes) != 0 {
				t.Errorf("the marker must be cleared with no cause kept; cleared %v, causes %v", f.refundIntents.cleared, f.refundIntents.causes)
			}
			entries := f.audit.find("refund")
			if len(entries) != 1 {
				t.Fatalf("the money owed by hand must reach the trail once; got %d entries", len(entries))
			}
			got := value(t, entries[0])
			if got["result"] != string(paymentstore.RefundManual) || got["amount_centavos"] == float64(0) {
				t.Errorf("the trail must say the refund is owed by hand, with its amount; got %v", got)
			}
		})
	}
}

// When the sweep finishes a refund already under way, it reads the booking's
// rows to find another payment still owed. A read that failed answered "in
// flight" and the marker was cleared with no alert: that second payment was
// never looked for again. The marker has to stand for the next sweep.
func TestTheSweepKeepsTheMarkerWhenThePaymentRowsCannotBeRead(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	booking.Status = bookingstore.BookingStatusCancelled
	booking.RefundStatus = bookingstore.RefundStatusPending
	f.payments.byBooking = payment
	f.payments.listErr = errors.New("connection reset")
	f.complexes.complex = linkedComplex(complexID, "")

	standing := time.Now().Add(-refundIntentGrace)
	booking.RefundIntentAt = &standing
	f.refundIntents.orphans = []*bookingstore.Booking{booking}
	f.service.SweepOrphanedRefundIntents(t.Context())

	if len(f.refundIntents.cleared) != 0 {
		t.Errorf("the marker must not be cleared while the rows behind it are unread; cleared %v", f.refundIntents.cleared)
	}
	if len(f.refundIntents.marked) != 1 {
		t.Errorf("the marker must stand for the next sweep; marked %v", f.refundIntents.marked)
	}
}

// The same unread rows on a cancellation itself: the marker has to stand for
// the sweep, and when it cannot be stood up the client is not left with
// nothing behind it silently (the alert says so; see alertPaymentRowsUnread).
func TestACancellationKeepsItsMarkerWhenThePaymentRowsCannotBeRead(t *testing.T) {
	for name, markErr := range map[string]error{"the marker stands": nil, "the marker cannot be written": errors.New("connection reset")} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, payment := paidBooking(complexID)
			booking.Status = bookingstore.BookingStatusCancelled
			booking.RefundStatus = bookingstore.RefundStatusPending
			f.payments.byBooking = payment
			f.payments.listErr = errors.New("connection reset")
			f.refundIntents.markErr = markErr

			f.service.AutoRefundIfPaid(t.Context(), booking)

			kept := len(f.refundIntents.marked) == 1 && len(f.refundIntents.cleared) == 0
			if markErr == nil && !kept {
				t.Errorf("the marker must stand for the sweep; marked %v, cleared %v", f.refundIntents.marked, f.refundIntents.cleared)
			}
			if markErr != nil && len(f.refundIntents.cleared) != 1 {
				t.Errorf("a marker that could not be stood up is cleared as before; cleared %v", f.refundIntents.cleared)
			}
		})
	}
}
