package bookings

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// expiredWithCheckout is a storefront booking already cancelled by hold expiry
// — by the inline release, which closes nothing and asks nobody — whose
// checkout link was created and so could have been paid.
func expiredWithCheckout(f *fixture) *bookingstore.CronBooking {
	candidate := cronCandidate("cancelled")
	candidate.CreatedAt = time.Now().Add(-time.Hour)
	preferenceID := "pref-expired"
	f.store.expiredToReconcile = []*bookingstore.CronBooking{candidate}
	f.payments.payment = &paymentstore.Payment{
		BookingID: candidate.ID, ComplexID: candidate.ComplexID,
		MPPreferenceID: &preferenceID, Status: "unpaid",
	}
	return candidate
}

// processingRecords makes the processing behind the hand-over record each
// payment it is handed against candidate, as the webhook's processing does
// once it confirms or refunds it.
func processingRecords(f *fixture, candidate *bookingstore.CronBooking) {
	f.reconciler.processed = func(mpPaymentID string) {
		id := mpPaymentID
		f.payments.ledger = append(f.payments.ledger, &paymentstore.Payment{
			BookingID: candidate.ID, ComplexID: candidate.ComplexID,
			Status: "refunded", MPPaymentID: &id,
		})
	}
}

// S1: the sweep's own expiry is recorded as one, so it is asked about again
// later and a late payment can reopen it.
func TestTheExpirySweepRecordsTheBookingsItExpires(t *testing.T) {
	f := newFixture(t)
	candidate := expiringCheckout(f, time.Minute)

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if len(f.store.updated) != 1 || f.store.updated[0].Status != "cancelled" {
		t.Fatalf("want the unpaid booking cancelled; got %v", f.store.updated)
	}
	if !slices.Equal(f.store.holdExpired, []uuid.UUID{candidate.ID}) {
		t.Errorf("want the expiry recorded with the cancellation; got %v", f.store.holdExpired)
	}
}

// A disconnect's release is not an expiry: nothing is recorded, so the
// booking is never reopened by a payment.
func TestReleasingUnpaidHoldsOnDisconnectRecordsNoExpiry(t *testing.T) {
	f := newFixture(t)
	hold := cronCandidate("pending")
	hold.CreatedAt = time.Now().Add(-time.Hour)
	f.store.unpaidHolds = []*bookingstore.CronBooking{hold}

	if err := f.service.ReleaseUnpaidHolds(t.Context(), hold.ComplexID); err != nil {
		t.Fatalf("ReleaseUnpaidHolds: %v", err)
	}
	if len(f.store.updated) != 1 {
		t.Fatalf("want the hold cancelled; got %d writes", len(f.store.updated))
	}
	if len(f.store.holdExpired) != 0 {
		t.Errorf("a disconnect must not record a hold expiry; got %v", f.store.holdExpired)
	}
}

// S1: an approved payment on a booking the inline release cancelled used to
// stay captured forever when its webhook was lost. The sweep closes the
// still-open link, asks MercadoPago, hands the payment to the processing the
// webhook runs, and never asks about that booking again.
func TestTheExpirySweepReconcilesAnExpiredBookingThatWasPaid(t *testing.T) {
	f := newFixture(t)
	candidate := expiredWithCheckout(f)
	f.checkout.searchResults = []mp.Payment{
		{ID: 41, Status: "rejected", ExternalReference: candidate.ID.String()},
		{ID: 42, Status: "approved", ExternalReference: candidate.ID.String()},
	}
	processingRecords(f, candidate)

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if !slices.Contains(f.checkout.expired, "pref-expired") {
		t.Errorf("want the checkout link closed before asking; got %v", f.checkout.expired)
	}
	if !slices.Equal(f.checkout.searches, []string{candidate.ID.String()}) {
		t.Fatalf("want MercadoPago asked about the booking; got %v", f.checkout.searches)
	}
	if !slices.Equal(f.reconciler.reconciled, []string{"42"}) {
		t.Fatalf("want the approved payment, and only it, processed; got %v", f.reconciler.reconciled)
	}
	if !slices.Equal(f.store.holdReconciled, []uuid.UUID{candidate.ID}) {
		t.Errorf("want the booking marked reconciled; got %v", f.store.holdReconciled)
	}

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)
	if len(f.checkout.searches) != 1 || len(f.reconciler.reconciled) != 1 {
		t.Errorf("a reconciled booking must not be asked about again; searches %v, reconciled %v",
			f.checkout.searches, f.reconciler.reconciled)
	}
}

