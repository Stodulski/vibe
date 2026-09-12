package payments

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/booklink"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/mpcred"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// processRefundedPayment handles a refund/chargeback on an already-recorded payment.
// Triggered when MP sends a webhook for a payment we already processed (e.g. dispute, chargeback,
// or refund initiated directly from MercadoPago by the buyer).
//
// It reports whether the webhook event behind it should be tried again: a write
// the database refused is an error, and every decision about the refund itself is
// nil.
//
// It is one cohesive payment-domain flow — idempotency check, persist the
// refund state, cancel the booking, notify — and splitting it would relocate
// sequential steps into helpers without reducing complexity, at the cost of
// disturbing this domain's tested control flow.
//
//nolint:funlen // see the cohesion note above
func (s *Service) processRefundedPayment(ctx context.Context, payment *paymentstore.Payment, mpPayment *mp.Payment, mpPaymentID string) error {
	// Skip if our record is already refunded.
	if payment.Status == "refunded" {
		s.logger.Info("mp webhook: payment already marked as refunded, skipping",
			"mp_payment_id", mpPaymentID,
			"booking_id", payment.BookingID,
		)
		return nil
	}

	// Who owns this refund — the whole of it, the write as well as the message.
	//
	// 'refund_pending' means a claim of ours is in flight: AutoRefundIfPaid or the
	// retry job called MercadoPago, and MercadoPago is now telling us about the
	// refund we just asked for. ClaimRefund commits the status and a durable
	// failed_refunds attempt row in one transaction (internal/payments/store/refunds.go), so
	// that status is never on a row nothing is coming back to: whoever holds the
	// claim records it, and if that process died the retry job finishes the job.
	//
	// This used to suppress only the notification, and the write below ran anyway.
	// Two writers on one column with two different arithmetics: this path assigns
	// refund_amount outright, while RecordRefundSuccess adds the claim's figure to
	// whatever the row already holds. A full refund survived that only because
	// RecordRefundSuccess caps the sum at the amount paid. A partial one did not —
	// MercadoPago moves 1000 of 2500, this path writes refund_amount = 1000, the
	// recorder then writes 1000 + 1000, and the row says 2000 came back on a
	// payment that returned half of that. The client is then told 2000, and the
	// 1500 still genuinely owed reads as 500.
	//
	// So this path stands down entirely. One refund, one owner.
	if payment.Status == "refund_pending" {
		s.logger.Info("mp webhook: a claim of ours already holds this refund, leaving the record to it",
			"mp_payment_id", mpPaymentID,
			"booking_id", payment.BookingID,
			"mp_status", mpPayment.Status,
		)
		return nil
	}

	// Fetch the booking.
	booking, err := s.bookings.GetByID(ctx, payment.BookingID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			s.logger.Error("mp webhook: refund — the payment names a booking that does not exist",
				"booking_id", payment.BookingID, "mp_payment_id", mpPaymentID)
			return nil
		}
		return fmt.Errorf("fetch booking %s for a refunded payment: %w", payment.BookingID, err)
	}

	// What the client paid, and what came back, both as MercadoPago has them.
	//
	// recordedTotal is this codebase's own belief about the first of those, and
	// it used to be the only figure in this comparison. That is what made a
	// partial refund readable as a full one: a payment row that understates what
	// was charged — the deposit moved between checkout and the webhook, the fee
	// rule changed, the fallback insert split MercadoPago's figure with a stale
	// deposit — makes any refund at least that large look like the whole of it,
	// and the payment then reads 'refunded' with the balance silently kept.
	// MercadoPago is the party that took the money, so its transaction_amount is
	// what full-versus-partial is measured against.
	recordedTotal := payment.Amount + payment.ServiceFee
	totalPaid := recordedTotal
	if charged := int(math.Round(mpPayment.TransactionAmount * 100)); charged > 0 && charged != recordedTotal {
		s.logger.Error("mp webhook: the payment row disagrees with MercadoPago about what was charged",
			"mp_payment_id", mpPaymentID,
			"booking_id", payment.BookingID,
			"recorded_centavos", recordedTotal,
			"charged_centavos", charged,
		)
		sentry.CaptureMessage(fmt.Sprintf("PAYMENT AMOUNT MISMATCH (refund measured against MercadoPago's figure): mp_payment_id=%s booking_id=%s recorded=%d charged=%d",
			mpPaymentID, payment.BookingID, recordedTotal, charged))
		totalPaid = charged
	}

	refundCentavos := int(math.Round(mpPayment.TransactionAmountRefunded * 100))
	if refundCentavos <= 0 {
		// MercadoPago reports a refund with no amount on it — a chargeback, most
		// often, where the field is not populated at all. The money is gone either
		// way, so the whole of it is assumed rather than none of it.
		refundCentavos = totalPaid
	}
	isFullRefund := refundCentavos >= totalPaid

	// Add chargeback note to booking for auditing.
	if mpPayment.Status == "charged_back" {
		chargebackNote := "Contracargo de MercadoPago"
		if booking.Notes != nil {
			chargebackNote = *booking.Notes + " | " + chargebackNote
		}
		booking.Notes = &chargebackNote
		sentry.CaptureMessage(fmt.Sprintf("MercadoPago CHARGEBACK processed: booking_id=%s mp_payment_id=%s", booking.ID, mpPaymentID))
	}

	// Update payment status.
	//
	// refund_amount is capped at this row's own amount + service_fee, because
	// the payments_refund_within_amount_paid CHECK refuses anything
	// larger and the write would fail outright. The cap is why the recorded and
	// charged totals are kept apart above rather than collapsed: the comparison
	// wants MercadoPago's figure, and the column will only hold ours.
	if isFullRefund {
		payment.Status = "refunded"
	}
	payment.RefundAmount = min(refundCentavos, recordedTotal)
	payment.StatusDetail = statusDetailPtr(mpPayment.StatusDetail)

	if isFullRefund && booking.Status != "completed" && booking.Status != "no_show" {
		// Full refund: update payment + cancel booking atomically.
		// Do NOT cancel completed/no_show bookings — service was already rendered.
		booking.Status = "cancelled"
		booking.RefundStatus = bookingRefundStatusAfterRefund(s.manualOwedForBooking(ctx, booking.ID))
		if err := s.payments.ConfirmWebhookPayment(ctx, payment, booking); err != nil {
			return fmt.Errorf("record the refund of payment %s and cancel its booking: %w", mpPaymentID, err)
		}
	} else {
		// Partial refund, or full refund on completed/no_show: only update payment.
		if isFullRefund {
			booking.RefundStatus = bookingRefundStatusAfterRefund(s.manualOwedForBooking(ctx, booking.ID))
			if err := s.payments.ConfirmWebhookPayment(ctx, payment, booking); err != nil {
				return fmt.Errorf("record the refund of payment %s on a played booking: %w", mpPaymentID, err)
			}
		} else {
			if err := s.payments.Update(ctx, payment); err != nil {
				return fmt.Errorf("record the partial refund of payment %s: %w", mpPaymentID, err)
			}
		}
	}

	s.realtime.PublishBookingChanged(booking.ComplexID)

	s.logger.Info("mp webhook: refund/chargeback processed",
		"mp_payment_id", mpPaymentID,
		"mp_status", mpPayment.Status,
		"mp_status_detail", mpPayment.StatusDetail,
		"booking_id", booking.ID,
		"refund_centavos", refundCentavos,
		"full_refund", isFullRefund,
	)

	// A partial refund is money the client got back, and they were told nothing
	// about it: the notification used to be gated on isFullRefund, so somebody
	// refunded half their deposit and heard from nobody. The amount written to the
	// row is what is announced, which for a partial refund is MercadoPago's own
	// running total rather than this instalment.
	//
	// It is unconditional now because reaching this line already means no claim of
	// ours holds this refund — the guard at the top of the function returned on
	// that case rather than only muting the message.
	s.sendRefundNotification(ctx, booking, payment.RefundAmount)

	// Money left this venue's account and nobody here decided it. The amount
	// recorded is MercadoPago's own figure for what moved on this event, not
	// the running total written to the row, because the question this entry
	// answers is what happened — the row already says where it ended up.
	s.record(booking.ComplexID, booking.ID, providerRefundAction(mpPayment.Status), moneyEvent{
		Actor:          actorProvider,
		PaymentID:      &payment.ID,
		MPPaymentID:    mpPaymentID,
		AmountCentavos: refundCentavos,
		Result:         string(paymentstore.RefundIssued),
		Reason:         mpPayment.StatusDetail,
	})
	return nil
}

