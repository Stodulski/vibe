package payments

import (
	"testing"

	"github.com/google/uuid"

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
