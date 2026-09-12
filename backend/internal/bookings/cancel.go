// cancel.go — what a cancellation does about the client's money, shared by the
// staff and public cancellation paths.
//
// The two paths differ on policy and that difference is deliberate: staff always
// refund, the public path only refunds inside the complex's cancellation window.
// Everything that is *not* policy lives here, because that is where the last
// round of defects came from — a window check that also swallowed the
// preference-expiry step, and two endpoints that each decided for themselves what
// to tell the client about their deposit.
package bookings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/mpcred"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// Refund methods reported by cancel-info. They answer "how would the money come
// back", which is a different question from "is money owed" and the one the
// endpoint used not to ask at all.
const (
	// refundByMercadoPago: the deposit returns automatically to the card or
	// account it was paid from.
	refundByMercadoPago = "mercadopago"
	// refundByHand: money was paid, and only a person at the complex can return
	// it — cash, a transfer, or a MercadoPago payment carrying no payment id.
	refundByHand = "manual"
	// refundNotApplicable: nothing was paid, or it is already back.
	refundNotApplicable = "none"
)

// refundMessage is the value the client is shown about their money, one per
// outcome, with the amount written into it when there is one.
//
// It is lowercase and starts mid-sentence: every consumer substitutes it after
// its own fixed lead-in ("Devolución: " in the WhatsApp template and the
// cancellation email, `"message"` in the API response beside the
// machine-readable status a frontend that wants its own wording reads
// instead). Meta's template validator is why the lead-in and the value split
// this way at all — a template body cannot end on a variable with nothing
// fixed after it, so "Devolución: {{5}}" is the shape and this function
// renders only what goes after the colon.
//
// There is one line here for every outcome a cancellation can produce. Six of
// them used to produce nothing: the client was told `{"refunded": false}` and
// left to work the rest out.
//
// It is a function rather than the map it was because the same six sentences
// now also go out as the cancellation email and the cancellation WhatsApp
// message, where there is no envelope field beside them to carry the amount —
// a client reading "en proceso" in WhatsApp has no way to ask how much. One
// renderer, three consumers, so the API response and the two messages cannot
// drift into three different accounts of the same money.
//
// The vocabulary is the product's: "devolución", never "reembolso", which is
// what the not-eligible line used to say.
func refundMessage(result paymentstore.RefundResult, amountCentavos int) string {
	amount := notifications.FormatARS(amountCentavos)
	switch result {
	case paymentstore.RefundNone:
		return "no hay pagos a devolver."
	case paymentstore.RefundNotEligible:
		if amountCentavos > 0 {
			return fmt.Sprintf("fuera de plazo, la seña de %s no se devuelve.", amount)
		}
		return "fuera de plazo, la seña no se devuelve."
	case paymentstore.RefundIssued:
		if amountCentavos > 0 {
			return fmt.Sprintf("%s enviados, se acreditan en los próximos días hábiles.", amount)
		}
		return "enviada, se acredita en los próximos días hábiles."
	case paymentstore.RefundAlreadyIssued:
		if amountCentavos > 0 {
			return fmt.Sprintf("%s, ya enviada.", amount)
		}
		return "ya enviada."
	case paymentstore.RefundQueued:
		if amountCentavos > 0 {
			return fmt.Sprintf("%s en proceso, se notificará al completarse.", amount)
		}
		return "en proceso, se notificará al completarse."
	case paymentstore.RefundManual:
		if amountCentavos > 0 {
			return fmt.Sprintf("%s, a cargo del complejo de forma manual.", amount)
		}
		return "a cargo del complejo de forma manual."
	default:
		return ""
	}
}

// refundNotice is the whole of what a cancelled client is told about their
// money, in one sentence, for the email and the WhatsApp message.
//
// A split outcome — an MercadoPago deposit returned automatically while a cash
// balance is still owed by hand — is two facts, and dropping the second is how
// somebody ends up believing they were made whole. The manual half is appended
// rather than replacing the first, because both happened.
//
// It returns the formatted amount alongside, which is what the email's preview
// line quotes; it is empty when no money is coming back at all.
func refundNotice(outcome paymentstore.RefundOutcome) (line, amount string) {
	line = refundMessage(outcome.Result, outcome.AmountCentavos)
	if outcome.ManualAmountCentavos > 0 {
		manual := refundMessage(paymentstore.RefundManual, outcome.ManualAmountCentavos)
		if line == "" {
			line = manual
		} else {
			line += " " + manual
		}
	}

	switch {
	case outcome.Result == paymentstore.RefundNotEligible:
		// Money exists, but it is staying where it is. Naming an amount in the
		// preview line of an email about not getting it back reads as a
		// promise.
	case outcome.AmountCentavos > 0:
		amount = notifications.FormatARS(outcome.AmountCentavos)
	case outcome.ManualAmountCentavos > 0:
		amount = notifications.FormatARS(outcome.ManualAmountCentavos)
	}
	return line, amount
}

