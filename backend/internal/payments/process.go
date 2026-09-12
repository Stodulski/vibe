package payments

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/booklink"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// processApprovedPayment confirms the booking an approved payment paid for, and
// reports whether the webhook event behind it should be tried again.
//
// A returned error is always something a retry can fix: the database refusing a
// write, or the complex not loading. Every decision this function makes about the
// payment itself — already settled, wrong amount, wrong collector — returns nil,
// because it is an answer rather than a failure and repeating it would only spend
// the event's retry budget.
//
// It is one cohesive payment-domain flow — idempotency check, persist the
// payment, confirm the booking, notify — and splitting it would relocate
// sequential steps into helpers without reducing complexity, at the cost of
// disturbing this domain's tested control flow.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) processApprovedPayment(ctx context.Context, booking *bookingstore.Booking, mpPayment *mp.Payment, mpPaymentID string) error {
	// Skip if booking is already confirmed, completed, or no_show (duplicate/late webhook).
	if booking.Status == "confirmed" || booking.Status == "completed" || booking.Status == "no_show" {
		h.logger.Info("mp webhook: booking already confirmed/completed, skipping",
			"mp_payment_id", mpPaymentID,
			"booking_id", booking.ID,
			"booking_status", booking.Status,
		)
		return nil
	}

	// Verify the money actually landed in this complex's MercadoPago account.
	//
	// Without this, any legitimate seller on the marketplace can create a preference with
	// their own token, set external_reference to a victim complex's booking id and the
	// correct amount, pay themselves, and have this webhook confirm the victim's booking —
	// or, for a booking already cancelled, attach a fictitious payment row and have the
	// refund path claim and pay out against it, with no automatic recovery once the retry
	// budget exhausts (payment-collector-verification spec).
	// A refusal here is final, but a complex that would not load is not: that is
	// the database being unavailable, and dropping the payment for it is the exact
	// failure this event's retry budget exists to prevent.
	//
	// This runs ahead of every branch below that writes a data.Payment row or moves
	// a booking's payment_status — including the already-cancelled branch immediately
	// below — so no branch of this function can ever act on a payment whose collector
	// is unproven.
	matches, err := h.collectorMatchesComplex(ctx, booking, mpPayment, mpPaymentID)
	if err != nil {
		return err
	}
	if !matches {
		return nil
	}

	// If the booking was already cancelled (e.g. cron expired it, or owner cancelled it),
	// issue an automatic refund instead of re-confirming.
	if booking.Status == "cancelled" {
		h.logger.Info("mp webhook: booking already cancelled, issuing automatic refund",
			"mp_payment_id", mpPaymentID,
			"booking_id", booking.ID,
		)

		// The money arrived for a booking that is already cancelled, so it has to go
		// straight back. That runs through the same claim → call → record path as
		// every other refund, which means the payment row has to exist and be
		// committed first: a claim is a row-level reservation, and there is nothing
		// to reserve until the payment is recorded.
		//
		// If that write fails the provider is deliberately never called. Money must
		// not move on behalf of a refund nothing durable knows about — which is the
		// exact failure this whole flow was rewritten to remove.
		return h.refundCancelledBookingPayment(ctx, booking, mpPayment, mpPaymentID)
	}

	// Verify amount: MP sends pesos (float), our DB stores centavos (int).
	// Use math.Round to avoid float truncation issues (e.g. 25.99*100 = 2598.99... → 2599).
	//
	// This check intentionally does NOT run for the already-cancelled branch above,
	// whose write already happened by this point. recordPaymentOwedARefund
	// deliberately builds that branch's payment from
	// MercadoPago's own TransactionAmount rather than this recomputed expected
	// amount: a deposit that changed since checkout would fail this comparison and
	// refuse a legitimate refund. Do not hoist this check alongside the collector
	// check above — only the collector check applies to every branch.
	actualCentavos := int(math.Round(mpPayment.TransactionAmount * 100))

	// The total paid includes deposit + service fee (7%, min 1000 ARS) paid by client.
	serviceFee := pricing.ServiceFee(booking.DepositAmount)
	expectedCentavos := booking.DepositAmount + serviceFee

	if actualCentavos != expectedCentavos {
		h.logger.Error("mp webhook: FRAUD ALERT - amount mismatch",
			"mp_payment_id", mpPaymentID,
			"expected_centavos", expectedCentavos,
			"actual_centavos", actualCentavos,
			"booking_id", booking.ID,
		)
		sentry.CaptureMessage(fmt.Sprintf("FRAUD ALERT: amount mismatch mp_payment_id=%s booking_id=%s expected=%d actual=%d", mpPaymentID, booking.ID, expectedCentavos, actualCentavos))
		return nil
	}

	// Re-fetch booking to catch concurrent cancellation (e.g. cron cancelled it while we validated the payment).
	booking, err = h.bookings.GetByID(ctx, booking.ID)
	if err != nil {
		return fmt.Errorf("re-fetch booking %s: %w", booking.ID, err)
	}

	// Re-validate price after re-fetch to detect concurrent price changes.
	revalidatedServiceFee := pricing.ServiceFee(booking.DepositAmount)
	revalidatedExpected := booking.DepositAmount + revalidatedServiceFee
	if revalidatedExpected != expectedCentavos {
		h.logger.Warn("mp webhook: price changed between validation and confirmation",
			"mp_payment_id", mpPaymentID,
			"booking_id", booking.ID,
			"original_expected", expectedCentavos,
			"new_expected", revalidatedExpected,
		)
		sentry.CaptureMessage(fmt.Sprintf("PRICE CHANGED during webhook: mp_payment_id=%s booking_id=%s original=%d new=%d", mpPaymentID, booking.ID, expectedCentavos, revalidatedExpected))
	}

	if booking.Status == "cancelled" {
		h.logger.Info("mp webhook: booking was cancelled concurrently, issuing auto-refund",
			"mp_payment_id", mpPaymentID,
			"booking_id", booking.ID,
		)
		// Call the cancelled-booking auto-refund path directly rather than
		// recursing back into processApprovedPayment: the collector check above
		// already ran for this booking on this call (its identity is unchanged by
		// the re-fetch), and recursing would re-enter the top of this function and
		// run that check a second time for no reason, plus risk a duplicate FRAUD
		// ALERT log line.
		return h.refundCancelledBookingPayment(ctx, booking, mpPayment, mpPaymentID)
	}
	if booking.Status == "confirmed" || booking.Status == "completed" {
		h.logger.Info("mp webhook: booking was confirmed/completed concurrently, skipping",
			"mp_payment_id", mpPaymentID,
			"booking_id", booking.ID,
		)
		return nil
	}

	// Look up existing payment created during public booking flow.
	existingPayment, err := h.payments.GetByBookingID(ctx, booking.ID)
	if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		return fmt.Errorf("look up the existing payment for booking %s: %w", booking.ID, err)
	}

	booking.Status = "confirmed"
	booking.CollectionStatus = bookingstore.CollectionStatusDepositPaid

	// settled is whichever row now holds this money, so the audit entry below
	// can name the payment it confirmed without either branch repeating it.
	var settled *paymentstore.Payment

	if existingPayment != nil {
		// The public booking flow already inserted this row at checkout; the
		// webhook only fills in the MercadoPago payment id and settles the status.
		existingPayment.Status = "deposit_paid"
		existingPayment.MPPaymentID = &mpPaymentID
		existingPayment.StatusDetail = statusDetailPtr(mpPayment.StatusDetail)

		if err := h.payments.ConfirmWebhookPayment(ctx, existingPayment, booking); err != nil {
			// H-23: ErrBookingCancelled refunds for the same reason
			// ErrSlotUnavailable does. The booking this money was for no longer
			// exists — cancelled when its payment expired and its hours went
			// back on sale — and the client paid anyway, which the cancel path
			// leaves possible on purpose. Requeuing instead retries a write
			// that can never succeed and never sends the money back.
			if errors.Is(err, bookingstore.ErrSlotUnavailable) || errors.Is(err, bookingstore.ErrBookingCancelled) {
				return h.refundBookingWhoseSlotIsGone(ctx, booking, mpPayment, mpPaymentID)
			}
			// The money is captured and the booking is still pending. Left as an
			// error, the event is requeued and this write is attempted again.
			return fmt.Errorf("confirm payment %s: %w", mpPaymentID, err)
		}
		settled = existingPayment
	} else {
		// Fallback: insert new payment (e.g. if booking was created without initial payment record).
		payment := &paymentstore.Payment{
			BookingID:    booking.ID,
			ComplexID:    booking.ComplexID,
			Amount:       booking.DepositAmount,
			ServiceFee:   serviceFee,
			Method:       "mercadopago",
			Status:       "deposit_paid",
			MPPaymentID:  &mpPaymentID,
			StatusDetail: statusDetailPtr(mpPayment.StatusDetail),
		}

		if err := h.payments.InsertAndConfirmBooking(ctx, payment, booking); err != nil {
			// H-23, same as above: a cancelled booking's late payment is
			// refunded, not retried.
			if errors.Is(err, bookingstore.ErrSlotUnavailable) || errors.Is(err, bookingstore.ErrBookingCancelled) {
				return h.refundBookingWhoseSlotIsGone(ctx, booking, mpPayment, mpPaymentID)
			}
			// Same as above: captured money against an unconfirmed booking is
			// exactly what has to survive a transient database failure.
			return fmt.Errorf("record payment %s and confirm its booking: %w", mpPaymentID, err)
		}
		settled = payment
	}

	// GetByID's error path already returns a nil client, so a lookup failure
	// here degrades to skipping the confirmation notification below rather
	// than failing the webhook — the payment is already recorded.
	client, _ := h.clients.GetByID(ctx, booking.ClientID)

	h.logger.Info("mp webhook: payment approved and booking confirmed",
		"mp_payment_id", mpPaymentID,
		"booking_id", booking.ID,
		"amount", booking.DepositAmount,
	)

	h.realtime.PublishBookingChanged(booking.ComplexID)

	// Money arriving is the first half of the money path, and it had no entry
	// either. It is recorded after the write commits, so a confirmation the
	// database refused — which returns above and is retried — is never
	// recorded as one that happened.
	h.record(booking.ComplexID, booking.ID, "payment_confirmed", moneyEvent{
		Actor:          actorProvider,
		PaymentID:      &settled.ID,
		MPPaymentID:    mpPaymentID,
		AmountCentavos: settled.Amount + settled.ServiceFee,
		Result:         "confirmed",
	})

	// Send booking confirmation notification (email + optionally WhatsApp).
	if client != nil {
		complex, cerr := h.complexes.GetByID(ctx, booking.ComplexID)
		court, courterr := h.courts.GetByID(ctx, booking.CourtID)
		if cerr == nil && courterr == nil {
			// Mint before building any link. InsertSafe already minted a token
			// for this booking at checkout, but that mint happened in a
			// different request and its plaintext cannot be read back — see
			// design.md's "two live tokens" consequence — so this path mints
			// its own rather than reusing it. A failed mint must not send a
			// confirmation email: returning an error here (rather than
			// logging and continuing) makes the webhook handler requeue the
			// event instead. The checkout token minted at InsertSafe is
			// deliberately not revoked here: MercadoPago redirects the
			// browser to the success back_url at roughly the moment this
			// webhook fires, and revoking it would break that redirect.
			confirmationToken, mintErr := h.linkTokens.Mint(ctx, booking.ID, booking.EndsAt.Add(h.cfg.LinkTokenBuffer))
			if mintErr != nil {
				return fmt.Errorf("mint booking link token for confirmation email %s: %w", booking.ID, mintErr)
			}
			cancelURL := booklink.Cancel(h.cfg.FrontendURL, complex.Slug, confirmationToken)
			cancelPath := booklink.CancelPath(complex.Slug, confirmationToken)
			mapsQuery := booklink.MapsQuery(complex.Name, complex.Address, complex.City, complex.Latitude, complex.Longitude)
			mapsURL := booklink.MapsURL(complex.Name, complex.Address, complex.City, complex.Latitude, complex.Longitude)
			address := booklink.Address(complex.Address, complex.City)
			clientEmail := ""
			if client.Email != nil {
				clientEmail = *client.Email
			}
			confirmDepositAmount, confirmBalanceAmount := notifications.PaymentAmounts(booking.Price, booking.DepositAmount, booking.CollectionStatus)
			h.notify.BookingConfirmed(notifications.BookingConfirmation{
				// The online route: this is a sale the owner did not enter, so
				// the owner's "Nueva reserva" email goes out.
				Source:        notifications.SourceOnlineCheckout,
				Email:         clientEmail,
				Phone:         client.Phone,
				ComplexName:   complex.Name,
				CourtName:     court.Name,
				ClientName:    client.FirstName + " " + client.LastName,
				Date:          booking.Date.Format("02/01"),
				StartTime:     timezone.HoursLabel(booking.StartsAt, booking.EndsAt),
				CancelURL:     cancelURL,
				CancelPath:    cancelPath,
				MapsQuery:     mapsQuery,
				Address:       address,
				MapsURL:       mapsURL,
				DepositAmount: confirmDepositAmount,
				BalanceAmount: confirmBalanceAmount,
				CancellationLine: notifications.CancellationLine(complex.CancellationHours, h.cfg.CancellationGracePeriod,
					pricing.WithinStandardWindow(booking, complex.CancellationHours)),
				OwnerID: complex.OwnerID.String(),
			})
		}
	}
	return nil
}

