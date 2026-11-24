package payments

import (
	"testing"

	"github.com/google/uuid"

	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/mp"
)

// MercadoPago's read of a payment as the platform can leave
// transaction_amount_refunded out, as it left collector_id out (2026-11-23).
// A refunded payment with no figure was then taken as refunded whole, and a
// partially refunded one that still reads 'approved' was not seen as refunded
// at all, so a later automatic refund claimed money already returned. The
// venue's own read of the payment carries the figure, and it decides.
func TestARefundedTotalThePlatformCannotSeeIsReadAsTheSeller(t *testing.T) {
	tests := []struct {
		name         string
		status       string
		statusDetail string
		// seller is what the venue's own read of the payment answers.
		seller      *mp.Payment
		sellerErr   error
		wantRefund  int
		wantFull    bool
		wantRetry   bool
		wantNoWrite bool
	}{
		{
			name:       "refunded in part, the seller's read says how much",
			status:     "refunded",
			seller:     &mp.Payment{ID: 123, Status: "refunded", TransactionAmount: 1_500.00, TransactionAmountRefunded: 500.00},
			wantRefund: 50_000,
		},
		{
			name:         "a partial refund on an approved payment, only the seller sees it",
			status:       "approved",
			statusDetail: "partially_refunded",
			seller:       &mp.Payment{ID: 123, Status: "approved", TransactionAmount: 1_500.00, TransactionAmountRefunded: 500.00},
			wantRefund:   50_000,
		},
		{
			name:     "charged back with no figure on either read, the whole of it is gone",
			status:   "charged_back",
			seller:   &mp.Payment{ID: 123, Status: "charged_back", TransactionAmount: 1_500.00},
			wantFull: true, wantRefund: 150_000,
		},
		{
			// Refunded is whole by definition: a figure the seller's read
			// cannot supply is not worth stalling the record of a refund on.
			name:       "refunded, MercadoPago fails the seller's read",
			status:     "refunded",
			sellerErr:  errProvider,
			wantFull:   true,
			wantRefund: 150_000,
		},
		{
			name:       "refunded, the venue's token has expired",
			status:     "refunded",
			sellerErr:  &mp.APIError{StatusCode: 401, Body: "invalid_token"},
			wantFull:   true,
			wantRefund: 150_000,
		},
		{
			name:        "charged back, MercadoPago fails the seller's read",
			status:      "charged_back",
			sellerErr:   errProvider,
			wantRetry:   true,
			wantNoWrite: true,
		},
		{
			name:         "partially refunded, MercadoPago fails the seller's read",
			status:       "approved",
			statusDetail: "partially_refunded",
			sellerErr:    errProvider,
			wantRetry:    true,
			wantNoWrite:  true,
		},
		{
			name:        "the seller's read answers for another payment",
			status:      "refunded",
			seller:      &mp.Payment{ID: 456, Status: "refunded", TransactionAmountRefunded: 500.00},
			wantRetry:   true,
			wantNoWrite: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, payment := paidBooking(complexID)
			f.bookings.booking = booking
			f.payments.byBooking = payment
			f.payments.byMPID = payment
			f.complexes.complex = linkedComplex(complexID, "111111111")
			f.clients.client = &clientstore.Client{ID: booking.ClientID}
			f.provider.payment = &mp.Payment{ID: 123, Status: tt.status, StatusDetail: tt.statusDetail, TransactionAmount: 1_500.00}
			f.provider.sellerPayment, f.provider.sellerPaymentErr = tt.seller, tt.sellerErr
			withCapturedSentryEvents(t)

			err := f.service.processPaymentWebhook(t.Context(), "123")

			if tt.wantRetry != (err != nil) {
				t.Fatalf("retryable = %v, want %v (err %v)", err != nil, tt.wantRetry, err)
			}
			if !calledAs(f.provider.callers, mustSeller(t, sellerTestToken)) {
				t.Errorf("the payment must be read again as the venue's seller account; callers %v", f.provider.callers)
			}
			if tt.wantNoWrite {
				if f.payments.updated != nil || f.payments.confirmed != nil {
					t.Errorf("an unread refund total must write nothing; updated %+v confirmed %+v", f.payments.updated, f.payments.confirmed)
				}
				return
			}
			written := f.payments.updated
			if written == nil {
				written = f.payments.confirmed
			}
			if written == nil {
				t.Fatal("the refund must be recorded on the payment")
			}
			if written.RefundAmount != tt.wantRefund {
				t.Errorf("refund_amount = %d, want %d (the seller's figure)", written.RefundAmount, tt.wantRefund)
			}
			if full := written.Status == "refunded"; full != tt.wantFull {
				t.Errorf("payment status %q: fully refunded = %v, want %v", written.Status, full, tt.wantFull)
			}
		})
	}
}

// The seller is only asked when the platform's read has no figure: a refund
// total the platform sees stays the only read.
func TestARefundedTotalThePlatformSeesIsNotReadAgain(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	f.bookings.booking = booking
	f.payments.byBooking = payment
	f.payments.byMPID = payment
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.provider.payment = &mp.Payment{ID: 123, Status: "refunded", TransactionAmount: 1_500.00, TransactionAmountRefunded: 500.00}
	f.provider.sellerPayment = &mp.Payment{ID: 123, Status: "refunded", TransactionAmountRefunded: 1_500.00}
	withCapturedSentryEvents(t)

	if err := f.service.processPaymentWebhook(t.Context(), "123"); err != nil {
		t.Fatalf("processPaymentWebhook: %v", err)
	}
	if calledAs(f.provider.callers, mustSeller(t, sellerTestToken)) {
		t.Error("the seller must not be asked when the platform's read already carries the refunded total")
	}
}

// A stored credential that will not decrypt cannot read the payment as the
// seller. For a payment MercadoPago calls refunded the whole of it is back, so
// it is recorded so; a partial refund still waits for a figure.
func TestARefundedPaymentWithAnUnreadableCredentialIsRecordedWhole(t *testing.T) {
	for _, tt := range []struct {
		name, status, detail string
		wantRetry            bool
	}{
		{name: "refunded", status: "refunded"},
		{name: "partially refunded", status: "approved", detail: "partially_refunded", wantRetry: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, payment := paidBooking(complexID)
			f.bookings.booking = booking
			f.payments.byBooking = payment
			f.payments.byMPID = payment
			f.complexes.complex = complexstore.NewComplexWithUnreadableCredentialForTest(complexID)
			f.clients.client = &clientstore.Client{ID: booking.ClientID}
			f.provider.payment = &mp.Payment{ID: 123, Status: tt.status, StatusDetail: tt.detail, TransactionAmount: 1_500.00}
			withCapturedSentryEvents(t)

			err := f.service.processPaymentWebhook(t.Context(), "123")

			if tt.wantRetry {
				if err == nil {
					t.Error("a partial refund with no readable figure must be retried")
				}
				return
			}
			if err != nil {
				t.Fatalf("a refunded payment must be recorded; got %v", err)
			}
			written := f.payments.updated
			if written == nil {
				written = f.payments.confirmed
			}
			if written == nil || written.Status != "refunded" || written.RefundAmount != 150_000 {
				t.Errorf("want the payment recorded refunded in whole; got %+v", written)
			}
		})
	}
}
