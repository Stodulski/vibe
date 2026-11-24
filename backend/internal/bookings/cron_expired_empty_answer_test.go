package bookings

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/mp"
)

// MercadoPago's search lags behind a payment approved in the last seconds of
// a hold. The reconciliation trusted its first empty answer, marked the booking
// settled for good and told the client it expired, so a payment that surfaced
// a minute later was never looked at again. The events sweep already asks
// twice; so does this one: a first answer with nothing paid is asked again on
// a later run, and only the second is trusted.
func TestAnExpiredBookingNobodyPaidIsAskedTwiceBeforeItIsSettled(t *testing.T) {
	f := newFixture(t)
	candidate := expiredWithCheckout(f)
	f.checkout.searchResults = []mp.Payment{{ID: 41, Status: "rejected", ExternalReference: candidate.ID.String()}}

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if len(f.store.holdReconciled) != 0 || len(f.notify.cancelled) != 0 {
		t.Fatalf("a first empty answer must not settle the booking nor tell the client; reconciled %v, notices %d",
			f.store.holdReconciled, len(f.notify.cancelled))
	}

	f.checkout.searchResults = []mp.Payment{{ID: 42, Status: "approved", ExternalReference: candidate.ID.String()}}
	processingRecords(f, candidate)
	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if !slices.Equal(f.reconciler.reconciled, []string{"42"}) {
		t.Fatalf("the payment the second ask finds must be processed; got %v", f.reconciler.reconciled)
	}
	if len(f.notify.cancelled) != 0 {
		t.Errorf("a paid booking must never be told it expired; got %d notices", len(f.notify.cancelled))
	}
	if !slices.Equal(f.store.holdReconciled, []uuid.UUID{candidate.ID}) {
		t.Errorf("want the booking settled after the second ask; got %v", f.store.holdReconciled)
	}
}

// A venue with no seller credential is searched as the platform, and the
// platform's search does not answer for the seller's payments: an empty answer
// there proves nothing. It used to read as "nobody paid", settling the booking
// and telling the client it expired. It is now never settled on that answer:
// asked again, and alerted on when the sweep gives up.
func TestAnEmptyAnswerToThePlatformIsNotReadAsNobodyPaid(t *testing.T) {
	captured := withCapturedSentryEvents(t)
	f := newFixture(t)
	candidate := expiredWithCheckout(f)
	candidate.SetSellerAccessTokenForTest("")

	for range expiredHoldMaxFailures + 2 {
		f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)
	}

	if len(f.checkout.searches) == 0 || f.checkout.searchCallers[0] != mp.AsPlatform() {
		t.Fatalf("a venue with no credential is asked about as the platform; callers %v", f.checkout.searchCallers)
	}
	if len(f.store.holdReconciled) != 0 {
		t.Errorf("an empty platform answer must never settle the booking; got %v", f.store.holdReconciled)
	}
	if len(f.notify.cancelled) != 0 {
		t.Errorf("the client must not be told nobody paid on a platform answer; got %d notices", len(f.notify.cancelled))
	}
	alerts := 0
	for _, m := range captured.messages() {
		if strings.Contains(m, "EXPIRED BOOKING NOT RECONCILED") && strings.Contains(m, candidate.ID.String()) {
			alerts++
		}
	}
	if alerts != 1 {
		t.Errorf("want one give-up alert naming the booking; got %v", captured.messages())
	}
}

// When the sweep gives up on an approved payment it could not settle, the
// alert carries the exact command that replays it, so whoever reads it can
// retry the payment once its cause is fixed without looking anything up.
func TestTheGiveUpAlertCarriesThePaymentsReplayCommand(t *testing.T) {
	captured := withCapturedSentryEvents(t)
	f := newFixture(t)
	candidate := expiredWithCheckout(f)
	// Handed over every run, recorded by nothing: the processing refuses it.
	f.checkout.searchResults = []mp.Payment{{ID: 191310158388, Status: "approved", ExternalReference: candidate.ID.String()}}

	for range expiredHoldMaxFailures + 2 {
		f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)
	}

	var alert string
	for _, m := range captured.messages() {
		if strings.Contains(m, "EXPIRED BOOKING NOT RECONCILED") {
			alert = m
		}
	}
	want := "railway run --service Backend go run ./cmd/replaypayment 191310158388"
	if !strings.Contains(alert, want) || !strings.Contains(alert, candidate.ID.String()) {
		t.Errorf("the give-up alert must name the booking and carry %q; got %q", want, alert)
	}
}