// processRefundedPaymentFromBooking handles a refund/chargeback when we don't have a payment record yet.
// This is an edge case (e.g. payment was refunded before our webhook processed the original approval).
// It reports whether the webhook event behind it should be tried again.
func (s *Service) processRefundedPaymentFromBooking(ctx context.Context, booking *bookingstore.Booking, mpPayment *mp.Payment, mpPaymentID string) error {
	if booking.Status == "cancelled" && booking.RefundStatus == bookingstore.RefundStatusFull {
		s.logger.Info("mp webhook: booking already cancelled+refunded, skipping",
			"mp_payment_id", mpPaymentID,
			"booking_id", booking.ID,
		)
		return nil
	}

	// Don't cancel completed/no_show bookings — service was already rendered.
	// Only update payment status.
	if booking.Status != "completed" && booking.Status != "no_show" {
		booking.Status = "cancelled"
	}
	booking.RefundStatus = bookingRefundStatusAfterRefund(s.manualOwedForBooking(ctx, booking.ID))
	if err := s.bookings.Update(ctx, booking); err != nil {
		return fmt.Errorf("cancel booking %s after an unrecorded refund: %w", booking.ID, err)
	}

	s.realtime.PublishBookingChanged(booking.ComplexID)

	s.logger.Info("mp webhook: refund/chargeback (no payment record) — booking cancelled",
		"mp_payment_id", mpPaymentID,
		"mp_status", mpPayment.Status,
		"booking_id", booking.ID,
	)

	// There is no payment row here, which is exactly why this path told the client
	// nothing: the booking was marked refunded and the message was skipped because
	// the amount had nowhere to come from. MercadoPago's own copy of the payment
	// has it, and the client is owed the sentence either way.
	refunded := int(math.Round(mpPayment.TransactionAmountRefunded * 100))
	if refunded <= 0 {
		refunded = int(math.Round(mpPayment.TransactionAmount * 100))
	}
	s.sendRefundNotification(ctx, booking, refunded)

	// Same event as above, from the path that has no payment row to name: the
	// refund arrived before this system ever recorded the payment it reverses.
	// PaymentID is left absent rather than invented.
	s.record(booking.ComplexID, booking.ID, providerRefundAction(mpPayment.Status), moneyEvent{
		Actor:          actorProvider,
		MPPaymentID:    mpPaymentID,
		AmountCentavos: refunded,
		Result:         string(paymentstore.RefundIssued),
		Reason:         "no payment record existed for the money this refund reverses",
	})
	return nil
}

// sendRefundNotification tells the client that refundCentavos came back.
//
// It takes the amount rather than the payment because two of its callers do not
// have a payment row to read it off: the retry job re-reads it after recording,
// and a refund that arrives for a booking with no payment record at all only has
// MercadoPago's own figure. Passing the struct is how a notification that
// announced whatever the stale in-memory copy happened to hold used to happen.
func (s *Service) sendRefundNotification(ctx context.Context, booking *bookingstore.Booking, refundCentavos int) {
	client, err := s.clients.GetByID(ctx, booking.ClientID)
	if err != nil {
		s.logger.Error("refund notification: failed to fetch client",
			"error", err,
			"booking_id", booking.ID,
		)
		return
	}

	complex, err := s.complexes.GetByID(ctx, booking.ComplexID)
	if err != nil {
		s.logger.Error("refund notification: failed to fetch complex",
			"error", err,
			"booking_id", booking.ID,
		)
		return
	}

	clientEmail := ""
	if client.Email != nil {
		clientEmail = *client.Email
	}
	amount := formatARS(refundCentavos)
	s.notify.DepositRefunded(notifications.Refund{
		Email:       clientEmail,
		Phone:       client.Phone,
		ComplexName: complex.Name,
		Amount:      amount,
		BookPath:    booklink.BookPath(complex.Slug),
		BookURL:     booklink.Book(s.cfg.FrontendURL, complex.Slug),
	})
}

