package payments

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// The webhook reads the payment row before it asks MercadoPago about it. A
// ClaimRefund that commits in between moves the row to refund_pending, and from
// then on the claim owns the refund. The webhook's write is conditional on the
// row not being refund_pending, and when it writes nothing the webhook stands
// down exactly like the refund_pending guard at the top of the function: no
// booking change, no notice, no audit entry, no realtime push.
func TestABookingRefundWebhookStandsDownWhenAClaimCommittedMeanwhile(t *testing.T) {
	for name, tc := range map[string]struct {
		mp            *mp.Payment
		paidByAnother bool
	}{
		"partial refund": {
			mp: &mp.Payment{ID: 123, Status: "approved", StatusDetail: "partially_refunded", TransactionAmount: 1500, TransactionAmountRefunded: 500},
		},
		"full refund": {
			mp: &mp.Payment{ID: 123, Status: "refunded", StatusDetail: "refunded", TransactionAmount: 1500, TransactionAmountRefunded: 1500},
		},
		"full refund on a booking another payment holds": {
			mp:            &mp.Payment{ID: 123, Status: "refunded", StatusDetail: "refunded", TransactionAmount: 1500, TransactionAmountRefunded: 1500},
			paidByAnother: true,
		},
		"chargeback on a booking another payment holds": {
			mp:            &mp.Payment{ID: 123, Status: "charged_back", TransactionAmount: 1500},
			paidByAnother: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, payment := paidBooking(complexID)
			f.bookings.booking = booking
			f.complexes.complex = linkedComplex(complexID, "111111111")
			f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}
			if tc.paidByAnother {
				otherID := "mp-999"
				f.payments.byBookingAll = []*paymentstore.Payment{payment, {
					ID: uuid.New(), BookingID: booking.ID, ComplexID: complexID, Amount: 150_000,
					Status: "deposit_paid", MPPaymentID: &otherID,
				}}
			}
			f.payments.claimedMeanwhile = true

			if err := f.service.processRefundedPayment(t.Context(), payment, tc.mp, "mp-123"); err != nil {
				t.Fatalf("standing down must not fail: %v", err)
			}

			if f.payments.updated != nil || f.payments.confirmed != nil {
				t.Errorf("nothing may be written; updated=%+v confirmed=%+v", f.payments.updated, f.payments.confirmed)
			}
			if len(f.notify.refunds) != 0 || len(f.notify.ownerRefunds) != 0 {
				t.Errorf("nobody may be told; got %+v %+v", f.notify.refunds, f.notify.ownerRefunds)
			}
			if len(f.audit.entries) != 0 {
				t.Errorf("nothing may be recorded; got %+v", f.audit.entries)
			}
			if len(f.realtime.published) != 0 {
				t.Errorf("nothing changed, so nothing may be published; got %v", f.realtime.published)
			}
			if len(f.waitlist.freed) != 0 {
				t.Errorf("no slot was freed; got %+v", f.waitlist.freed)
			}
			if f.bookings.updated != nil {
				t.Errorf("the booking may not be written; got %+v", f.bookings.updated)
			}
			if !strings.Contains(f.logs.String(), "while the webhook was reading it") {
				t.Errorf("the stand-down must be logged; got %s", f.logs.String())
			}
		})
	}
}

// The same race on a registration payment: the panel refund is not recorded,
// audited or published once a claim holds the row.
func TestARegistrationPanelRefundStandsDownWhenAClaimCommittedMeanwhile(t *testing.T) {
	f, reg, paid, _ := registrationFixture(t)
	f.payments.byMPID = recordedRegistrationPayment(reg, "deposit_paid", 0)
	f.provider.payment = panelRefund(paid, 2000)
	f.payments.claimedMeanwhile = true

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatalf("standing down must not fail: %v", err)
	}
	if f.payments.updated != nil {
		t.Errorf("nothing may be written; got %+v", f.payments.updated)
	}
	if len(f.audit.entries) != 0 {
		t.Errorf("nothing may be recorded; got %+v", f.audit.entries)
	}
	if len(f.realtime.published) != 0 {
		t.Errorf("nothing may be published; got %v", f.realtime.published)
	}
	if !strings.Contains(f.logs.String(), "while the webhook was reading it") {
		t.Errorf("the stand-down must be logged; got %s", f.logs.String())
	}
}
