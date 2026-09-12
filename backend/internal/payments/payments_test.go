package payments

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

const webhookBody = `{"type":"payment","data":{"id":"mp-123"}}`

func webhookRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
}

// An unsigned or wrongly-signed webhook must be dropped before anything in the
// body is read, let alone acted on. Anyone can POST to this endpoint.
func TestWebhookRejectsABadSignatureBeforeDoingAnything(t *testing.T) {
	f := newFixture(t)
	f.provider.signatureErr = errProvider

	w := httptest.NewRecorder()
	f.handler.MercadoPagoWebhook(w, webhookRequest(t, webhookBody))

	if len(f.locks.attempts) != 0 {
		t.Error("an unverified webhook must not even reach the idempotency lock")
	}
	if f.payments.confirmed != nil || f.payments.inserted != nil {
		t.Error("an unverified webhook must not touch a payment")
	}
	// MercadoPago retries anything that is not 2xx, and retrying a forged
	// request forever helps nobody.
	if w.Code != http.StatusOK {
		t.Errorf("want 200 so the provider stops retrying; got %d", w.Code)
	}
}

func TestWebhookVerifiesTheSignatureAgainstTheAdvertisedID(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.MercadoPagoWebhook(w, webhookRequest(t, webhookBody))

	if len(f.provider.verified) != 1 || f.provider.verified[0] != "mp-123" {
		t.Errorf("the signature must be checked against the payment id in the body; got %v", f.provider.verified)
	}
}

// MercadoPago delivers the same webhook more than once; the second delivery
// must do nothing rather than confirm the booking twice.
func TestDuplicateWebhookDeliveryIsSkipped(t *testing.T) {
	f := newFixture(t)
	f.locks.taken = true // another instance already holds it

	_ = f.service.processPaymentWebhook(t.Context(), "mp-123")

	if f.payments.confirmed != nil || f.payments.inserted != nil {
		t.Error("a duplicate delivery must not process the payment again")
	}
	if len(f.locks.attempts) != 1 || f.locks.attempts[0] != "mp_webhook:mp-123" {
		t.Errorf("the lock must be keyed on the payment id; got %v", f.locks.attempts)
	}
}

func TestTheIdempotencyLockIsAlwaysReleased(t *testing.T) {
	f := newFixture(t)
	f.payments.mpIDErr = errProvider // fail immediately after taking the lock

	_ = f.service.processPaymentWebhook(t.Context(), "mp-123")

	if f.locks.released != 1 {
		t.Errorf("the lock must be released even when handling fails; released %d times", f.locks.released)
	}
}

// A booking that is already confirmed, completed or played must not be
// re-confirmed: it would resend the confirmation and rewrite the payment.
func TestApprovedPaymentSkipsABookingThatIsAlreadySettled(t *testing.T) {
	for _, status := range []string{"confirmed", "completed", "no_show"} {
		t.Run(status, func(t *testing.T) {
			f := newFixture(t)
			booking, _ := paidBooking(uuid.New())
			booking.Status = status

			_ = f.service.processApprovedPayment(t.Context(), booking, &mp.Payment{ID: 123}, "mp-123")

			if f.payments.inserted != nil || f.payments.confirmed != nil {
				t.Error("the payment must not be recorded again")
			}
			if len(f.notify.confirmations) != 0 {
				t.Error("the client must not be told twice")
			}
		})
	}
}

// Money arriving for a booking that was already cancelled has to be refunded,
// not silently kept — as long as the money is proven to have come from the
// complex's own collector (payment-collector-verification spec, scenario 2).
func TestApprovedPaymentForACancelledBookingIsRefunded(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, _ := paidBooking(complexID)
	booking.Status = "cancelled"
	f.complexes.complex = linkedComplex(complexID, "111111111")

	_ = f.service.processApprovedPayment(t.Context(), booking, &mp.Payment{ID: 123, TransactionAmount: 1500, CollectorID: 111111111}, "mp-123")

	if len(f.provider.refunds) == 0 {
		t.Error("payment for a cancelled booking must be refunded to the client")
	}
	if len(f.notify.confirmations) != 0 {
		t.Error("a cancelled booking must not send a confirmation")
	}
}

