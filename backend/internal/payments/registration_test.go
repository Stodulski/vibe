package payments

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

type stubRegistrations struct {
	reg *eventstore.Registration
	ev  *eventstore.Event
}

func (s *stubRegistrations) GetRegistration(context.Context, uuid.UUID) (*eventstore.Registration, error) {
	if s.reg == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.reg, nil
}

func (s *stubRegistrations) GetEvent(context.Context, uuid.UUID) (*eventstore.Event, error) {
	return s.ev, nil
}

type stubRegistrationNotifier struct {
	registered []notifications.EventRegistration
	cancelled  []notifications.EventCancellation
}

func (s *stubRegistrationNotifier) EventRegistered(n notifications.EventRegistration) {
	s.registered = append(s.registered, n)
}

func (s *stubRegistrationNotifier) EventCancelled(n notifications.EventCancellation) {
	s.cancelled = append(s.cancelled, n)
}

// registrationFixture wires the event sign-up ports onto the payments fixture,
// with one held registration of 8.000 plus its service fee.
func registrationFixture(t *testing.T) (*fixture, *eventstore.Registration, *mp.Payment, *stubRegistrationNotifier) {
	t.Helper()
	f := newFixture(t)
	complexID := uuid.New()
	f.complexes.complex = linkedComplex(complexID, "999")
	email := "ana@example.com"
	reg := &eventstore.Registration{
		ID: uuid.New(), EventID: uuid.New(), ComplexID: complexID, FirstName: "Ana", LastName: "Diaz",
		Phone: "+5491100000000", Email: &email, Status: eventstore.RegistrationHeld, Price: 800_000, ServiceFee: 20_000,
	}
	ev := &eventstore.Event{
		ID: reg.EventID, ComplexID: complexID, Name: "Canchas libres", Date: time.Now().AddDate(0, 0, 3),
		StartTime: "19:00", DurationMinutes: 120, Capacity: 20, Price: 800_000,
	}
	notifier := &stubRegistrationNotifier{}
	f.service.registrationLedger = f.payments
	f.service.registrations = &stubRegistrations{reg: reg, ev: ev}
	f.service.registrationNotify = notifier
	paid := &mp.Payment{
		ID: 123, Status: "approved", TransactionAmount: 8200, CollectorID: 999,
		Metadata: map[string]any{"registration_id": reg.ID.String()},
	}
	return f, reg, paid, notifier
}

func checkoutRow(reg *eventstore.Registration) *paymentstore.Payment {
	return &paymentstore.Payment{
		ID: uuid.New(), RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: reg.Price, ServiceFee: reg.ServiceFee,
		Method: "mercadopago", Status: "unpaid",
	}
}

// S3: an approved payment confirms the registration, settles the checkout row the
// sign-up wrote, and tells the person by email.
func TestAnApprovedRegistrationPaymentConfirmsTheSpotAndTellsThePerson(t *testing.T) {
	f, reg, paid, notifier := registrationFixture(t)
	checkout := checkoutRow(reg)
	f.payments.byRegistration = []*paymentstore.Payment{checkout}

	if err := f.service.processRegistrationPayment(t.Context(), reg.ID, paid, "mp-123"); err != nil {
		t.Fatal(err)
	}
	if f.payments.regConfirmed != checkout || checkout.Status != "deposit_paid" || checkout.MPPaymentID == nil || *checkout.MPPaymentID != "mp-123" {
		t.Errorf("the checkout row must be settled with the provider's payment id; got %+v", f.payments.regConfirmed)
	}
	if len(notifier.registered) != 1 || notifier.registered[0].Email != "ana@example.com" || notifier.registered[0].EventName != "Canchas libres" {
		t.Errorf("the person must be told their spot is confirmed; got %+v", notifier.registered)
	}
	if len(f.provider.refunds) != 0 {
		t.Error("a confirmed spot must not be refunded")
	}
}

