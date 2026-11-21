package payments

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/events/notice"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// registrationEntity is the audit entity type a registration's money events are
// keyed on, as a booking's are keyed on "booking".
const registrationEntity = "open_play_registration"

// RegistrationLedger is the ledger side of open-play event sign-ups: the
// payments of a registration, and settling an approved payment against it.
type RegistrationLedger interface {
	ListByRegistrationID(ctx context.Context, registrationID uuid.UUID) ([]*paymentstore.Payment, error)
	ConfirmRegistrationPayment(ctx context.Context, p *paymentstore.Payment, registrationID uuid.UUID) error
}

// RegistrationReader is the events side this module reads: the registration a
// payment names and the event it is for.
type RegistrationReader interface {
	GetRegistration(ctx context.Context, id uuid.UUID) (*eventstore.Registration, error)
	GetEvent(ctx context.Context, id uuid.UUID) (*eventstore.Event, error)
}

// RegistrationNotifier tells a person what happened to their spot and their
// money: their payment confirmed it, or a refund that was still on its way when
// they were told finished.
type RegistrationNotifier interface {
	EventRegistered(notifications.EventRegistration)
	EventCancelled(notifications.EventCancellation)
	// OwnerEventRegistered tells the owner a person paid their spot.
	OwnerEventRegistered(notifications.OwnerEventNotice)
	// EventRefunded tells a person part of their payment came back while their
	// spot stays.
	EventRefunded(notifications.EventRefund)
}

// registrationIDOf reads the open-play registration a MercadoPago payment was
// made for, from the metadata sign-up put on the preference. A booking's
// payment carries none.
func registrationIDOf(p *mp.Payment) (uuid.UUID, bool) {
	raw, ok := p.Metadata["registration_id"].(string)
	if !ok || raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// registrationPaymentsEnabled reports whether this service was wired for event
// sign-ups. A payment for one that arrives at a service that was not is
// recorded in the log and dropped: there is nothing it can do with it.
func (s *Service) registrationPaymentsEnabled() bool {
	return s.registrationLedger != nil && s.registrations != nil
}

// processRegistrationPayment handles a MercadoPago payment made for an event
// registration, and reports whether the event behind it should be tried again.
// Like processApprovedPayment, every decision about the payment itself returns
// nil; only what a retry can fix is an error.
func (s *Service) processRegistrationPayment(ctx context.Context, regID uuid.UUID, mpPayment *mp.Payment, mpPaymentID string) error {
	if !s.registrationPaymentsEnabled() {
		s.logger.Error("mp webhook: payment names an event registration but sign-ups are not wired",
			"mp_payment_id", mpPaymentID, "registration_id", regID)
		return nil
	}
	reg, err := s.registrations.GetRegistration(ctx, regID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			s.logger.Error("mp webhook: payment names a registration that does not exist",
				"registration_id", regID, "mp_payment_id", mpPaymentID)
			return nil
		}
		return fmt.Errorf("fetch registration %s: %w", regID, err)
	}

	switch mpPayment.Status {
	case "approved":
		return s.approveRegistrationPayment(ctx, reg, mpPayment, mpPaymentID)
	case "refunded", "charged_back":
		// A refund on a payment we never recorded: nothing of ours moved.
		s.logger.Error("mp webhook: refund or chargeback on an unrecorded registration payment — review required",
			"mp_payment_id", mpPaymentID, "registration_id", regID, "status", mpPayment.Status)
		sentry.CaptureMessage(fmt.Sprintf("REGISTRATION PAYMENT %s on unrecorded payment: mp_payment_id=%s registration_id=%s", mpPayment.Status, mpPaymentID, regID))
	default:
		// A rejected card is one failed attempt: the person can pay the same
		// preference with another until the hold runs out. Pending payments
		// resolve with another webhook.
		s.logger.Info("mp webhook: registration payment not approved",
			"status", mpPayment.Status, "status_detail", mpPayment.StatusDetail,
			"mp_payment_id", mpPaymentID, "registration_id", regID)
	}
	return nil
}