// The already-cancelled branch is where a fictitious payment row used to slip
// through: a genuine seller on the marketplace can create a preference with
// their own token, point external_reference at someone else's already-cancelled
// booking, and pay themselves — the amount check does not even apply to this
// branch, so only the collector check stands between this and a payment row
// durably attached to a victim complex (payment-collector-verification spec,
// scenario 1).
func TestApprovedPaymentForACancelledBookingWithAForeignCollectorWritesNothing(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, _ := paidBooking(complexID)
	booking.Status = "cancelled"
	beforeCollection, beforeRefund := booking.CollectionStatus, booking.RefundStatus
	ownSellerID := "111111111"
	f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &ownSellerID}

	_ = f.service.processApprovedPayment(t.Context(), booking,
		&mp.Payment{ID: 123, TransactionAmount: 1500, CollectorID: 999999999, ExternalReference: booking.ID.String()}, "mp-123")

	if f.payments.inserted != nil || f.payments.confirmed != nil {
		t.Error("no payment row may be written for a cancelled booking's payment collected by a foreign seller")
	}
	if len(f.payments.claimed) != 0 {
		t.Error("ClaimRefund must never be called for an unverified collector")
	}
	if booking.CollectionStatus != beforeCollection || booking.RefundStatus != beforeRefund {
		t.Errorf("neither payment axis may move toward a refund; got collection_status=%q refund_status=%q",
			booking.CollectionStatus, booking.RefundStatus)
	}
	if len(f.provider.refunds) != 0 {
		t.Error("RefundPayment must never be called for an unverified collector")
	}
}

// The concurrent-cancellation re-entry used to recurse into
// processApprovedPayment, and that was the only path that
// could run the collector check twice, because the old check sat below the
// cancelled branch's early return and was never reached on re-entry. Now that
// the check dominates every branch, the direct call to
// refundCancelledBookingPayment (rather than recursing back into
// processApprovedPayment) is what keeps the check running exactly once.
//
// ClaimRefund is made to fail so the call stops before
// refundCancelledBookingPayment reaches sellerCredential, which calls the same
// h.complexes.GetByID dependency for an unrelated reason (the seller token) —
// failing the claim isolates the count to the collector check alone.
func TestConcurrentCancellationRunsTheCollectorCheckOnce(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	initialBooking, mpPayment := pendingBooking(complexID)
	sellerID := "111111111"
	mpPayment.CollectorID = 111111111
	f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &sellerID}
	f.payments.claimErr = errDatabase

	// The booking was cancelled by another process (e.g. the expiry cron)
	// between the amount/collector validation above and the re-fetch at :101 —
	// the re-fetch returns a different, now-cancelled, booking.
	cancelledBooking := *initialBooking
	cancelledBooking.Status = "cancelled"
	f.bookings.booking = &cancelledBooking

	_ = f.service.processApprovedPayment(t.Context(), initialBooking, mpPayment, "mp-123")

	if f.complexes.calls != 1 {
		t.Errorf("the collector check must run exactly once across the concurrent-cancellation re-entry; complexes.GetByID called %d times", f.complexes.calls)
	}
	if f.payments.inserted == nil {
		t.Fatal("the payment must still be recorded on the way to the failed claim, proving refundCancelledBookingPayment was reached")
	}
}

func TestRejectedPaymentLeavesASettledBookingAlone(t *testing.T) {
	f := newFixture(t)
	booking, _ := paidBooking(uuid.New())
	booking.Status = "confirmed"

	_ = f.service.processRejectedPayment(t.Context(), booking, &mp.Payment{ID: 123}, "mp-123")

	if f.bookings.updated != nil {
		t.Error("a confirmed booking must not be cancelled by a rejected payment attempt")
	}
}