// The payment must be exactly what the registration was asked to pay, and be
// collected by the complex's own seller.
func TestAnApprovedRegistrationPaymentOfTheWrongAmountOrCollectorConfirmsNothing(t *testing.T) {
	f, reg, paid, notifier := registrationFixture(t)
	f.payments.byRegistration = []*paymentstore.Payment{checkoutRow(reg)}

	paid.TransactionAmount = 1
	if err := f.service.processRegistrationPayment(t.Context(), reg.ID, paid, "mp-123"); err != nil {
		t.Fatal(err)
	}
	paid.TransactionAmount, paid.CollectorID = 8200, 12345
	if err := f.service.processRegistrationPayment(t.Context(), reg.ID, paid, "mp-123"); err != nil {
		t.Fatal(err)
	}
	if f.payments.regConfirmed != nil || len(notifier.registered) != 0 || len(f.provider.refunds) != 0 {
		t.Error("a payment of the wrong amount or from another seller must change nothing")
	}
}

// S4/S5: when the spot is gone by the time the payment lands — the hold ran
// out and the last spot went to someone else, or the event was cancelled — the
// money goes straight back.
func TestAnApprovedRegistrationPaymentForAGoneSpotIsRefunded(t *testing.T) {
	for _, why := range []error{paymentstore.ErrRegistrationUnavailable, paymentstore.ErrRegistrationAlreadyPaid} {
		t.Run(why.Error(), func(t *testing.T) {
			f, reg, paid, notifier := registrationFixture(t)
			checkout := checkoutRow(reg)
			f.payments.byRegistration = []*paymentstore.Payment{checkout}
			f.payments.regConfirmErr = why

			if err := f.service.processRegistrationPayment(t.Context(), reg.ID, paid, "mp-123"); err != nil {
				t.Fatal(err)
			}
			if len(notifier.registered) != 0 {
				t.Error("nobody is told they have a spot")
			}
			if len(f.provider.refunds) != 1 || f.provider.refunds[0] != 150_000.0/100 {
				t.Errorf("the provider must be asked to refund the claimed amount; got %v", f.provider.refunds)
			}
			if len(f.payments.recordedSuccess) != 1 || f.payments.recordedSuccess[0].RegistrationID == nil {
				t.Errorf("the refund must be recorded against the registration; got %+v", f.payments.recordedSuccess)
			}
		})
	}
}

// S5: the owner cancelling refunds every paid participant through MercadoPago,
// and a participant with nothing paid has nothing refunded.
func TestRefundRegistrationRefundsEachCollectedPaymentAndIgnoresTheRest(t *testing.T) {
	f, reg, _, _ := registrationFixture(t)
	mp1, mp2 := "mp-1", "mp-2"
	collected := &paymentstore.Payment{ID: uuid.New(), RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: 800_000, ServiceFee: 20_000, Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &mp1}
	extra := &paymentstore.Payment{ID: uuid.New(), RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: 800_000, ServiceFee: 20_000, Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &mp2}
	unpaid := checkoutRow(reg)
	f.payments.byRegistration = []*paymentstore.Payment{unpaid, collected, extra}
	f.payments.claimAmount = 820_000

	outcome := f.service.RefundRegistration(t.Context(), reg.ID)

	if outcome.Result != paymentstore.RefundIssued {
		t.Errorf("outcome = %+v, want issued", outcome)
	}
	if len(f.provider.refunds) != 2 {
		t.Errorf("both collected payments are refunded and the unpaid checkout row is not; got %v", f.provider.refunds)
	}
	for _, claimed := range f.payments.claimed {
		if claimed == unpaid.ID {
			t.Error("a checkout row that never collected money must not be claimed")
		}
	}
}

func TestRefundRegistrationOfARegistrationWithNoMoneyReportsNothingToReturn(t *testing.T) {
	f, reg, _, _ := registrationFixture(t)
	f.payments.byRegistration = []*paymentstore.Payment{checkoutRow(reg)}

	outcome := f.service.RefundRegistration(t.Context(), reg.ID)
	if outcome.Result != paymentstore.RefundNone || len(f.provider.refunds) != 0 {
		t.Errorf("outcome = %+v refunds = %v", outcome, f.provider.refunds)
	}
}