// Nothing approved: reconciled once, nothing processed.
func TestTheExpirySweepSettlesAnExpiredBookingNobodyPaid(t *testing.T) {
	f := newFixture(t)
	candidate := expiredWithCheckout(f)
	f.checkout.searchResults = []mp.Payment{{ID: 41, Status: "rejected", ExternalReference: candidate.ID.String()}}

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if len(f.reconciler.reconciled) != 0 {
		t.Errorf("nothing approved, nothing to process; got %v", f.reconciler.reconciled)
	}
	if !slices.Equal(f.store.holdReconciled, []uuid.UUID{candidate.ID}) {
		t.Errorf("want the booking marked reconciled; got %v", f.store.holdReconciled)
	}
}

// A payment MercadoPago is still processing may yet be approved: the booking
// is asked about again on the next run, and that is not a failure.
func TestTheExpirySweepAsksAgainWhileAPaymentIsInProcess(t *testing.T) {
	f := newFixture(t)
	candidate := expiredWithCheckout(f)
	f.checkout.searchResults = []mp.Payment{{ID: 43, Status: "in_process", ExternalReference: candidate.ID.String()}}

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if len(f.store.holdReconciled) != 0 || f.store.holdFailures[candidate.ID] != 0 {
		t.Fatalf("want the booking neither settled nor failed; reconciled %v, failures %v",
			f.store.holdReconciled, f.store.holdFailures)
	}

	f.checkout.searchResults = []mp.Payment{{ID: 43, Status: "approved", ExternalReference: candidate.ID.String()}}
	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if !slices.Equal(f.reconciler.reconciled, []string{"43"}) {
		t.Errorf("want the payment processed once it was approved; got %v", f.reconciler.reconciled)
	}
}

// MercadoPago unreachable: counted, asked again, and after the bound the
// sweep alerts once and stops asking.
func TestTheExpirySweepGivesUpOnAnExpiredBookingItCannotAskAbout(t *testing.T) {
	captured := withCapturedSentryEvents(t)
	f := newFixture(t)
	candidate := expiredWithCheckout(f)
	f.checkout.searchErr = errors.New("mercadopago unavailable")

	for range expiredHoldMaxFailures + 2 {
		f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)
	}

	if len(f.checkout.searches) != expiredHoldMaxFailures {
		t.Errorf("want exactly %d attempts; got %d", expiredHoldMaxFailures, len(f.checkout.searches))
	}
	if len(f.store.holdReconciled) != 0 {
		t.Errorf("a booking nobody could ask about must not read as reconciled; got %v", f.store.holdReconciled)
	}
	alerts := 0
	for _, m := range captured.messages() {
		if strings.Contains(m, "EXPIRED BOOKING NOT RECONCILED") && strings.Contains(m, candidate.ID.String()) {
			alerts++
		}
	}
	if alerts != 1 {
		t.Errorf("want one alert naming the booking; got %v", captured.messages())
	}
}

// A payment the webhook did record is not replayed.
func TestTheExpirySweepDoesNotReplayAPaymentOnAnExpiredBooking(t *testing.T) {
	f := newFixture(t)
	candidate := expiredWithCheckout(f)
	recorded := "42"
	f.payments.ledger = []*paymentstore.Payment{{
		BookingID: candidate.ID, MPPaymentID: &recorded, Status: "deposit_paid",
	}}
	f.checkout.searchResults = []mp.Payment{{ID: 42, Status: "approved", ExternalReference: candidate.ID.String()}}

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if len(f.reconciler.reconciled) != 0 {
		t.Errorf("a recorded payment must not be replayed; got %v", f.reconciler.reconciled)
	}
	if !slices.Equal(f.store.holdReconciled, []uuid.UUID{candidate.ID}) {
		t.Errorf("want the booking marked reconciled; got %v", f.store.holdReconciled)
	}
}