// approveRegistrationPayment confirms the registration an approved payment is
// for, or sends the money back when it cannot: the collector is checked first,
// then the amount, then the spot.
func (s *Service) approveRegistrationPayment(ctx context.Context, reg *eventstore.Registration, mpPayment *mp.Payment, mpPaymentID string) error {
	matches, err := s.collectorMatches(ctx, reg.ComplexID, "registration_id", reg.ID, mpPayment, mpPaymentID)
	if err != nil || !matches {
		return err
	}
	actual := int(math.Round(mpPayment.TransactionAmount * 100))
	if expected := reg.Price + reg.ServiceFee; actual != expected {
		s.logger.Error("mp webhook: FRAUD ALERT - registration amount mismatch",
			"mp_payment_id", mpPaymentID, "expected_centavos", expected, "actual_centavos", actual, "registration_id", reg.ID)
		sentry.CaptureMessage(fmt.Sprintf("FRAUD ALERT: amount mismatch mp_payment_id=%s registration_id=%s expected=%d actual=%d", mpPaymentID, reg.ID, expected, actual))
		return nil
	}

	checkout, err := s.registrationCheckoutRow(ctx, reg.ID)
	if err != nil {
		return err
	}
	payment := checkout
	if payment == nil {
		payment = &paymentstore.Payment{
			RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: reg.Price, ServiceFee: reg.ServiceFee, Method: "mercadopago",
		}
	}
	payment.Status = "deposit_paid"
	payment.MPPaymentID = &mpPaymentID
	payment.StatusDetail = statusDetailPtr(mpPayment.StatusDetail)

	switch err := s.registrationLedger.ConfirmRegistrationPayment(ctx, payment, reg.ID); {
	case err == nil:
		s.logger.Info("mp webhook: registration confirmed", "registration_id", reg.ID, "mp_payment_id", mpPaymentID)
		s.recordEntity(reg.ComplexID, registrationEntity, reg.ID, "registration_paid",
			moneyEvent{Actor: actorProvider, PaymentID: &payment.ID, MPPaymentID: mpPaymentID, AmountCentavos: actual, Result: "confirmed"})
		s.realtime.PublishBookingChanged(reg.ComplexID)
		s.announceRegistration(ctx, reg.ID)
		return nil
	case errors.Is(err, paymentstore.ErrRegistrationAlreadyPaid):
		return s.refundExtraRegistrationPayment(ctx, reg, mpPayment, mpPaymentID, "already paid")
	case errors.Is(err, paymentstore.ErrRegistrationUnavailable):
		return s.refundRegistrationWhoseSpotIsGone(ctx, reg, mpPayment, mpPaymentID)
	default:
		return fmt.Errorf("confirm registration %s: %w", reg.ID, err)
	}
}

// registrationCheckoutRow is the row sign-up wrote when it opened the checkout,
// the one still waiting for its payment, or nil when there is none.
func (s *Service) registrationCheckoutRow(ctx context.Context, regID uuid.UUID) (*paymentstore.Payment, error) {
	rows, err := s.registrationLedger.ListByRegistrationID(ctx, regID)
	if err != nil {
		return nil, fmt.Errorf("look up the payments of registration %s: %w", regID, err)
	}
	for _, p := range rows {
		if p.MPPaymentID == nil && p.Status == "unpaid" {
			return p, nil
		}
	}
	return nil, nil
}

