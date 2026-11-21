//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// dueAttemptOf brings a claim's attempt row due and reads it back the way the
// retry job does.
func dueAttemptOf(t *testing.T, f *datatest.Fixture, attemptID uuid.UUID) *paymentstore.FailedRefund {
	t.Helper()
	if _, err := f.DB.Exec(context.Background(),
		`UPDATE failed_refunds SET next_retry_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, attemptID); err != nil {
		t.Fatalf("bringing the attempt due: %v", err)
	}
	due, err := f.Stores.FailedRefunds.GetPendingDue(context.Background())
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	for _, fr := range due {
		if fr.ID == attemptID {
			return fr
		}
	}
	t.Fatalf("attempt %s was not due", attemptID)
	return nil
}

// S18: the cause a claim is made with is stored with its attempt and read back
// by the retry job, so a refund finished later is told the same reason.
func TestARefundCauseRoundTripsThroughTheAttemptRow(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())

	settled := f.CreateBooking(t, datatest.BookingOptions{Status: "confirmed"})
	mpDuplicate := "mp-" + uuid.NewString()
	duplicate, err := f.Stores.Payments.InsertAndClaimRefundWithCause(ctx, &paymentstore.Payment{
		BookingID: settled.ID, ComplexID: f.ComplexID, Amount: 150_000, ServiceFee: 7_500,
		Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &mpDuplicate,
	}, paymentstore.RefundCauseDuplicatePayment)
	if err != nil {
		t.Fatalf("InsertAndClaimRefundWithCause: %v", err)
	}
	if duplicate.Cause != paymentstore.RefundCauseDuplicatePayment {
		t.Errorf("the claim must carry its cause; got %q", duplicate.Cause)
	}
	if got := dueAttemptOf(t, f, duplicate.AttemptID).Cause; got != paymentstore.RefundCauseDuplicatePayment {
		t.Errorf("the retry job must read the duplicate cause back; got %q", got)
	}

	late := f.CreateBooking(t, datatest.BookingOptions{Status: "confirmed", StartTime: "12:00", EndTime: "13:30"})
	mpLate := "mp-" + uuid.NewString()
	latePayment := &paymentstore.Payment{
		BookingID: late.ID, ComplexID: f.ComplexID, Amount: 150_000,
		Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &mpLate,
	}
	if err := f.Stores.Payments.Insert(ctx, latePayment); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	lateClaim, err := f.Stores.Payments.ClaimRefundWithCause(ctx, latePayment.ID, paymentstore.RefundCauseLatePayment)
	if err != nil {
		t.Fatalf("ClaimRefundWithCause: %v", err)
	}
	if got := dueAttemptOf(t, f, lateClaim.AttemptID).Cause; got != paymentstore.RefundCauseLatePayment {
		t.Errorf("the retry job must read the late-payment cause back; got %q", got)
	}
}

// S18: a refund claimed without a cause stores NULL, as every row written before
// the column did, and reads back as no cause.
func TestARefundClaimedWithoutACauseStoresNull(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	booking := f.CreateBooking(t, datatest.BookingOptions{Status: "confirmed"})
	mpID := "mp-" + uuid.NewString()
	payment := &paymentstore.Payment{
		BookingID: booking.ID, ComplexID: f.ComplexID, Amount: 150_000,
		Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &mpID,
	}
	if err := f.Stores.Payments.Insert(ctx, payment); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	claim, err := f.Stores.Payments.ClaimRefund(ctx, payment.ID)
	if err != nil {
		t.Fatalf("ClaimRefund: %v", err)
	}
	var isNull bool
	if err := f.DB.QueryRow(context.Background(),
		`SELECT cause IS NULL FROM failed_refunds WHERE id = $1`, claim.AttemptID).Scan(&isNull); err != nil {
		t.Fatalf("reading the attempt: %v", err)
	}
	if !isNull {
		t.Error("a refund somebody asked for must store no cause")
	}
	if got := dueAttemptOf(t, f, claim.AttemptID).Cause; got != paymentstore.RefundCauseNone {
		t.Errorf("no cause must read back empty; got %q", got)
	}
}