// AutoRefundIfPaid issues a refund for a cancelled booking that was paid for,
// and reports what became of the money.
//
// Every cancellation path goes through here — the owner cancelling from the
// dashboard and a client cancelling from the public page — so the money always
// comes back the same way, and exactly once.
//
// The shape is claim, call, record. ClaimRefund commits a reservation and a
// durable attempt row before MercadoPago is called; the call itself runs with no
// transaction and no row lock held; then exactly one of the two recorders closes
// the attempt out. Anything that goes wrong after the claim — this process dying,
// the database refusing the write, the client disconnecting — leaves that attempt
// queued, and the retry job finishes the job.
//
// It used to return nothing, which is what made three separate defects possible:
// the cancel endpoint reported "no refund" after a successful one, cancel-info
// promised refunds this function silently declined to make, and money that could
// only be returned by hand left no trace anywhere. Every exit below now names
// itself, and every exit that leaves money owed with nothing automatic behind it
// goes through owedManually, which alerts.
//
// It never returns RefundNotEligible: whether a cancellation deserves its money
// back is the caller's policy — it differs between the staff and public paths on
// purpose — and this function only ever answers what it can actually do.
func (s *Service) AutoRefundIfPaid(ctx context.Context, booking *bookingstore.Booking) paymentstore.RefundOutcome {
	outcome := s.autoRefundIfPaid(ctx, booking)

	// One entry per call, written around the body rather than at each of its
	// exits. There are more than a dozen of those, spread across four
	// functions, and an exit added later would otherwise leave the trail
	// silent with nothing to notice it. The outcome is the whole answer to
	// "what happened to this client's money", including the two results that
	// need a person: RefundManual, and a RefundQueued whose budget is spent.
	//
	// It names no payment row, because the exits that stop inside refundable()
	// never hold one — the booking is the identity every exit shares, and it
	// is what the entry is keyed on anyway.
	//
	// Who asked for the cancellation is not recorded here and does not need to
	// be: internal/bookings writes that against the same booking id, from the
	// request that had the actor.
	s.record(booking.ComplexID, booking.ID, "refund", refundEvent(actorSystem, outcome))
	return outcome
}

// autoRefundIfPaid is AutoRefundIfPaid's body; see its doc comment. It is
// split out so that every one of its exits is recorded by exactly one call to
// the trail.
//
// A booking can carry more than one unrefunded payment row — a deposit paid
// through MercadoPago and a balance the owner confirmed in cash — because
// ConfirmPayment inserts a second row rather than replacing the first
// (internal/bookings/actions.go). refundable splits those rows into the ones
// this function can send to MercadoPago (auto) and the ones nothing here can
// touch (manualOwed, cash/transfer). Every auto row is claimed and refunded
// exactly as a single payment used to be; the cash remainder is reported
// once, through owedManually, so the "MANUAL REFUND OWED" alert fires for the
// whole of it rather than being lost behind whichever row GetByBookingID
// used to prefer.
func (s *Service) autoRefundIfPaid(ctx context.Context, booking *bookingstore.Booking) paymentstore.RefundOutcome {
	// A return not reached through a committed ClaimRefund leaves the
	// refund-intent marker set; see clearRefundIntentUnlessClaimed.
	committed := false
	defer s.clearRefundIntentUnlessClaimed(ctx, booking.ID, &committed)

	auto, manualOwed, manualReason, stop := s.refundable(ctx, booking)
	if stop != nil {
		return *stop
	}

	if len(auto) == 0 {
		// Every unrefunded row is cash or a transfer: there is nothing to send
		// to MercadoPago, and the whole remainder is owed by hand. This is
		// exactly today's single-row cash behavior when there is only one row.
		return s.owedManually(booking, manualOwed, manualReason)
	}

	// A client closing the tab must not abandon a refund midway. The claim below is
	// what makes an abandoned refund recoverable, so it above all must not be
	// cancelled halfway through by an unrelated HTTP request ending.
	ctx = context.WithoutCancel(ctx)

	var outcome paymentstore.RefundOutcome
	autoTotal := 0
	for _, payment := range auto {
		outcome = s.autoRefundOnePayment(ctx, booking, payment, manualOwed, &committed)
		autoTotal += outcome.AmountCentavos
	}
	outcome.AmountCentavos = autoTotal

	if manualOwed > 0 {
		// The auto row(s) above are handled exactly as before; the cash/transfer
		// remainder gets its own alert and its own second trail entry — the same
		// "two entries rather than one merged" precedent process.go's
		// refundCancelledBookingPayment uses for a shortfall (see reportShortfall).
		manual := s.owedManually(booking, manualOwed, manualReason)
		outcome.ManualAmountCentavos = manual.AmountCentavos
		s.record(booking.ComplexID, booking.ID, "refund", refundEvent(actorSystem, manual))
	}

	return outcome
}

// autoRefundOnePayment claims, issues and records a refund for one payment
// row that refundable already identified as carrying a MercadoPago id with
// money still owed on it. It is exactly the single-row logic
// AutoRefundIfPaid used to run once, now run once per auto row so a booking
// with more than one refundable MercadoPago payment refunds every one of
// them rather than only the one GetByBookingID happened to prefer.
//
// manualOwed is refundable()'s already-computed cash/transfer balance for the
// whole booking, passed through so RecordRefundSuccess can write
// 'partial_refund' instead of clobbering it to 'refunded' when this row's
// full refund still leaves money owed by hand.
func (s *Service) autoRefundOnePayment(ctx context.Context, booking *bookingstore.Booking, payment *paymentstore.Payment, manualOwed int, committed *bool) paymentstore.RefundOutcome {
	owed := payment.Amount + payment.ServiceFee - payment.RefundAmount

	claim, err := s.payments.ClaimRefund(ctx, payment.ID)
	if err != nil {
		switch {
		case errors.Is(err, paymentstore.ErrAlreadyRefunded):
			// The money is already back. Nothing to do, and not a failure.
			return paymentstore.RefundOutcome{
				Result:         paymentstore.RefundAlreadyIssued,
				AmountCentavos: payment.RefundAmount,
				Reason:         "the payment was already refunded",
			}
		case errors.Is(err, paymentstore.ErrRefundInFlight):
			s.logger.Info("auto-refund: another claim already holds this refund",
				"booking_id", booking.ID, "payment_id", payment.ID)
			return paymentstore.RefundOutcome{
				Result:         paymentstore.RefundQueued,
				AmountCentavos: owed,
				Reason:         "another claim already holds this refund",
			}
		default:
			// Nothing was reserved and nothing was queued, so no retry job will
			// ever come back to this. It is owed by hand.
			s.logger.Error("auto-refund: failed to claim the refund", "error", err, "booking_id", booking.ID, "payment_id", payment.ID)
			return s.owedManually(booking, owed, "the refund could not be claimed, so nothing was queued")
		}
	}
	// The claim committed and already cleared the marker in its own transaction.
	*committed = true

	settled, outcome := s.issueRefund(ctx, *claim)
	if outcome != nil {
		return *outcome
	}

	refundTotal, err := s.payments.RecordRefundSuccess(ctx, settled, manualOwed)
	if err != nil {
		// The money has left the account. The claim's attempt row is still queued
		// with its backoff, so the retry job will replay this refund; MercadoPago's
		// idempotency key for it is derived from the payment id and the amount (see
		// internal/mp/mp.go), so the replay returns the original refund rather than
		// sending the money twice, and the record is written on that pass.
		s.logger.Error("auto-refund: refund issued but not recorded, left queued for retry",
			"error", err, "booking_id", booking.ID, "mp_payment_id", claim.MPPaymentID, "attempt_id", claim.AttemptID)
		sentry.CaptureMessage(fmt.Sprintf("auto-refund RECORD FAILED (money refunded, queued for retry): booking_id=%s mp_payment_id=%s attempt_id=%s", booking.ID, claim.MPPaymentID, claim.AttemptID))
		return paymentstore.RefundOutcome{
			Result:         paymentstore.RefundQueued,
			AmountCentavos: settled.RefundCentavos,
			Reason:         "the refund was issued but could not be recorded, and stays queued",
		}
	}

	s.logger.Info("auto-refund: refund issued successfully",
		"booking_id", booking.ID, "mp_payment_id", settled.MPPaymentID, "amount", centavosToPesos(settled.RefundCentavos))
	// The client is only told once the refund is recorded, and told the amount that
	// was actually written rather than whatever the in-memory struct still holds.
	s.sendRefundNotification(ctx, booking, refundTotal)

	// MercadoPago moved less than was claimed. The part that did move is back and
	// the client has been told about it; the rest is owed with the attempt row
	// already resolved behind it, which is exactly what owedManually is for.
	if short := refundShortfall(*claim, settled); short > 0 {
		return s.owedManually(booking, short,
			"mercadopago refunded less than was claimed, and the remainder has no queued attempt behind it")
	}
	return paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: refundTotal}
}