// refundBookingWhoseSlotIsGone cancels a booking that lost its slot while its
// payment was in flight, and sends the money back.
//
// The store refuses a confirmation only when another live booking genuinely
// covers these hours (see PaymentModel.guardSlotStillFree), so reaching here
// means the court is sold to somebody else and this booking can never be
// honoured. That is a decision rather than a failure: nil is returned so the
// webhook event is not retried, because retrying cannot change who owns the slot.
//
// It is reported to Sentry rather than only logged. A stale pending booking being
// overtaken is expected; a client paying for hours that were given away is a real
// conflict, and somebody has to know it happened.
func (h *Handler) refundBookingWhoseSlotIsGone(ctx context.Context, booking *bookingstore.Booking, mpPayment *mp.Payment, mpPaymentID string) error {
	h.logger.Error("mp webhook: the slot was taken while the payment was in flight, refusing to confirm and refunding",
		"mp_payment_id", mpPaymentID,
		"booking_id", booking.ID,
		"court_id", booking.CourtID,
		"date", booking.Date.Format("2006-01-02"),
		"start_time", booking.StartTime,
		"starts_at", booking.StartsAt.Format(time.RFC3339),
		"ends_at", booking.EndsAt.Format(time.RFC3339),
	)
	sentry.CaptureMessage(fmt.Sprintf("SLOT TAKEN BEFORE CONFIRMATION (booking cancelled, payment refunded): mp_payment_id=%s booking_id=%s court_id=%s date=%s slot=%s/%s",
		mpPaymentID, booking.ID, booking.CourtID, booking.Date.Format("2006-01-02"),
		booking.StartsAt.Format(time.RFC3339), booking.EndsAt.Format(time.RFC3339)))

	// The booking never becomes live, so it is cancelled here before the refund
	// path records its payment. That path writes the booking through
	// InsertAndConfirmBooking, which would otherwise carry the 'confirmed' status
	// this function exists to refuse — and be refused by the same guard, leaving
	// the money captured with no refund ever claimed.
	booking.Status = "cancelled"

	return h.refundCancelledBookingPayment(ctx, booking, mpPayment, mpPaymentID)
}