func TestAutoRefundSkipsWhatCannotBeRefunded(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*fixture, *bookingstore.Booking, *paymentstore.Payment)
	}{
		{
			name: "the booking was never paid",
			prepare: func(f *fixture, b *bookingstore.Booking, _ *paymentstore.Payment) {
				b.CollectionStatus = bookingstore.CollectionStatusUnpaid
			},
		},
		{
			name: "the payment did not go through MercadoPago",
			prepare: func(f *fixture, _ *bookingstore.Booking, p *paymentstore.Payment) {
				p.MPPaymentID = nil
				f.payments.byBooking = p
			},
		},
		{
			name: "it was refunded already",
			prepare: func(f *fixture, _ *bookingstore.Booking, p *paymentstore.Payment) {
				p.Status = "refunded"
				f.payments.byBooking = p
			},
		},
		{
			name: "there is no payment record at all",
			prepare: func(f *fixture, _ *bookingstore.Booking, _ *paymentstore.Payment) {
				f.payments.bookingErr = data.ErrRecordNotFound
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			booking, payment := paidBooking(uuid.New())
			f.payments.byBooking = payment
			tt.prepare(f, booking, payment)

			f.service.AutoRefundIfPaid(t.Context(), booking)

			if len(f.provider.refunds) != 0 {
				t.Errorf("no refund should have been attempted; got %v", f.provider.refunds)
			}
		})
	}
}

func TestAutoRefundIssuesTheRefundAndTellsTheClient(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	f.payments.byBooking = payment
	f.complexes.complex = linkedComplex(complexID, "")
	f.complexes.complex.Name = "Vibe"
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", Phone: "+5491155551234"}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	f.service.AutoRefundIfPaid(t.Context(), booking)

	if len(f.provider.refunds) != 1 {
		t.Fatalf("want one refund; got %v", f.provider.refunds)
	}
	if len(f.payments.claimed) != 1 || f.payments.claimed[0] != payment.ID {
		t.Errorf("the refund must be claimed against the payment before the provider is called; got %v", f.payments.claimed)
	}
	if len(f.payments.recordedSuccess) != 1 {
		t.Errorf("the refund must be recorded once the provider accepted it; got %d records", len(f.payments.recordedSuccess))
	}
	if len(f.payments.recordedFailure) != 0 {
		t.Error("a successful refund must not be requeued")
	}
	if len(f.notify.refunds) != 1 {
		t.Fatalf("the client must be told their deposit came back; got %d notifications", len(f.notify.refunds))
	}

	// The refund notice reaches WhatsApp too. A client who gave a number and
	// got their confirmation there used to hear about their money only by
	// email — the payload had no phone field at all, so the channel was not
	// disabled, it was unrepresentable.
	sent := f.notify.refunds[0]
	if sent.Phone != "+5491155551234" {
		t.Errorf("the refund notice cannot reach WhatsApp; got phone %q", sent.Phone)
	}
	if sent.Amount == "" {
		t.Errorf("the refund notice must name the amount; got %q", sent.Amount)
	}
	if sent.BookPath != f.complexes.complex.Slug+"/book" {
		t.Errorf("the refund notice offers no way back; got book path %q", sent.BookPath)
	}
	if sent.BookURL != "https://vibe.test/"+f.complexes.complex.Slug+"/book" {
		t.Errorf("the refund email offers no way back; got %q", sent.BookURL)
	}
}

// If MercadoPago rejects the refund, nothing may read as refunded and the attempt
// must stay queued. There is no rollback to make any more: the claim never
// recorded a refund in the first place, it recorded that one is owed.
func TestAFailedRefundStaysQueuedAndIsNotRecordedAsDone(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	f.payments.byBooking = payment
	f.complexes.complex = linkedComplex(complexID, "")
	f.provider.refundErr = errProvider

	f.service.AutoRefundIfPaid(t.Context(), booking)

	if len(f.payments.claimed) != 1 {
		t.Fatalf("the refund must have been claimed before the provider was called; got %v", f.payments.claimed)
	}
	if len(f.payments.recordedSuccess) != 0 {
		t.Error("nothing may be recorded as refunded when the provider rejected the refund")
	}
	if len(f.payments.recordedFailure) != 1 {
		t.Fatalf("the attempt must be requeued for retry; got %d", len(f.payments.recordedFailure))
	}
	requeued := f.payments.recordedFailure[0]
	if requeued.claim.PaymentID != payment.ID {
		t.Errorf("the requeued attempt names the wrong payment: %+v", requeued.claim)
	}
	if requeued.cause == "" {
		t.Error("the requeued attempt must record why the provider refused it")
	}
	if len(f.notify.refunds) != 0 {
		t.Error("the client must not be told about a refund the provider refused")
	}
}

