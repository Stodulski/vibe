package payments

import (
	"testing"

	"github.com/google/uuid"

	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// MercadoPago's transaction_amount_refunded is the running total refunded on a
// payment, not the amount of the refund that produced the event. A partial refund
// after an earlier one used to be recorded in the trail as the whole running total,
// so the owner's history said more money came back on that event than did.
func TestAMercadoPagoRunningTotalIsRecordedAsWhatMovedOnThisEvent(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	payment.ServiceFee = 100_000   // total paid: 250_000
	payment.RefundAmount = 100_000 // an earlier partial refund already recorded
	f.bookings.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe"}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}

	// MercadoPago now reports 2000.00 refunded in total: 1000.00 more than before.
	err := f.service.processRefundedPayment(t.Context(), payment, &mp.Payment{
		ID: 123, Status: "refunded", TransactionAmount: 2500, TransactionAmountRefunded: 2000,
	}, "mp-123")
	if err != nil {
		t.Fatalf("recording the provider's partial refund must not fail: %v", err)
	}

	entry := only(t, f, "refund_external")
	if got := value(t, entry)["amount_centavos"]; got != float64(100_000) {
		t.Errorf("the event moved 1000.00, not the running total; want 100000, got %v", got)
	}
}

// A refund MercadoPago rejects because the money already moved is resolved by
// reading the payment back. The claim asked for 1500.00; the payment shows
// 2500.00 refunded in total, of which 1000.00 was already ours. Only the claimed
// 1500.00 may be recorded as this refund, not the running total.
func TestARejectedRefundRecordsOnlyWhatThisClaimMovedNotTheRunningTotal(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	payment.ServiceFee = 100_000 // total paid: 250_000
	payment.RefundAmount = 100_000
	booking.Status = "cancelled"
	f.payments.byBooking = payment
	f.payments.claimAmount = 150_000
	f.complexes.complex = linkedComplex(complexID, "")
	f.clients.client = &clientstore.Client{ID: booking.ClientID}

	f.provider.refundErr = &mp.APIError{StatusCode: 400, Body: `{"message":"payment already refunded"}`}
	f.provider.payment = &mp.Payment{ID: 123, Status: "refunded", TransactionAmountRefunded: 2500.00}

	outcome := f.service.AutoRefundIfPaid(t.Context(), booking)

	if len(f.payments.recordedSuccess) != 1 {
		t.Fatalf("the money already moved, so the refund must resolve as a success; got %d", len(f.payments.recordedSuccess))
	}
	if got := f.payments.recordedSuccess[0].RefundCentavos; got != 150_000 {
		t.Errorf("the claim moved 150000 on top of the 100000 already recorded; want 150000 recorded, got %d", got)
	}
	if outcome.Result != paymentstore.RefundIssued {
		t.Errorf("want result=%q; got %q (reason %q)", paymentstore.RefundIssued, outcome.Result, outcome.Reason)
	}
}
