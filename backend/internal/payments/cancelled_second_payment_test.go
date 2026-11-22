package payments

import (
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/pricing"
)

// A cancelled booking whose first MercadoPago payment was already refunded
// receives a second approved payment. That payment is the client's money too:
// it gets a row and a refund of its own, the first payment's row is left as it
// was, and a replay of the event refunds nothing twice. The second refund used
// to be claimed against the first payment's row, which the claim refused as
// already refunded, so the second payment stayed captured with only a log.
func TestASecondPaymentOnACancelledBookingGetsItsOwnRefund(t *testing.T) {
	cases := []struct {
		name         string
		byHoldExpiry bool
		slotErr      error
		reason       string
	}{
		{name: "cancelled by the owner or the client", reason: latePaymentReason},
		{name: "released by the visitor", byHoldExpiry: true, slotErr: bookingstore.ErrBookingCancelled, reason: latePaymentReason},
		{name: "expired with its hours taken", byHoldExpiry: true, slotErr: bookingstore.ErrSlotUnavailable, reason: slotTakenReason},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, mpPayment := pendingBooking(complexID)
			booking.Status = bookingstore.BookingStatusCancelled
			booking.CancelledByHoldExpiry = tc.byHoldExpiry
			booking.CollectionStatus = bookingstore.CollectionStatusDepositPaid
			booking.RefundStatus = bookingstore.RefundStatusFull
			mpPayment.CollectorID = 111111111
			mpPayment.ExternalReference = booking.ID.String()

			fee := pricing.ServiceFee(booking.DepositAmount)
			firstID := "mp-111"
			first := &paymentstore.Payment{
				ID: uuid.New(), BookingID: booking.ID, ComplexID: complexID,
				Amount: booking.DepositAmount, ServiceFee: fee, Method: "mercadopago",
				Status: "refunded", RefundAmount: booking.DepositAmount + fee, MPPaymentID: &firstID,
			}
			f.payments.byBooking = first
			f.payments.byBookingAll = []*paymentstore.Payment{first}
			f.payments.slotErr = tc.slotErr
			f.payments.claimAmount = booking.DepositAmount + fee
			f.complexes.complex = linkedComplex(complexID, "111111111")
			f.bookings.booking = booking
			f.provider.payment = mpPayment
			readableNotice(f, booking)

			if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
				t.Fatalf("the second payment must be refunded without an error; got %v", err)
			}

			if len(f.provider.refunds) != 1 {
				t.Fatalf("want the second payment refunded once; got %v", f.provider.refunds)
			}
			if *first.MPPaymentID != firstID || first.Status != "refunded" || first.RefundAmount != booking.DepositAmount+fee {
				t.Errorf("the first payment's row must stay as it was; got mp=%s status=%s refund=%d",
					*first.MPPaymentID, first.Status, first.RefundAmount)
			}
			var second *paymentstore.Payment
			for _, p := range f.payments.byBookingAll {
				if p.MPPaymentID != nil && *p.MPPaymentID == "mp-123" {
					second = p
				}
			}
			if second == nil || second.ID == first.ID {
				t.Fatalf("the second payment must get a row of its own; got %+v", second)
			}
			if second.Status != "refunded" {
				t.Errorf("the second payment's row must read refunded; got %q", second.Status)
			}
			if len(f.notify.refunds) != 1 || f.notify.refunds[0].Reason != tc.reason {
				t.Errorf("want the client told once, with the reason; got %+v", f.notify.refunds)
			}

			if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
				t.Fatalf("a replay must be a no-op; got %v", err)
			}
			if len(f.provider.refunds) != 1 || len(f.notify.refunds) != 1 {
				t.Errorf("a replay must not refund or notify again; refunds %v, notices %d",
					f.provider.refunds, len(f.notify.refunds))
			}
		})
	}
}