// A refund MercadoPago refuses stays queued, and the person is told when the
// retry job finishes it.
func TestARefundThatFailsStaysQueuedForTheRetryJob(t *testing.T) {
	f, reg, _, _ := registrationFixture(t)
	mpID := "mp-1"
	f.payments.byRegistration = []*paymentstore.Payment{{
		ID: uuid.New(), RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: 800_000, ServiceFee: 20_000,
		Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &mpID,
	}}
	f.provider.refundErr = errProvider
	f.complexes.complex = linkedComplex(reg.ComplexID, "999")

	outcome := f.service.RefundRegistration(t.Context(), reg.ID)
	if outcome.Result != paymentstore.RefundQueued || len(f.payments.recordedFailure) != 1 {
		t.Errorf("outcome = %+v failures = %d, want queued with the failure recorded", outcome, len(f.payments.recordedFailure))
	}
}

func TestTheRetryJobNotifiesAPersonWhoseQueuedRefundFinished(t *testing.T) {
	f, reg, _, notifier := registrationFixture(t)
	cancelledBy := eventstore.CancelledByOwner
	reg.CancelledBy = &cancelledBy
	mpID := "mp-1"
	payment := &paymentstore.Payment{
		ID: uuid.New(), RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: 800_000, ServiceFee: 20_000,
		Method: "mercadopago", Status: "refund_pending", MPPaymentID: &mpID,
	}
	f.payments.byRegistration = []*paymentstore.Payment{payment}
	f.failedRefunds.pending = []*paymentstore.FailedRefund{{
		ID: uuid.New(), PaymentID: payment.ID, RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: 820_000, MPPaymentID: mpID,
	}}
	f.payments.claimAmount = 820_000

	f.service.RetryFailedRefunds(t.Context())

	if len(f.payments.recordedSuccess) != 1 {
		t.Fatalf("the queued refund must be completed; recorded = %d", len(f.payments.recordedSuccess))
	}
	if len(notifier.cancelled) != 1 || !notifier.cancelled[0].ByOwner || notifier.cancelled[0].RefundAmount == "" {
		t.Errorf("the person must be told the refund arrived; got %+v", notifier.cancelled)
	}
	if len(f.bookings.markedOwed) != 0 {
		t.Error("a registration refund must not touch a booking")
	}
}

// Redelivery of an approval already recorded is the no-op, and an external
// refund is alerted rather than guessed at.
func TestAWebhookForARecordedRegistrationPaymentChangesNothing(t *testing.T) {
	f, reg, paid, notifier := registrationFixture(t)
	mpID := "mp-123"
	f.payments.byMPID = &paymentstore.Payment{ID: uuid.New(), RegistrationID: &reg.ID, Status: "deposit_paid", MPPaymentID: &mpID}
	f.provider.payment = paid

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatal(err)
	}
	paid.Status = "refunded"
	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatal(err)
	}
	if f.payments.regConfirmed != nil || len(notifier.registered) != 0 || len(f.provider.refunds) != 0 {
		t.Error("a recorded registration payment must not be processed again")
	}
}

// The webhook routes a payment by the registration in its metadata, ahead of the
// booking lookup.
func TestTheWebhookRoutesARegistrationPaymentToTheRegistrationFlow(t *testing.T) {
	f, reg, paid, notifier := registrationFixture(t)
	f.payments.byRegistration = []*paymentstore.Payment{checkoutRow(reg)}
	f.provider.payment = paid

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatal(err)
	}
	if len(notifier.registered) != 1 || f.bookings.updated != nil {
		t.Errorf("registered = %d, booking touched = %v", len(notifier.registered), f.bookings.updated != nil)
	}
}