// refundCancelledBookingPayment records a payment that landed for an
// already-cancelled booking and refunds it.
//
// The booking is left cancelled and carrying a refund-intent marker from the
// moment the payment is recorded (refund-intent-durability spec), so the
// window between that commit and ClaimRefund's is covered by the same sweep
// that covers the two cancel paths. ClaimRefund clears the marker inside its
// own transaction and moves the payment to 'refund_pending'; every exit that
// commits no claim clears it here, exactly as AutoRefundIfPaid does.
//
// Two things it now refuses to invent. It takes the MercadoPago payment rather
// than only its id, because the amount to record is what the client was actually
// charged: deriving it from booking.DepositAmount meant a deposit that had moved
// since checkout produced a payment row — and therefore a refund, since
// ClaimRefund computes the refundable balance from that row — for money nobody
// paid. And it reuses the checkout payment row when there is one, because
// inserting a second left the first orphaned with the preference id on it while
// GetByBookingID's ordering handed everything after it the new row.
func (h *Handler) refundCancelledBookingPayment(ctx context.Context, booking *bookingstore.Booking, mpPayment *mp.Payment, mpPaymentID string) error {
	payment, err := h.recordPaymentOwedARefund(ctx, booking, mpPayment, mpPaymentID)
	if err != nil {
		h.logger.Error("mp webhook: failed to record the payment owed a refund, provider not called",
			"error", err,
			"mp_payment_id", mpPaymentID,
			"booking_id", booking.ID,
		)
		sentry.CaptureMessage(fmt.Sprintf("AUTO-REFUND NOT CLAIMED (payment record failed, money still held): mp_payment_id=%s booking_id=%s", mpPaymentID, booking.ID))
		// Nothing was written and no money moved, so the retry replays this whole
		// path cleanly.
		return fmt.Errorf("record the payment owed a refund for booking %s: %w", booking.ID, err)
	}

	// From here the refund-intent marker is committed on the booking. Whichever
	// way this returns without a committed claim, it is cleared — and the window
	// in between is the sweep's, not a hole.
	committed := false
	defer h.clearRefundIntentUnlessClaimed(ctx, booking.ID, &committed)

	claim, err := h.payments.ClaimRefund(ctx, payment.ID)
	if err != nil {
		h.logger.Error("mp webhook: failed to claim the refund for a cancelled booking",
			"error", err,
			"mp_payment_id", mpPaymentID,
			"booking_id", booking.ID,
		)
		sentry.CaptureMessage(fmt.Sprintf("AUTO-REFUND NOT CLAIMED: mp_payment_id=%s booking_id=%s error=%v", mpPaymentID, booking.ID, err))
		if errors.Is(err, paymentstore.ErrAlreadyRefunded) || errors.Is(err, paymentstore.ErrRefundInFlight) {
			// Another claim owns this refund, or it is already back. Not ours to retry.
			return nil
		}
		return fmt.Errorf("claim the refund for cancelled booking %s: %w", booking.ID, err)
	}
	// The claim committed and already cleared the marker in its own transaction.
	committed = true

	// The claim already committed a queued attempt before either the seller
	// token or the provider was asked for, so the refund queue owns this from
	// here regardless of which one refuses. Retrying the webhook event as well
	// would give the same money two owners.
	settled, outcome := h.issueRefund(ctx, *claim)
	if outcome != nil {
		// Queued for the retry job, or owed by hand. Either way it is the end
		// of this path's involvement, and the entry says which — the exhausted
		// and credential-refused arms of this are exactly what somebody comes
		// looking for afterwards.
		h.record(booking.ComplexID, booking.ID, "refund",
			claimEvent(*claim, outcome.AmountCentavos, outcome.Result, outcome.Reason))
		return nil
	}

	manualOwed := h.manualOwedForBooking(ctx, booking.ID)
	if _, err := h.payments.RecordRefundSuccess(ctx, settled, manualOwed); err != nil {
		// See AutoRefundIfPaid: the attempt stays queued and the retry job replays
		// the call, which MercadoPago deduplicates on its idempotency key.
		h.logger.Error("mp webhook: auto-refund issued but not recorded, left queued for retry",
			"error", err,
			"mp_payment_id", mpPaymentID,
			"attempt_id", claim.AttemptID,
		)
		sentry.CaptureMessage(fmt.Sprintf("AUTO-REFUND RECORD FAILED (money refunded, queued for retry): mp_payment_id=%s booking_id=%s attempt_id=%s", mpPaymentID, booking.ID, claim.AttemptID))
		h.record(booking.ComplexID, booking.ID, "refund", claimEvent(*claim, settled.RefundCentavos,
			paymentstore.RefundQueued, "the refund was issued but could not be recorded, and stays queued"))
		return nil
	}

	h.logger.Info("mp webhook: auto-refund issued for cancelled booking",
		"mp_payment_id", mpPaymentID,
		"booking_id", booking.ID,
		"refund_amount", centavosToPesos(settled.RefundCentavos),
	)
	h.record(booking.ComplexID, booking.ID, "refund",
		claimEvent(*claim, settled.RefundCentavos, paymentstore.RefundIssued, ""))

	// A shortfall gets its own second entry rather than replacing the one
	// above: the part that moved is genuinely refunded and the client has been
	// told so, and the remainder is owed with nothing queued behind it.
	// Collapsing the two would lose whichever half the reader needed.
	h.reportShortfall("mp webhook", "refund", booking.ComplexID, booking.ID, *claim, settled)
	return nil
}

