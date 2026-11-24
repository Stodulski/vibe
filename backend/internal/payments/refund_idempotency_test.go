package payments

import (
	"testing"

	"github.com/google/uuid"

	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// MercadoPago deduplicates a refund by its X-Idempotency-Key. Keyed on the
// payment and the amount, two legitimate refunds of the same sum on one
// payment (a partial refund, then another of the same size) collapsed into
// one: the second answered with the first and the client never got the rest.
// Each attempt row is one refund, so the key follows the attempt: two attempts
// send two keys, and the retry job working the same attempt again sends the
// same one, which is what still stops a replay from moving the money twice.
func TestEachRefundAttemptReachesMercadoPagoUnderItsOwnKey(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	f.complexes.complex = linkedComplex(complexID, "")
	mpID := "mp-" + uuid.NewString()
	paymentID := uuid.New()
	first := &paymentstore.FailedRefund{ID: uuid.New(), PaymentID: paymentID, BookingID: uuid.New(), ComplexID: complexID, Amount: 50_000, MPPaymentID: mpID}
	second := &paymentstore.FailedRefund{ID: uuid.New(), PaymentID: paymentID, BookingID: first.BookingID, ComplexID: complexID, Amount: 50_000, MPPaymentID: mpID}
	f.failedRefunds.pending = []*paymentstore.FailedRefund{first, second}
	withCapturedSentryEvents(t)

	f.service.RetryFailedRefunds(t.Context())
	f.service.RetryFailedRefunds(t.Context())

	keys := f.provider.refundKeys
	if len(keys) != 4 {
		t.Fatalf("want 4 refund calls (two attempts, twice); got %d: %v", len(keys), keys)
	}
	if keys[0] == "" || keys[1] == "" {
		t.Fatalf("every refund must carry an idempotency key; got %v", keys)
	}
	if keys[0] == keys[1] {
		t.Errorf("two refunds of the same amount on one payment must not share a key; both sent %q", keys[0])
	}
	if keys[2] != keys[0] || keys[3] != keys[1] {
		t.Errorf("a retry of an attempt must send that attempt's key again; first pass %v, second pass %v", keys[:2], keys[2:])
	}
}

// A row queued before the refund key moved from "payment-amount" to the attempt
// may hold a refund MercadoPago already made under the old key and this system
// never recorded. Sent again under the new key, MercadoPago would take it as a
// new refund whenever the payment still has room (the row understating what
// MercadoPago charged), and the client would be paid twice. The retry job
// reads the payment as the seller first and settles the row when the refund is
// already there, without asking MercadoPago to refund anything.
func TestTheRetryJobSettlesARefundThePaymentAlreadyShowsWithoutRefundingAgain(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	f.complexes.complex = linkedComplex(complexID, "")
	mpID := "123"
	bookingID := uuid.New()
	row := &paymentstore.Payment{ID: uuid.New(), BookingID: bookingID, ComplexID: complexID, Amount: 50_000, Status: "refund_pending", MPPaymentID: &mpID}
	f.payments.byBookingAll = []*paymentstore.Payment{row}
	attempt := &paymentstore.FailedRefund{ID: uuid.New(), PaymentID: row.ID, BookingID: bookingID, ComplexID: complexID, Amount: 50_000, MPPaymentID: mpID}
	f.failedRefunds.pending = []*paymentstore.FailedRefund{attempt}
	// MercadoPago charged 2000 (the row says 500) and already refunded 500.
	f.provider.sellerPayment = &mp.Payment{ID: 123, Status: "approved", TransactionAmount: 2_000, TransactionAmountRefunded: 500}
	withCapturedSentryEvents(t)

	f.service.RetryFailedRefunds(t.Context())

	if len(f.provider.refunds) != 0 {
		t.Errorf("a refund the payment already shows must not be sent again; sent %v", f.provider.refunds)
	}
	if len(f.payments.recordedSuccess) != 1 || f.payments.recordedSuccess[0].AttemptID != attempt.ID {
		t.Errorf("the attempt must be settled as refunded; got %+v", f.payments.recordedSuccess)
	}
}

// The refund the claim path sends first and the one the retry job later sends
// for the same attempt row carry the same key, so a retry of a refund that
// went through is answered with that refund instead of a second one.
func TestTheClaimPathAndTheRetryJobSendTheSameKeyForOneAttempt(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	booking.Status = "cancelled"
	f.payments.byBooking = payment
	f.payments.claimAmount = 150_000
	f.complexes.complex = linkedComplex(complexID, "")
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.provider.refundErr = errProvider
	withCapturedSentryEvents(t)

	f.service.AutoRefundIfPaid(t.Context(), booking)
	if len(f.payments.recordedFailure) != 1 || len(f.provider.refundKeys) != 1 {
		t.Fatalf("want one refund call, requeued once; requeues %v, keys %v", f.payments.recordedFailure, f.provider.refundKeys)
	}
	claimed := f.payments.recordedFailure[0].claim

	f.provider.refundErr = nil
	f.failedRefunds.pending = []*paymentstore.FailedRefund{{
		ID: claimed.AttemptID, PaymentID: claimed.PaymentID, BookingID: claimed.BookingID,
		ComplexID: claimed.ComplexID, Amount: claimed.RefundCentavos, MPPaymentID: claimed.MPPaymentID,
	}}
	f.service.RetryFailedRefunds(t.Context())

	keys := f.provider.refundKeys
	if len(keys) != 2 || keys[0] != keys[1] {
		t.Errorf("the claim's first call and the retry of its row must send one key; got %v", keys)
	}
}
