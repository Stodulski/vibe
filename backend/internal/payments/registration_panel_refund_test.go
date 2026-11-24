package payments

import (
	"testing"

	"github.com/google/uuid"

	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// recordedRegistrationPayment is a registration's approved payment of 8.000 plus
// its 200 fee, already recorded, with refunded centavos already on the row.
func recordedRegistrationPayment(reg *eventstore.Registration, status string, refunded int) *paymentstore.Payment {
	mpID := "mp-123"
	return &paymentstore.Payment{
		ID: uuid.New(), RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: reg.Price, ServiceFee: reg.ServiceFee,
		Method: "mercadopago", Status: status, MPPaymentID: &mpID, RefundAmount: refunded,
	}
}

// panelRefund is MercadoPago's view of the same payment after a partial refund
// made from its own panel: still approved, with the running refunded total.
func panelRefund(paid *mp.Payment, refundedPesos float64) *mp.Payment {
	paid.StatusDetail = "partially_refunded"
	paid.TransactionAmountRefunded = refundedPesos
	return paid
}

// S1h: a partial refund made from MercadoPago's panel on a registration payment
// is recorded on the row and in the trail; the spot stays as it is and nothing is
// refunded by the backend.
func TestAPartialPanelRefundOnARegistrationPaymentIsRecorded(t *testing.T) {
	f, reg, paid, notifier := registrationFixture(t)
	reg.Status = eventstore.RegistrationConfirmed
	f.payments.byMPID = recordedRegistrationPayment(reg, "deposit_paid", 0)
	f.provider.payment = panelRefund(paid, 2000)

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatalf("recording a panel refund must not fail: %v", err)
	}

	if f.payments.updated == nil || f.payments.updated.RefundAmount != 200_000 {
		t.Fatalf("want the payment to record 200_000 refunded; got %+v", f.payments.updated)
	}
	if f.payments.updated.Status != "deposit_paid" {
		t.Errorf("a partial refund must keep the payment status; got %q", f.payments.updated.Status)
	}
	if f.payments.regConfirmed != nil || len(notifier.cancelled) != 0 || len(notifier.registered) != 0 {
		t.Error("the registration must stay as it is")
	}
	if reg.Status != eventstore.RegistrationConfirmed {
		t.Errorf("the registration must stay confirmed; got %q", reg.Status)
	}
	if len(f.provider.refunds) != 0 {
		t.Errorf("the backend must not issue a refund of its own; got %v", f.provider.refunds)
	}
	entries := f.audit.find("refund_external")
	if len(entries) != 1 {
		t.Fatalf("want one refund_external entry; got %d", len(entries))
	}
	if entries[0].EntityType != registrationEntity || entries[0].EntityID == nil || *entries[0].EntityID != reg.ID {
		t.Errorf("the entry must be keyed on the registration; got %s %v", entries[0].EntityType, entries[0].EntityID)
	}
	if got := value(t, entries[0])["amount_centavos"]; got != float64(200_000) {
		t.Errorf("want the entry to record 200_000 moved; got %v", got)
	}
}

// S1i: recording the panel refund pushes the change to the owner's open screens,
// the way every other registration write does, so an open participant list shows
// the refund without a reload.
func TestAPartialPanelRefundOnARegistrationIsPublished(t *testing.T) {
	f, reg, paid, _ := registrationFixture(t)
	f.payments.byMPID = recordedRegistrationPayment(reg, "deposit_paid", 0)
	f.provider.payment = panelRefund(paid, 2000)

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatal(err)
	}
	if len(f.realtime.published) != 1 || f.realtime.published[0] != reg.ComplexID {
		t.Errorf("want one change published for complex %s; got %v", reg.ComplexID, f.realtime.published)
	}
}

// A second partial refund records the new running total on the row and only the
// difference as what moved.
func TestASecondPanelRefundOnARegistrationRecordsOnlyTheDifference(t *testing.T) {
	f, reg, paid, _ := registrationFixture(t)
	f.payments.byMPID = recordedRegistrationPayment(reg, "deposit_paid", 200_000)
	f.provider.payment = panelRefund(paid, 3000)

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatal(err)
	}
	if f.payments.updated == nil || f.payments.updated.RefundAmount != 300_000 {
		t.Fatalf("want the payment to hold 300_000 refunded; got %+v", f.payments.updated)
	}
	entries := f.audit.find("refund_external")
	if len(entries) != 1 {
		t.Fatalf("want one refund_external entry; got %d", len(entries))
	}
	if got := value(t, entries[0])["amount_centavos"]; got != float64(100_000) {
		t.Errorf("want the entry to record 100_000 moved this time; got %v", got)
	}
}