// A concurrent path already refunding is not an error: the claim's row lock is
// what prevents the double refund, and losing that race means the work is already
// owned by whoever won it. Both refusals have to be silent, but only one of them
// means the money is already back.
func TestAConcurrentRefundIsNotIssuedTwice(t *testing.T) {
	tests := []struct {
		name    string
		refusal error
	}{
		{name: "the payment is already refunded", refusal: paymentstore.ErrAlreadyRefunded},
		{name: "another claim holds the refund", refusal: paymentstore.ErrRefundInFlight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			booking, payment := paidBooking(uuid.New())
			f.payments.byBooking = payment
			f.payments.claimErr = tt.refusal

			f.service.AutoRefundIfPaid(t.Context(), booking)

			if len(f.provider.refunds) != 0 {
				t.Error("a refund already claimed elsewhere must not be issued twice")
			}
			if len(f.payments.recordedFailure) != 0 {
				t.Error("losing the race is not a failure and must not requeue anything")
			}
			if len(f.notify.refunds) != 0 {
				t.Error("only the claim that actually refunded may tell the client")
			}
		})
	}
}

// The refund has to be issued against the complex's own MercadoPago
// credentials; the platform token would refund from the wrong account.
func TestRefundsUseTheComplexOwnSellerToken(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	sellerToken := "seller-access-token"
	f.payments.byBooking = payment
	f.complexes.complex = complexstore.NewComplexForTest(complexID, &sellerToken, nil)

	f.service.AutoRefundIfPaid(t.Context(), booking)

	wantCaller := mustSeller(t, sellerToken)
	found := false
	for _, caller := range f.provider.callers {
		if caller == wantCaller {
			found = true
		}
	}
	if !found {
		t.Errorf("the refund must be issued as the complex's own seller; got %d call(s), none of them that one", len(f.provider.callers))
	}
}

func TestRetryQueueResolvesASucceedingRefund(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	frID := uuid.New()
	mpID := "mp-123"

	f.failedRefunds.pending = []*paymentstore.FailedRefund{{
		ID: frID, BookingID: booking.ID, ComplexID: complexID,
		PaymentID: payment.ID, Amount: 150_000, MPPaymentID: mpID,
	}}
	f.payments.byBooking = payment
	f.bookings.booking = booking
	f.complexes.complex = linkedComplex(complexID, "")

	f.service.RetryFailedRefunds(t.Context())

	if len(f.provider.refunds) != 1 {
		t.Fatalf("the queued refund must be retried; got %v", f.provider.refunds)
	}
	if len(f.payments.recordedSuccess) != 1 {
		t.Fatalf("a succeeding retry must be recorded; got %d records", len(f.payments.recordedSuccess))
	}
	// One store call, so the payment, the booking and the attempt move in the same
	// transaction. Resolving the attempt separately, and first, is what used to
	// leave a refunded client holding a court that still read as sold.
	recorded := f.payments.recordedSuccess[0]
	if recorded.AttemptID != frID || recorded.PaymentID != payment.ID {
		t.Errorf("the recorded refund names the wrong attempt or payment: %+v", recorded)
	}
	if recorded.RefundCentavos != 150_000 {
		t.Errorf("the queued amount must be the one recorded; got %d", recorded.RefundCentavos)
	}
}

func TestRetryQueueKeepsRetryingAFailingRefund(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	frID := uuid.New()
	mpID := "mp-123"

	f.failedRefunds.pending = []*paymentstore.FailedRefund{{
		ID: frID, BookingID: booking.ID, ComplexID: complexID,
		PaymentID: payment.ID, Amount: 150_000, MPPaymentID: mpID, RetryCount: 1,
	}}
	f.complexes.complex = linkedComplex(complexID, "")
	f.provider.refundErr = errProvider

	f.service.RetryFailedRefunds(t.Context())

	if len(f.payments.recordedSuccess) != 0 {
		t.Error("a refund the provider rejected must not be recorded as done")
	}
	if len(f.payments.recordedFailure) != 1 {
		t.Fatalf("a failing refund must be requeued, never dropped; got %d", len(f.payments.recordedFailure))
	}
	if len(f.failedRefunds.processing) != 1 || f.failedRefunds.processing[0] != frID {
		t.Errorf("the attempt must be claimed as in-flight before the provider is called; got %v", f.failedRefunds.processing)
	}
}

