package events

import (
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/events/notice"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/mp"
)

// expiringHold puts a held registration with an email on the stub store and
// queues it for the next sweep to close.
func expiringHold(t *testing.T, f *fixture) *eventstore.Registration {
	t.Helper()
	email := "ana@example.com"
	reg := &eventstore.Registration{ID: uuid.New(), EventID: f.store.event.ID, ComplexID: f.complex.ID,
		FirstName: "Ana", Email: &email, Status: eventstore.RegistrationHeld, Price: 800_000, ServiceFee: 20_000}
	f.store.remember(reg, reg.ID.String())
	f.store.expired = append(f.store.expired, eventstore.ExpiredHold{ID: reg.ID, ComplexID: f.complex.ID, EventID: f.store.event.ID})
	return reg
}

func approved(id int64, reg *eventstore.Registration) mp.Payment {
	return mp.Payment{ID: id, Status: "approved", ExternalReference: reg.ID.String()}
}

func (f *fixture) openChecks() int { return len(f.store.checks) }

// S6: a hold MercadoPago holds no payment for (it never reached a checkout, or
// nobody paid it) expires exactly as before: closed, told once, never handed to
// the payment processing. MercadoPago is asked as the seller, by the
// registration the checkout was opened for, and asked only once.
func TestAnExpiredHoldWithNoPaymentExpiresAsBefore(t *testing.T) {
	f := newFixture(t)
	reg := expiringHold(t, f)

	f.svc.ExpireHolds(t.Context())
	f.svc.ExpireHolds(t.Context())

	if reg.Status != eventstore.RegistrationExpired {
		t.Errorf("status = %s, want expired", reg.Status)
	}
	if len(f.notify.cancelled) != 1 || f.notify.cancelled[0].RegistrationID != reg.ID.String() ||
		f.notify.cancelled[0].RefundLine != notice.LineExpired {
		t.Fatalf("notices = %+v, want one expiry notice", f.notify.cancelled)
	}
	seller, err := mp.AsSeller("seller-token")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.checkout.searched) != 1 || f.checkout.searched[0] != reg.ID.String() || f.checkout.callers[0] != seller {
		t.Errorf("searches = %v callers = %v, want one by the registration as the seller", f.checkout.searched, f.checkout.callers)
	}
	if len(f.reconcile.reconciled) != 0 || f.openChecks() != 0 {
		t.Errorf("reconciled = %v open checks = %d, want none", f.reconcile.reconciled, f.openChecks())
	}
}

// S6: an approved payment no webhook delivered is handed to the payment
// processing (which confirms the spot while there is room) instead of the
// person being told their spot expired.
func TestAnApprovedPaymentOnAnExpiredHoldIsProcessedNotToldExpired(t *testing.T) {
	f := newFixture(t)
	reg := expiringHold(t, f)
	f.checkout.found = map[string][]mp.Payment{reg.ID.String(): {
		{ID: 1, Status: "rejected", ExternalReference: reg.ID.String()},
		approved(2, reg),
	}}
	f.reconcile.confirms = map[string]*eventstore.Registration{"2": reg}

	f.svc.ExpireHolds(t.Context())

	if len(f.reconcile.reconciled) != 1 || f.reconcile.reconciled[0] != "2" {
		t.Fatalf("reconciled = %v, want only the approved payment", f.reconcile.reconciled)
	}
	if reg.Status != eventstore.RegistrationConfirmed {
		t.Errorf("status = %s, want confirmed by the processing", reg.Status)
	}
	if len(f.notify.cancelled) != 0 {
		t.Errorf("notices = %+v, want no expiry notice for a paid spot", f.notify.cancelled)
	}
	if f.openChecks() != 0 {
		t.Errorf("open checks = %d, want the answered check closed", f.openChecks())
	}
}

// S6: only an approved payment of this very checkout counts: another
// reference's, or one that was not approved, leaves the hold expired and told.
func TestOnlyAnApprovedPaymentOfTheCheckoutIsProcessed(t *testing.T) {
	f := newFixture(t)
	reg := expiringHold(t, f)
	f.checkout.found = map[string][]mp.Payment{reg.ID.String(): {
		{ID: 3, Status: "approved", ExternalReference: uuid.NewString()},
		{ID: 4, Status: "pending", ExternalReference: reg.ID.String()},
	}}

	f.svc.ExpireHolds(t.Context())

	if len(f.reconcile.reconciled) != 0 {
		t.Errorf("reconciled = %v, want none", f.reconcile.reconciled)
	}
	if len(f.notify.cancelled) != 1 {
		t.Errorf("notices = %+v, want the expiry notice", f.notify.cancelled)
	}
}

// S6: while MercadoPago cannot be asked the case is kept, untold, and asked
// again later; once MercadoPago answers with no payment the person is told.
func TestAnUnreachableMercadoPagoKeepsTheCheckForALaterRun(t *testing.T) {
	f := newFixture(t)
	reg := expiringHold(t, f)
	f.checkout.searchErr = errBoom

	f.svc.ExpireHolds(t.Context())
	if len(f.notify.cancelled) != 0 || f.openChecks() != 1 || f.store.checks[0].Failures != 1 {
		t.Fatalf("notices = %+v checks = %+v, want no notice and one deferred failure", f.notify.cancelled, f.store.checks)
	}
	// Not due yet: the next run does not ask again.
	f.svc.ExpireHolds(t.Context())
	if len(f.checkout.searched) != 1 {
		t.Fatalf("searches = %d, want a deferred check left alone", len(f.checkout.searched))
	}

	f.checkout.searchErr = nil
	f.store.makeDue()
	f.svc.ExpireHolds(t.Context())
	if len(f.notify.cancelled) != 1 || f.notify.cancelled[0].RegistrationID != reg.ID.String() || f.openChecks() != 0 {
		t.Fatalf("notices = %+v open checks = %d, want the notice once MercadoPago answered", f.notify.cancelled, f.openChecks())
	}
}

