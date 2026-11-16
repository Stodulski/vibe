package payments

import (
	"testing"

	"github.com/google/uuid"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	"github.com/stodulski/vibe-server/internal/mp"
)

// A partial refund made from MercadoPago's own panel leaves the payment
// 'approved' (status_detail partially_refunded) with the amount refunded so far.
// The webhook used to skip every recorded payment that was not 'refunded', so the
// money that went back was never recorded anywhere. It is now: the payment holds
// the refunded amount and the booking, which still has money in it, stays on.
func TestAPartialRefundFromTheMercadoPagoPanelIsRecorded(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	f.payments.byMPID = payment
	f.payments.byBooking = payment
	f.bookings.booking = booking
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}
	f.provider.payment = &mp.Payment{
		ID: 123, Status: "approved", StatusDetail: "partially_refunded",
		TransactionAmount: 1500, TransactionAmountRefunded: 500,
		CollectorID: 111111111, ExternalReference: booking.ID.String(),
	}

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatalf("recording a panel refund must not fail: %v", err)
	}

	if f.payments.updated == nil || f.payments.updated.RefundAmount != 50_000 {
		t.Fatalf("want the payment to record 50_000 refunded; got %+v", f.payments.updated)
	}
	if f.payments.updated.Status == "refunded" {
		t.Errorf("a partial refund must not mark the payment refunded")
	}
	if booking.Status != bookingstore.BookingStatusConfirmed {
		t.Errorf("a booking that still holds money stays confirmed; got %q", booking.Status)
	}
	if len(f.provider.refunds) != 0 {
		t.Errorf("the backend must not issue a refund of its own; got %v", f.provider.refunds)
	}
}

// The echo of a refund Vibe already recorded carries nothing new: the payment
// already holds what MercadoPago reports, so nothing is written again.
func TestTheEchoOfARecordedPartialRefundWritesNothing(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	payment.RefundAmount = 50_000
	f.payments.byMPID = payment
	f.payments.byBooking = payment
	f.bookings.booking = booking
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.provider.payment = &mp.Payment{
		ID: 123, Status: "approved", StatusDetail: "partially_refunded",
		TransactionAmount: 1500, TransactionAmountRefunded: 500,
		CollectorID: 111111111, ExternalReference: booking.ID.String(),
	}

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.payments.updated != nil {
		t.Errorf("an already recorded refund must not be written again; got %+v", f.payments.updated)
	}
}

// A second partial refund tells the client what came back this time, not the
// running total: 500 then 300 must not read as 500 then 800.
func TestASecondPanelRefundAnnouncesOnlyWhatMovedThisTime(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	payment.RefundAmount = 50_000
	f.bookings.booking = booking
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}

	err := f.service.processRefundedPayment(t.Context(), payment, &mp.Payment{
		ID: 123, Status: "approved", StatusDetail: "partially_refunded",
		TransactionAmount: 1500, TransactionAmountRefunded: 800,
	}, "mp-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.notify.refunds) != 1 || f.notify.refunds[0].Amount != "$300" {
		t.Fatalf("want one notice of $300; got %+v", f.notify.refunds)
	}
}

// A row that already holds everything it can hold (refund_amount is capped at
// the row's own total) is not routed again on every later webhook just because
// MercadoPago's figure is larger than the row could ever store.
func TestARefundAboveWhatTheRowCanHoldIsNotRoutedAgain(t *testing.T) {
	_, payment := paidBooking(uuid.New())
	payment.RefundAmount = payment.Amount + payment.ServiceFee
	if refundedBeyondRecorded(payment, &mp.Payment{TransactionAmountRefunded: 9_999}) {
		t.Error("a row already holding its whole total must not be routed again")
	}
}