// clearRefundIntentUnlessClaimed is AutoRefundIfPaid's deferred cleanup: when
// *committed is still false at return time, no ClaimRefund ever committed, so
// the refund-intent marker (refund-intent-durability spec) is cleared here —
// on a context detached from the caller's own, because a client disconnecting
// the instant the handler answers must not silently leave the marker behind.
// It only logs on failure; clearing the marker is bookkeeping for the sweep,
// never the reason a refund call fails.
func (s *Service) clearRefundIntentUnlessClaimed(ctx context.Context, bookingID uuid.UUID, committed *bool) {
	if *committed {
		return
	}
	if err := s.refundIntents.ClearRefundIntent(context.WithoutCancel(ctx), bookingID); err != nil {
		s.logger.Error("auto-refund: failed to clear the refund intent marker",
			"error", err, "booking_id", bookingID)
	}
}

// issueRefund asks MercadoPago to execute an already-claimed refund, using the
// complex's own seller credential — never the platform's.
//
// A non-nil outcome is the whole answer and the caller returns (or, for the
// retry job, continues past) it unchanged: either the credential refused
// before the provider was ever called, or the provider rejected the refund it
// was asked to make. Both cases already alerted and requeued through
// refuseForCredential/recordRefundFailure by the time this returns, so the
// caller adds nothing further. A nil outcome means the provider accepted the
// refund and it is the caller's job to record that.
//
// The claim returned alongside is the one to record, and it is not always the
// claim that went in: its RefundCentavos carries what MercadoPago says it
// actually moved. See reconcileRefundedAmount — the response used to be
// discarded, so every recorder wrote the amount this codebase had asked for
// rather than the amount the provider answered with.
func (s *Service) issueRefund(ctx context.Context, claim paymentstore.RefundClaim) (paymentstore.RefundClaim, *paymentstore.RefundOutcome) {
	seller, err := s.sellerCredential(ctx, claim.ComplexID)
	if err != nil {
		outcome := s.refuseForCredential(ctx, claim, err)
		return claim, &outcome
	}
	refund, err := s.provider.RefundPayment(ctx, claim.MPPaymentID, centavosToPesos(claim.RefundCentavos), seller)
	if err != nil {
		return s.handleRefundRequestError(ctx, claim, seller, err)
	}
	return s.handleRefundResponse(ctx, claim, refund)
}

// handleRefundResponse decides success or failure from the refund MercadoPago
// actually returned, rather than assuming a 2xx means the money moved.
//
// MP's refund object carries its own status independent of the HTTP status:
// 'approved' is a completed refund; 'in_process' is accepted but settles
// asynchronously — the later payment.updated/refund webhook still flows
// through processRefundedPayment, so it is recorded as a success here and the
// webhook is just a confirmation, not a second write. 'rejected' and
// 'cancelled' are MercadoPago explicitly declining to move the money despite
// the 2xx, and recording those as success would tell a client their money is
// coming back when it never left.
func (s *Service) handleRefundResponse(ctx context.Context, claim paymentstore.RefundClaim, refund *mp.Refund) (paymentstore.RefundClaim, *paymentstore.RefundOutcome) {
	switch refund.Status {
	case "", "approved":
		claim.RefundCentavos = s.reconcileRefundedAmount(claim, refund)
		return claim, nil
	case "in_process":
		s.logger.Info("refund: accepted by MercadoPago, settles asynchronously",
			"booking_id", claim.BookingID,
			"mp_payment_id", claim.MPPaymentID,
			"status", refund.Status,
		)
		claim.RefundCentavos = s.reconcileRefundedAmount(claim, refund)
		return claim, nil
	default:
		// "rejected", "cancelled", or any status MP might add later that is not
		// one of the two success states above.
		cause := fmt.Errorf("mp: refund returned status %q", refund.Status)
		outcome := s.recordRefundFailure(ctx, claim, cause)
		return claim, &outcome
	}
}

// handleRefundRequestError routes a failed refund call by what kind of
// failure it was.
//
// A 5xx or a timeout is an outage: MercadoPago never looked at the request,
// so recordRefundFailure's ordinary requeue-and-retry is exactly right. A
// 4xx is MercadoPago having looked at the request and refused it outright —
// including "this payment was already refunded", which is not actually a
// failure. Before spending the retry budget on that, this fetches the
// payment (with the same seller credential the refund itself used) and reads
// transaction_amount_refunded: if the provider already moved at least what
// this claim asked for, the refund is done and this resolves it as a
// success instead of burning an attempt and alerting on a refund that
// already happened.
func (s *Service) handleRefundRequestError(ctx context.Context, claim paymentstore.RefundClaim, seller mp.Caller, cause error) (paymentstore.RefundClaim, *paymentstore.RefundOutcome) {
	var apiErr *mp.APIError
	if !errors.As(cause, &apiErr) || apiErr.StatusCode < 400 || apiErr.StatusCode >= 500 {
		outcome := s.recordRefundFailure(ctx, claim, cause)
		return claim, &outcome
	}

	payment, err := s.provider.GetPayment(ctx, claim.MPPaymentID, seller)
	if err != nil {
		s.logger.Error("refund: could not fetch the payment after a 4xx refund rejection",
			"error", err, "booking_id", claim.BookingID, "mp_payment_id", claim.MPPaymentID)
		outcome := s.recordRefundFailure(ctx, claim, cause)
		return claim, &outcome
	}

	refundedCentavos := int(math.Round(payment.TransactionAmountRefunded * 100))
	if refundedCentavos < claim.RefundCentavos {
		outcome := s.recordRefundFailure(ctx, claim, cause)
		return claim, &outcome
	}

	s.logger.Info("refund already present at provider, resolving without spending a retry",
		"booking_id", claim.BookingID,
		"mp_payment_id", claim.MPPaymentID,
		"claimed_centavos", claim.RefundCentavos,
		"refunded_centavos", refundedCentavos,
	)
	claim.RefundCentavos = refundedCentavos
	return claim, nil
}

