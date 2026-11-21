package bookings

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/mp"
)

// S8: the window exists so nobody blocks a court without meaning to buy. A
// visitor whose payment MercadoPago is still processing when the window ends
// is buying, so the hold is extended instead of the court going back on sale.
func TestTheExpirySweepExtendsAHoldWhosePaymentIsInProcess(t *testing.T) {
	for _, status := range []string{"in_process", "pending", "authorized"} {
		t.Run(status, func(t *testing.T) {
			f := newFixture(t)
			candidate := expiringCheckout(f, time.Minute)
			f.checkout.searchResults = []mp.Payment{{
				ID: 51, Status: status, ExternalReference: candidate.ID.String(),
				DateCreated: time.Now().Add(-2 * time.Minute),
			}}

			f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

			if len(f.store.updated) != 0 || len(f.store.holdExpired) != 0 {
				t.Fatalf("a booking being paid must not be expired; updated %v, expired %v", f.store.updated, f.store.holdExpired)
			}
			if !slices.Equal(f.store.holdExtended, []uuid.UUID{candidate.ID}) {
				t.Errorf("want the hold extended so its hours stay taken; got %v", f.store.holdExtended)
			}
			if len(f.reconciler.reconciled) != 0 {
				t.Errorf("nothing is approved yet, nothing to process; got %v", f.reconciler.reconciled)
			}
		})
	}
}

// The extension is bounded by the payment attempt itself: a payment still
// "in process" an hour after it was started no longer holds the court, and the
// booking expires as before (its late approval is still reconciled later).
func TestTheExpirySweepStopsExtendingAPaymentInProcessForTooLong(t *testing.T) {
	f := newFixture(t)
	candidate := expiringCheckout(f, time.Minute)
	f.checkout.searchResults = []mp.Payment{{
		ID: 52, Status: "in_process", ExternalReference: candidate.ID.String(),
		DateCreated: time.Now().Add(-inProcessHoldLimit - time.Minute),
	}}

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if len(f.store.holdExtended) != 0 {
		t.Errorf("past the bound the hold is not extended; got %v", f.store.holdExtended)
	}
	if !slices.Equal(f.store.holdExpired, []uuid.UUID{candidate.ID}) {
		t.Errorf("want the booking expired as before; got %v", f.store.holdExpired)
	}
}

// A rejected attempt is no purchase in progress: the booking expires on time.
func TestTheExpirySweepDoesNotExtendForARejectedPayment(t *testing.T) {
	f := newFixture(t)
	candidate := expiringCheckout(f, time.Minute)
	f.checkout.searchResults = []mp.Payment{{
		ID: 53, Status: "rejected", ExternalReference: candidate.ID.String(),
		DateCreated: time.Now().Add(-2 * time.Minute),
	}}

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if len(f.store.holdExtended) != 0 || !slices.Equal(f.store.holdExpired, []uuid.UUID{candidate.ID}) {
		t.Errorf("want the booking expired and not extended; extended %v, expired %v", f.store.holdExtended, f.store.holdExpired)
	}
}
