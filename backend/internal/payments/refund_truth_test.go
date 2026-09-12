package payments

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// TestARefundRecordsWhatMercadoPagoMovedNotWhatWasClaimed is the whole of
// finding 2 on the refund side.
//
// ClaimRefund computes the amount to send from the payment row under a lock,
// and that figure was then recorded, announced to the client and read back by
// refundable() as though it were what happened — because issueRefund threw
// MercadoPago's response away (`if _, err := h.provider.RefundPayment(...)`).
// The two agree almost always, and "almost always" is the shape of every
// money bug: when MercadoPago moves less than it was asked for, the payment
// row says the client is square, the email says the full amount is on its
// way, and the remainder is never spoken about again.
//
// The claim reserves 1500.00 and MercadoPago answers with 1000.00. What must
// be recorded is 1000.00.
//
// Mutation: in issueRefund, drop the reconciliation and hand the original
// claim to the recorder —
//
//	if _, err := h.provider.RefundPayment(...); err != nil { ... }
//	return claim, nil
//
// — and re-run. This test must fail on the recorded amount.
func TestARefundRecordsWhatMercadoPagoMovedNotWhatWasClaimed(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	booking.Status = "cancelled"
	f.payments.byBooking = payment
	f.payments.claimAmount = 150_000
	f.complexes.complex = linkedComplex(complexID, "")
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	// MercadoPago accepted the refund, for less than it was asked for.
	f.provider.refundAmount = 1_000.00

	sentryEvents := withCapturedSentryEvents(t)

	outcome := f.handler.AutoRefundIfPaid(t.Context(), booking)

	if len(f.payments.recordedSuccess) != 1 {
		t.Fatalf("an accepted refund must be recorded exactly once; got %d", len(f.payments.recordedSuccess))
	}
	if got := f.payments.recordedSuccess[0].RefundCentavos; got != 100_000 {
		t.Errorf("the recorded refund must be the amount MercadoPago says it moved, not the amount claimed — "+
			"a row that overstates the refund reads back as a settled payment and the balance is never returned; want 100000, got %d", got)
	}

	if len(f.notify.refunds) != 1 {
		t.Fatalf("the client must still be told about the money that did come back; got %d messages", len(f.notify.refunds))
	}
	if amount := f.notify.refunds[0].Amount; !strings.Contains(amount, "1.000") {
		t.Errorf("the client must be told the amount that was actually recorded, not the amount claimed; got %q", amount)
	}

	// The attempt row was resolved for the money that moved, so nothing queued
	// is left holding the remainder.
	if !outcome.NeedsAHuman() {
		t.Errorf("a short refund leaves the balance owed with no queued attempt behind it, so it must ask for a person; got result=%q reason=%q",
			outcome.Result, outcome.Reason)
	}
	if outcome.AmountCentavos != 50_000 {
		t.Errorf("the outcome must name the shortfall, which is what somebody has to return by hand; want 50000, got %d", outcome.AmountCentavos)
	}

	messages := sentryEvents.messages()
	if _, found := findMessage(messages, "REFUND AMOUNT MISMATCH"); !found {
		t.Errorf("a provider that moved a different sum than was claimed means this codebase and MercadoPago "+
			"disagree about a payment, and somebody has to look at it; captured %q", messages)
	}
	if _, found := findMessage(messages, "MANUAL REFUND OWED"); !found {
		t.Errorf("the shortfall must alert as money owed by hand; captured %q", messages)
	}
}

// TestAnAcceptedRefundWithNoAmountKeepsTheClaimedFigure is the guard on the
// test above.
//
// "Trust the provider" has an edge that would be worse than the defect it
// fixes: MercadoPago's refund response decoded to its zero value — an older
// API shape, a body that did not carry the field — is the absence of an
// answer, not the assertion that nothing moved. Reading it as zero would
// record a refund of nothing for money that has left the account, and leave
// the payment reading as fully owing.
//
// Mutation: delete the `refund.Amount <= 0` arm of reconcileRefundedAmount's
// guard and re-run — the recorded amount becomes 0 and this test fails.
func TestAnAcceptedRefundWithNoAmountKeepsTheClaimedFigure(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	booking.Status = "cancelled"
	f.payments.byBooking = payment
	f.payments.claimAmount = 150_000
	f.complexes.complex = linkedComplex(complexID, "")
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	// Accepted, with no amount on the response.
	f.provider.refundAmount = 0

	outcome := f.handler.AutoRefundIfPaid(t.Context(), booking)

	if len(f.payments.recordedSuccess) != 1 {
		t.Fatalf("an accepted refund must be recorded exactly once; got %d", len(f.payments.recordedSuccess))
	}
	if got := f.payments.recordedSuccess[0].RefundCentavos; got != 150_000 {
		t.Errorf("a response carrying no amount is an absent answer, not a refund of zero — the claimed figure "+
			"is the only other number in play; want 150000, got %d", got)
	}
	if outcome.Result != paymentstore.RefundIssued {
		t.Errorf("nothing here is short, so this is an ordinary issued refund; got result=%q reason=%q", outcome.Result, outcome.Reason)
	}
}