// reconcileRefundedAmount answers how much money MercadoPago actually moved
// for a refund it accepted.
//
// The amount sent to the provider is computed here — ClaimRefund derives it
// from the payment row under a FOR UPDATE lock — and the provider's own
// answer used to be thrown away at the call site, so a refund MercadoPago
// executed for a different sum than it was asked for was recorded, announced
// to the client and read back by refundable() as the sum this codebase had
// asked for. The row then said the money was fully back when part of it never
// left.
//
// MercadoPago is the source of truth here and this returns its figure, because
// it is the party that moved the money and this codebase is not. The one
// exception is an accepted refund whose response carries no usable amount: a
// zero there is the absence of an answer — an older API shape, a body that
// decoded to the zero value — and not the assertion that nothing moved, so
// treating it as one would record a refund of zero for money that did leave
// the account. That case keeps the claimed figure, which is the only other
// number in play.
//
// A disagreement is never silent. It means the payment row and MercadoPago
// disagree about this payment, and whichever of the two is wrong, a person has
// to look at it.
func (s *Service) reconcileRefundedAmount(claim paymentstore.RefundClaim, refund *mp.Refund) int {
	if refund == nil || refund.Amount <= 0 {
		return claim.RefundCentavos
	}

	moved := int(math.Round(refund.Amount * 100))
	if moved == claim.RefundCentavos {
		return moved
	}

	s.logger.Error("refund: MercadoPago moved a different amount than was claimed",
		"booking_id", claim.BookingID,
		"mp_payment_id", claim.MPPaymentID,
		"attempt_id", claim.AttemptID,
		"claimed_centavos", claim.RefundCentavos,
		"moved_centavos", moved,
	)
	sentry.CaptureMessage(fmt.Sprintf("REFUND AMOUNT MISMATCH (recording MercadoPago's figure): booking_id=%s mp_payment_id=%s attempt_id=%s claimed=%d moved=%d",
		claim.BookingID, claim.MPPaymentID, claim.AttemptID, claim.RefundCentavos, moved))
	return moved
}

// refundShortfall reports how much of a claimed refund MercadoPago did not
// move, given the claim as it was reserved and the claim as the provider
// settled it.
//
// A shortfall is money the client is still owed with nothing automatic behind
// it: RecordRefundSuccess resolves the attempt row for the amount that did
// move, so the retry queue is closed on this refund by the time anyone can
// notice. The remainder is re-claimable — the payment falls back to
// 'deposit_paid' with a partial refund_amount, which claimable() reads as a
// balance still owing — but nothing re-claims it on its own, so it is a person's
// job and has to be reported as one.
func refundShortfall(claimed, settled paymentstore.RefundClaim) int {
	if settled.RefundCentavos >= claimed.RefundCentavos {
		return 0
	}
	return claimed.RefundCentavos - settled.RefundCentavos
}

// refundable answers what there is to refund for a cancelled booking.
//
// A non-nil stop is the whole answer and the caller returns it: nothing about
// this booking's money can be sent to MercadoPago. These are the four returns
// AutoRefundIfPaid used to make with no value and, in two cases, no trace.
//
// It used to read a single payment row through GetByBookingID, which prefers
// the MercadoPago row when a booking has more than one (idx_payments_booking
// is not unique — see db/queries/payments.sql). A booking with a deposit paid
// online and a balance ConfirmPayment recorded in cash has exactly that: two
// rows. Reading only the preferred one made refundable() answer entirely off
// the MercadoPago deposit and never see the cash row at all, so a cancelled
// booking's cash balance was refunded from nowhere — owedManually never ran
// for it, and the "MANUAL REFUND OWED" alert this whole function exists to
// raise never fired. ListByBookingID reads every row instead, and the ones
// that carry no MercadoPago id are summed into manualOwed rather than
// dropped.
func (s *Service) refundable(ctx context.Context, booking *bookingstore.Booking) (auto []*paymentstore.Payment, manualOwed int, manualReason string, stop *paymentstore.RefundOutcome) {
	// Read off the two axes payment_status was split into. The refund axis answers
	// first because it is the one that can stop this call; the collection axis
	// then says whether there is anything to send back at all. The order is not
	// a preference: the bookings_refund_needs_collected_money CHECK makes
	// refund_status <> 'none' imply collection_status <> 'unpaid', so no row can
	// satisfy both a refund case and the never-paid default.
	switch {
	case booking.RefundStatus == bookingstore.RefundStatusFull:
		return nil, 0, "", &paymentstore.RefundOutcome{Result: paymentstore.RefundAlreadyIssued, Reason: "the booking was already refunded"}
	case booking.RefundStatus == bookingstore.RefundStatusPending:
		// A claim is already committed against this booking's payment, so the
		// money is on its way and this call must not start a second one.
		return nil, 0, "", &paymentstore.RefundOutcome{Result: paymentstore.RefundQueued, Reason: "a refund for this booking is already in flight"}
	case booking.CollectionStatus != bookingstore.CollectionStatusUnpaid:
		// Refundable — fall through. A partially refunded booking already has
		// its MercadoPago-backed rows refunded; a re-sweep here re-derives the
		// still-outstanding manual balance and re-alerts on it. The already
		// refunded rows are skipped by claimable() via ErrAlreadyRefunded, so
		// nothing here can send that money back twice.
	default:
		return nil, 0, "", &paymentstore.RefundOutcome{Result: paymentstore.RefundNone, Reason: "the booking was never paid"}
	}

	payments, err := s.payments.ListByBookingID(ctx, booking.ID)
	if err != nil {
		s.logger.Error("auto-refund: failed to fetch payments", "error", err, "booking_id", booking.ID)
		manual := s.owedManually(booking, booking.DepositAmount, "the payment records could not be read")
		return nil, 0, "", &manual
	}
	if len(payments) == 0 {
		// The booking reads as paid and there is no payment row to refund
		// against. Somebody's money is here and only a person can find it.
		manual := s.owedManually(booking, booking.DepositAmount, "the booking reads as paid but carries no payment record")
		return nil, 0, "", &manual
	}

	alreadyRefunded := 0
	for _, payment := range payments {
		alreadyRefunded += payment.RefundAmount

		owed := payment.Amount + payment.ServiceFee - payment.RefundAmount
		if payment.Status == "refunded" || owed <= 0 {
			continue
		}
		if payment.MPPaymentID != nil && *payment.MPPaymentID != "" {
			auto = append(auto, payment)
		}
	}
	// manualOwed/manualMethods is the same split, computed once and shared with
	// every other caller that has to answer "is this booking's cash owed by
	// hand": the retry job, the webhook writers and the new manual-refund
	// endpoint all need it too, and this used to be the only place it was
	// derived.
	var manualMethods []string
	manualOwed, manualMethods = manualBalance(payments)

	if len(auto) == 0 && manualOwed == 0 {
		// Every row is already refunded, or leaves nothing owed.
		return nil, 0, "", &paymentstore.RefundOutcome{
			Result:         paymentstore.RefundAlreadyIssued,
			AmountCentavos: alreadyRefunded,
			Reason:         "the payment was already refunded",
		}
	}

	if manualOwed > 0 {
		manualReason = fmt.Sprintf("the %s payment(s) carry no mercadopago id, so they cannot be refunded automatically",
			strings.Join(uniqueOrdered(manualMethods), ", "))
	}

	return auto, manualOwed, manualReason, nil
}