// A genuine seller on the marketplace can create a preference with their own token,
// point external_reference at someone else's booking, and pay themselves the right
// amount. The signature is real and the amount matches, so the only thing standing
// between that and a free court is checking who actually collected the money.
func TestApprovedPaymentIsRefusedWhenAnotherSellerCollectedTheMoney(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	ownSellerID := "111111111"
	f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &ownSellerID}
	f.bookings.booking = booking
	mpPayment.CollectorID = 999999999 // the attacker's own seller account

	_ = f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123")

	if f.payments.inserted != nil || f.payments.confirmed != nil {
		t.Error("a booking must not be paid for by money that landed in another seller's account")
	}
	if booking.Status == "confirmed" {
		t.Error("the booking must not be confirmed")
	}
	if len(f.notify.confirmations) != 0 {
		t.Error("no confirmation may be sent for a payment collected by someone else")
	}
}

// Anything that leaves the collector unproven has to refuse too: trusting
// external_reference on its own is exactly the hole being closed.
func TestApprovedPaymentIsRefusedWhenTheCollectorCannotBeVerified(t *testing.T) {
	empty := ""
	sellerID := "111111111"

	tests := []struct {
		name    string
		prepare func(*fixture, *mp.Payment, uuid.UUID)
		// wantRetry marks the refusals that are an outage rather than a verdict.
		// Those have to come back as an error so the recorded webhook event is
		// retried; dropping a genuine payment because the database blinked is the
		// exact failure the durable inbox exists to prevent.
		wantRetry bool
	}{
		{
			name: "the complex has no MercadoPago account linked",
			prepare: func(f *fixture, p *mp.Payment, complexID uuid.UUID) {
				f.complexes.complex = &complexstore.Complex{ID: complexID}
				p.CollectorID = 111111111
			},
		},
		{
			name: "the complex's MercadoPago user id is blank",
			prepare: func(f *fixture, p *mp.Payment, complexID uuid.UUID) {
				f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &empty}
				p.CollectorID = 111111111
			},
		},
		{
			name: "MercadoPago sent no collector_id",
			prepare: func(f *fixture, p *mp.Payment, complexID uuid.UUID) {
				f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &sellerID}
				p.CollectorID = 0
			},
		},
		{
			name: "the complex could not be loaded",
			prepare: func(f *fixture, p *mp.Payment, _ uuid.UUID) {
				f.complexes.err = errDatabase
				p.CollectorID = 111111111
			},
			wantRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking, mpPayment := pendingBooking(complexID)
			f.bookings.booking = booking
			tt.prepare(f, mpPayment, complexID)

			err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123")

			if f.payments.inserted != nil || f.payments.confirmed != nil {
				t.Error("an unverifiable collector must not confirm the booking")
			}
			if len(f.notify.confirmations) != 0 {
				t.Error("no confirmation may be sent when the collector is unverified")
			}
			if tt.wantRetry && err == nil {
				t.Error("a refusal caused by an outage must be reported as retryable, not swallowed")
			}
			if !tt.wantRetry && err != nil {
				t.Errorf("a verdict about the collector is final and must not be retried; got %v", err)
			}
		})
	}
}

// The counterpart: money collected by the complex's own seller account still confirms,
// so the check above rejects impostors rather than everything.
func TestApprovedPaymentIsConfirmedWhenTheComplexCollectedTheMoney(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	sellerID := "111111111"
	mpPayment.CollectorID = 111111111

	f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &sellerID, Name: "Vibe", Slug: "vibe"}
	f.bookings.booking = booking
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", LastName: "Diaz"}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	_ = f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123")

	if f.payments.inserted == nil {
		t.Fatal("a payment collected by the complex itself must be recorded")
	}
	if booking.Status != "confirmed" || booking.CollectionStatus != bookingstore.CollectionStatusDepositPaid {
		t.Errorf("the booking must be confirmed; got status=%q collection_status=%q", booking.Status, booking.CollectionStatus)
	}
	if len(f.notify.confirmations) != 1 {
		t.Errorf("the client must be told their booking is confirmed; got %d notifications", len(f.notify.confirmations))
	}
}