// recordPaymentOwedARefund writes the payment behind a refund that is about to be
// claimed, and returns the row the claim will reserve.
//
// The checkout row created when the booking was made is preferred: it already
// carries the preference id and the amounts the client agreed to, and one booking
// is meant to have one payment.
//
// The write commits 'deposit_paid' plus the refund-intent marker, and this is
// the whole of the difference from what it used to do. It used to commit
// payment_status='refund_pending' here, before any claim existed — the exact
// value refundable() (refund.go) reads as "a claim is already committed against
// this booking's payment, so the money is on its way". Nothing was on its way:
// ClaimRefund had not run. A crash or a database refusal between the two
// commits therefore left a cancelled, paid booking that every later refund
// path declined as already in flight, with no failed_refunds row for the retry
// job, no marker for the sweep, and no alert — and the webhook event's own
// retry could not repair it either, because on redelivery the payment now
// carries the MercadoPago id and processPaymentWebhook short-circuits on
// "payment already processed". The money stayed, permanently and silently.
//
// The marker is the record refund-intent-durability requires and forbids
// expressing as 'refund_pending' for precisely this reason. The two cancel
// paths already write it the same way (internal/bookings), so the shape
// SweepOrphanedRefundIntents recovers is identical whichever path produced it.
func (h *Handler) recordPaymentOwedARefund(ctx context.Context, booking *bookingstore.Booking, mpPayment *mp.Payment, mpPaymentID string) (*paymentstore.Payment, error) {
	existing, err := h.payments.GetByBookingID(ctx, booking.ID)
	if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		return nil, fmt.Errorf("look up the payment for booking %s: %w", booking.ID, err)
	}

	// The client's money is here and the booking is cancelled, which is what
	// collection_status 'deposit_paid' says and what claimable() needs to read
	// to reserve it. The refund axis is left alone: recordPaymentOwedARefund
	// records that money arrived, and the refund it owes is carried by
	// refund_intent_at below, not by pre-declaring a refund that has not
	// started.
	booking.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	now := time.Now()
	booking.RefundIntentAt = &now

	if existing != nil {
		existing.Status = "deposit_paid"
		existing.MPPaymentID = &mpPaymentID
		if err := h.payments.ConfirmWebhookPayment(ctx, existing, booking); err != nil {
			return nil, err
		}
		return existing, nil
	}

	// No checkout row: split what MercadoPago says the client paid rather than
	// what the booking currently says it should have cost.
	paid := int(math.Round(mpPayment.TransactionAmount * 100))
	serviceFee := pricing.ServiceFee(booking.DepositAmount)
	amount := paid - serviceFee
	if paid <= 0 || amount < 0 {
		// MercadoPago sent no usable figure. Fall back to what was asked for,
		// which is the only other number in play.
		amount, serviceFee = booking.DepositAmount, pricing.ServiceFee(booking.DepositAmount)
	}

	payment := &paymentstore.Payment{
		BookingID:   booking.ID,
		ComplexID:   booking.ComplexID,
		Amount:      amount,
		ServiceFee:  serviceFee,
		Method:      "mercadopago",
		Status:      "deposit_paid",
		MPPaymentID: &mpPaymentID,
	}
	if err := h.payments.InsertAndConfirmBooking(ctx, payment, booking); err != nil {
		return nil, err
	}
	return payment, nil
}