// MercadoPago reporting more than the row can hold (a CHECK caps refund_amount
// at the row's total) is written capped at that total.
func TestAPanelRefundAboveTheRegistrationRowTotalIsCapped(t *testing.T) {
	f, reg, paid, _ := registrationFixture(t)
	f.payments.byMPID = recordedRegistrationPayment(reg, "deposit_paid", 0)
	f.provider.payment = panelRefund(paid, 9000)

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatal(err)
	}
	if f.payments.updated == nil || f.payments.updated.RefundAmount != 820_000 {
		t.Fatalf("want the refund capped at 820_000; got %+v", f.payments.updated)
	}
}

// The echo of a refund Vibe already recorded, and a refund one of Vibe's own
// claims still holds, write nothing: the refund lifecycle owns those.
func TestTheEchoOfAnInAppRegistrationRefundWritesNothing(t *testing.T) {
	for name, row := range map[string]func(*eventstore.Registration) *paymentstore.Payment{
		"already recorded": func(reg *eventstore.Registration) *paymentstore.Payment {
			return recordedRegistrationPayment(reg, "deposit_paid", 200_000)
		},
		"claim in flight": func(reg *eventstore.Registration) *paymentstore.Payment {
			return recordedRegistrationPayment(reg, "refund_pending", 0)
		},
	} {
		t.Run(name, func(t *testing.T) {
			f, reg, paid, _ := registrationFixture(t)
			confirmSpot(reg)
			f.payments.byMPID = row(reg)
			f.provider.payment = panelRefund(paid, 2000)

			if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
				t.Fatal(err)
			}
			if f.payments.updated != nil {
				t.Errorf("nothing may be written; got %+v", f.payments.updated)
			}
			if len(f.audit.entries) != 0 {
				t.Errorf("nothing may be recorded; got %+v", f.audit.entries)
			}
		})
	}
}

// A full refund from the panel keeps today's behaviour: alerted for review,
// nothing written.
func TestAFullPanelRefundOnARegistrationIsStillOnlyAlerted(t *testing.T) {
	f, reg, paid, notifier := registrationFixture(t)
	f.payments.byMPID = recordedRegistrationPayment(reg, "deposit_paid", 0)
	paid.Status = "refunded"
	paid.StatusDetail = "refunded"
	paid.TransactionAmountRefunded = 8200
	f.provider.payment = paid

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatal(err)
	}
	if f.payments.updated != nil || len(f.audit.entries) != 0 || len(notifier.cancelled) != 0 || len(f.provider.refunds) != 0 {
		t.Errorf("a full panel refund must only be alerted; updated=%+v entries=%d", f.payments.updated, len(f.audit.entries))
	}
}

// A partial panel refund the platform's read carries no figure for is read as
// the venue's own seller account, as on the booking path: without it the
// refund was never recorded and a later refund of the spot claimed money
// already returned.
func TestAPartialPanelRefundThePlatformCannotSeeIsReadAsTheSeller(t *testing.T) {
	f, reg, paid, _ := registrationFixture(t)
	reg.Status = eventstore.RegistrationConfirmed
	f.payments.byMPID = recordedRegistrationPayment(reg, "deposit_paid", 0)
	f.complexes.complex = linkedComplex(reg.ComplexID, "111111111")
	f.provider.payment = panelRefund(paid, 0)
	f.provider.sellerPayment = &mp.Payment{ID: 123, Status: "approved", StatusDetail: "partially_refunded", TransactionAmountRefunded: 2000}

	if err := f.service.processPaymentWebhook(t.Context(), "123"); err != nil {
		t.Fatalf("recording a panel refund must not fail: %v", err)
	}

	if f.payments.updated == nil || f.payments.updated.RefundAmount != 200_000 {
		t.Fatalf("want the payment to record the seller's 200_000 refunded; got %+v", f.payments.updated)
	}
}
