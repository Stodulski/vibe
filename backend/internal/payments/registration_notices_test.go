package payments

import (
	"testing"

	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// S9: a paid sign-up is told to the owner once, naming the person, the event
// and what they paid. A redelivered approval is the no-op it always was.
func TestAPaidRegistrationIsToldToTheOwner(t *testing.T) {
	f, reg, paid, notifier := registrationFixture(t)
	f.payments.byRegistration = []*paymentstore.Payment{checkoutRow(reg)}

	if err := f.service.processRegistrationPayment(t.Context(), reg.ID, paid, "mp-123"); err != nil {
		t.Fatal(err)
	}
	if len(notifier.ownerRegistered) != 1 {
		t.Fatalf("owner notices = %+v, want one", notifier.ownerRegistered)
	}
	got := notifier.ownerRegistered[0]
	if got.OwnerID != f.complexes.complex.OwnerID.String() || got.RegistrationID != reg.ID.String() ||
		got.EventName != "Canchas abiertas" || got.ParticipantName != "Ana Diaz" || got.Amount == "" {
		t.Errorf("owner notice = %+v", got)
	}
}

// S9: a payment that confirms nothing (the wrong amount, the spot gone) tells the
// owner nothing.
func TestAPaymentThatConfirmsNothingIsNotToldToTheOwner(t *testing.T) {
	f, reg, paid, notifier := registrationFixture(t)
	f.payments.byRegistration = []*paymentstore.Payment{checkoutRow(reg)}
	paid.TransactionAmount = 1
	if err := f.service.processRegistrationPayment(t.Context(), reg.ID, paid, "mp-123"); err != nil {
		t.Fatal(err)
	}
	f.payments.regConfirmErr = paymentstore.ErrRegistrationUnavailable
	paid.TransactionAmount = 8200
	_ = f.service.processRegistrationPayment(t.Context(), reg.ID, paid, "mp-124")
	if len(notifier.ownerRegistered) != 0 {
		t.Errorf("owner notices = %+v, want none", notifier.ownerRegistered)
	}
}

// S11: a partial refund made from MercadoPago's panel is told to the participant
// once it is recorded: what came back this time, with their spot kept.
func TestAPartialPanelRefundIsToldToTheParticipant(t *testing.T) {
	f, reg, paid, notifier := registrationFixture(t)
	reg.Status = eventstore.RegistrationConfirmed
	f.payments.byMPID = recordedRegistrationPayment(reg, "deposit_paid", 100_000)
	f.provider.payment = panelRefund(paid, 3000)

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatal(err)
	}
	if len(notifier.refunded) != 1 {
		t.Fatalf("refund notices = %+v, want one", notifier.refunded)
	}
	got := notifier.refunded[0]
	if got.Email != "ana@example.com" || got.RegistrationID != reg.ID.String() || got.RefundedTotal != 300_000 ||
		got.Amount != "$2.000" {
		t.Errorf("refund notice = %+v; want the 2.000 moved this time, keyed on the 3.000 total", got)
	}
	if len(notifier.cancelled) != 0 {
		t.Error("the spot is kept: no cancellation notice")
	}
}

// S11: a panel refund the webhook did not record (a claim of ours holds it) is
// the refund lifecycle's to tell, not this one's.
func TestAPanelRefundNotRecordedHereIsNotTold(t *testing.T) {
	f, reg, paid, notifier := registrationFixture(t)
	f.payments.byMPID = recordedRegistrationPayment(reg, "deposit_paid", 0)
	f.payments.claimedMeanwhile = true
	f.provider.payment = panelRefund(paid, 2000)

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatal(err)
	}
	if len(notifier.refunded) != 0 {
		t.Errorf("refund notices = %+v, want none", notifier.refunded)
	}
}