// manualBalance answers how much of a booking's payments is owed by hand and
// which methods carried it, given every payment row on the booking.
//
// A row is manual when it carries no MercadoPago id — cash, a transfer, or a
// MercadoPago payment that never got an id — and still has money owed on it.
// This used to be inlined in refundable()'s own loop; it is shared now
// because every other place that has to answer "is cash still owed on this
// booking" needs the identical rule: the partial-refund status write in
// RecordRefundSuccess and the webhook writers, and the manual-refund endpoint
// that returns this money by hand.
func manualBalance(payments []*paymentstore.Payment) (owedCentavos int, methods []string) {
	for _, payment := range payments {
		owed := payment.Amount + payment.ServiceFee - payment.RefundAmount
		if payment.Status == "refunded" || owed <= 0 {
			continue
		}
		if payment.MPPaymentID == nil || *payment.MPPaymentID == "" {
			owedCentavos += owed
			methods = append(methods, payment.Method)
		}
	}
	return owedCentavos, methods
}

// manualOwedForBooking reads a booking's whole payment ledger and answers
// manualBalance's question for callers that, unlike autoRefundIfPaid, do not
// already hold the ledger from refundable(): the retry job and the webhook
// paths only ever see a single claim or a single MercadoPago payment, not the
// sibling cash row that might still be owed.
//
// A read failure is logged and treated as zero owed rather than propagated:
// the caller here is already past the point of no return — the refund was
// already issued or the webhook already told this system money moved — and
// refusing to record a successful refund because a second, unrelated read
// failed would leave that money floating with nothing to reconcile it. The
// booking still reads 'refunded' in that case, exactly as before this
// change; a booking with a genuine manual balance and an unreadable ledger
// gets a second chance to be caught the next time anything re-derives it.
func (s *Service) manualOwedForBooking(ctx context.Context, bookingID uuid.UUID) int {
	payments, err := s.payments.ListByBookingID(ctx, bookingID)
	if err != nil {
		s.logger.Error("refund: failed to read the payment ledger while checking for a manual balance",
			"error", err, "booking_id", bookingID)
		return 0
	}
	owed, _ := manualBalance(payments)
	return owed
}

// bookingRefundStatusAfterRefund is the booking-level refund_status a fully
// refunded MercadoPago row leaves behind, given the booking's manual
// cash/transfer balance. It is what the webhook writers above use instead of
// hardcoding 'full' — the same clobber cancelRefundedBooking used to make (see
// the payment_status enum in db/migrations/001_init.sql).
//
// It answers on the refund axis only. What the booking collected is a separate
// column since the payment_status split and no writer here touches it: a deposit-only
// booking whose deposit came back reads (deposit_paid, full), which under the
// single payment_status enum read 'refunded' and lost the deposit half.
func bookingRefundStatusAfterRefund(manualOwedCentavos int) string {
	if manualOwedCentavos > 0 {
		return bookingstore.RefundStatusPartial
	}
	return bookingstore.RefundStatusFull
}

