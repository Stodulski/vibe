package payments

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
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

// A second approved payment on a cancelled booking whose refund claim fails for
// a moment (the database away) must leave nothing recorded, so MercadoPago's
// redelivery of the event records and refunds it from the start. The row used
// to be committed before the claim, so the redelivery stopped at "payment
// already processed" and the second payment stayed captured for good.
func TestASecondPaymentWhoseClaimFailsIsRefundedOnRedelivery(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	booking.Status = bookingstore.BookingStatusCancelled
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
	f.payments.claimAmount = booking.DepositAmount + fee
	f.payments.claimErr = errors.New("connection reset")
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	f.provider.payment = mpPayment
	readableNotice(f, booking)

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err == nil {
		t.Fatal("a claim that failed for a moment must fail the event so it is redelivered")
	}
	if f.payments.byMPID != nil {
		t.Fatalf("nothing may be recorded for the second payment while its refund is unclaimed; got %+v", f.payments.byMPID)
	}

	f.payments.claimErr = nil
	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatalf("the redelivery must refund the second payment; got %v", err)
	}
	if len(f.provider.refunds) != 1 {
		t.Fatalf("want the second payment refunded once on redelivery; got %v", f.provider.refunds)
	}
}

// The orphan sweep and every cancel path refund from the payment rows, not from
// the booking's refund status: a cancelled booking already marked fully
// refunded by its first payment still owes a second MercadoPago payment that
// was recorded and never refunded, and AutoRefundIfPaid sends it back. It used
// to stop at "the booking was already refunded" and clear the marker silently.
func TestAutoRefundSendsBackAnUnrefundedRowOnABookingMarkedRefunded(t *testing.T) {
	for _, status := range []bookingstore.RefundStatus{bookingstore.RefundStatusFull, bookingstore.RefundStatusPending} {
		t.Run(string(status), func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			bookingID := uuid.New()
			booking := &bookingstore.Booking{
				ID: bookingID, ComplexID: complexID, ClientID: uuid.New(), CourtID: uuid.New(),
				Status: "cancelled", CollectionStatus: bookingstore.CollectionStatusDepositPaid,
				RefundStatus: status, Price: 500_000, DepositAmount: 150_000,
			}
			mpFirst, mpSecond := "mp-first", "mp-second"
			first := &paymentstore.Payment{
				ID: uuid.New(), BookingID: bookingID, ComplexID: complexID, Method: "mercadopago",
				Amount: 150_000, Status: "refunded", RefundAmount: 150_000, MPPaymentID: &mpFirst,
			}
			second := &paymentstore.Payment{
				ID: uuid.New(), BookingID: bookingID, ComplexID: complexID, Method: "mercadopago",
				Amount: 150_000, Status: "deposit_paid", MPPaymentID: &mpSecond,
			}
			f.payments.byBookingAll = []*paymentstore.Payment{first, second}
			f.payments.claimAmount = 150_000
			f.provider.refundAmount = 1_500.00
			f.complexes.complex = linkedComplex(complexID, "")
			f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}

			outcome := f.service.AutoRefundIfPaid(t.Context(), booking)

			if outcome.Result != paymentstore.RefundIssued {
				t.Fatalf("the unrefunded second payment must be refunded; got %q (reason %q)", outcome.Result, outcome.Reason)
			}
			if len(f.payments.claimed) != 1 || f.payments.claimed[0] != second.ID {
				t.Fatalf("only the second payment's row may be claimed; got %v", f.payments.claimed)
			}
		})
	}
}

// Two approved payments read a cancelled booking's untouched checkout row at
// once. The one whose write the row refuses (another payment took it after the
// read) is recorded on a row of its own, with its refund claimed in the same
// write, and refunded; the row the other payment took is not written over.
func TestAPaymentRefusedTheCheckoutRowIsRefundedOnARowOfItsOwn(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	booking.Status = bookingstore.BookingStatusCancelled
	mpPayment.CollectorID = 111111111
	mpPayment.ExternalReference = booking.ID.String()

	fee := pricing.ServiceFee(booking.DepositAmount)
	preference := "pref-1"
	checkout := &paymentstore.Payment{
		ID: uuid.New(), BookingID: booking.ID, ComplexID: complexID,
		Amount: booking.DepositAmount, ServiceFee: fee, Method: "mercadopago",
		Status: "unpaid", MPPreferenceID: &preference,
	}
	f.payments.byBooking = checkout
	f.payments.byBookingAll = []*paymentstore.Payment{checkout}
	f.payments.confirmErr = paymentstore.ErrPaymentRowTaken
	f.payments.claimAmount = booking.DepositAmount + fee
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	f.provider.payment = mpPayment
	readableNotice(f, booking)

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatalf("a payment refused the checkout row must still be refunded; got %v", err)
	}
	if len(f.payments.insertedRows) != 1 || f.payments.insertedRows[0].ID == checkout.ID {
		t.Fatalf("want the payment recorded on a row of its own; got %+v", f.payments.insertedRows)
	}
	if len(f.provider.refunds) != 1 {
		t.Errorf("want the payment refunded once; got %v", f.provider.refunds)
	}
}

// On a pending booking, an approved payment refused the checkout row (another
// payment settled it after the read) is not confirmed on a new row: the event
// is failed and retried, and the retry finds the booking settled and refunds it
// as a duplicate. Confirming it on a new row would record a second payment on
// the booking with no refund behind it.
func TestAPaymentRefusedTheCheckoutRowOfAPendingBookingIsRetried(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	mpPayment.CollectorID = 111111111
	mpPayment.ExternalReference = booking.ID.String()

	preference := "pref-1"
	checkout := &paymentstore.Payment{
		ID: uuid.New(), BookingID: booking.ID, ComplexID: complexID,
		Amount: booking.DepositAmount, ServiceFee: pricing.ServiceFee(booking.DepositAmount),
		Method: "mercadopago", Status: "unpaid", MPPreferenceID: &preference,
	}
	f.payments.byBooking = checkout
	f.payments.confirmErr = paymentstore.ErrPaymentRowTaken
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	f.provider.payment = mpPayment

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err == nil {
		t.Fatal("the event must fail so it is retried against the settled booking")
	}
	if len(f.payments.insertedRows) != 0 || len(f.provider.refunds) != 0 {
		t.Errorf("nothing may be recorded or refunded on this attempt; rows %+v, refunds %v",
			f.payments.insertedRows, f.provider.refunds)
	}
}