// The money-path defect this guard closes: a booking whose slot was given away
// while its payment was in flight must not be confirmed, and the client must get
// their money back rather than a court somebody else is already on.
//
// Both confirmation writes are exercised, because the webhook takes one or the
// other depending on whether the checkout already left a payment row behind,
// and both refundable refusals are exercised against each of them.
//
// H-23 is why the second refusal is here. A public booking that goes unpaid
// past its expiry is cancelled outright by the next InsertSafe that wants its
// hours, and that path deliberately leaves the MercadoPago preference alive —
// so the payment can still arrive, against a booking that no longer exists.
// The guard answers that with ErrBookingCancelled rather than
// ErrSlotUnavailable, and for a while only the second one reached this refund:
// the first fell through to the retryable branch, so the event was requeued
// forever against a write that could never succeed and the client's money
// never moved. Whatever the guard is spelled, the client gets it back.
func TestApprovedPaymentIsRefusedAndRefundedWhenTheSlotWasTaken(t *testing.T) {
	guards := []struct {
		name string
		err  error
	}{
		{name: "the slot was taken by somebody else", err: bookingstore.ErrSlotUnavailable},
		{name: "the booking was cancelled when its payment expired", err: bookingstore.ErrBookingCancelled},
	}

	tests := []struct {
		name    string
		prepare func(f *fixture, booking *bookingstore.Booking)
		// wantInsert says whether a payment row has to be created. The checkout
		// case must not create one: the row already exists, and inserting a
		// second orphans the first — which still carries the preference id.
		wantInsert bool
		// refundedPayment is the row the claim has to name.
		refundedPayment func(f *fixture) uuid.UUID
	}{
		{
			name: "an existing checkout payment",
			prepare: func(f *fixture, booking *bookingstore.Booking) {
				f.payments.byBooking = &paymentstore.Payment{
					ID: uuid.New(), BookingID: booking.ID, Amount: booking.DepositAmount, Status: "pending",
				}
			},
			wantInsert:      false,
			refundedPayment: func(f *fixture) uuid.UUID { return f.payments.byBooking.ID },
		},
		{
			name:            "no payment recorded yet",
			prepare:         func(*fixture, *bookingstore.Booking) {},
			wantInsert:      true,
			refundedPayment: func(f *fixture) uuid.UUID { return f.payments.inserted.ID },
		},
	}

	for _, guard := range guards {
		for _, tt := range tests {
			t.Run(guard.name+", "+tt.name, func(t *testing.T) {
				runRefundedRefusal(t, guard.err, tt.prepare, tt.wantInsert, tt.refundedPayment)
			})
		}
	}
}

func runRefundedRefusal(
	t *testing.T,
	guardErr error,
	prepare func(f *fixture, booking *bookingstore.Booking),
	wantInsert bool,
	refundedPayment func(f *fixture) uuid.UUID,
) {
	t.Helper()

	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	mpPayment.CollectorID = 111111111

	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.complexes.complex.Name, f.complexes.complex.Slug = "Vibe", "vibe"
	f.bookings.booking = booking
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", LastName: "Diaz"}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}
	f.payments.slotErr = guardErr
	prepare(f, booking)

	err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123")

	// A slot lost to somebody else is a decision, not an outage: retrying the
	// event cannot change who owns the court, and it would spend the budget
	// and end in a false alert.
	if err != nil {
		t.Errorf("losing the slot is a decision and must not be reported as retryable; got %v", err)
	}
	if booking.Status != "cancelled" {
		t.Errorf("a booking that lost its slot must be cancelled, not confirmed; got %q", booking.Status)
	}
	if len(f.notify.confirmations) != 0 {
		t.Error("a booking that was never confirmed must not be announced as confirmed")
	}
	if wantInsert && f.payments.inserted == nil {
		t.Fatal("the captured payment must be recorded before it can be refunded")
	}
	if !wantInsert && f.payments.inserted != nil {
		t.Fatal("a second payment row must not be inserted when the checkout already left one")
	}
	if want := refundedPayment(f); len(f.payments.claimed) != 1 || f.payments.claimed[0] != want {
		t.Fatalf("the refund must be claimed against the recorded payment %s; got %v", want, f.payments.claimed)
	}
	if len(f.provider.refunds) != 1 {
		t.Fatalf("the money must go back to the client; got %v", f.provider.refunds)
	}
	if len(f.payments.recordedSuccess) != 1 {
		t.Errorf("the issued refund must be recorded; got %d records", len(f.payments.recordedSuccess))
	}
}