// refundEnvelope renders an outcome for a cancellation response.
func refundEnvelope(outcome paymentstore.RefundOutcome) httpx.Envelope {
	e := httpx.Envelope{
		"status":  string(outcome.Result),
		"message": refundMessage(outcome.Result, outcome.AmountCentavos),
	}
	if outcome.AmountCentavos > 0 {
		e["amount"] = outcome.AmountCentavos
	}
	// A booking with more than one payment row can split: an MP deposit
	// refunded automatically alongside a cash balance nothing here can send
	// back. "amount" and "message" above still describe the automatic half —
	// this is the only place the manual half would otherwise go unseen.
	if outcome.ManualAmountCentavos > 0 {
		e["manual_amount"] = outcome.ManualAmountCentavos
		e["manual_message"] = refundMessage(paymentstore.RefundManual, outcome.ManualAmountCentavos)
	}
	return e
}

// refundStatusAfter reports the booking's refund status as it now stands in the
// database, which is not what the in-memory booking holds.
//
// This is the whole of defect 2. The refund path writes the refund status onto a
// row it locks for itself, deliberately, so that a stale struct loaded before
// the provider call cannot overwrite it. The cancel endpoint then read the
// status off that same stale struct and told every successfully refunded client
// that nothing had been refunded.
//
// It answers on the refund axis only. Since the payment_status split a refund no longer
// overwrites what the booking collected, so the collection status on the
// in-memory struct is still correct and is left alone.
func refundStatusAfter(booking *bookingstore.Booking, outcome paymentstore.RefundOutcome) string {
	switch {
	case outcome.ManualAmountCentavos > 0:
		// The automatic half came back — MoneyReturned() would also be true
		// here — but a cash/transfer row is still owed by hand, which is what
		// RecordRefundSuccess/cancelRefundedBooking wrote to the row instead
		// of 'full'. This case must run before MoneyReturned()'s so the
		// response matches what was actually persisted.
		return bookingstore.RefundStatusPartial
	case outcome.MoneyReturned():
		return bookingstore.RefundStatusFull
	case outcome.Result == paymentstore.RefundQueued:
		return bookingstore.RefundStatusPending
	default:
		return booking.RefundStatus
	}
}

// preferenceExpiryBudget bounds the whole of expireCheckoutPreference once it
// is detached from the caller's cancellation.
//
// It is a ceiling, not a delay: the payment read carries internal/data's own
// 3-second query budget and each MercadoPago call carries the mp client's
// 8-second timeout, so 25 seconds is the sum of the read and two attempts with
// room to spare, and a connected client never waits for it. The number has to
// exist at all because context.WithoutCancel strips the parent's DEADLINE along
// with its cancellation — see internal/data/timeout.go's detachedQueryContext,
// where this codebase already paid for learning that.
const preferenceExpiryBudget = 25 * time.Second