// TestARejectedRefundWithFundsAlreadyAtProviderResolvesAsSuccess covers the
// "already refunded" idempotency case: MercadoPago answers the refund POST
// with a 4xx (the payment was already refunded, a duplicate idempotency key,
// etc.), and the payment itself already carries at least the claimed amount
// in transaction_amount_refunded. Burning a retry attempt and alerting on
// that would be wrong — the money already moved — so this must resolve
// through the ordinary success path instead.
func TestARejectedRefundWithFundsAlreadyAtProviderResolvesAsSuccess(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	booking.Status = "cancelled"
	f.payments.byBooking = payment
	f.payments.claimAmount = 150_000
	f.complexes.complex = linkedComplex(complexID, "")
	f.clients.client = &clientstore.Client{ID: booking.ClientID}

	// The refund POST is rejected with a 4xx...
	f.provider.refundErr = &mp.APIError{StatusCode: 400, Body: `{"message":"payment already refunded"}`}
	// ...but the payment itself shows the money already moved.
	f.provider.payment = &mp.Payment{ID: 123, Status: "refunded", TransactionAmountRefunded: 1500.00}

	outcome := f.handler.AutoRefundIfPaid(t.Context(), booking)

	if len(f.payments.recordedFailure) != 0 {
		t.Fatalf("a refund already present at the provider must not spend a retry attempt; got %d recorded failures: %+v",
			len(f.payments.recordedFailure), f.payments.recordedFailure)
	}
	if len(f.payments.recordedSuccess) != 1 {
		t.Fatalf("the refund must resolve as a success once the provider confirms the money already moved; got %d", len(f.payments.recordedSuccess))
	}
	if got := f.payments.recordedSuccess[0].RefundCentavos; got != 150_000 {
		t.Errorf("want the claimed amount (fully covered by the provider) recorded; got %d", got)
	}
	if outcome.Result != paymentstore.RefundIssued {
		t.Errorf("want result=%q; got result=%q reason=%q", paymentstore.RefundIssued, outcome.Result, outcome.Reason)
	}
}

// TestAPartialRefundIsNotReadAsFullBecauseTheRowUnderstatesWhatWasPaid is
// finding 86 on the confirm-side flow.
//
// processRefundedPayment decides full-versus-partial, and it used to decide it
// against `payment.Amount + payment.ServiceFee` — this codebase's own arithmetic
// about what the client paid. MercadoPago's transaction_amount, which is the
// figure of the party that actually took the money, sat unused two fields away.
//
// When the row understates the charge, any refund at least that large reads as
// the whole of it: the payment goes to 'refunded', the booking to
// cancelled+'refunded', and refundable() answers RefundAlreadyIssued forever
// after. The client paid 1000.00, MercadoPago returned 600.00 of it, and the
// remaining 400.00 is quietly kept.
//
// Mutation: measure against the row again —
//
//	totalPaid := recordedTotal   // and delete the charged-amount branch
//
// — and re-run. 60000 >= 50000 makes this a full refund and the test fails on
// the payment status.
func TestAPartialRefundIsNotReadAsFullBecauseTheRowUnderstatesWhatWasPaid(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	// The row believes 500.00 was paid. It is wrong, and nothing in this flow
	// can tell that from the row alone.
	payment.Amount, payment.ServiceFee = 50_000, 0
	f.bookings.booking = booking
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.complexes.complex = linkedComplex(complexID, "")

	mpPayment := &mp.Payment{
		ID:                        123,
		Status:                    "refunded",
		TransactionAmount:         1_000.00,
		TransactionAmountRefunded: 600.00,
	}

	sentryEvents := withCapturedSentryEvents(t)

	if err := f.handler.processRefundedPayment(t.Context(), payment, mpPayment, "mp-123"); err != nil {
		t.Fatalf("processing a refund webhook: %v", err)
	}

	if payment.Status == "refunded" {
		t.Errorf("MercadoPago returned 600.00 of the 1000.00 it charged, so the payment is not settled — " +
			"marking it 'refunded' makes refundable() decline the balance for good")
	}
	if f.payments.confirmed != nil {
		t.Errorf("a partial refund must not run the full-refund write that cancels the booking and marks it refunded")
	}
	if f.payments.updated == nil {
		t.Fatalf("a partial refund must still record what came back")
	}
	// Capped by payments_refund_within_amount_paid, which is the
	// reason the recorded and charged totals are kept apart in the first place.
	if got := f.payments.updated.RefundAmount; got != 50_000 {
		t.Errorf("refund_amount is bounded by this row's own amount + service_fee, or the write is refused outright; want 50000, got %d", got)
	}

	messages := sentryEvents.messages()
	if _, found := findMessage(messages, "PAYMENT AMOUNT MISMATCH"); !found {
		t.Errorf("a payment row that disagrees with MercadoPago about what was charged is a reconciliation failure "+
			"and has to be reported, whichever of the two is wrong; captured %q", messages)
	}
}