// The other side of the error contract: a confirmation that failed because the
// database was unavailable is retryable, and must not be mistaken for a slot
// somebody else owns — that would cancel a booking and refund a client over a
// blip.
func TestAConfirmationThatFailsOnTheDatabaseIsRetriedRatherThanRefunded(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	sellerID := "111111111"
	mpPayment.CollectorID = 111111111

	f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &sellerID}
	f.bookings.booking = booking
	f.payments.byBooking = &paymentstore.Payment{ID: uuid.New(), BookingID: booking.ID, Amount: booking.DepositAmount}
	f.payments.confirmErr = errDatabase

	err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123")

	if err == nil {
		t.Error("a confirmation lost to the database must be reported as retryable")
	}
	if len(f.provider.refunds) != 0 {
		t.Errorf("no money may move because a write failed; got %v", f.provider.refunds)
	}
	if booking.Status == "cancelled" {
		t.Error("a transient failure must not cancel the booking")
	}
}

// Recording is the step that turns a refund which already left the account into a
// refund this system knows about. If it fails, the client must not be told their
// money is back — and, unlike before, the attempt claimed up front is still queued,
// so the retry job finishes the job instead of the money vanishing silently.
func TestAnAutoRefundThatCannotBeRecordedIsNotAnnouncedToTheClient(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	f.payments.byBooking = payment
	f.complexes.complex = linkedComplex(complexID, "")
	f.complexes.complex.Name = "Vibe"
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}
	f.payments.successErr = errRecord

	f.service.AutoRefundIfPaid(t.Context(), booking)

	if len(f.provider.refunds) != 1 {
		t.Fatalf("the provider refund must still have been attempted; got %v", f.provider.refunds)
	}
	if len(f.payments.recordedSuccess) != 0 {
		t.Error("a failed record must not be counted as a successful one")
	}
	if len(f.notify.refunds) != 0 {
		t.Error("the client must not be told the refund is done while the database never recorded it")
	}
	// The claim is the durable trace: it committed before the provider was called,
	// so the money that moved is still tracked by a queued attempt.
	if len(f.payments.claimed) != 1 {
		t.Error("the refund must have been claimed before the provider was called")
	}
}

// Money arriving for a cancelled booking is owed straight back, and the attempt to
// return it has to be durable before it is made.
//
// This replaces a test that asserted the opposite — that the refund is issued and
// queued even when the payment record could not be written. That behaviour cannot
// work against a real database: failed_refunds.payment_id is a foreign key onto
// payments, so with the payment insert failed the queue insert names a nil payment
// and violates the constraint. The stub had no foreign keys, so the test was green
// over a path that always failed in production, and the money moved with nothing
// recording it. Under claim → call → record the provider is simply not called.
func TestAnAutoRefundIsNotIssuedWhenThePaymentCannotBeRecorded(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, _ := paidBooking(complexID)
	booking.Status = "cancelled"
	booking.DepositAmount = 150_000
	sellerID := "111111111"
	f.complexes.complex = &complexstore.Complex{ID: complexID, MPUserID: &sellerID}
	f.payments.insertErr = errDatabase

	_ = f.service.processApprovedPayment(t.Context(), booking, &mp.Payment{ID: 123, TransactionAmount: 2500, CollectorID: 111111111}, "mp-123")

	if len(f.provider.refunds) != 0 {
		t.Errorf("money must not move for a refund nothing durable recorded; got %v", f.provider.refunds)
	}
	if len(f.payments.claimed) != 0 {
		t.Error("there is no payment row to claim against when the insert failed")
	}
}

// The counterpart: when the payment record is written, the refund is claimed
// against it and only then sent to the provider.
func TestAnAutoRefundForACancelledBookingIsClaimedBeforeItIsIssued(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, _ := paidBooking(complexID)
	booking.Status = "cancelled"
	booking.DepositAmount = 150_000
	f.complexes.complex = linkedComplex(complexID, "111111111")

	_ = f.service.processApprovedPayment(t.Context(), booking, &mp.Payment{ID: 123, TransactionAmount: 2500, CollectorID: 111111111}, "mp-123")

	if f.payments.inserted == nil {
		t.Fatal("the payment that arrived must be recorded before it is refunded")
	}
	if len(f.payments.claimed) != 1 || f.payments.claimed[0] != f.payments.inserted.ID {
		t.Fatalf("the refund must be claimed against the recorded payment; got %v", f.payments.claimed)
	}
	if len(f.provider.refunds) != 1 {
		t.Fatalf("the refund must then be issued; got %v", f.provider.refunds)
	}
	if len(f.payments.recordedSuccess) != 1 {
		t.Errorf("the issued refund must be recorded; got %d records", len(f.payments.recordedSuccess))
	}
}

