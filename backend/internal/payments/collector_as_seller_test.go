package payments

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/mp"
)

// MercadoPago answered the platform's GET /v1/payments for a payment a venue
// collected with collector_id 0 (2026-11-23, an approved $7000 deposit). The
// collector check refused it as unverifiable, so the client paid and the booking
// never confirmed. The same payment read as the venue's own seller account names
// the collector, and that read decides it.
func TestAPaymentWithNoCollectorForThePlatformIsCheckedAsTheSeller(t *testing.T) {
	tests := []struct {
		name string
		// seller is what the venue's own read of the payment answers.
		seller    *mp.Payment
		sellerErr error
		// credential is the state of the venue's stored seller credential.
		credential  credentialState
		wantConfirm bool
		wantRetry   bool
		wantAlert   string
	}{
		{
			name:        "the seller's read names the venue",
			seller:      &mp.Payment{ID: 123, Status: "approved", CollectorID: 111111111},
			wantConfirm: true,
		},
		{
			name:      "the seller's read names another seller",
			seller:    &mp.Payment{ID: 123, Status: "approved", CollectorID: 999999999},
			wantAlert: "FRAUD ALERT: collector mismatch",
		},
		{
			name:      "the seller's read has no collector either",
			seller:    &mp.Payment{ID: 123, Status: "approved"},
			wantAlert: "COLLECTOR UNVERIFIED: missing MercadoPago identity",
		},
		{
			name:      "the seller's read answers for another payment",
			seller:    &mp.Payment{ID: 456, Status: "approved", CollectorID: 111111111},
			wantRetry: true,
		},
		{
			name:      "MercadoPago fails the seller's read",
			sellerErr: errProvider,
			wantRetry: true,
		},
		{
			name:      "the venue's token has expired",
			sellerErr: &mp.APIError{StatusCode: 401, Body: "invalid_token"},
			wantRetry: true,
		},
		{
			name:      "the venue's account cannot see the payment",
			sellerErr: fmt.Errorf("fetch: %w", &mp.APIError{StatusCode: 404, Body: "Payment not found"}),
			wantAlert: "COLLECTOR UNVERIFIED: missing MercadoPago identity",
		},
		{
			name:       "the venue never connected MercadoPago",
			seller:     &mp.Payment{ID: 123, Status: "approved", CollectorID: 111111111},
			credential: notConnected,
			wantAlert:  "COLLECTOR UNVERIFIED: missing MercadoPago identity",
		},
		{
			name:       "the venue's stored credential will not decrypt",
			seller:     &mp.Payment{ID: 123, Status: "approved", CollectorID: 111111111},
			credential: unreadable,
			wantRetry:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentryEvents := withCapturedSentryEvents(t)
			f := newFixture(t)
			complexID := uuid.New()
			booking, mpPayment := pendingBooking(complexID)
			mpPayment.CollectorID = 0
			f.bookings.booking = booking
			f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", LastName: "Diaz"}
			f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}
			complex := linkedComplex(complexID, "111111111")
			switch tt.credential {
			case readable:
			case notConnected:
				complex = complexstore.NewComplexForTest(complexID, nil, nil)
			case unreadable:
				complex = complexstore.NewComplexWithUnreadableCredentialForTest(complexID)
			}
			sellerID := "111111111"
			complex.MPUserID = &sellerID
			complex.Name, complex.Slug = "Vibe", "vibe"
			f.complexes.complex = complex
			f.provider.sellerPayment, f.provider.sellerPaymentErr = tt.seller, tt.sellerErr

			err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "123")

			confirmed := f.payments.inserted != nil && booking.Status == "confirmed" && len(f.notify.confirmations) == 1
			if confirmed != tt.wantConfirm {
				t.Errorf("confirmed = %v (status=%q, notices=%d), want %v", confirmed, booking.Status, len(f.notify.confirmations), tt.wantConfirm)
			}
			if !tt.wantConfirm && (f.payments.inserted != nil || len(f.notify.confirmations) != 0) {
				t.Error("an unproven collector must write no payment and send no confirmation")
			}
			if tt.wantRetry && err == nil {
				t.Error("a failed read of the payment is an outage and must come back as retryable")
			}
			if !tt.wantRetry && err != nil {
				t.Errorf("a verdict about the collector is final; got error %v", err)
			}
			if tt.wantAlert != "" {
				msg, found := findMessage(sentryEvents.messages(), tt.wantAlert)
				if !found {
					t.Errorf("want Sentry alert %q; captured %q", tt.wantAlert, sentryEvents.messages())
				}
				// An unverified collector is refused until its cause is fixed;
				// the alert says how to process the payment again then.
				if strings.HasPrefix(tt.wantAlert, "COLLECTOR UNVERIFIED") && !strings.Contains(msg, "go run ./cmd/replaypayment 123") {
					t.Errorf("the alert must carry the payment's replay command; got %q", msg)
				}
			}
			if tt.credential == readable && !calledAs(f.provider.callers, mustSeller(t, sellerTestToken)) {
				t.Errorf("the payment must be read again as the venue's seller account; callers %v", f.provider.callers)
			}
		})
	}
}

// The platform's read naming the collector stays the only read: the seller is
// asked only when the platform cannot say.
func TestAPaymentWhoseCollectorThePlatformSeesIsNotReadAgain(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	mpPayment.CollectorID = 111111111
	f.bookings.booking = booking
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", LastName: "Diaz"}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.provider.sellerPayment = &mp.Payment{ID: 123, CollectorID: 999999999}

	if err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "123"); err != nil {
		t.Fatalf("processApprovedPayment: %v", err)
	}

	if booking.Status != "confirmed" {
		t.Errorf("the booking must confirm on the platform's read; got %q", booking.Status)
	}
	if calledAs(f.provider.callers, mustSeller(t, sellerTestToken)) {
		t.Error("the payment must not be read as the seller when the platform already names the collector")
	}
}

func calledAs(callers []mp.Caller, want mp.Caller) bool {
	for _, c := range callers {
		if c == want {
			return true
		}
	}
	return false
}

type credentialState int

const (
	readable credentialState = iota
	notConnected
	unreadable
)
