package bookings

import (
	"errors"
	"testing"
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// expiringCheckout is an unpaid storefront booking past its payment window
// whose checkout link (preference) was created, so it could have been paid.
func expiringCheckout(f *fixture, expiredAgo time.Duration) *bookingstore.CronBooking {
	candidate := cronCandidate("pending")
	candidate.CreatedAt = time.Now().Add(-f.service.cfg.PaymentExpiry - expiredAgo)
	preferenceID := "pref-1"
	f.store.expiredPending = []*bookingstore.CronBooking{candidate}
	f.payments.payment = &paymentstore.Payment{
		BookingID: candidate.ID, ComplexID: candidate.ComplexID,
		MPPreferenceID: &preferenceID, Status: "unpaid",
	}
	return candidate
}

// A payment MercadoPago approved but never told us about (a lost or refused
// webhook) used to be found by nobody: the sweep cancelled the booking and sold
// its hours again while the client's money stayed captured. The sweep now asks
// MercadoPago first and hands an approved payment to the processing a webhook
// would have run, instead of expiring the booking.
func TestTheExpirySweepProcessesAnApprovedPaymentInsteadOfExpiring(t *testing.T) {
	f := newFixture(t)
	candidate := expiringCheckout(f, time.Minute)
	f.checkout.searchResults = []mp.Payment{
		{ID: 41, Status: "rejected", ExternalReference: candidate.ID.String()},
		{ID: 42, Status: "approved", ExternalReference: candidate.ID.String()},
	}

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if len(f.checkout.searches) != 1 || f.checkout.searches[0] != candidate.ID.String() {
		t.Fatalf("want MercadoPago asked about the booking's checkout; got %v", f.checkout.searches)
	}
	if len(f.reconciler.reconciled) != 1 || f.reconciler.reconciled[0] != "42" {
		t.Fatalf("want the approved payment, and only it, processed; got %v", f.reconciler.reconciled)
	}
	if len(f.store.updated) != 0 {
		t.Errorf("a paid booking must not be expired; got %d writes", len(f.store.updated))
	}
	if len(f.notify.cancelled) != 0 {
		t.Errorf("the client of a paid booking must not be told it was cancelled; got %d", len(f.notify.cancelled))
	}
}

// Nothing approved: the booking expires exactly as before.
func TestTheExpirySweepExpiresACheckoutNobodyPaid(t *testing.T) {
	f := newFixture(t)
	candidate := expiringCheckout(f, time.Minute)
	f.checkout.searchResults = []mp.Payment{
		{ID: 41, Status: "rejected", ExternalReference: candidate.ID.String()},
	}

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if len(f.reconciler.reconciled) != 0 {
		t.Errorf("nothing approved, nothing to process; got %v", f.reconciler.reconciled)
	}
	if len(f.store.updated) != 1 || f.store.updated[0].Status != "cancelled" {
		t.Fatalf("want the unpaid booking cancelled; got %v", f.store.updated)
	}
}

// When MercadoPago cannot be asked, a booking is kept for the next run rather
// than expired blind; past the give-up window it is expired anyway, since its
// hours already read as free and a late payment is refunded by the
// confirmation path.
func TestTheExpirySweepKeepsABookingItCouldNotAskAboutForAWhile(t *testing.T) {
	t.Run("recently expired: kept", func(t *testing.T) {
		f := newFixture(t)
		expiringCheckout(f, time.Minute)
		f.checkout.searchErr = errors.New("mercadopago unavailable")

		f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

		if len(f.store.updated) != 0 {
			t.Errorf("a booking MercadoPago could not be asked about must be kept; got %d writes", len(f.store.updated))
		}
	})

	t.Run("past the give-up window: expired", func(t *testing.T) {
		f := newFixture(t)
		expiringCheckout(f, reconcileGiveUp+time.Minute)
		f.checkout.searchErr = errors.New("mercadopago unavailable")

		f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

		if len(f.store.updated) != 1 || f.store.updated[0].Status != "cancelled" {
			t.Fatalf("want the booking expired once the give-up window passed; got %v", f.store.updated)
		}
	})
}

// An approved payment already on the ledger has been processed; replaying it
// changes nothing, and the captured money means the booking is not expired.
func TestTheExpirySweepDoesNotReplayARecordedPayment(t *testing.T) {
	f := newFixture(t)
	candidate := expiringCheckout(f, time.Minute)
	recorded := "42"
	f.payments.ledger = []*paymentstore.Payment{{
		BookingID: candidate.ID, MPPaymentID: &recorded, Status: "deposit_paid",
	}}
	f.checkout.searchResults = []mp.Payment{{ID: 42, Status: "approved", ExternalReference: candidate.ID.String()}}

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if len(f.reconciler.reconciled) != 0 {
		t.Errorf("a recorded payment must not be replayed; got %v", f.reconciler.reconciled)
	}
	if len(f.store.updated) != 0 {
		t.Errorf("a booking holding captured money must not be expired; got %d writes", len(f.store.updated))
	}
}