// TestAnUnclaimedRefundLeavesTheSweepAMarkerRatherThanAFalseRefundPending is
// change 5's remainder on the one refund path that never had a claim behind
// its state write.
//
// A payment landing for an already-cancelled booking is recorded first,
// because a claim is a reservation on a payment row and there is nothing to
// reserve until the row exists. That first write used to commit
// refund_status='pending' — the exact value refundable() reads as "a
// claim is already committed against this booking's payment, so the money is
// on its way". Between that commit and ClaimRefund's, nothing was on its way.
//
// A crash or a database refusal in that window left a cancelled, paid booking
// that every later refund path declined as already in flight, with no
// failed_refunds row for the retry job and no marker for the sweep. The
// webhook event's own retry could not repair it either: on redelivery the
// payment now carries the MercadoPago id, and processPaymentWebhook
// short-circuits on "payment already processed, skipping".
//
// This drives that window by failing ClaimRefund, and asserts on what the
// store was handed — the booking as it was committed alongside the payment,
// not as the flow left it in memory.
//
// Mutation: restore the old write in recordPaymentOwedARefund —
//
//	booking.RefundStatus = data.RefundStatusPending   // and drop the marker
//
// — and re-run. Both assertions below must fail.
func TestAnUnclaimedRefundLeavesTheSweepAMarkerRatherThanAFalseRefundPending(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	booking.Status = "cancelled"
	booking.CollectionStatus = data.CollectionStatusUnpaid
	f.payments.byBooking = payment
	// The claim is what fails: the database refused it, or this process is about
	// to die. Either way the payment row is already committed.
	f.payments.claimErr = errDatabase

	mpPayment := &mp.Payment{ID: 123, Status: "approved", TransactionAmount: 1_500.00}

	err := f.handler.refundCancelledBookingPayment(t.Context(), booking, mpPayment, "mp-123")
	if err == nil {
		t.Fatalf("a claim the database refused must be returned as retryable")
	}

	committed := f.payments.confirmedBooking
	if committed == nil {
		t.Fatalf("the payment behind the refund must be recorded against its booking before any claim")
	}

	if committed.RefundStatus == data.RefundStatusPending {
		t.Errorf("no claim exists yet, so committing refund_status 'pending' asserts a refund is in flight when none is — " +
			"refundable() reads it that way and declines the refund this write was meant to guarantee")
	}
	if committed.RefundIntentAt == nil {
		t.Errorf("the commit that records the payment must carry the refund-intent marker, or the crash window " +
			"before ClaimRefund leaves money with no queued attempt, no marker and nothing that ever looks for it")
	}
}

// TestAClaimedRefundLeavesNoMarkerBehind is the other half of the marker's
// lifecycle on this path, and the reason the test above is not enough on its
// own: a marker that is set and never cleared makes the sweep re-claim a
// booking whose refund is already done.
//
// ClaimRefund clears it inside its own committed transaction, so the ordinary
// path must leave nothing for the sweep to find — and the deferred cleanup
// must not clear it a second time on top of that.
//
// Mutation: set `committed = true` before ClaimRefund is called in
// refundCancelledBookingPayment, so the cleanup is disarmed on the failure
// path too, and re-run TestAnUnclaimedRefundLeavesTheSweepAMarkerRatherThan...
// alongside this one.
func TestAClaimedRefundLeavesNoMarkerBehind(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	booking.Status = "cancelled"
	booking.CollectionStatus = data.CollectionStatusUnpaid
	f.payments.byBooking = payment
	f.complexes.complex = linkedComplex(complexID, "")
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.provider.refundAmount = 1_500.00
	f.payments.claimAmount = 150_000

	mpPayment := &mp.Payment{ID: 123, Status: "approved", TransactionAmount: 1_500.00}

	if err := f.handler.refundCancelledBookingPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
		t.Fatalf("refunding a payment for a cancelled booking: %v", err)
	}

	if len(f.payments.claimed) != 1 {
		t.Fatalf("the refund must go through exactly one claim; got %d", len(f.payments.claimed))
	}
	if len(f.refundIntents.cleared) != 0 {
		t.Errorf("a committed claim clears the marker inside its own transaction, so clearing it again here is a "+
			"second round trip that can only undo a marker another path has since set; got %v", f.refundIntents.cleared)
	}
}