// expireCheckoutPreference kills the MercadoPago checkout link an unpaid booking
// still holds.
//
// It is not a refund and it is not subject to the refund window, which is what
// went wrong: in the public path this step sat inside the `if withinRefundWindow`
// branch, so a late cancellation freed the slot and left the payment link live.
// Somebody else booked and paid for those hours, the first client could still pay
// theirs, and the result was a charge that had to be auto-refunded — costing the
// venue the very penalty the window exists to collect.
//
// Three things this function used to get wrong once the branch was hoisted out:
//
//   - The credential read was `if tok, err := ...; err == nil`, which threw the
//     error away. A stored credential that cannot be decrypted then fell through
//     to the empty string, and mp.UpdatePreferenceExpired reads an empty seller
//     token as "use the platform's own token" — so the request went to
//     MercadoPago authenticated as the wrong party, failed, and the only trace
//     was one log line. The link stayed live. A credential we cannot read is
//     refused here, the same way the refund path refuses it (see
//     internal/payments/refund.go's refuseForCredential).
//   - One attempt, no retry. The cron path that expires the same preferences
//     retries once; the request path did not, so a single transient failure was
//     the whole story.
//   - No alert. A live checkout on a cancelled booking is money about to be
//     taken for hours somebody else now owns, and nothing above a log line said so.
//
// The captured messages name the complex and the preference and never the
// booking, because this runs on PublicCancel — one of the three token-authorized
// public routes, whose spec (openspec/specs/booking-link-credential) forbids a
// resolved booking id from reaching Sentry. The logger, which is not Sentry,
// still carries it.
func (h *Handler) expireCheckoutPreference(ctx context.Context, booking *bookingstore.Booking, complex *complexstore.Complex) {
	// Detached from the caller's cancellation for the same reason
	// releaseSlotLocks is: every call site hands this the request's own context,
	// and a client who cancels and closes the tab cancelled it. The read below
	// would then fail before the preference id was even known, and the function
	// returned having done nothing — leaving a payable checkout on a booking that
	// no longer exists, which is the whole thing this function is for.
	ctx, cancelBudget := context.WithTimeout(context.WithoutCancel(ctx), preferenceExpiryBudget)
	defer cancelBudget()

	payment, err := h.payments.GetByBookingID(ctx, booking.ID)
	if err != nil || payment.MPPreferenceID == nil || *payment.MPPreferenceID == "" {
		return
	}
	preferenceID := *payment.MPPreferenceID

	caller, credErr := expiryCaller(complex)
	if credErr != nil {
		// Calling as the platform here would send the request as the wrong
		// party. Refuse, and say so loudly: the link is still live.
		h.logger.Error("cancel: the seller credential could not be read, so the checkout link was left open",
			"error", credErr, "booking_id", booking.ID, "complex_id", complex.ID, "preference_id", preferenceID)
		sentry.CaptureMessage(fmt.Sprintf("PREFERENCE EXPIRATION SKIPPED, CREDENTIAL UNREADABLE (client may still pay): complex_id=%s preference_id=%s",
			complex.ID, preferenceID))
		return
	}

	expErr := h.checkout.UpdatePreferenceExpired(ctx, preferenceID, caller)
	if expErr == nil {
		return
	}
	h.logger.Error("cancel: first attempt to expire the MercadoPago preference failed, retrying",
		"error", expErr, "booking_id", booking.ID, "preference_id", preferenceID)

	if retryErr := h.checkout.UpdatePreferenceExpired(ctx, preferenceID, caller); retryErr != nil {
		h.logger.Error("cancel: retry failed to expire the MercadoPago preference",
			"error", retryErr, "booking_id", booking.ID, "complex_id", complex.ID, "preference_id", preferenceID)
		sentry.CaptureMessage(fmt.Sprintf("PREFERENCE EXPIRATION FAILED (client may still pay): complex_id=%s preference_id=%s",
			complex.ID, preferenceID))
	}
}

// expiryCaller names who the request that closes a checkout link is made as.
//
// A venue with no stored credential leaves the platform as the only party left
// to ask, and asking is better than leaving the link payable: MercadoPago
// rejects a marketplace-authenticated expiry of somebody else's preference, so
// the cost of trying is one failed call. That arm is spelled out here because
// it used to be the silent meaning of an empty token, which is also what a
// credential that would not decrypt produced — and that one must never take
// it. It comes back as an error for the caller to refuse and alert on.
func expiryCaller(complex *complexstore.Complex) (mp.Caller, error) {
	token, err := complex.SellerAccessToken()
	switch {
	case err == nil:
		return mp.AsSeller(token)
	case errors.Is(err, mpcred.ErrMPCredentialUnreadable):
		return mp.Caller{}, err
	default:
		return mp.AsPlatform(), nil
	}
}

// refundMethod reports how a refund for this booking would actually reach the
// client, or that none would.
//
// cancel-info used to answer that question from the cancellation window alone,
// so a booking paid in cash was told it qualified for a refund the refund path
// then declined to make, silently. The window says whether money is owed; only
// the payment says whether anything can send it.
//
// A booking can carry more than one payment row (a deposit paid online and a
// balance ConfirmPayment recorded in cash), and reading only one of them —
// GetByBookingID's single, MercadoPago-preferred row — answered this off
// whichever row happened to win that preference and missed the other
// entirely. Any unrefunded row with no MercadoPago id means a person has to
// act regardless of whatever else is on the booking, so that answer wins
// even when an MP row is also present: refundByHand is always the safe,
// client-facing answer for a mixed booking.
func (h *Handler) refundMethod(ctx context.Context, booking *bookingstore.Booking) string {
	// The refund axis answers first and the collection axis is the fallback,
	// which is the same order the single payment_status enum enforced by having
	// only one value: a row that is mid-refund is not also merely "paid".
	switch {
	case booking.RefundStatus == bookingstore.RefundStatusPending:
		// Already on its way back through MercadoPago.
		return refundByMercadoPago
	case booking.RefundStatus == bookingstore.RefundStatusPartial:
		// The automatic half is already back; what remains is exactly the
		// cash/transfer balance only a person can return.
		return refundByHand
	case booking.RefundStatus == bookingstore.RefundStatusFull:
		// Already refunded.
		return refundNotApplicable
	case booking.CollectionStatus == bookingstore.CollectionStatusUnpaid:
		// Nothing was ever collected.
		return refundNotApplicable
	}
	// Collected and no refund under way: the payment ledger decides.

	payments, err := h.payments.ListByBookingID(ctx, booking.ID)
	if err != nil {
		// The booking reads as paid and its payments cannot be read. Promising an
		// automatic refund here is the failure being fixed; a person decides.
		h.logger.Error("cancel-info: the booking reads as paid but its payments could not be read",
			"error", err, "booking_id", booking.ID)
		return refundByHand
	}
	if len(payments) == 0 {
		// The booking reads as paid and carries no payment row at all. Same
		// refusal as an unreadable payment: a person has to find this money.
		h.logger.Error("cancel-info: the booking reads as paid but carries no payment record",
			"booking_id", booking.ID)
		return refundByHand
	}

	hasUnrefundedMP := false
	for _, payment := range payments {
		if payment.Status == "refunded" {
			continue
		}
		if payment.MPPaymentID == nil || *payment.MPPaymentID == "" {
			// A person has to act on this row regardless of what else is on the
			// booking, so this is the whole answer.
			return refundByHand
		}
		hasUnrefundedMP = true
	}
	if hasUnrefundedMP {
		return refundByMercadoPago
	}
	return refundNotApplicable
}