// announceRegistration tells the person their spot is confirmed. It never
// fails the confirmation: the money is recorded, and a notice that cannot be
// built is logged.
func (s *Service) announceRegistration(ctx context.Context, regID uuid.UUID) {
	if s.registrationNotify == nil {
		return
	}
	reg, err := s.registrations.GetRegistration(ctx, regID)
	if err != nil {
		s.logger.Error("mp webhook: cannot announce a confirmed registration", "error", err, "registration_id", regID)
		return
	}
	ev, err := s.registrations.GetEvent(ctx, reg.EventID)
	if err != nil {
		s.logger.Error("mp webhook: cannot announce a confirmed registration, event unreadable", "error", err, "registration_id", regID)
		return
	}
	complex, err := s.complexes.GetByID(ctx, reg.ComplexID)
	if err != nil {
		s.logger.Error("mp webhook: cannot announce a confirmed registration, complex unreadable", "error", err, "registration_id", regID)
		return
	}
	s.registrationNotify.EventRegistered(notice.Registered(reg, ev, complex, s.cfg.PublicURL))
	// The owner hears of every paid spot, as of every paid booking: a sign-up is
	// always the person's own, never the owner's click (S9).
	s.registrationNotify.OwnerEventRegistered(notice.Owner(reg, ev, complex))
}

// refundRegistrationWhoseSpotIsGone sends back a payment that arrived for a
// registration that cannot be confirmed any more: it was cancelled, its event
// was cancelled, or its hold ran out and the spot went to someone else. A
// decision rather than a failure, so it is reported to Sentry and the event is
// not retried.
func (s *Service) refundRegistrationWhoseSpotIsGone(ctx context.Context, reg *eventstore.Registration, mpPayment *mp.Payment, mpPaymentID string) error {
	s.logger.Error("mp webhook: the spot was gone while the payment was in flight, refusing to confirm and refunding",
		"mp_payment_id", mpPaymentID, "registration_id", reg.ID)
	sentry.CaptureMessage(fmt.Sprintf("REGISTRATION SPOT GONE (auto-refunding): mp_payment_id=%s registration_id=%s", mpPaymentID, reg.ID))
	outcome, err := s.claimAndRefundRegistrationPayment(ctx, reg, mpPayment, mpPaymentID)
	if err == nil && outcome.Result == paymentstore.RefundIssued {
		// The same notice the retry job sends when it finishes such a refund,
		// keyed on the registration and the amount, so one refund is one email.
		s.notifyRegistrationRefund(ctx, reg.ID, outcome.AmountCentavos)
	}
	return err
}

// refundExtraRegistrationPayment sends back a second payment on a registration
// that is already confirmed.
func (s *Service) refundExtraRegistrationPayment(ctx context.Context, reg *eventstore.Registration, mpPayment *mp.Payment, mpPaymentID, why string) error {
	s.logger.Error("mp webhook: duplicate registration payment, refunding it",
		"mp_payment_id", mpPaymentID, "registration_id", reg.ID, "why", why)
	outcome, err := s.claimAndRefundRegistrationPayment(ctx, reg, mpPayment, mpPaymentID)
	if err == nil && outcome.Result == paymentstore.RefundIssued {
		// The spot is kept: the person is told this payment came back. The row
		// is new, so what it refunded is both this move and its running total.
		s.notifyRegistrationPanelRefund(ctx, reg.ID, outcome.AmountCentavos, outcome.AmountCentavos)
	}
	return err
}

