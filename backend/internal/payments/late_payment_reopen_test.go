package payments

import (
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/pricing"
)

// expiredCheckout is a storefront booking cancelled only because its payment
// window ran out, its checkout row still unpaid, and the MercadoPago payment
// that pays it exactly, from the complex's own collector.
func expiredCheckout(f *fixture) (*bookingstore.Booking, *mp.Payment) {
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	booking.Status = bookingstore.BookingStatusCancelled
	booking.CancelledByHoldExpiry = true
	mpPayment.CollectorID = 111111111
	preferenceID := "pref-1"
	f.payments.byBooking = &paymentstore.Payment{
		ID: uuid.New(), BookingID: booking.ID, ComplexID: complexID,
		Amount: booking.DepositAmount, ServiceFee: pricing.ServiceFee(booking.DepositAmount),
		Status: "unpaid", MPPreferenceID: &preferenceID,
	}
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	readableNotice(f, booking)
	return booking, mpPayment
}

// S5: a late payment on a booking only the expiry cancelled confirms it while
// its hours are still free, as the same payment a minute earlier would have.
// The client gets the confirmation, not a refund notice, and nothing is
// refunded.
func TestALatePaymentReopensABookingCancelledByHoldExpiryWhenItsHoursAreFree(t *testing.T) {
	f := newFixture(t)
	booking, mpPayment := expiredCheckout(f)

	if err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
		t.Fatalf("got %v", err)
	}

	if f.payments.confirmed == nil || f.payments.confirmed.MPPaymentID == nil || *f.payments.confirmed.MPPaymentID != "mp-123" {
		t.Fatalf("want the checkout row settled with the payment; got %+v", f.payments.confirmed)
	}
	if f.payments.confirmedBooking.Status != bookingstore.BookingStatusConfirmed ||
		f.payments.confirmedBooking.CollectionStatus == bookingstore.CollectionStatusUnpaid {
		t.Errorf("want the booking written confirmed and collected; got status=%q collection=%q",
			f.payments.confirmedBooking.Status, f.payments.confirmedBooking.CollectionStatus)
	}
	if len(f.provider.refunds) != 0 || len(f.payments.claimed) != 0 {
		t.Errorf("a reopened booking keeps its money; refunds %v, claims %v", f.provider.refunds, f.payments.claimed)
	}
	if len(f.notify.confirmations) != 1 {
		t.Errorf("want the client told the booking is confirmed; got %d confirmations", len(f.notify.confirmations))
	}
	if len(f.notify.refunds) != 0 {
		t.Errorf("a reopened booking must not send a refund notice; got %+v", f.notify.refunds)
	}
}

// S5: when its hours went to somebody else meanwhile, the payment is refunded
// with the slot-taken reason, and nothing is confirmed.
func TestALatePaymentOnABookingCancelledByHoldExpiryIsRefundedWhenItsHoursAreTaken(t *testing.T) {
	f := newFixture(t)
	booking, mpPayment := expiredCheckout(f)
	f.payments.slotErr = bookingstore.ErrSlotUnavailable

	if err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
		t.Fatalf("got %v", err)
	}

	if len(f.provider.refunds) != 1 {
		t.Fatalf("want the payment refunded; got %v", f.provider.refunds)
	}
	if len(f.notify.refunds) != 1 || f.notify.refunds[0].Reason != slotTakenReason {
		t.Errorf("want one notice with the slot-taken reason; got %+v", f.notify.refunds)
	}
	if len(f.notify.confirmations) != 0 {
		t.Error("a refunded booking must not send a confirmation")
	}
}

// S5: the confirmation path declines a payment whose amount does not match.
// For a pending booking that leaves it pending for a person to look at; for a
// booking that is already cancelled it would leave the money captured on a
// booking nobody holds, so it goes back as any late payment always did.
func TestALatePaymentWithTheWrongAmountOnAnExpiredBookingIsStillRefunded(t *testing.T) {
	f := newFixture(t)
	booking, mpPayment := expiredCheckout(f)
	mpPayment.TransactionAmount++
	withCapturedSentryEvents(t)

	if err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
		t.Fatalf("got %v", err)
	}

	if len(f.provider.refunds) != 1 {
		t.Fatalf("want the mismatched payment on a cancelled booking refunded; got %v", f.provider.refunds)
	}
	if len(f.notify.refunds) != 1 || f.notify.refunds[0].Reason != latePaymentReason {
		t.Errorf("want one notice with the late-payment reason; got %+v", f.notify.refunds)
	}
	if len(f.notify.confirmations) != 0 {
		t.Error("a refunded booking must not send a confirmation")
	}
}

// The previous behavior holds for every other cancellation: a late payment on
// a booking the owner or the client cancelled is refunded even into free
// hours, with the late-payment reason, and never confirms it.
func TestALatePaymentOnABookingTheOwnerCancelledIsStillRefunded(t *testing.T) {
	f := newFixture(t)
	booking, mpPayment := expiredCheckout(f)
	booking.CancelledByHoldExpiry = false

	if err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
		t.Fatalf("got %v", err)
	}

	if len(f.provider.refunds) != 1 {
		t.Fatalf("want the payment refunded; got %v", f.provider.refunds)
	}
	if len(f.notify.refunds) != 1 || f.notify.refunds[0].Reason != latePaymentReason {
		t.Errorf("want one notice with the late-payment reason; got %+v", f.notify.refunds)
	}
	if f.payments.confirmedBooking != nil && f.payments.confirmedBooking.Status == bookingstore.BookingStatusConfirmed {
		t.Error("an owner-cancelled booking must never be written confirmed")
	}
	if len(f.notify.confirmations) != 0 {
		t.Error("a refunded booking must not send a confirmation")
	}
}