// releaseHeldSlots frees the checkout lock a cancelled booking still holds.
//
// PublicBook took one lock per slot under the old fixed-slot model and only
// ever released them on its own three failure paths — never on cancellation.
// Nothing but AcquireLock reads that table, so the effect is precise: the
// court is free in every availability query and every staff view, and the one
// place that cannot sell it again is the public booking page, until the
// five-minute sweeper runs.
//
// The lock is addressed by court, date and start time rather than by booking
// id. It has to be: AcquireLock runs before InsertSafe, and InsertSafe is what
// generates the booking id, so there is nothing to write into
// slot_locks.booking_id at the moment the lock is taken.
//
// A booking now takes exactly one lock, over its whole span, rather than one
// per fixed-size chunk — see AcquireLock's call site in public.go. This
// releases that one row.
//
// Risk carried forward deliberately: a slot_locks row created under the old
// per-chunk model (one row per court.DurationMinutes-sized chunk, at starts
// after the booking's own StartTime) is not reached by this single-row
// release, because it is keyed at a different start_time. That is judged
// acceptable rather than worth a range-delete here: slot_locks only matters
// for the ~15-minute checkout window before a booking is confirmed, every row
// carries its own expires_at, and CleanExpired's sweeper reclaims anything
// this release does not — the same backstop this function already relies on
// for a release the database refuses. No live inventory is permanently lost;
// at worst a pre-existing chunk row squats on its own slot until its TTL
// passes, same as it would have before this change shipped.
func (h *Handler) releaseHeldSlots(ctx context.Context, booking *bookingstore.Booking) {
	h.releaseSlotLock(ctx, booking.CourtID, booking.Date, booking.StartTime)
}

// slotReleaseBudget bounds the detached release below.
//
// context.WithoutCancel strips the parent's DEADLINE as well as its
// cancellation, so a context detached and left at that reports no deadline at
// all — and this repository sets no statement_timeout anywhere. A wedged
// PostgreSQL would then turn "clean up after yourself" into a goroutine and a
// pooled connection held forever. Same trap, same answer, as
// internal/data/timeout.go's detachedQueryContext, which documents where this
// codebase hit it before.
//
// The budget was originally sized to cover a loop of up to MaxSlotCount
// releases; a booking now takes exactly one lock, so this bounds the single
// DELETE SlotLockModel.ReleaseLock issues, with the same margin as before.
const slotReleaseBudget = 10 * time.Second

// releaseSlotLock releases the one slot lock a booking's span holds.
// Errors are logged but not propagated: the lock's own TTL is the backstop.
//
// The release runs detached from the caller's cancellation. Every call site
// passes the request's own context and every call site is a failure or teardown
// path — the insert lost the race, MercadoPago refused the checkout, the booking
// was cancelled. A client who closes the tab while MercadoPago is slow cancels
// that context, which is the ordinary case rather than an exotic one, and the
// release then failed with context.Canceled before the DELETE ever reached
// PostgreSQL. Nothing but AcquireLock reads slot_locks, so the effect was
// precise and invisible: a court that every availability query and every staff
// view called free, and that the public booking page alone refused to sell,
// for the whole SlotLockTTL. The client did nothing wrong and the venue lost
// the hours.
func (h *Handler) releaseSlotLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), slotReleaseBudget)
	defer cancel()

	if relErr := h.locks.ReleaseLock(ctx, courtID, date, startTime); relErr != nil {
		h.logger.Error("failed to release slot lock", "error", relErr, "court_id", courtID, "slot_start", startTime)
	}
}