// A checkout link that will not close is retried every run, and alerts once,
// when the sweep gives up — not once per run, which would bury every other
// alert under a dozen copies of this one.
func TestAnExpiredBookingWhoseLinkWillNotCloseAlertsOnce(t *testing.T) {
	captured := withCapturedSentryEvents(t)
	f := newFixture(t)
	candidate := expiredWithCheckout(f)
	f.checkout.expireErr = errors.New("mercadopago refused")

	for range expiredHoldMaxFailures + 2 {
		f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)
	}

	if len(f.store.holdReconciled) != 0 {
		t.Errorf("a booking whose link stayed open must not read as reconciled; got %v", f.store.holdReconciled)
	}
	messages := captured.messages()
	if len(messages) != 1 || !strings.Contains(messages[0], "EXPIRED BOOKING NOT RECONCILED") ||
		!strings.Contains(messages[0], candidate.ID.String()) {
		t.Errorf("want exactly one alert, the give-up naming the booking; got %v", messages)
	}
}

// The public path creates the checkout link before it saves the payment row,
// and a failed save is only logged. Such a booking could still be paid: the
// expiry sweep asks MercadoPago about it by its id before expiring it, and a
// payment found is processed instead.
func TestTheExpirySweepAsksAboutACheckoutWhosePaymentRowWasNeverSaved(t *testing.T) {
	f := newFixture(t)
	candidate := expiringCheckout(f, time.Minute)
	f.payments.payment = nil
	f.checkout.searchResults = []mp.Payment{{ID: 42, Status: "approved", ExternalReference: candidate.ID.String()}}

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if !slices.Equal(f.checkout.searches, []string{candidate.ID.String()}) {
		t.Fatalf("want MercadoPago asked about the booking; got %v", f.checkout.searches)
	}
	if !slices.Equal(f.reconciler.reconciled, []string{"42"}) {
		t.Fatalf("want the approved payment processed; got %v", f.reconciler.reconciled)
	}
	if len(f.store.updated) != 0 || len(f.notify.cancelled) != 0 {
		t.Errorf("a paid booking must not be expired nor its client told so; writes %d, notices %d",
			len(f.store.updated), len(f.notify.cancelled))
	}
}

// After expiry, the same booking with no payment row is still asked about by
// the reconciliation; one that never reached MercadoPago costs a single search
// and is then settled for good.
func TestAnExpiredBookingWithNoPaymentRowIsAskedAboutOnce(t *testing.T) {
	t.Run("paid: processed", func(t *testing.T) {
		f := newFixture(t)
		candidate := expiredWithCheckout(f)
		f.payments.payment = nil
		f.checkout.searchResults = []mp.Payment{{ID: 42, Status: "approved", ExternalReference: candidate.ID.String()}}
		processingRecords(f, candidate)

		f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

		if !slices.Equal(f.reconciler.reconciled, []string{"42"}) {
			t.Fatalf("want the approved payment processed; got %v", f.reconciler.reconciled)
		}
		if !slices.Equal(f.store.holdReconciled, []uuid.UUID{candidate.ID}) {
			t.Errorf("want the booking marked reconciled; got %v", f.store.holdReconciled)
		}
	})
	t.Run("never reached MercadoPago: one search", func(t *testing.T) {
		f := newFixture(t)
		candidate := expiredWithCheckout(f)
		f.payments.payment = nil

		f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)
		f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

		if !slices.Equal(f.checkout.searches, []string{candidate.ID.String()}) {
			t.Errorf("want exactly one search; got %v", f.checkout.searches)
		}
		if len(f.checkout.expired) != 0 {
			t.Errorf("no link is known, so nothing is closed; got %v", f.checkout.expired)
		}
		if !slices.Equal(f.store.holdReconciled, []uuid.UUID{candidate.ID}) {
			t.Errorf("want the booking marked reconciled; got %v", f.store.holdReconciled)
		}
	})
}

