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
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/waitlist"
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

	refundCentavos := refundedTotalOf(mpPayment)
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
	}

	// Update payment status.
	//
	// refund_amount is capped at this row's own amount + service_fee, because
	// the payments_refund_within_amount_paid CHECK refuses anything
	// larger and the write would fail outright. The cap is why the recorded and
	// charged totals are kept apart above rather than collapsed: the comparison
	// wants MercadoPago's figure, and the column will only hold ours.
	// What this event moved. MercadoPago's transaction_amount_refunded is the total
	// refunded on the payment so far, so this instalment is the difference from what
	// the row already holds. Reading the total as the instalment is what made the
	// trail record a later partial refund as everything refunded to date.
	movedNow := recordRefundedTotal(payment, refundCentavos)

	if isFullRefund {
		payment.Status = "refunded"
	}
	payment.StatusDetail = statusDetailPtr(mpPayment.StatusDetail)

	// Only a live booking can be cancelled by this refund, so only one is announced
	// as cancelled. Whether it was live is read before the write: a booking Vibe
	// already cancelled was announced when it was cancelled, and the webhook for the
	// money coming back must not announce it a second time.
	wasLive := liveBooking(booking.Status)
	cancelledByRefund := false

	// A booking another MercadoPago payment still holds money for stays as it is:
	// the refund is of this payment alone, a duplicate refunded from MercadoPago's own
	// panel. Cash and transfer rows do not count here, so refunding the deposit of a
	// booking with a counter payment still cancels it as before.
	paidElsewhere := false
	if isFullRefund && booking.Status != bookingstore.BookingStatusCancelled {
		paidElsewhere, err = s.bookingHoldsOtherMercadoPagoMoney(ctx, booking.ID, mpPaymentID)
		if err != nil {
			return fmt.Errorf("check the other payments of booking %s for a refunded payment: %w", booking.ID, err)
		}
	}

	// Every branch below writes conditionally, and written says whether it did.
	var written bool
	switch {
	case paidElsewhere && mpPayment.Status == "charged_back":
		// The booking's status and refund state are written back unchanged; the write
		// is for the chargeback note added above.
		if written, err = s.ledger.RecordWebhookRefund(ctx, payment, booking); err != nil {
			return fmt.Errorf("record the chargeback of payment %s on a booking paid by another: %w", mpPaymentID, err)
		}
	case paidElsewhere:
		if written, err = s.ledger.UpdateUnlessRefundPending(ctx, payment); err != nil {
			return fmt.Errorf("record the refund of payment %s on a booking paid by another: %w", mpPaymentID, err)
		}
	case isFullRefund && booking.Status != "completed" && booking.Status != "no_show":
		// Full refund: update payment + cancel booking atomically.
		// Do NOT cancel completed/no_show bookings — service was already rendered.
		booking.Status = "cancelled"
		booking.RefundStatus = bookingRefundStatusAfterRefund(s.manualOwedForBooking(ctx, booking.ID))
		if written, err = s.ledger.RecordWebhookRefund(ctx, payment, booking); err != nil {
			return fmt.Errorf("record the refund of payment %s and cancel its booking: %w", mpPaymentID, err)
		}
		cancelledByRefund = wasLive
	default:
		// Partial refund, or full refund on completed/no_show: only update payment.
		if isFullRefund {
			booking.RefundStatus = bookingRefundStatusAfterRefund(s.manualOwedForBooking(ctx, booking.ID))
			if written, err = s.ledger.RecordWebhookRefund(ctx, payment, booking); err != nil {
				return fmt.Errorf("record the refund of payment %s on a played booking: %w", mpPaymentID, err)
			}
		} else {
			if written, err = s.ledger.UpdateUnlessRefundPending(ctx, payment); err != nil {
				return fmt.Errorf("record the partial refund of payment %s: %w", mpPaymentID, err)
			}
		}
	}

	// The row was read before MercadoPago was asked about it. A claim of ours that
	// committed in between moved it to 'refund_pending', and the write above was
	// refused rather than overwrite that: the claim owns the refund, the same as
	// when the guard at the top of this function sees refund_pending. Nothing was
	// written, so nothing is announced, published or recorded.
	if !written {
		s.logger.Info("mp webhook: a claim of ours took this refund while the webhook was reading it, leaving the record to it",
			"mp_payment_id", mpPaymentID,
			"booking_id", payment.BookingID,
			"mp_status", mpPayment.Status,
		)
		return nil
	}

	s.realtime.PublishBookingChanged(booking.ComplexID)

	// Reported only once the write landed: a webhook that stood down above
	// recorded nothing, so it must not say a chargeback was processed.
	if mpPayment.Status == "charged_back" {
		sentry.CaptureMessage(fmt.Sprintf("MercadoPago CHARGEBACK processed: booking_id=%s mp_payment_id=%s", booking.ID, mpPaymentID))
	}

	s.logger.Info("mp webhook: refund/chargeback processed",
		"mp_payment_id", mpPaymentID,
		"mp_status", mpPayment.Status,
		"mp_status_detail", mpPayment.StatusDetail,
		"booking_id", booking.ID,
		"refund_centavos", movedNow,
		"refunded_total_centavos", refundCentavos,
		"full_refund", isFullRefund,
	)

	// A partial refund is money the client got back, and they were told nothing
	// about it: the notification used to be gated on isFullRefund, so somebody
	// refunded half their deposit and heard from nobody. What is announced is what
	// moved on this event: a second partial refund of 300 after one of 500 reads
	// "$300", not the running total of 800. An event that moved nothing new (a
	// chargeback with no figure) falls back to the amount the row now holds.
	//
	// It is unconditional now because reaching this line already means no claim of
	// ours holds this refund — the guard at the top of the function returned on
	// that case rather than only muting the message.
	s.sendRefundNotification(ctx, booking, announcedRefund(movedNow, payment), refundKey(mpPaymentID, payment.RefundAmount), cancelledByRefund)
	if cancelledByRefund {
		s.announceRefundedCancellation(ctx, booking)
	}

	// Money left this venue's account and nobody here decided it. The amount
	// recorded is MercadoPago's own figure for what moved on this event, not
	// the running total written to the row, because the question this entry
	// answers is what happened — the row already says where it ended up.
	s.record(booking.ComplexID, booking.ID, providerRefundAction(mpPayment.Status), moneyEvent{
		Actor:          actorProvider,
		PaymentID:      &payment.ID,
		MPPaymentID:    mpPaymentID,
		AmountCentavos: movedNow,
		Result:         string(paymentstore.RefundIssued),
		Reason:         mpPayment.StatusDetail,
	})
	return nil
}