// claimAndRefundRegistrationPayment records the payment a refund is about to be
// claimed against, and refunds it. The row is always new, and insert and claim
// are one transaction, so a claim that fails leaves nothing that reads as
// already processed and MercadoPago's retry starts over. The checkout row
// sign-up wrote is left unpaid, like the checkout of any hold nobody paid:
// settling it first and claiming after was two commits, and a crash between
// them left a collected row the retry skipped as handled (S17).
//
// If recording fails the provider is never called: money must not move on
// behalf of a refund nothing durable knows about. The error is returned so the
// event is retried. The outcome says whether the money already moved, which is
// when the caller may tell the person; a queued refund is told by the retry job.
func (s *Service) claimAndRefundRegistrationPayment(ctx context.Context, reg *eventstore.Registration, mpPayment *mp.Payment, mpPaymentID string) (paymentstore.RefundOutcome, error) {
	paid := int(math.Round(mpPayment.TransactionAmount * 100))
	fee := min(reg.ServiceFee, paid)
	claim, err := s.refunds.InsertAndClaimRefund(ctx, &paymentstore.Payment{
		RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: paid - fee, ServiceFee: fee,
		Method: "mercadopago", Status: "deposit_paid", MPPaymentID: &mpPaymentID, StatusDetail: statusDetailPtr(mpPayment.StatusDetail),
	})
	switch {
	case err == nil:
	case errors.Is(err, paymentstore.ErrAlreadyRefunded), errors.Is(err, paymentstore.ErrRefundInFlight):
		return paymentstore.RefundOutcome{Result: paymentstore.RefundNone}, nil
	default:
		sentry.CaptureMessage(fmt.Sprintf("AUTO-REFUND NOT CLAIMED: mp_payment_id=%s registration_id=%s error=%v", mpPaymentID, reg.ID, err))
		return paymentstore.RefundOutcome{}, fmt.Errorf("claim the refund for registration %s: %w", reg.ID, err)
	}
	return s.settleRegistrationClaim(ctx, claim), nil
}

// settleRegistrationClaim issues the refund a committed claim reserved and
// records how it ended. From a committed claim on, the refund queue owns the
// money, so every outcome is recorded or queued, never returned.
func (s *Service) settleRegistrationClaim(ctx context.Context, claim *paymentstore.RefundClaim) paymentstore.RefundOutcome {
	settled, outcome := s.issueRefund(ctx, *claim)
	if outcome != nil {
		s.recordClaim(*claim, "refund", claimEvent(*claim, outcome.AmountCentavos, outcome.Result, outcome.Reason))
		return *outcome
	}
	total, err := s.refunds.RecordRefundSuccess(ctx, settled, 0)
	if err != nil {
		// See AutoRefundIfPaid: the attempt stays queued and the retry job replays
		// the call, which MercadoPago deduplicates on its idempotency key.
		s.logger.Error("registration refund issued but not recorded, left queued for retry",
			"error", err, "mp_payment_id", claim.MPPaymentID, "attempt_id", claim.AttemptID)
		sentry.CaptureMessage(fmt.Sprintf("REGISTRATION REFUND RECORD FAILED (money refunded, queued for retry): mp_payment_id=%s attempt_id=%s", claim.MPPaymentID, claim.AttemptID))
		queued := paymentstore.RefundOutcome{
			Result: paymentstore.RefundQueued, AmountCentavos: settled.RefundCentavos,
			Reason: "the refund was issued but could not be recorded, and stays queued",
		}
		s.recordClaim(*claim, "refund", claimEvent(*claim, queued.AmountCentavos, queued.Result, queued.Reason))
		return queued
	}
	s.recordClaim(*claim, "refund", claimEvent(*claim, settled.RefundCentavos, paymentstore.RefundIssued, ""))
	s.reportShortfall("registration refund", "refund", claim.ComplexID, claim.BookingID, *claim, settled)
	return paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: total}
}

// RefundRegistration sends back every MercadoPago payment a registration holds,
// through the same claim, provider call and record lifecycle a booking's refund
// uses, and reports what became of the money. Exported for events: the owner
// cancelling an event refunds every paid participant, and a participant
// cancelling in time refunds themselves.
//
// It never notifies the person on success: the caller tells them in the one
// cancellation notice, quoting this outcome. A refund still queued is notified
// by the retry job when it finishes (notifyRegistrationRefund).
func (s *Service) RefundRegistration(ctx context.Context, regID uuid.UUID) paymentstore.RefundOutcome {
	if !s.registrationPaymentsEnabled() {
		return paymentstore.RefundOutcome{Result: paymentstore.RefundNone, Reason: "sign-up payments are not wired"}
	}
	payments, err := s.registrationLedger.ListByRegistrationID(ctx, regID)
	if err != nil {
		s.logger.Error("registration refund: cannot read the ledger", "error", err, "registration_id", regID)
		sentry.CaptureMessage(fmt.Sprintf("REGISTRATION REFUND NOT ATTEMPTED: registration_id=%s — its payments could not be read", regID))
		return paymentstore.RefundOutcome{Result: paymentstore.RefundManual, LedgerUnread: true, Reason: "the payments of the registration could not be read"}
	}
	var results []paymentstore.RefundOutcome
	for _, p := range payments {
		if !refundableRegistrationPayment(p) {
			continue
		}
		results = append(results, s.refundRegistrationPayment(ctx, p))
	}
	if len(results) == 0 {
		return paymentstore.RefundOutcome{Result: paymentstore.RefundNone, Reason: "the registration holds no money to return"}
	}
	return combineRefundOutcomes(results)
}