// collectorMatchesComplex reports whether the MercadoPago payment was collected by the
// seller account belonging to the booking's complex.
//
// It fails closed: anything that leaves the collector unproven — the complex not
// loading, the complex having no linked MercadoPago account, or MercadoPago not sending
// a collector_id — refuses the confirmation rather than trusting external_reference.
//
// The two refusals are reported apart. A missing or mismatched identity is a
// verdict and false forever; a complex that would not load is the database being
// unavailable, which is returned as an error so the recorded event is retried
// rather than a genuine payment being thrown away for an outage.
func (h *Handler) collectorMatchesComplex(ctx context.Context, booking *bookingstore.Booking, mpPayment *mp.Payment, mpPaymentID string) (bool, error) {
	complex, err := h.complexes.GetByID(ctx, booking.ComplexID)
	if err != nil {
		h.logger.Error("mp webhook: cannot verify collector - failed to fetch complex",
			"error", err,
			"mp_payment_id", mpPaymentID,
			"booking_id", booking.ID,
			"complex_id", booking.ComplexID,
		)
		sentry.CaptureMessage(fmt.Sprintf("COLLECTOR UNVERIFIED: complex lookup failed mp_payment_id=%s booking_id=%s complex_id=%s", mpPaymentID, booking.ID, booking.ComplexID))
		return false, fmt.Errorf("verify the collector of payment %s: %w", mpPaymentID, err)
	}

	if complex.MPUserID == nil || *complex.MPUserID == "" || mpPayment.CollectorID == 0 {
		h.logger.Error("mp webhook: cannot verify collector - missing MercadoPago identity",
			"mp_payment_id", mpPaymentID,
			"booking_id", booking.ID,
			"complex_id", booking.ComplexID,
			"complex_has_mp_user_id", complex.MPUserID != nil && *complex.MPUserID != "",
			"collector_id", mpPayment.CollectorID,
		)
		sentry.CaptureMessage(fmt.Sprintf("COLLECTOR UNVERIFIED: missing MercadoPago identity mp_payment_id=%s booking_id=%s complex_id=%s collector_id=%d", mpPaymentID, booking.ID, booking.ComplexID, mpPayment.CollectorID))
		return false, nil
	}

	collectorID := strconv.FormatInt(mpPayment.CollectorID, 10)
	if collectorID != *complex.MPUserID {
		h.logger.Error("mp webhook: FRAUD ALERT - collector mismatch",
			"mp_payment_id", mpPaymentID,
			"booking_id", booking.ID,
			"complex_id", booking.ComplexID,
			"expected_collector_id", *complex.MPUserID,
			"actual_collector_id", collectorID,
		)
		sentry.CaptureMessage(fmt.Sprintf("FRAUD ALERT: collector mismatch mp_payment_id=%s booking_id=%s complex_id=%s expected=%s actual=%s", mpPaymentID, booking.ID, booking.ComplexID, *complex.MPUserID, collectorID))
		return false, nil
	}

	return true, nil
}