// S6: an approved payment that could not be handed over (the event could not
// be recorded) keeps the check for the next run, and is not told expired.
func TestAnApprovedPaymentThatCannotBeRecordedIsRetried(t *testing.T) {
	f := newFixture(t)
	reg := expiringHold(t, f)
	f.checkout.found = map[string][]mp.Payment{reg.ID.String(): {approved(5, reg)}}
	f.reconcile.err = errBoom

	f.svc.ExpireHolds(t.Context())
	if len(f.notify.cancelled) != 0 || f.openChecks() != 1 || f.store.checks[0].Failures != 1 {
		t.Fatalf("notices = %+v checks = %+v, want the check deferred and nothing told", f.notify.cancelled, f.store.checks)
	}

	f.reconcile.err = nil
	f.store.makeDue()
	f.svc.ExpireHolds(t.Context())
	if len(f.reconcile.reconciled) != 2 || f.openChecks() != 0 || len(f.notify.cancelled) != 0 {
		t.Fatalf("reconciled = %v open checks = %d notices = %+v, want handed over on the retry, closed, untold",
			f.reconcile.reconciled, f.openChecks(), f.notify.cancelled)
	}
}

// S6: a registration is asked a bounded number of times. When MercadoPago never
// answers the sweep gives up and tells the person what it knows: the spot
// expired.
func TestTheSweepGivesUpAfterTheLastFailedCheck(t *testing.T) {
	f := newFixture(t)
	reg := expiringHold(t, f)
	f.checkout.searchErr = errBoom

	for range maxPaymentChecks + 5 {
		f.svc.ExpireHolds(t.Context())
		f.store.makeDue()
	}
	if len(f.checkout.searched) != maxPaymentChecks {
		t.Fatalf("searches = %d, want exactly %d", len(f.checkout.searched), maxPaymentChecks)
	}
	if f.openChecks() != 0 {
		t.Errorf("open checks = %d, want the check given up", f.openChecks())
	}
	if len(f.notify.cancelled) != 1 || f.notify.cancelled[0].RegistrationID != reg.ID.String() {
		t.Errorf("notices = %+v, want one expiry notice on giving up", f.notify.cancelled)
	}
}

// S6: giving up on an approved payment that never could be handed over leaves
// it to a person, and never tells its payer the spot expired.
func TestGivingUpOnAnApprovedPaymentNeverTellsItsPayerExpired(t *testing.T) {
	f := newFixture(t)
	reg := expiringHold(t, f)
	f.checkout.found = map[string][]mp.Payment{reg.ID.String(): {approved(6, reg)}}
	f.reconcile.err = errBoom

	for range maxPaymentChecks + 2 {
		f.svc.ExpireHolds(t.Context())
		f.store.makeDue()
	}
	if len(f.reconcile.reconciled) != maxPaymentChecks || f.openChecks() != 0 {
		t.Fatalf("reconciled = %d open checks = %d, want %d attempts then given up", len(f.reconcile.reconciled), f.openChecks(), maxPaymentChecks)
	}
	if len(f.notify.cancelled) != 0 {
		t.Errorf("notices = %+v, want none for a payer whose payment was seen", f.notify.cancelled)
	}
}

// S6: a hold a new sign-up of its phone expired is checked silently: no payment
// closes it without a notice, an approved one is still processed.
func TestASilentCheckProcessesAPaymentButTellsNothing(t *testing.T) {
	f := newFixture(t)
	unpaid := expiringHold(t, f)
	paid := expiringHold(t, f)
	f.store.expired = nil
	for _, reg := range []*eventstore.Registration{unpaid, paid} {
		reg.Status = eventstore.RegistrationExpired
		f.store.openCheck(reg.ID, reg.ComplexID, reg.EventID, false)
	}
	f.checkout.found = map[string][]mp.Payment{paid.ID.String(): {approved(7, paid)}}

	f.svc.ExpireHolds(t.Context())

	if len(f.reconcile.reconciled) != 1 || f.reconcile.reconciled[0] != "7" {
		t.Errorf("reconciled = %v, want the paid hold's payment", f.reconcile.reconciled)
	}
	if len(f.notify.cancelled) != 0 || f.openChecks() != 0 {
		t.Errorf("notices = %+v open checks = %d, want both closed, nothing told", f.notify.cancelled, f.openChecks())
	}
}

// S6: a late webhook that confirmed the spot while MercadoPago was being asked
// wins: the person is not told a spot they just paid for expired.
func TestASpotConfirmedDuringTheCheckIsNotToldExpired(t *testing.T) {
	f := newFixture(t)
	reg := expiringHold(t, f)
	f.checkout.onSearch = func() { reg.Status = eventstore.RegistrationConfirmed }

	f.svc.ExpireHolds(t.Context())

	if len(f.notify.cancelled) != 0 {
		t.Errorf("notices = %+v, want none for a confirmed spot", f.notify.cancelled)
	}
}