// refundableRegistrationPayment reports whether a payment row holds MercadoPago
// money that has been collected and not fully returned.
func refundableRegistrationPayment(p *paymentstore.Payment) bool {
	if p.Method != "mercadopago" || p.MPPaymentID == nil {
		return false
	}
	switch p.Status {
	case "deposit_paid", "fully_paid", "partial_refund", "refund_pending":
		return true
	}
	return false
}

// refundRegistrationPayment claims, issues and records the refund of one
// payment row.
func (s *Service) refundRegistrationPayment(ctx context.Context, p *paymentstore.Payment) paymentstore.RefundOutcome {
	owed := p.Amount + p.ServiceFee - p.RefundAmount
	claim, err := s.refunds.ClaimRefund(ctx, p.ID)
	switch {
	case err == nil:
	case errors.Is(err, paymentstore.ErrAlreadyRefunded):
		return paymentstore.RefundOutcome{Result: paymentstore.RefundAlreadyIssued, AmountCentavos: p.RefundAmount, Reason: "the payment was already refunded"}
	case errors.Is(err, paymentstore.ErrRefundInFlight):
		return paymentstore.RefundOutcome{Result: paymentstore.RefundQueued, AmountCentavos: owed, Reason: "another claim already holds this refund"}
	default:
		// Nothing was reserved and nothing was queued, so no retry job will ever
		// come back to this: it is owed by hand.
		s.logger.Error("registration refund: failed to claim", "error", err, "payment_id", p.ID)
		sentry.CaptureMessage(fmt.Sprintf("MANUAL REFUND OWED: registration payment_id=%s amount=%d — the refund could not be claimed, so nothing was queued", p.ID, owed))
		return paymentstore.RefundOutcome{Result: paymentstore.RefundManual, AmountCentavos: owed, Reason: "the refund could not be claimed, so nothing was queued"}
	}
	return s.settleRegistrationClaim(ctx, claim)
}

// notifyRegistrationRefund tells a person that a refund the retry job finished
// is back, the second half of the cancellation notice that said it was on its
// way.
func (s *Service) notifyRegistrationRefund(ctx context.Context, regID uuid.UUID, refundTotal int) {
	if s.registrationNotify == nil || !s.registrationPaymentsEnabled() {
		return
	}
	reg, err := s.registrations.GetRegistration(ctx, regID)
	if err != nil {
		s.logger.Error("refund-retry: failed to fetch the registration", "error", err, "registration_id", regID)
		return
	}
	ev, err := s.registrations.GetEvent(ctx, reg.EventID)
	if err != nil {
		s.logger.Error("refund-retry: failed to fetch the event", "error", err, "registration_id", regID)
		return
	}
	complex, err := s.complexes.GetByID(ctx, reg.ComplexID)
	if err != nil {
		s.logger.Error("refund-retry: failed to fetch the complex", "error", err, "registration_id", regID)
		return
	}
	byOwner := reg.CancelledBy != nil && *reg.CancelledBy == eventstore.CancelledByOwner
	s.registrationNotify.EventCancelled(notice.Cancelled(reg, ev, complex, s.cfg.PublicURL, byOwner, notice.LineRefunded, refundTotal))
}