// processRejectedPayment cancels the booking a rejected payment failed to pay
// for, and reports whether the webhook event behind it should be tried again.
func (h *Handler) processRejectedPayment(ctx context.Context, booking *bookingstore.Booking, mpPayment *mp.Payment, mpPaymentID string) error {
	// Only cancel if the booking is still pending. If it was already confirmed
	// (e.g. a different payment attempt succeeded), do NOT cancel it.
	if booking.Status != "pending" {
		h.logger.Info("mp webhook: payment rejected but booking is not pending, skipping cancellation",
			"mp_payment_id", mpPaymentID,
			"booking_id", booking.ID,
			"booking_status", booking.Status,
		)
		return nil
	}

	booking.Status = "cancelled"
	if err := h.bookings.Update(ctx, booking); err != nil {
		return fmt.Errorf("cancel booking %s after a rejected payment: %w", booking.ID, err)
	}

	h.recordRejectedPaymentDetail(ctx, booking.ID, mpPayment, mpPaymentID)

	h.realtime.PublishBookingChanged(booking.ComplexID)

	h.logger.Info("mp webhook: payment rejected/cancelled, booking cancelled",
		"mp_payment_id", mpPaymentID,
		"booking_id", booking.ID,
		"status", mpPayment.Status,
		"status_detail", mpPayment.StatusDetail,
	)
	return nil
}