// uniqueOrdered returns values with duplicates removed, keeping the order of
// first appearance — so a booking with two cash rows reports "the cash
// payment(s)..." once rather than "the cash, cash payment(s)...".
func uniqueOrdered(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// owedManually records money that is owed to a client and that nothing in this
// system can return, so that a person hears about it.
//
// Every branch that reaches it used to be a bare `return` from AutoRefundIfPaid.
// A booking paid in cash produced no log line at all: the client was told they
// qualified for a refund, the booking was cancelled, and the only record that
// money was owed was the client's memory.
func (s *Service) owedManually(booking *bookingstore.Booking, centavos int, reason string) paymentstore.RefundOutcome {
	s.logger.Error("auto-refund: a refund is owed that this system cannot issue",
		"booking_id", booking.ID,
		"complex_id", booking.ComplexID,
		"amount", centavos,
		"reason", reason,
	)
	sentry.CaptureMessage(fmt.Sprintf("MANUAL REFUND OWED: booking_id=%s complex_id=%s amount=%d — %s", booking.ID, booking.ComplexID, centavos, reason))
	return paymentstore.RefundOutcome{Result: paymentstore.RefundManual, AmountCentavos: centavos, Reason: reason}
}

// recordRefundFailure requeues a claimed refund the provider rejected, alerts
// when its retry budget is spent, and reports which of the two happened.
func (s *Service) recordRefundFailure(ctx context.Context, claim paymentstore.RefundClaim, cause error) paymentstore.RefundOutcome {
	s.logger.Error("auto-refund: MercadoPago rejected the refund, queued for retry",
		"error", cause,
		"booking_id", claim.BookingID,
		"mp_payment_id", claim.MPPaymentID,
		"attempt_id", claim.AttemptID,
	)

	exhausted, err := s.payments.RecordRefundFailure(ctx, claim, cause.Error())
	if err != nil {
		s.logger.Error("auto-refund: failed to requeue the refund attempt",
			"error", err, "attempt_id", claim.AttemptID, "booking_id", claim.BookingID)
		// The attempt row is still 'pending' from the claim, so the retry job picks
		// it up regardless; only its backoff and error message are missing.
		return paymentstore.RefundOutcome{
			Result:         paymentstore.RefundQueued,
			AmountCentavos: claim.RefundCentavos,
			Reason:         "the provider rejected the refund and the requeue could not be written, but the claim is still queued",
		}
	}
	if exhausted {
		s.logger.Error("auto-refund: EXHAUSTED all retries, manual intervention required",
			"attempt_id", claim.AttemptID,
			"booking_id", claim.BookingID,
			"mp_payment_id", claim.MPPaymentID,
			"amount", claim.RefundCentavos,
		)
		sentry.CaptureMessage(fmt.Sprintf("REFUND EXHAUSTED: booking_id=%s mp_payment_id=%s amount=%d — manual refund required", claim.BookingID, claim.MPPaymentID, claim.RefundCentavos))
		return paymentstore.RefundOutcome{
			Result:         paymentstore.RefundManual,
			AmountCentavos: claim.RefundCentavos,
			Reason:         "the refund exhausted its retry budget",
		}
	}
	return paymentstore.RefundOutcome{
		Result:         paymentstore.RefundQueued,
		AmountCentavos: claim.RefundCentavos,
		Reason:         "the provider rejected the refund and it is queued for retry",
	}
}

// centavosToPesos converts a stored currency amount to the pesos MercadoPago's
// API speaks in.
func centavosToPesos(centavos int) float64 {
	return float64(centavos) / 100.0
}

// formatARS writes an amount in centavos the way an Argentine reader expects
// it: "$3.000,50", with a dot between thousands and a comma before the
// centavos. The refund email used to say "$3000.50", which reads as a foreign
// amount in the one message that tells a client how much money came back.
//
// The rendering itself lives in internal/notifications, which is where the
// rest of the money copy is: the cancellation sentences quote amounts too, and
// two formatters would eventually disagree about a separator inside one
// conversation.
func formatARS(centavos int) string {
	return notifications.FormatARS(centavos)
}

// sellerCredential names the complex as the MercadoPago caller a refund is
// issued as, or returns the arm of failure internal/mpcred already
// distinguishes as a typed sentinel: a wrapped GetByID failure (the complex
// could not be fetched — a transient database read), mpcred.ErrMPNotConnected
// (the venue never connected MercadoPago, or disconnected it), or
// mpcred.ErrMPCredentialUnreadable (a stored credential exists but will not
// decrypt).
//
// It used to collapse all three into "", and callers fell back to the
// marketplace token — a refund issued against the platform token is a refund
// from the wrong account, and MercadoPago answering "no" to it arrives as an
// ordinary provider rejection indistinguishable from a network blip. No
// caller may call the provider when the returned error is non-nil;
// refuseForCredential is what turns each arm into the right alert and
// retry-budget treatment instead. mp.AsSeller is a second floor under that
// rule: there is no token this can hand back that reaches MercadoPago as the
// platform.
func (s *Service) sellerCredential(ctx context.Context, complexID uuid.UUID) (mp.Caller, error) {
	complex, err := s.complexes.GetByID(ctx, complexID)
	if err != nil {
		return mp.Caller{}, fmt.Errorf("fetch complex %s for its seller credential: %w", complexID, err)
	}
	token, err := complex.SellerAccessToken()
	if err != nil {
		return mp.Caller{}, err
	}
	return mp.AsSeller(token)
}

// refuseForCredential turns a sellerCredential failure into a queued refusal
// instead of a call to MercadoPago made as anyone but the seller.
//
// It always runs against an already-committed attempt row: every call site
// holds a ClaimRefund (or, for the retry job, a pre-existing failed_refunds
// row) before asking for the seller token, so this never produces the
// untracked owedManually refusal — recordRefundFailure is the seam that
// already exists for a provider that rejected the refund, and this reuses it
// rather than adding a second classification rule.
//
// UNAVAILABLE's cause embeds "seller credential unavailable", the one marker
// providerOutageMarkers gained for this, so it is treated as a transient
// outage and does not spend a retry. UNREADABLE and MISSING do not match any
// marker, so each attempt both alerts and spends the budget — deliberately:
// the operator was told at t=0 either way, and a credential re-linked inside
// the retry window finishes the refund by itself.
func (s *Service) refuseForCredential(ctx context.Context, claim paymentstore.RefundClaim, err error) paymentstore.RefundOutcome {
	reason, cause := "UNAVAILABLE", fmt.Sprintf("seller credential unavailable: %v", err)
	switch {
	case errors.Is(err, mpcred.ErrMPCredentialUnreadable):
		reason, cause = "UNREADABLE", fmt.Sprintf("seller credential unreadable for complex %s: %v", claim.ComplexID, err)
	case errors.Is(err, mpcred.ErrMPNotConnected):
		reason, cause = "MISSING", fmt.Sprintf("seller credential missing for complex %s: %v", claim.ComplexID, err)
	}

	s.logger.Error("auto-refund: seller credential refusal, MercadoPago was never called",
		"reason", reason,
		"error", err,
		"booking_id", claim.BookingID,
		"complex_id", claim.ComplexID,
		"attempt_id", claim.AttemptID,
	)
	sentry.CaptureMessage(fmt.Sprintf("SELLER CREDENTIAL %s: booking_id=%s complex_id=%s attempt_id=%s error=%v", reason, claim.BookingID, claim.ComplexID, claim.AttemptID, err))

	return s.recordRefundFailure(ctx, claim, errors.New(cause))
}

// RetryFailedRefunds works the queue of refunds still owed to a client.
//
// Under the claim-first design a queued attempt is the ordinary state of a refund
// in progress rather than an error path, so this job no longer reconstructs what
// happened: every row it sees was written by ClaimRefund and carries the payment,
// the booking and the exact amount. It replays the provider call and hands the
// outcome to the same two recorders AutoRefundIfPaid uses.
//
// That is what collapsed this function. It used to fetch the payment and booking,
// mark the attempt resolved, then separately mark the payment refunded — in that
// order, from two unrelated statements, so a crash between them left a refunded
// client with a court that still read as sold and an attempt already closed.
// RecordRefundSuccess now writes the money state and resolves the attempt in one
// transaction, which removes the ordering question entirely.
//
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // one cohesive queue-worker loop; splitting it would relocate
func (s *Service) RetryFailedRefunds(ctx context.Context) {
	pending, err := s.failedRefunds.GetPendingDue(ctx)
	if err != nil {
		s.logger.Error("refund-retry: failed to fetch pending refunds", "error", err)
		return
	}
	if len(pending) == 0 {
		return
	}

	processed := 0
	for _, fr := range pending {
		// The in-flight marker: it stops a second worker picking up the same row on
		// the next tick, and GetPendingDue reclaims it if this process dies here.
		if err := s.failedRefunds.MarkProcessing(ctx, fr.ID); err != nil {
			s.logger.Error("refund-retry: failed to mark as processing", "error", err, "id", fr.ID)
			continue
		}

		claim := paymentstore.RefundClaim{
			AttemptID:      fr.ID,
			PaymentID:      fr.PaymentID,
			BookingID:      fr.BookingID,
			ComplexID:      fr.ComplexID,
			MPPaymentID:    fr.MPPaymentID,
			RefundCentavos: fr.Amount,
		}

		// Replaying a refund is safe: the idempotency key MercadoPago sees is derived
		// from the payment id and the amount (see internal/mp/mp.go), so a refund this
		// job already made returns the original rather than moving the money again.
		settled, outcome := s.issueRefund(ctx, claim)
		if outcome != nil {
			// The provider refused again, or the seller credential did. The
			// row is either requeued with a longer backoff or exhausted, and
			// an exhausted refund is money owed with nothing automatic left
			// behind it — the entry is how an owner finds that without
			// reading the platform's alerts.
			s.record(fr.ComplexID, fr.BookingID, "refund_retry",
				claimEvent(claim, outcome.AmountCentavos, outcome.Result, outcome.Reason))
			continue
		}

		manualOwed := s.manualOwedForBooking(ctx, fr.BookingID)
		refundTotal, err := s.payments.RecordRefundSuccess(ctx, settled, manualOwed)
		if err != nil {
			s.logger.Error("refund-retry: refund issued but not recorded, left queued for retry",
				"error", err, "id", fr.ID, "payment_id", fr.PaymentID)
			sentry.CaptureMessage(fmt.Sprintf("refund-retry RECORD FAILED (money refunded, queued for retry): booking_id=%s mp_payment_id=%s attempt_id=%s", fr.BookingID, fr.MPPaymentID, fr.ID))
			s.record(fr.ComplexID, fr.BookingID, "refund_retry", claimEvent(claim, settled.RefundCentavos,
				paymentstore.RefundQueued, "the refund was issued but could not be recorded, and stays queued"))
			continue
		}

		s.record(fr.ComplexID, fr.BookingID, "refund_retry",
			claimEvent(claim, refundTotal, paymentstore.RefundIssued, ""))

		s.notifyRetriedRefund(ctx, fr, refundTotal)

		s.logger.Info("refund-retry: successfully refunded",
			"id", fr.ID,
			"booking_id", fr.BookingID,
			"mp_payment_id", fr.MPPaymentID,
			"amount", centavosToPesos(settled.RefundCentavos),
		)
		// The attempt row this job just resolved covered the whole claimed amount,
		// so a shortfall leaves the remainder with nothing queued behind it. Same
		// reasoning as AutoRefundIfPaid's own shortfall exit; this job has no
		// outcome to return, so the alert and the entry are the whole report.
		s.reportShortfall("refund-retry", "refund_retry", fr.ComplexID, fr.BookingID, claim, settled)
		processed++
	}

	if processed > 0 {
		s.logger.Info("refund-retry: completed", "processed", processed, "total", len(pending))
	}
}

// notifyRetriedRefund tells the client about a refund the retry job completed.
// The amount is the total RecordRefundSuccess actually wrote, so the message
// cannot disagree with the row.
func (s *Service) notifyRetriedRefund(ctx context.Context, fr *paymentstore.FailedRefund, refundTotal int) {
	booking, bErr := s.bookings.GetByID(ctx, fr.BookingID)
	if bErr != nil {
		s.logger.Error("refund-retry: failed to fetch booking", "error", bErr, "booking_id", fr.BookingID)
		return
	}
	s.sendRefundNotification(ctx, booking, refundTotal)
}

// refundIntentGrace is how long a booking's refund-intent marker
// (refund-intent-durability spec) must stand before the sweep may claim it.
// It has to outlast one honest in-request refund — the MercadoPago client's
// own timeout is 8 seconds — so five minutes is comfortably past it, the same
// reasoning failed_refunds.go's staleRefundProcessing gives for its own
// five-minute window.
const refundIntentGrace = 5 * time.Minute

// refundIntentBatch is how many orphans one sweep reads, mirroring
// failed_refunds.go's refundSweepBatch: the sweep is sequential and a
// MercadoPago call takes about eight seconds, so this is what a two-minute
// run can actually work through.
const refundIntentBatch = 12

// SweepOrphanedRefundIntents finds cancellations whose refund-intent marker
// has stood past its grace period — the crash window between a cancel commit
// and AutoRefundIfPaid's ClaimRefund — claims each one exclusively, and
// drives it through the same AutoRefundIfPaid path an ordinary cancellation
// uses.
//
// The sweep clears nothing itself: AutoRefundIfPaid's own exits and
// ClaimRefund's committed transaction (internal/payments/store/refunds.go) own every
// clearing path, so the marker's lifecycle is identical whether the call
// originated from a request or from here.
func (s *Service) SweepOrphanedRefundIntents(ctx context.Context) {
	orphans, err := s.refundIntents.GetRefundIntentOrphans(ctx, refundIntentGrace, refundIntentBatch)
	if err != nil {
		s.logger.Error("refund-intent-sweep: failed to fetch orphans", "error", err)
		return
	}
	if len(orphans) == 0 {
		return
	}

	claimed := 0
	for _, b := range orphans {
		// b.RefundIntentAt is guaranteed non-nil: GetRefundIntentOrphans'
		// predicate is refund_intent_at IS NOT NULL.
		if err := s.refundIntents.ClaimRefundIntent(ctx, b.ID, *b.RefundIntentAt); err != nil {
			if errors.Is(err, data.ErrRecordNotFound) {
				// Another instance already took this row on this tick.
				s.logger.Info("refund-intent-sweep: lost the claim race", "booking_id", b.ID)
				continue
			}
			s.logger.Error("refund-intent-sweep: failed to claim orphan", "error", err, "booking_id", b.ID)
			continue
		}
		claimed++

		outcome := s.AutoRefundIfPaid(ctx, b)
		s.logger.Info("refund-intent-sweep: processed orphan",
			"booking_id", b.ID,
			"result", outcome.Result,
			"amount", outcome.AmountCentavos,
		)
		if outcome.NeedsAHuman() {
			s.logger.Error("refund-intent-sweep: a refund is owed that must be returned by hand",
				"booking_id", b.ID, "amount", outcome.AmountCentavos, "reason", outcome.Reason)
		}
	}

	s.logger.Info("refund-intent-sweep: completed", "candidates", len(orphans), "claimed", claimed)
}