// bookingHoldsOtherMercadoPagoMoney reports whether a MercadoPago payment of the
// booking, other than mpPaymentID, still holds money the complex received.
func (s *Service) bookingHoldsOtherMercadoPagoMoney(ctx context.Context, bookingID uuid.UUID, mpPaymentID string) (bool, error) {
	holding, err := s.otherPaymentsHoldingMoney(ctx, bookingID, mpPaymentID)
	if err != nil {
		return false, err
	}
	for _, p := range holding {
		if p.MPPaymentID != nil && *p.MPPaymentID != "" {
			return true, nil
		}
	}
	return false, nil
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

	// Read before the write below, for the same reason as in processRefundedPayment.
	wasLive := liveBooking(booking.Status)

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
	s.sendRefundNotification(ctx, booking, refunded, refundKey(mpPaymentID, refunded), wasLive)
	if wasLive {
		s.announceRefundedCancellation(ctx, booking)
	}

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
//
// refundID names which refund of the booking this notice is about (see refundKey).
// It is the deduplication key's half that tells a second partial refund from a
// redelivery of the first; without it the queue dropped the second notice.
//
// cancelled says this refund cancelled a live booking, MercadoPago having
// refunded it on its own side. The client's notice then says the booking is off
// as well: it is the only email they get about it, and it used to talk about the
// money alone, leaving them to show up for a game that no longer existed. It is
// one email rather than two so one event does not reach the client twice.
func (s *Service) sendRefundNotification(ctx context.Context, booking *bookingstore.Booking, refundCentavos int, refundID string, cancelled bool) {
	s.sendRefundNotificationWithReason(ctx, booking, refundCentavos, refundID, cancelled, "")
}

// sendRefundNotificationWithReason is sendRefundNotification with the sentence
// saying why the money came back (refund_notices.go), for the refunds the
// webhook issues on its own: the client did not ask for them.
func (s *Service) sendRefundNotificationWithReason(ctx context.Context, booking *bookingstore.Booking, refundCentavos int, refundID string, cancelled bool, reason string) {
	ref, ok := s.refundNotice(ctx, booking, refundCentavos, refundID)
	if !ok {
		return
	}
	ref.Reason = reason
	if cancelled {
		ref.Cancelled = true
		ref.Date = booking.Date.Format("02/01")
		ref.StartTime = timezone.HoursLabel(booking.StartsAt, booking.EndsAt)
		if court, err := s.courts.GetByID(ctx, booking.CourtID); err == nil {
			ref.CourtName = court.Name
		} else {
			s.logger.Warn("refund notification: court unreadable, the cancellation notice names no court",
				"error", err, "booking_id", booking.ID)
		}
	}
	if err := s.notify.DepositRefunded(ref); err != nil {
		// The refund is recorded; failing the caller would not resend this.
		// The ids are what a resend needs, and the dedup key makes one safe.
		s.logger.Error("refund notification: the client's notice was not queued",
			"error", err, "booking_id", booking.ID, "refund", refundID, "cancelled", cancelled)
		sentry.CaptureMessage(fmt.Sprintf("REFUND NOTICE NOT QUEUED (client not told): booking_id=%s refund=%s", booking.ID, refundID))
	}
}

// refundNotice reads what every refund notice names (the client, the complex and
// its book link) and builds the notice for refundCentavos. ok is false when the
// client or the complex cannot be read; the failure is logged here.
func (s *Service) refundNotice(ctx context.Context, booking *bookingstore.Booking, refundCentavos int, refundID string) (notifications.Refund, bool) {
	client, err := bookingstore.ContactOf(ctx, s.clients, booking)
	if err != nil {
		s.logger.Error("refund notification: failed to fetch client",
			"error", err,
			"booking_id", booking.ID,
		)
		return notifications.Refund{}, false
	}

	complex, err := s.complexes.GetByID(ctx, booking.ComplexID)
	if err != nil {
		s.logger.Error("refund notification: failed to fetch complex",
			"error", err,
			"booking_id", booking.ID,
		)
		return notifications.Refund{}, false
	}

	return notifications.Refund{
		Email:        client.Email,
		ComplexName:  complex.Name,
		Amount:       formatARS(refundCentavos),
		BookURL:      booklink.Book(s.cfg.PublicURL, complex.Slug),
		ComplexPhone: complex.Phone,
		OwnerID:      complex.OwnerID.String(),
		BookingID:    booking.ID.String(),
		RefundID:     refundID,
	}, true
}

// leaveCashOwedAfterExhaustedRefund runs once an automatic refund has spent its
// retry budget. The refund is not abandoned: the retry job keeps re-selecting
// exhausted rows, so MercadoPago can still pay it. The client is told nothing
// until the money is actually back (the refunded notice). When cash is still
// owed by hand, the booking is left at 'partial' so the owner can close that
// balance out; that money is genuinely owed by hand. The automatic half is not
// back, so nothing here marks the booking as anything more than that.
func (s *Service) leaveCashOwedAfterExhaustedRefund(ctx context.Context, claim paymentstore.RefundClaim) {
	if claim.BookingID == uuid.Nil {
		// An event registration has no cash balance to leave owed.
		return
	}
	booking, err := s.bookings.GetByID(ctx, claim.BookingID)
	if err != nil {
		s.logger.Error("refund: failed to fetch the booking after an exhausted refund",
			"error", err, "booking_id", claim.BookingID, "attempt_id", claim.AttemptID)
		return
	}

	if manualOwed := s.manualOwedForBooking(ctx, booking.ID); manualOwed > 0 {
		before := booking.RefundStatus
		s.markManualBalanceOwed(ctx, booking)
		// Told only when this call is what left the cash owed: the write
		// committed and moved the booking to 'partial'. Only the cash balance
		// is announced; the automatic half is still the retry job's. Only a
		// cancelled booking owes its cash back: on one that stands (a second
		// payment's refund ran out) the cash is the valid payment of the
		// booking, and ManualRefund refuses it anyway.
		if booking.Status == bookingstore.BookingStatusCancelled &&
			before == bookingstore.RefundStatusNone && booking.RefundStatus == bookingstore.RefundStatusPartial {
			s.announceRefundOwedByHand(ctx, booking, manualOwed, claim.AttemptID)
		}
	}
}

// refundKey names one refund of a booking for its client notice: the MercadoPago
// payment it came back on, and the running total it reached on that payment. A
// further partial refund reaches a higher total, so it is a new notice, while a
// redelivery of the same event reaches the same total and is still one notice.
func refundKey(mpPaymentID string, totalCentavos int) string {
	return fmt.Sprintf("%s:%d", mpPaymentID, totalCentavos)
}

// liveBooking reports whether a booking still holds its court: pending or
// confirmed. A cancelled, completed or no-show booking does not.
func liveBooking(status bookingstore.BookingStatus) bool {
	return status == bookingstore.BookingStatusPending || status == bookingstore.BookingStatusConfirmed
}

// announceRefundedCancellation runs once a refund MercadoPago issued has cancelled
// a live booking: it hands the freed day to the waitlist and tells the owner. It
// runs after the cancellation has committed, and a lookup that fails is logged
// rather than failing the webhook, because the money has already moved.
func (s *Service) announceRefundedCancellation(ctx context.Context, booking *bookingstore.Booking) {
	complex, err := s.complexes.GetByID(ctx, booking.ComplexID)
	if err != nil {
		s.logger.Error("refund cancellation: failed to fetch complex",
			"error", err, "booking_id", booking.ID)
		return
	}

	// The slot is free whether or not the owner can be told, so the waitlist is
	// handed it first. Detached from the webhook's context, as the cancel paths do.
	if s.waitlist != nil {
		detached := context.WithoutCancel(ctx)
		freed := waitlist.Freed{
			ComplexID: complex.ID, ComplexName: complex.Name, ComplexSlug: complex.Slug, Day: booking.Date,
		}
		s.run(func() { s.waitlist.SlotFreed(detached, freed) })
	}

	court, err := s.courts.GetByID(ctx, booking.CourtID)
	if err != nil {
		s.logger.Error("refund cancellation: failed to fetch court",
			"error", err, "booking_id", booking.ID)
		return
	}
	client, err := bookingstore.ContactOf(ctx, s.clients, booking)
	if err != nil {
		s.logger.Error("refund cancellation: failed to fetch client",
			"error", err, "booking_id", booking.ID)
		return
	}

	s.notify.OwnerBookingRefunded(notifications.OwnerBookingRefund{
		OwnerID:     complex.OwnerID.String(),
		ComplexName: complex.Name,
		CourtName:   court.Name,
		ClientName:  client.FullName(),
		Date:        booking.Date.Format("02/01"),
		StartTime:   timezone.HoursLabel(booking.StartsAt, booking.EndsAt),
		BookingID:   booking.ID.String(),
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
	return s.refundCancelledBooking(ctx, booking, false)
}

// refundCancelledBooking is AutoRefundIfPaid with who is asking. fromSweep is
// the sweep finishing a refund whose marker stood past its grace: that refund
// is claimed with the cause stored next to the marker, and a claim that fails
// for it again is owed by hand rather than marked for yet another sweep.
func (s *Service) refundCancelledBooking(ctx context.Context, booking *bookingstore.Booking, fromSweep bool) paymentstore.RefundOutcome {
	outcome := s.autoRefundIfPaid(ctx, booking, fromSweep)

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
func (s *Service) autoRefundIfPaid(ctx context.Context, booking *bookingstore.Booking, fromSweep bool) paymentstore.RefundOutcome {
	run := &refundRun{fromSweep: fromSweep}
	if fromSweep {
		run.cause = paymentstore.RefundCause(booking.RefundIntentCause)
	}
	// A return not reached through a committed ClaimRefund clears the
	// refund-intent marker (clearRefundIntentUnlessClaimed), unless a claim
	// that failed for a moment stood it up again for the sweep.
	defer func() {
		keep := run.committed || run.retrying
		s.clearRefundIntentUnlessClaimed(ctx, booking.ID, &keep)
	}()

	auto, manualOwed, manualReason, stop := s.refundable(ctx, booking)
	if stop != nil {
		if stop.Reason == paymentRowsUnread {
			// The rows behind a refund already under way could not be read, so
			// whether another payment is still owed is unknown. The sweep reads
			// them again; every read that fails alerts.
			run.retrying = s.standRefundIntentAgain(ctx, booking.ID)
			s.alertPaymentRowsUnread(booking.ID, run.retrying)
		}
		return *stop
	}

	if len(auto) == 0 {
		// Every unrefunded row is cash or a transfer: there is nothing to send
		// to MercadoPago, and the whole remainder is owed by hand. The booking
		// is marked 'partial' so the owner's manual-refund action can close it
		// out (markManualBalanceOwed).
		s.markManualBalanceOwed(ctx, booking)
		return s.owedManually(booking, manualOwed, manualReason)
	}

	// A client closing the tab must not abandon a refund midway. The claim below is
	// what makes an abandoned refund recoverable, so it above all must not be
	// cancelled halfway through by an unrelated HTTP request ending.
	ctx = context.WithoutCancel(ctx)

	rowOutcomes := make([]paymentstore.RefundOutcome, 0, len(auto))
	for _, payment := range auto {
		rowOutcomes = append(rowOutcomes, s.autoRefundOnePayment(ctx, booking, payment, manualOwed, run))
	}
	s.retryFailedClaims(ctx, booking, rowOutcomes, run)
	outcome := combineRefundOutcomes(rowOutcomes)

	if manualOwed > 0 {
		// The auto row(s) above are handled exactly as before; the cash/transfer
		// remainder gets its own alert and its own second trail entry — the same
		// "two entries rather than one merged" precedent process.go's
		// refundCancelledBookingPayment uses for a shortfall (see reportShortfall).
		manual := s.owedManually(booking, manualOwed, manualReason)
		outcome.ManualAmountCentavos += manual.AmountCentavos
		s.record(booking.ComplexID, booking.ID, "refund", refundEvent(actorSystem, manual))
	}

	return outcome
}

// refundRun is what the rows of one AutoRefundIfPaid call share.
type refundRun struct {
	// cause is why the money is owed: empty for a cancellation, the cause
	// stored next to the marker when the sweep finishes a refund.
	cause paymentstore.RefundCause
	// fromSweep is the sweep's own call; see refundCancelledBooking.
	fromSweep bool
	// committed is set once a ClaimRefund commits, which cleared the marker
	// inside its own transaction.
	committed bool
	// retrying is set once a claim that failed for a moment stood the marker
	// up again for the sweep.
	retrying bool
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
func (s *Service) autoRefundOnePayment(ctx context.Context, booking *bookingstore.Booking, payment *paymentstore.Payment, manualOwed int, run *refundRun) paymentstore.RefundOutcome {
	owed := payment.Amount + payment.ServiceFee - payment.RefundAmount

	claim, err := s.claimRefund(ctx, payment.ID, run.cause)
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
			// ever come back to this. The marker is what brings the sweep back
			// to it: stood up again from now, since an earlier row's claim may
			// already have cleared it. The sweep's own failed claim is owed by
			// hand instead, so a claim that keeps failing alerts rather than
			// being retried in silence.
			s.logger.Error("auto-refund: failed to claim the refund", "error", err, "booking_id", booking.ID, "payment_id", payment.ID)
			if !run.fromSweep && !claimFailsForGood(err) {
				// retryFailedClaims stands the marker up once every row is
				// done: a later row's committed claim clears it.
				return paymentstore.RefundOutcome{
					Result:         paymentstore.RefundQueued,
					AmountCentavos: owed,
					Reason:         claimRetriedBySweep,
				}
			}
			return s.owedManually(booking, owed, claimNotQueued)
		}
	}
	// The claim committed and already cleared the marker in its own transaction.
	run.committed = true

	settled, outcome := s.issueRefund(ctx, *claim)
	if outcome != nil {
		return *outcome
	}

	refundTotal, err := s.refunds.RecordRefundSuccess(ctx, settled, manualOwed)
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
	// A refund the sweep finished for a cause says why, as the webhook would have.
	s.announceAutomaticRefund(ctx, booking, claim.MPPaymentID, refundTotal, run.cause)

	// MercadoPago moved less than was claimed. The part that did move is back and
	// the client has been told about it; the rest is owed with the attempt row
	// already resolved behind it, which is exactly what owedManually is for.
	if short := refundShortfall(*claim, settled); short > 0 {
		return s.owedManually(booking, short,
			"mercadopago refunded less than was claimed, and the remainder has no queued attempt behind it")
	}
	return paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: refundTotal}
}

// The reasons a failed claim answers with: retried by the sweep, or owed by hand.
// paymentRowsUnread is the reason unclaimedOnlineRows gives when it cannot read
// a booking's rows.
const (
	paymentRowsUnread = "the booking's refund is under way, but its payment rows could not be read to look for another payment still owed"

	claimRetriedBySweep = "the refund could not be claimed for a moment, and the sweep retries it"
	claimNotQueued      = "the refund could not be claimed, so nothing was queued"
)

// claimFailsForGood reports whether a claim failed in a way no retry can fix:
// the payment row is gone, or it carries no MercadoPago id to refund against.
// Such a refund is owed by hand at once rather than after a sweep fails it again.
func claimFailsForGood(err error) bool {
	return errors.Is(err, data.ErrRecordNotFound) || errors.Is(err, paymentstore.ErrNoMercadoPagoID)
}

// alertPaymentRowsUnread reports a booking whose payment rows could not be read
// while its refund was under way, saying whether the sweep will read them again.
// When the marker could not be stood up either, nothing comes back for them and
// a person has to look.
func (s *Service) alertPaymentRowsUnread(bookingID uuid.UUID, kept bool) {
	if kept {
		sentry.CaptureMessage(fmt.Sprintf("AUTO-REFUND ROWS UNREAD (refund intent kept for the sweep): booking_id=%s", bookingID))
		return
	}
	sentry.CaptureMessage(fmt.Sprintf("AUTO-REFUND ROWS UNREAD AND REFUND INTENT LOST (check the booking's payments by hand): booking_id=%s", bookingID))
}

// retryFailedClaims stands the refund-intent marker up again for the rows whose
// claim failed for a moment, once every row of the call is done. Doing it per
// row lost it: a later row's committed claim clears the marker inside its own
// transaction. When the marker cannot be written, nothing would come back for
// those rows, so each is owed by hand instead.
func (s *Service) retryFailedClaims(ctx context.Context, booking *bookingstore.Booking, rowOutcomes []paymentstore.RefundOutcome, run *refundRun) {
	failed := false
	for _, o := range rowOutcomes {
		failed = failed || o.Reason == claimRetriedBySweep
	}
	if !failed {
		return
	}
	if s.standRefundIntentAgain(ctx, booking.ID) {
		run.retrying = true
		return
	}
	for i, o := range rowOutcomes {
		if o.Reason == claimRetriedBySweep {
			rowOutcomes[i] = s.owedManually(booking, o.AmountCentavos, claimNotQueued)
		}
	}
}

// claimRefund claims a payment's refund with the cause it is owed for; a
// cancellation's, with no cause, is a plain ClaimRefund.
func (s *Service) claimRefund(ctx context.Context, paymentID uuid.UUID, cause paymentstore.RefundCause) (*paymentstore.RefundClaim, error) {
	if cause == paymentstore.RefundCauseNone {
		return s.refunds.ClaimRefund(ctx, paymentID)
	}
	return s.refunds.ClaimRefundWithCause(ctx, paymentID, cause)
}

// standRefundIntentAgain stands a booking's refund-intent marker up again so
// the sweep retries a refund whose claim failed for a moment, and reports
// whether it did. A write that fails leaves nothing to come back for the
// money, so the caller owes it by hand.
func (s *Service) standRefundIntentAgain(ctx context.Context, bookingID uuid.UUID) bool {
	if err := s.refundIntents.MarkRefundIntent(context.WithoutCancel(ctx), bookingID); err != nil {
		s.logger.Error("auto-refund: failed to stand the refund intent marker up again",
			"error", err, "booking_id", bookingID)
		return false
	}
	return true
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

	// MercadoPago reports the total refunded on the payment, not what this claim
	// moved. The claim's share is that total less what the row already records, so
	// money recorded by an earlier refund is never counted again as this one.
	recorded, err := s.recordedRefundedCentavos(ctx, claim)
	if err != nil {
		s.logger.Error("refund: could not read the payment's recorded refunds after a 4xx refund rejection",
			"error", err, "booking_id", claim.BookingID, "mp_payment_id", claim.MPPaymentID)
		outcome := s.recordRefundFailure(ctx, claim, cause)
		return claim, &outcome
	}
	refundedCentavos := int(math.Round(payment.TransactionAmountRefunded * 100))
	if refundedCentavos-recorded < claim.RefundCentavos {
		outcome := s.recordRefundFailure(ctx, claim, cause)
		return claim, &outcome
	}

	s.logger.Info("refund already present at provider, resolving without spending a retry",
		"booking_id", claim.BookingID,
		"mp_payment_id", claim.MPPaymentID,
		"claimed_centavos", claim.RefundCentavos,
		"refunded_centavos", refundedCentavos,
		"recorded_centavos", recorded,
	)
	// The claim keeps the amount it asked for. MercadoPago now shows at least that much
	// moved on top of what was recorded, and that is all this refund may account for.
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
		return s.unclaimedOnlineRows(ctx, booking, paymentstore.RefundOutcome{Result: paymentstore.RefundAlreadyIssued, Reason: "the booking was already refunded"})
	case booking.RefundStatus == bookingstore.RefundStatusPending:
		// A claim is already committed against this booking's payment, so the
		// money is on its way and this call must not start a second one.
		return s.unclaimedOnlineRows(ctx, booking, paymentstore.RefundOutcome{Result: paymentstore.RefundQueued, Reason: "a refund for this booking is already in flight"})
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

// unclaimedOnlineRows is refundable() for a booking whose refund status already
// reads refunded or in flight. That status is the booking's, and a booking can
// carry more than one MercadoPago payment: a second approved payment on a
// cancelled booking is a row of its own, and the first payment's refund is what
// set the status. Deciding from the status alone stopped at "already refunded"
// while that second payment sat recorded and never refunded, and the orphan
// sweep cleared its marker on the same answer.
//
// So the rows decide, for the rows the status does not cover. When another
// MercadoPago row's refund is what the status records (a row refunded, partly
// refunded or claimed), every MercadoPago row that still holds money
// (paymentstore.ReceivedMoney) is returned to be refunded; claimable() still
// refuses anything already back or claimed, under its lock. When no row's refund
// stands behind the status, it was written by hand (a staff edit records a refund
// made outside Vibe), and the status answers as stop: refunding the row then
// would pay the client twice.
//
// A failed read answers with stop too, what this booking's status said before
// any row was read, with its reason saying the rows went unread. The rows are
// read again on the next ask, and autoRefundIfPaid keeps the refund-intent
// marker standing so there is a next ask: clearing it here left a second
// payment nobody would ever come back for.
func (s *Service) unclaimedOnlineRows(ctx context.Context, booking *bookingstore.Booking, stop paymentstore.RefundOutcome) ([]*paymentstore.Payment, int, string, *paymentstore.RefundOutcome) {
	payments, err := s.payments.ListByBookingID(ctx, booking.ID)
	if err != nil {
		s.logger.Error("auto-refund: failed to fetch payments", "error", err, "booking_id", booking.ID)
		stop.Reason = paymentRowsUnread
		return nil, 0, "", &stop
	}
	var auto []*paymentstore.Payment
	statusEarned := false
	for _, payment := range payments {
		online := payment.MPPaymentID != nil && *payment.MPPaymentID != ""
		if !online {
			continue
		}
		if payment.RefundAmount > 0 || payment.Status == "refunded" || payment.Status == "refund_pending" {
			statusEarned = true
		}
		owed := payment.Amount + payment.ServiceFee - payment.RefundAmount
		if paymentstore.ReceivedMoney(payment.Status) && payment.RefundAmount == 0 && owed > 0 {
			auto = append(auto, payment)
		}
	}
	if !statusEarned || len(auto) == 0 {
		return nil, 0, "", &stop
	}
	return auto, 0, "", nil
}

// manualBalance answers how much of a booking's payments is owed by hand and
// which methods carried it, given every payment row on the booking.
//
// A row is manual when it carries no MercadoPago id — cash, a transfer, or a
// MercadoPago payment that never got an id — was actually paid (see ReceivedMoney),
// and still has money owed on it.
// This used to be inlined in refundable()'s own loop; it is shared now
// because every other place that has to answer "is cash still owed on this
// booking" needs the identical rule: the partial-refund status write in
// RecordRefundSuccess and the webhook writers, and the manual-refund endpoint
// that returns this money by hand.
func manualBalance(payments []*paymentstore.Payment) (owedCentavos int, methods []string) {
	for _, payment := range payments {
		owed := payment.Amount + payment.ServiceFee - payment.RefundAmount
		// A checkout row is written 'unpaid' before the client pays anything, and it
		// has no MercadoPago id until they do. Without the status check it reads as a
		// counter payment that was never made, and its full price is owed back.
		if payment.Status == "refunded" || owed <= 0 || !paymentstore.ReceivedMoney(payment.Status) {
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
	if bookingID == uuid.Nil {
		return 0
	}
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
func bookingRefundStatusAfterRefund(manualOwedCentavos int) bookingstore.RefundStatus {
	if manualOwedCentavos > 0 {
		return bookingstore.RefundStatusPartial
	}
	return bookingstore.RefundStatusFull
}

// markManualBalanceOwed writes refund_status 'partial' on a booking whose only money
// is a cash or transfer balance. The owner's "mark manual refund" action is offered
// on 'partial' alone, and the mixed MercadoPago-plus-cash path reaches that status
// through RecordRefundSuccess. A booking with no MercadoPago row never reaches that
// writer, so it stayed 'none' and the owner had nothing to act on.
//
// It is called only from autoRefundIfPaid's cash-only exit, which runs only once the
// cancellation policy has said the client is owed a refund, and when no automatic
// row was claimed, so nothing else writes the booking's refund status in this call.
// A failed write is alerted, and the caller's booking keeps the status the row holds.
//
// Only refund_status is written (MarkManualBalanceOwed), never the whole row: the
// booking here is the snapshot read before the refund ran, and writing it back
// would revert an owner edit that landed in between.
func (s *Service) markManualBalanceOwed(ctx context.Context, booking *bookingstore.Booking) {
	if booking.RefundStatus != bookingstore.RefundStatusNone {
		return
	}
	// Detached from the caller: a client closing the tab must not leave the owner
	// without the status that lets them act on the money.
	if err := s.bookings.MarkManualBalanceOwed(context.WithoutCancel(ctx), booking); err != nil {
		s.logger.Error("auto-refund: could not record the cash balance as owed by hand",
			"error", err, "booking_id", booking.ID)
		sentry.CaptureMessage(fmt.Sprintf("MANUAL BALANCE NOT RECORDED (booking reads 'none', owed by hand): booking_id=%s complex_id=%s", booking.ID, booking.ComplexID))
		return
	}
	booking.RefundStatus = bookingstore.RefundStatusPartial
}

// combineRefundOutcomes folds the answers for a booking's automatic rows into the
// one outcome its caller reports. A booking paid through several MercadoPago rows
// has each of them refunded, and the outcome used to be whichever row came last, so
// a refund still queued behind a claim could be hidden by a later success. The
// worst automatic answer wins, and the sum covers every automatic row.
//
// A row handed to a person (RefundManual) is not automatic: its money is reported
// as ManualAmountCentavos beside the automatic half, which is where NeedsAHuman
// looks. When no automatic row exists, the manual answer stands on its own.
func combineRefundOutcomes(results []paymentstore.RefundOutcome) paymentstore.RefundOutcome {
	var (
		combined      paymentstore.RefundOutcome
		manualTotal   int
		manualReason  string
		automaticSeen bool
	)
	for _, r := range results {
		if r.Result == paymentstore.RefundManual {
			if manualReason == "" {
				manualReason = r.Reason
			}
			manualTotal += r.AmountCentavos
			continue
		}
		if !automaticSeen || refundResultRank(r.Result) > refundResultRank(combined.Result) {
			combined.Result = r.Result
			combined.Reason = r.Reason
		}
		combined.AmountCentavos += r.AmountCentavos
		automaticSeen = true
	}
	if !automaticSeen {
		return paymentstore.RefundOutcome{Result: paymentstore.RefundManual, AmountCentavos: manualTotal, Reason: manualReason}
	}
	combined.ManualAmountCentavos = manualTotal
	return combined
}

// refundResultRank orders automatic answers from best to worst, so the one a
// multi-row refund reports is the one that most needs attention.
func refundResultRank(result paymentstore.RefundResult) int {
	switch result {
	case paymentstore.RefundQueued:
		return 2
	case paymentstore.RefundIssued:
		return 1
	case paymentstore.RefundNone, paymentstore.RefundNotEligible, paymentstore.RefundAlreadyIssued, paymentstore.RefundManual:
		return 0
	}
	return 0
}

// recordedRefundedCentavos reads how much the claim's own payment row already holds
// as refunded. That is the baseline MercadoPago's running total is measured from.
func (s *Service) recordedRefundedCentavos(ctx context.Context, claim paymentstore.RefundClaim) (int, error) {
	var payments []*paymentstore.Payment
	var err error
	if claim.RegistrationID != nil {
		payments, err = s.registrationLedger.ListByRegistrationID(ctx, *claim.RegistrationID)
	} else {
		payments, err = s.payments.ListByBookingID(ctx, claim.BookingID)
	}
	if err != nil {
		return 0, err
	}
	for _, payment := range payments {
		if payment.ID == claim.PaymentID {
			return payment.RefundAmount, nil
		}
	}
	return 0, data.ErrRecordNotFound
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

	exhausted, err := s.refunds.RecordRefundFailure(ctx, claim, cause.Error())
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
		s.leaveCashOwedAfterExhaustedRefund(ctx, claim)
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
			RegistrationID: fr.RegistrationID,
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
			s.recordClaim(claim, "refund_retry",
				claimEvent(claim, outcome.AmountCentavos, outcome.Result, outcome.Reason))
			continue
		}

		manualOwed := s.manualOwedForBooking(ctx, fr.BookingID)
		refundTotal, err := s.refunds.RecordRefundSuccess(ctx, settled, manualOwed)
		if err != nil {
			s.logger.Error("refund-retry: refund issued but not recorded, left queued for retry",
				"error", err, "id", fr.ID, "payment_id", fr.PaymentID)
			sentry.CaptureMessage(fmt.Sprintf("refund-retry RECORD FAILED (money refunded, queued for retry): booking_id=%s mp_payment_id=%s attempt_id=%s", fr.BookingID, fr.MPPaymentID, fr.ID))
			s.recordClaim(claim, "refund_retry", claimEvent(claim, settled.RefundCentavos,
				paymentstore.RefundQueued, "the refund was issued but could not be recorded, and stays queued"))
			continue
		}

		s.recordClaim(claim, "refund_retry",
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
// cannot disagree with the row. The cause the claim stored on the attempt gives
// the same reason, and the same owner notice, the immediate path gives; an
// attempt with none (a cancellation's refund, or a row older than the column)
// gets the plain notice.
func (s *Service) notifyRetriedRefund(ctx context.Context, fr *paymentstore.FailedRefund, refundTotal int) {
	if fr.RegistrationID != nil {
		s.notifyRegistrationRefund(ctx, *fr.RegistrationID, fr.MPPaymentID, refundTotal)
		return
	}
	booking, bErr := s.bookings.GetByID(ctx, fr.BookingID)
	if bErr != nil {
		s.logger.Error("refund-retry: failed to fetch booking", "error", bErr, "booking_id", fr.BookingID)
		return
	}
	s.announceAutomaticRefund(ctx, booking, fr.MPPaymentID, refundTotal, fr.Cause)
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
// and AutoRefundIfPaid's ClaimRefund, or a payment on a cancelled booking whose
// claim failed for a moment (refundCancelledBookingPayment leaves the marker
// standing for exactly this) — claims each one exclusively, and
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

		outcome := s.refundCancelledBooking(ctx, b, true)
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

// refundedTotalOf is MercadoPago's running total refunded on a payment, in
// centavos: transaction_amount_refunded counts every refund made so far, not
// the one this event is about.
func refundedTotalOf(mpPayment *mp.Payment) int {
	return int(math.Round(mpPayment.TransactionAmountRefunded * 100))
}

// recordRefundedTotal writes MercadoPago's running refunded total onto the row
// and returns what this event moved: the difference from what the row already
// held. refund_amount is capped at the row's own amount + service_fee, because
// the payments_refund_within_amount_paid CHECK refuses anything larger and the
// write would fail outright. Booking and registration payments share it so an
// instalment is computed one way only.
func recordRefundedTotal(payment *paymentstore.Payment, refundedTotal int) (movedNow int) {
	movedNow = max(refundedTotal-payment.RefundAmount, 0)
	payment.RefundAmount = min(refundedTotal, payment.Amount+payment.ServiceFee)
	return movedNow
}

// announcedRefund is the figure a person is told came back: what moved on this
// event, so a second partial refund of 300 after one of 500 reads "$300", not
// the running total of 800. An event that moved nothing new (a chargeback with
// no figure) falls back to the amount the row now holds.
func announcedRefund(movedNow int, payment *paymentstore.Payment) int {
	if movedNow > 0 {
		return movedNow
	}
	return payment.RefundAmount
}
