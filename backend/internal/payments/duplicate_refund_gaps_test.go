package payments

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// The duplicate payment is recorded and its refund claimed in one write. When that
// write fails, nothing is left behind, so MercadoPago's retry of the event finds no
// row for the payment and refunds it from the start. A row written without its claim
// read as already processed on the retry, and the money stayed held for good.
func TestADuplicatePaymentWhoseClaimFailedIsRefundedOnTheRetry(t *testing.T) {
	withCapturedSentryEvents(t)
	f := newFixture(t)
	complexID := uuid.New()
	booking, deposit := paidBooking(complexID)
	booking.Status = bookingstore.BookingStatusConfirmed
	recorded := "mp-OLD"
	deposit.MPPaymentID = &recorded
	f.payments.byBooking = deposit
	f.payments.byBookingAll = []*paymentstore.Payment{deposit}
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	f.provider.payment = &mp.Payment{
		ID: 123, Status: "approved", TransactionAmount: 1500,
		CollectorID: 111111111, ExternalReference: booking.ID.String(),
	}

	f.payments.claimErr = errors.New("database unavailable")
	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err == nil {
		t.Fatal("a claim that failed must fail the event, so MercadoPago's retry comes back")
	}
	if len(f.payments.insertedRows) != 0 {
		t.Fatalf("a failed claim must leave no row behind; got %d rows", len(f.payments.insertedRows))
	}

	f.payments.claimErr = nil
	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatalf("the retry must go through; got %v", err)
	}
	if len(f.provider.refunds) != 1 {
		t.Fatalf("the retry must refund the duplicate payment; got %d refunds", len(f.provider.refunds))
	}
	if len(f.payments.insertedRows) != 1 || f.payments.insertedRows[0].Status != "refunded" {
		t.Errorf("the duplicate payment must end recorded once and refunded; got %d rows", len(f.payments.insertedRows))
	}
}

// A duplicate payment refunded from MercadoPago's own panel is a refund of that
// payment alone. The booking still holds the payment that settled it, so it stays
// confirmed and nobody is told it was cancelled.
func TestARefundFromMercadoPagoOfADuplicatePaymentLeavesThePaidBookingAlone(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, deposit := paidBooking(complexID)
	booking.Status = bookingstore.BookingStatusConfirmed
	recorded := "mp-OLD"
	deposit.MPPaymentID = &recorded
	dupID := "mp-123"
	duplicate := &paymentstore.Payment{
		ID: uuid.New(), BookingID: booking.ID, ComplexID: complexID,
		Amount: deposit.Amount, ServiceFee: deposit.ServiceFee,
		Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &dupID,
	}
	f.payments.byMPID = duplicate
	f.payments.byBooking = duplicate
	f.payments.byBookingAll = []*paymentstore.Payment{deposit, duplicate}
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	paid := float64(duplicate.Amount+duplicate.ServiceFee) / 100
	f.provider.payment = &mp.Payment{
		ID: 123, Status: "refunded", TransactionAmount: paid, TransactionAmountRefunded: paid,
		CollectorID: 111111111, ExternalReference: booking.ID.String(),
	}

	if err := f.service.processPaymentWebhook(t.Context(), dupID); err != nil {
		t.Fatalf("the refund must be recorded; got %v", err)
	}

	if duplicate.Status != "refunded" {
		t.Errorf("the duplicate payment must read refunded; got %q", duplicate.Status)
	}
	if b := f.payments.confirmedBooking; b != nil && b.Status != bookingstore.BookingStatusConfirmed {
		t.Errorf("the booking still holds its first payment, so it must stay confirmed; got %q", b.Status)
	}
	if booking.Status != bookingstore.BookingStatusConfirmed {
		t.Errorf("the booking must stay confirmed; got %q", booking.Status)
	}
	if len(f.notify.ownerRefunds) != 0 || len(f.waitlist.freed) != 0 {
		t.Error("nothing cancelled the booking, so the owner is not told so and no slot goes to the waitlist")
	}
}