// L19: a booking the inline release cancelled (another visitor's booking took
// its stale hours, inside that visitor's request) is told it expired, as one
// the sweep expired is, once MercadoPago holds no payment for it. The notice
// is the sweep's own expiry notice, so its deduplication key (":expired")
// makes it a no-op for a booking the sweep already told when it expired it.
func TestAnExpiredBookingIsToldOnceMercadoPagoHoldsNoPayment(t *testing.T) {
	f := newFixture(t)
	candidate := expiredWithCheckout(f)
	f.checkout.searchResults = []mp.Payment{{ID: 43, Status: "rejected", ExternalReference: candidate.ID.String()}}

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if len(f.notify.cancelled) != 1 {
		t.Fatalf("want one expiry notice; got %d", len(f.notify.cancelled))
	}
	got := f.notify.cancelled[0]
	if got.Source != notifications.SourceExpiredUnpaid || got.BookingID != candidate.ID.String() ||
		got.Email != candidate.ClientEmail || got.RefundLine != notifications.ExpiredUnpaidRefundLine || got.OwnerID != "" {
		t.Errorf("want the sweep's expiry notice to the client, never the owner; got %+v", got)
	}
}

// No expiry notice while a payment may still confirm or refund the booking:
// an approved payment is the processing's to tell, and one in flight is
// asked about again; nor for a booking nobody can be written to.
func TestAnExpiredBookingIsNotToldWhileAPaymentMayStillSettleIt(t *testing.T) {
	cases := map[string]func(*fixture, *bookingstore.CronBooking){
		"approved": func(f *fixture, b *bookingstore.CronBooking) {
			f.checkout.searchResults = []mp.Payment{{ID: 44, Status: "approved", ExternalReference: b.ID.String()}}
		},
		"in process": func(f *fixture, b *bookingstore.CronBooking) {
			f.checkout.searchResults = []mp.Payment{{ID: 45, Status: "in_process", ExternalReference: b.ID.String()}}
		},
		"unreachable": func(f *fixture, _ *bookingstore.CronBooking) { f.checkout.searchErr = errors.New("down") },
		"no email":    func(_ *fixture, b *bookingstore.CronBooking) { b.ClientEmail = "" },
		// T12 (2): a late payment already refunded (its webhook arrived,
		// the booking could not be reopened) is no longer "approved" at
		// MercadoPago, yet the client paid and was refunded: "no hay pagos a
		// devolver" would be false.
		"refunded": func(f *fixture, b *bookingstore.CronBooking) {
			f.checkout.searchResults = []mp.Payment{{ID: 46, Status: "refunded", ExternalReference: b.ID.String()}}
		},
		"charged back": func(f *fixture, b *bookingstore.CronBooking) {
			f.checkout.searchResults = []mp.Payment{{ID: 47, Status: "charged_back", ExternalReference: b.ID.String()}}
		},
		// The booking is no longer unpaid: money moved on it.
		"no longer unpaid": func(_ *fixture, b *bookingstore.CronBooking) {
			b.CollectionStatus = bookingstore.CollectionStatusDepositPaid
		},
		"refund owed": func(_ *fixture, b *bookingstore.CronBooking) {
			b.RefundStatus = bookingstore.RefundStatusPending
		},
		// T12 (2): the visitor released it on screen; it did not expire.
		"released by its visitor": func(_ *fixture, b *bookingstore.CronBooking) { b.ReleasedByClient = true },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			setup(f, expiredWithCheckout(f))

			f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

			if len(f.notify.cancelled) != 0 {
				t.Errorf("want no expiry notice; got %+v", f.notify.cancelled)
			}
		})
	}
}