// processRecordedRegistrationPayment is a webhook for a registration payment we
// already recorded. A redelivery of the approval is the no-op. A full refund or
// chargeback the provider reports is either our own refund coming back to us
// (already recorded by the refund lifecycle) or money moved from outside, which
// nothing here may guess at: it is alerted for a person to review. A partial
// refund made from MercadoPago's own panel is recorded.
func (s *Service) processRecordedRegistrationPayment(ctx context.Context, payment *paymentstore.Payment, mpPayment *mp.Payment, mpPaymentID string) error {
	if mpPayment.Status == "refunded" || mpPayment.Status == "charged_back" {
		if payment.Status != "refunded" {
			s.logger.Error("mp webhook: registration payment refunded or charged back at the provider — review required",
				"mp_payment_id", mpPaymentID, "payment_id", payment.ID, "status", mpPayment.Status, "recorded_status", payment.Status)
			sentry.CaptureMessage(fmt.Sprintf("REGISTRATION PAYMENT %s at the provider: mp_payment_id=%s payment_id=%s recorded=%s",
				mpPayment.Status, mpPaymentID, payment.ID, payment.Status))
		}
		return nil
	}
	if refundedBeyondRecorded(payment, mpPayment) {
		return s.recordRegistrationPanelRefund(ctx, payment, mpPayment, mpPaymentID)
	}
	if payment.Status == "deposit_paid" || payment.Status == "fully_paid" {
		owed, err := s.collectedForASpotNeverGiven(ctx, payment)
		if err != nil {
			return err
		}
		if owed {
			return s.finishOrphanedRegistrationRefund(ctx, payment, mpPaymentID)
		}
	}
	s.logger.Info("mp webhook: registration payment already processed, skipping",
		"mp_payment_id", mpPaymentID, "mp_status", mpPayment.Status)
	return nil
}

// recordRegistrationPanelRefund records a partial refund made from MercadoPago's
// own panel on a registration payment MercadoPago still calls 'approved'. The row
// takes the running total, the trail takes what moved this time, and the
// registration stays as it is: money is still held for it, and whether the spot
// goes with the refund is the owner's call, not the webhook's. Nothing is refunded
// from here.
//
// The participant is told what came back once the refund is recorded, and that
// the spot is still theirs (S11).
func (s *Service) recordRegistrationPanelRefund(ctx context.Context, payment *paymentstore.Payment, mpPayment *mp.Payment, mpPaymentID string) error {
	// A claim of ours holds this refund: the refund lifecycle records it, as on
	// the booking path. One refund, one owner.
	if payment.Status == "refund_pending" {
		s.logger.Info("mp webhook: a claim of ours already holds this registration refund, leaving the record to it",
			"mp_payment_id", mpPaymentID, "payment_id", payment.ID)
		return nil
	}

	movedNow := recordRefundedTotal(payment, refundedTotalOf(mpPayment))
	payment.StatusDetail = statusDetailPtr(mpPayment.StatusDetail)
	written, err := s.ledger.UpdateUnlessRefundPending(ctx, payment)
	if err != nil {
		return fmt.Errorf("record the partial refund of registration payment %s: %w", mpPaymentID, err)
	}
	// A claim that committed after the row was read owns the refund now, as in the
	// guard above: the write was refused, and nothing is recorded or published.
	if !written {
		s.logger.Info("mp webhook: a claim of ours took this registration refund while the webhook was reading it, leaving the record to it",
			"mp_payment_id", mpPaymentID, "payment_id", payment.ID)
		return nil
	}
	s.realtime.PublishBookingChanged(payment.ComplexID)

	s.logger.Info("mp webhook: partial refund of a registration payment recorded",
		"mp_payment_id", mpPaymentID,
		"payment_id", payment.ID,
		"registration_id", *payment.RegistrationID,
		"refund_centavos", movedNow,
		"refunded_total_centavos", payment.RefundAmount,
	)
	s.recordEntity(payment.ComplexID, registrationEntity, *payment.RegistrationID, providerRefundAction(mpPayment.Status), moneyEvent{
		Actor:          actorProvider,
		PaymentID:      &payment.ID,
		MPPaymentID:    mpPaymentID,
		AmountCentavos: movedNow,
		Result:         string(paymentstore.RefundIssued),
		Reason:         mpPayment.StatusDetail,
	})
	s.notifyRegistrationPanelRefund(ctx, *payment.RegistrationID, movedNow, payment.RefundAmount)
	return nil
}