// recordRejectedPaymentDetail persists MercadoPago's reason for the rejection
// onto the payment row the public checkout flow inserted at 'unpaid', so the
// specific decline reason (e.g. "cc_rejected_insufficient_amount") survives
// past this log line. A read/write failure here is logged and swallowed: the
// booking is already cancelled by the time this runs, and losing the detail
// column is not worth requeuing the whole webhook event for.
func (h *Handler) recordRejectedPaymentDetail(ctx context.Context, bookingID uuid.UUID, mpPayment *mp.Payment, mpPaymentID string) {
	existingPayment, err := h.payments.GetByBookingID(ctx, bookingID)
	if err != nil {
		if !errors.Is(err, data.ErrRecordNotFound) {
			h.logger.Error("mp webhook: failed to look up the payment row for a rejected payment",
				"error", err, "booking_id", bookingID, "mp_payment_id", mpPaymentID)
		}
		return
	}

	// The payments.status enum has no "rejected" value (unpaid/deposit_paid/
	// fully_paid/refunded/refund_pending/partial_refund — see
	// internal/db/models.go), so the row's status is left as-is; only the MP id
	// and status_detail are filled in. The payment_status split took bookings off this
	// enum; payments.status is what it still describes, and giving that column
	// its own vocabulary is the other half of F11 and a separate change.
	existingPayment.MPPaymentID = &mpPaymentID
	existingPayment.StatusDetail = statusDetailPtr(mpPayment.StatusDetail)
	if err := h.payments.Update(ctx, existingPayment); err != nil {
		h.logger.Error("mp webhook: failed to record the rejection detail on the payment row",
			"error", err, "booking_id", bookingID, "mp_payment_id", mpPaymentID)
	}
}

// statusDetailPtr turns MercadoPago's status_detail into the nilable pointer
// the payments.status_detail column expects — an empty string means the
// field was absent from the payload, not that MercadoPago sent an empty
// detail, so it is stored as NULL rather than "".
func statusDetailPtr(detail string) *string {
	if detail == "" {
		return nil
	}
	return &detail
}
