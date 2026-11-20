package payments

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// S17: a payment that lands on a spot that is gone is recorded and its refund
// claimed in one write. When the claim fails, nothing reads as already processed,
// so MercadoPago's retry of the event refunds it from the start. Settling the
// checkout row first and claiming after left a row the retry skipped as handled,
// and the money stayed held for good.
func TestAPaymentForAGoneSpotWhoseClaimFailedIsRefundedOnTheRetry(t *testing.T) {
	withCapturedSentryEvents(t)
	f, reg, paid, _ := registrationFixture(t)
	f.payments.byRegistration = []*paymentstore.Payment{checkoutRow(reg)}
	f.payments.regConfirmErr = paymentstore.ErrRegistrationUnavailable
	f.provider.payment = paid

	f.payments.claimErr = errors.New("database unavailable")
	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err == nil {
		t.Fatal("a claim that failed must fail the event, so MercadoPago's retry comes back")
	}
	// What a real lookup by MercadoPago id would find now: a row the first
	// attempt left carrying this payment.
	if f.payments.updated != nil && f.payments.updated.MPPaymentID != nil {
		f.payments.byMPID = f.payments.updated
	}
	if len(f.payments.insertedRows) == 1 {
		f.payments.byMPID = f.payments.insertedRows[0]
	}

	f.payments.claimErr = nil
	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatalf("the retry must go through; got %v", err)
	}
	if len(f.provider.refunds) != 1 {
		t.Fatalf("the retry must refund the payment for the gone spot; got %d refunds", len(f.provider.refunds))
	}
}

// S17: a collected payment recorded against a registration it never confirmed is
// money a crash left between recording it and claiming its refund (before the
// two were one write). Its webhook coming back finishes the refund. A payment
// that confirmed its registration is not touched, whatever became of the spot
// since: a late cancellation keeps it.
func TestARecordedPaymentOfARegistrationItNeverConfirmedIsRefundedOnTheRetry(t *testing.T) {
	for name, tc := range map[string]struct {
		confirmed  bool
		wantRefund bool
	}{
		"never confirmed": {confirmed: false, wantRefund: true},
		"confirmed once":  {confirmed: true, wantRefund: false},
	} {
		t.Run(name, func(t *testing.T) {
			f, reg, paid, _ := registrationFixture(t)
			reg.Status = eventstore.RegistrationCancelled
			if tc.confirmed {
				at := time.Now().Add(-time.Hour)
				reg.ConfirmedAt = &at
			}
			mpID := "mp-123"
			row := checkoutRow(reg)
			row.Status, row.MPPaymentID = "deposit_paid", &mpID
			f.payments.byMPID = row
			f.provider.payment = paid

			if err := f.service.processPaymentWebhook(t.Context(), mpID); err != nil {
				t.Fatal(err)
			}
			if got := len(f.provider.refunds) == 1; got != tc.wantRefund {
				t.Fatalf("refunded = %v, want %v (refunds %v)", got, tc.wantRefund, f.provider.refunds)
			}
		})
	}
}

// unreadableLedger is a ledger whose registration payments cannot be read.
type unreadableLedger struct{ RegistrationLedger }

func (unreadableLedger) ListByRegistrationID(context.Context, uuid.UUID) ([]*paymentstore.Payment, error) {
	return nil, errors.New("database unavailable")
}

// S17: when the payments of a registration cannot be read, nobody knows whether
// money is owed: the outcome says so, rather than reading as a refund on its way.
func TestARefundWhoseLedgerCannotBeReadSaysItIsUnknown(t *testing.T) {
	f, reg, _, _ := registrationFixture(t)
	f.service.registrationLedger = unreadableLedger{f.payments}
	got := f.service.RefundRegistration(t.Context(), reg.ID)
	if !got.LedgerUnread || got.Result != paymentstore.RefundManual || len(f.provider.refunds) != 0 {
		t.Fatalf("outcome = %+v; want a manual outcome marked as unread, nothing refunded", got)
	}
}