// notifyRegistrationPanelRefund tells a person part of their payment came back
// from the owner's MercadoPago panel. It runs after the refund is recorded and
// never fails it: a notice that cannot be built is logged.
func (s *Service) notifyRegistrationPanelRefund(ctx context.Context, regID uuid.UUID, movedCentavos, refundedTotal int) {
	if s.registrationNotify == nil || movedCentavos <= 0 {
		return
	}
	reg, err := s.registrations.GetRegistration(ctx, regID)
	if err != nil {
		s.logger.Error("mp webhook: cannot tell a participant of their panel refund", "error", err, "registration_id", regID)
		return
	}
	ev, err := s.registrations.GetEvent(ctx, reg.EventID)
	if err != nil {
		s.logger.Error("mp webhook: cannot tell a participant of their panel refund, event unreadable", "error", err, "registration_id", regID)
		return
	}
	complex, err := s.complexes.GetByID(ctx, reg.ComplexID)
	if err != nil {
		s.logger.Error("mp webhook: cannot tell a participant of their panel refund, complex unreadable", "error", err, "registration_id", regID)
		return
	}
	s.registrationNotify.EventRefunded(notice.Refunded(reg, ev, complex, s.cfg.PublicURL, movedCentavos, refundedTotal))
}

// collectedForASpotNeverGiven reports whether a collected payment row belongs to
// a registration that was never confirmed. Confirming writes the payment and the
// registration's confirmation in one transaction, so such a row can only be a
// refund that was recorded and then never claimed: the spot-gone path settled
// the checkout row and claimed its refund in two commits until S17, and a crash
// between them left this. A registration confirmed once (cancelled since or not)
// keeps its payment as its cancellation decided.
func (s *Service) collectedForASpotNeverGiven(ctx context.Context, payment *paymentstore.Payment) (bool, error) {
	if !s.registrationPaymentsEnabled() || payment.RegistrationID == nil {
		return false, nil
	}
	reg, err := s.registrations.GetRegistration(ctx, *payment.RegistrationID)
	if err != nil {
		return false, fmt.Errorf("read the registration of payment %s: %w", payment.ID, err)
	}
	return reg.ConfirmedAt == nil && reg.Status != eventstore.RegistrationConfirmed, nil
}

// finishOrphanedRegistrationRefund claims and issues the refund of a collected
// payment whose spot was never given (collectedForASpotNeverGiven).
func (s *Service) finishOrphanedRegistrationRefund(ctx context.Context, payment *paymentstore.Payment, mpPaymentID string) error {
	s.logger.Error("mp webhook: a collected registration payment was never confirmed nor refunded, refunding it now",
		"mp_payment_id", mpPaymentID, "payment_id", payment.ID, "registration_id", *payment.RegistrationID)
	sentry.CaptureMessage(fmt.Sprintf("REGISTRATION REFUND RECOVERED (auto-refunding): mp_payment_id=%s payment_id=%s", mpPaymentID, payment.ID))
	claim, err := s.refunds.ClaimRefund(ctx, payment.ID)
	switch {
	case err == nil:
	case errors.Is(err, paymentstore.ErrAlreadyRefunded), errors.Is(err, paymentstore.ErrRefundInFlight):
		return nil
	default:
		return fmt.Errorf("claim the refund of registration payment %s: %w", payment.ID, err)
	}
	s.settleRegistrationClaim(ctx, claim)
	return nil
}