// TestACancelledBookingIsRefundedEvenWhenItsDepositMoved is the explicit form of
// a guard that until now existed only by accident.
//
// The amount-equality check deliberately does not run on the already-cancelled
// branch: recordPaymentOwedARefund builds that payment from MercadoPago's own
// TransactionAmount, because a deposit that changed between checkout and
// cancellation would fail the recomputed comparison and refuse a refund the
// client is owed.
//
// TestApprovedPaymentForACancelledBookingIsRefunded happens to cover the same
// mutation, but only because paidBooking leaves DepositAmount at zero while the
// payment carries 1500 pesos — a mismatch nobody wrote on purpose. Anyone
// tidying that fixture to look realistic would delete the coverage without
// touching a test name. This one states the case in its setup, so the guard
// survives the fixture.
func TestACancelledBookingIsRefundedEvenWhenItsDepositMoved(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, _ := paidBooking(complexID)
	booking.Status = "cancelled"
	// The venue re-priced the court after the client paid. Checkout captured
	// 1500 pesos; the booking now expects a different deposit entirely.
	booking.DepositAmount = 900_00
	f.complexes.complex = linkedComplex(complexID, "111111111")

	_ = f.service.processApprovedPayment(t.Context(), booking,
		&mp.Payment{ID: 123, TransactionAmount: 1500, CollectorID: 111111111}, "mp-123")

	if len(f.provider.refunds) == 0 {
		t.Error("a cancelled booking whose deposit changed since checkout must still be " +
			"refunded: the amount check must not reach this branch")
	}
}

// The online confirmation is the one the owner does get an email about, and
// the one whose client actually paid a deposit. Both facts used to be missing:
// the confirmation carried no money at all, and a complex with no coordinates
// lost the WhatsApp channel outright because its map button had nothing to
// bind to.
func TestTheOnlineConfirmationSaysWhatWasPaidAndWhatIsOwed(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	sellerID := "111111111"
	mpPayment.CollectorID = 111111111

	complex := linkedComplex(complexID, sellerID)
	complex.Name = "Vibe Palermo"
	complex.Slug = "vibe-palermo"
	complex.Address = "Av. Santa Fe 1200"
	complex.City = "Buenos Aires"
	complex.CancellationHours = 24
	f.complexes.complex = complex
	f.bookings.booking = booking
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", Phone: "+5491155551234"}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Cancha 1"}

	if err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
		t.Fatalf("processApprovedPayment = %v", err)
	}
	if len(f.notify.confirmations) != 1 {
		t.Fatalf("the client must be told their booking is confirmed; got %d", len(f.notify.confirmations))
	}
	sent := f.notify.confirmations[0]

	// This is a sale the owner did not enter, so the owner's email goes out.
	if sent.Source != notifications.SourceOnlineCheckout {
		t.Errorf("the online route must identify itself; got source %q", sent.Source)
	}
	if sent.DepositAmount != "$1.500" {
		t.Errorf("the confirmation must say what was paid as a deposit; got %q", sent.DepositAmount)
	}
	if sent.BalanceAmount != "$3.500" {
		t.Errorf("the confirmation must say what is owed at the venue; got %q", sent.BalanceAmount)
	}
	if !strings.Contains(sent.CancellationLine, "24 horas") {
		t.Errorf("the confirmation must quote the complex's own window; got %q", sent.CancellationLine)
	}
	if sent.MapsQuery == "" {
		t.Error("a complex with only a written address must still get a map button")
	}
	if sent.Address != "Av. Santa Fe 1200, Buenos Aires" {
		t.Errorf("the confirmation email must say where the venue is; got %q", sent.Address)
	}
	if sent.MapsURL == "" {
		t.Error("the confirmation email must be able to open a map")
	}
}
