package bookings

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
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

// lapsingCheckout is a storefront booking with a checkout whose window ends in
// left (negative: ended that long ago), as PaymentsLapsingSoon returns it.
func lapsingCheckout(f *fixture, left time.Duration) *bookingstore.CronBooking {
	candidate := cronCandidate("pending")
	candidate.CreatedAt = time.Now().Add(-f.service.cfg.PaymentExpiry + left)
	f.store.lapsingSoon = append(f.store.lapsingSoon, candidate)
	return candidate
}

// S8: the hold must not lapse while its payment is in process. The sweep
// that expires bookings only sees them once their window is over, so the
// in-process sweep looks ahead: a booking about to lapse whose payment is in
// process is extended before its hours read as free, and is not expired or
// closed.
func TestKeepPayingHoldsExtendsAHoldAboutToLapseWhilePaying(t *testing.T) {
	for _, status := range []string{"in_process", "pending", "authorized"} {
		t.Run(status, func(t *testing.T) {
			f := newFixture(t)
			candidate := lapsingCheckout(f, 30*time.Second)
			f.checkout.searchResults = []mp.Payment{{
				ID: 61, Status: status, ExternalReference: candidate.ID.String(),
				DateCreated: time.Now().Add(-2 * time.Minute),
			}}

			f.service.KeepPayingHolds(t.Context(), f.reconciler)

			if !slices.Equal(f.store.holdExtended, []uuid.UUID{candidate.ID}) {
				t.Errorf("want the hold extended before it lapses; got %v", f.store.holdExtended)
			}
			if f.store.holdExtendedFor[candidate.ID] < 2*KeepPayingHoldsEvery {
				t.Errorf("the extension must reach past the run after next (%v); got %v",
					2*KeepPayingHoldsEvery, f.store.holdExtendedFor[candidate.ID])
			}
			if len(f.store.updated) != 0 || len(f.store.holdExpired) != 0 || len(f.checkout.expired) != 0 {
				t.Errorf("a booking being paid is neither expired nor closed; updated %v, expired %v, closed %v",
					f.store.updated, f.store.holdExpired, f.checkout.expired)
			}
		})
	}
}

// S8: nobody is paying, or the attempt failed or is past its bound: the hold
// is left to lapse on time, untouched by the look-ahead.
func TestKeepPayingHoldsLeavesAHoldNobodyIsPaying(t *testing.T) {
	cases := map[string][]mp.Payment{
		"no payment":   nil,
		"rejected":     {{ID: 62, Status: "rejected", DateCreated: time.Now().Add(-time.Minute)}},
		"too long ago": {{ID: 63, Status: "in_process", DateCreated: time.Now().Add(-inProcessHoldLimit - time.Minute)}},
	}
	for name, payments := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			candidate := lapsingCheckout(f, 30*time.Second)
			for i := range payments {
				payments[i].ExternalReference = candidate.ID.String()
			}
			f.checkout.searchResults = payments

			f.service.KeepPayingHolds(t.Context(), f.reconciler)

			if len(f.store.holdExtended) != 0 || len(f.store.updated) != 0 || len(f.checkout.expired) != 0 {
				t.Errorf("want the booking untouched; extended %v, updated %v, closed %v",
					f.store.holdExtended, f.store.updated, f.checkout.expired)
			}
		})
	}
}

// S8: an approved payment whose notification was lost is processed before the
// hold lapses, rather than after its hours were already sold again.
func TestKeepPayingHoldsProcessesAnApprovedPaymentBeforeItLapses(t *testing.T) {
	f := newFixture(t)
	candidate := lapsingCheckout(f, 30*time.Second)
	f.checkout.searchResults = []mp.Payment{{ID: 64, Status: "approved", ExternalReference: candidate.ID.String()}}

	f.service.KeepPayingHolds(t.Context(), f.reconciler)

	if !slices.Equal(f.reconciler.reconciled, []string{"64"}) {
		t.Errorf("want the approved payment processed; got %v", f.reconciler.reconciled)
	}
	if len(f.store.updated) != 0 || len(f.checkout.expired) != 0 {
		t.Errorf("the booking is not expired; updated %v, closed %v", f.store.updated, f.checkout.expired)
	}
}

// MercadoPago unreachable: nothing is extended or expired; the expiry sweep
// decides about the booking once it lapses, as before.
func TestKeepPayingHoldsDoesNothingWhenMercadoPagoCannotBeAsked(t *testing.T) {
	f := newFixture(t)
	lapsingCheckout(f, 30*time.Second)
	f.checkout.searchErr = errors.New("mercadopago down")

	f.service.KeepPayingHolds(t.Context(), f.reconciler)

	if len(f.store.holdExtended) != 0 || len(f.store.updated) != 0 || len(f.reconciler.reconciled) != 0 {
		t.Errorf("want nothing done; extended %v, updated %v, reconciled %v",
			f.store.holdExtended, f.store.updated, f.reconciler.reconciled)
	}
}

// L18 minor: the look-ahead processes an approved payment only while the hold
// is live. Once the window ended, the expiry sweep owns the booking: it
// replays the payment within reconcileGiveUp and raises the one alert when it
// gives up. Replaying it here too, every minute, wrote an inbox row (and the
// processing's alert) per minute for a payment the processing keeps declining.
func TestKeepPayingHoldsLeavesALapsedApprovedPaymentToTheExpirySweep(t *testing.T) {
	f := newFixture(t)
	candidate := lapsingCheckout(f, -2*time.Minute)
	f.checkout.searchResults = []mp.Payment{
		{ID: 65, Status: "approved", ExternalReference: candidate.ID.String()},
		{ID: 66, Status: "in_process", ExternalReference: candidate.ID.String(), DateCreated: time.Now().Add(-time.Minute)},
	}

	f.service.KeepPayingHolds(t.Context(), f.reconciler)

	if len(f.reconciler.reconciled) != 0 {
		t.Errorf("a lapsed booking's approved payment is the expiry sweep's to replay; got %v", f.reconciler.reconciled)
	}
	if len(f.store.holdExtended) != 0 {
		t.Errorf("an approved payment is no reason to extend the hold; got %v", f.store.holdExtended)
	}
}