// L19: the expiry note is what the owner reads on the cancelled booking. It
// is the inline release's own note, with no raw Go duration ("(10m0s)") in
// it, and a late payment that reopens the booking takes it off entirely.
func TestTheExpirySweepWritesTheReadableExpiryNote(t *testing.T) {
	f := newFixture(t)
	expiringCheckout(f, time.Minute)

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if len(f.store.updated) != 1 || f.store.updated[0].Notes == nil {
		t.Fatalf("want the booking expired with a note; got %+v", f.store.updated)
	}
	note := *f.store.updated[0].Notes
	if note != bookingstore.HoldExpiryNote {
		t.Errorf("note = %q, want %q", note, bookingstore.HoldExpiryNote)
	}
	if rest := bookingstore.WithoutHoldExpiryNote(&note); rest != nil {
		t.Errorf("a reopened booking must lose the whole note; %q is left", *rest)
	}
}

// T12 (1): a booking its visitor released is asked about once more, like an
// expiry: an approved payment the release's single search missed is handed
// to the processing (which refunds it: a release is not reopenable), and the
// booking is not asked about again.
func TestTheExpirySweepAsksAgainAboutAReleasedCheckout(t *testing.T) {
	f := newFixture(t)
	candidate := expiredWithCheckout(f)
	candidate.ReleasedByClient = true
	f.checkout.searchResults = []mp.Payment{{ID: 48, Status: "approved", ExternalReference: candidate.ID.String()}}
	processingRecords(f, candidate)

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if !slices.Equal(f.reconciler.reconciled, []string{"48"}) {
		t.Fatalf("want the approved payment handed over; got %v", f.reconciler.reconciled)
	}
	if !slices.Equal(f.store.holdReconciled, []uuid.UUID{candidate.ID}) {
		t.Errorf("want the release marked reconciled; got %v", f.store.holdReconciled)
	}
	if len(f.notify.cancelled) != 0 {
		t.Errorf("want no expiry notice for a released booking; got %+v", f.notify.cancelled)
	}
}

// A cancelled booking paid twice: the sweep hands both approved payments over,
// and while one of them is still neither recorded nor refunded the booking is
// not marked reconciled. It is counted as a failed attempt instead, so it is
// asked about again and alerts at the bound, rather than leaving that money
// captured with only a log line.
func TestTheExpirySweepKeepsAskingWhileAnApprovedPaymentIsUnrecorded(t *testing.T) {
	f := newFixture(t)
	candidate := expiredWithCheckout(f)
	f.checkout.searchResults = []mp.Payment{
		{ID: 42, Status: "approved", ExternalReference: candidate.ID.String()},
		{ID: 43, Status: "approved", ExternalReference: candidate.ID.String()},
	}
	recordOnly := map[string]bool{"42": true}
	f.reconciler.processed = func(mpPaymentID string) {
		if !recordOnly[mpPaymentID] {
			return
		}
		id := mpPaymentID
		f.payments.ledger = append(f.payments.ledger, &paymentstore.Payment{
			BookingID: candidate.ID, ComplexID: candidate.ComplexID,
			Status: "refunded", MPPaymentID: &id,
		})
	}

	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if !slices.Equal(f.reconciler.reconciled, []string{"42", "43"}) {
		t.Fatalf("want both approved payments handed over; got %v", f.reconciler.reconciled)
	}
	if len(f.store.holdReconciled) != 0 {
		t.Fatalf("payment 43 is neither recorded nor refunded, so the booking must not be marked reconciled; got %v",
			f.store.holdReconciled)
	}
	if f.store.holdFailures[candidate.ID] != 1 {
		t.Errorf("want the attempt counted as failed, so it is retried and alerts at the bound; got %v", f.store.holdFailures)
	}

	recordOnly["43"] = true
	f.service.ReleaseExpiredPayments(t.Context(), f.reconciler)

	if !slices.Equal(f.reconciler.reconciled, []string{"42", "43", "43"}) {
		t.Errorf("want only the unrecorded payment handed over again; got %v", f.reconciler.reconciled)
	}
	if !slices.Equal(f.store.holdReconciled, []uuid.UUID{candidate.ID}) {
		t.Errorf("once every approved payment is recorded the booking is reconciled; got %v", f.store.holdReconciled)
	}
}
