// cron.go — the four scheduled sweeps of the booking domain.
//
// They used to live in cmd/api/cron.go, written directly against the stores,
// which made that file a second place booking rules were decided: the expiry
// sweep carried its own copy of the preference-expiry code the cancel path
// already had, and the two had already drifted. cmd/api now keeps only the
// schedule — one line per job — and the rules are here, beside the request
// paths that share them.
package bookings

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/booklink"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/waitlist"
)

// Reminder2h sends the two-hour reminder by email. Reminders are informational only — bookings are never
// auto-cancelled for lack of confirmation.
//
// A booking is queued first and flagged second. The flag is what stops later
// ticks from selecting it again, so it is written only once ReminderDue has
// returned nil: the reminder is in the queue, or the queue already had it. A
// refused enqueue leaves the booking unflagged, and the next tick tries again.
// The other order loses the reminder for good if the process dies between the
// two.
//
// The cost of this order is a repeat when the flag write fails after a
// successful enqueue: the next tick queues the reminder again, and the dedup key
// makes that a no-op only while the first job's row is kept. Each tick also
// mints a fresh cancel link before it enqueues, so every tick whose enqueue is
// refused leaves one more booking_link_tokens row behind; CleanLinkTokens deletes
// them once they pass retention.
func (s *Service) Reminder2h(ctx context.Context) {
	// time.Now() and not timezone.Now(): the store compares instants, so the
	// location this value carries changes nothing. Argentina enters the
	// calculation once, inside booking_starts_at, where the stored date and
	// start_time are read as local wall clock.
	bookings, err := s.store.GetForReminder2hEnriched(ctx, time.Now())
	if err != nil {
		s.logger.Error("cron_reminder_2h: failed to get bookings", "error", err)
		return
	}

	sent := 0
	for _, b := range bookings {
		// A fresh access token per reminder, rather than the one the booking
		// was confirmed with: booking_link_tokens stores only a hash, so the
		// plaintext minted at confirmation is unreadable from here — see
		// booklinkstore.Store.Mint on why every process that emits a link
		// mints its own row. A mint that fails costs the cancel link and
		// nothing else: the reminder still goes out, without the link, rather
		// than not at all.
		cancelURL := ""
		token, mintErr := s.linkTokens.Mint(ctx, b.ID, b.EndsAt.Add(s.cfg.LinkTokenBuffer))
		if mintErr != nil {
			s.logger.Error("cron_reminder_2h: failed to mint a cancel link, sending the reminder without one",
				"error", mintErr, "booking_id", b.ID)
		} else {
			cancelURL = booklink.Cancel(s.cfg.PublicURL, b.ComplexSlug, token)
		}

		if err := s.notify.ReminderDue(notifications.Reminder{
			Email:       b.ClientEmail,
			ComplexName: b.ComplexName,
			CourtName:   b.CourtName,
			Date:        b.Date.Format("02/01"),
			StartTime:   timezone.HoursLabel(b.StartsAt, b.EndsAt),
			Address:     booklink.Address(b.ComplexAddress, b.ComplexCity),
			// What is left to pay at the venue. Two hours out this is the fact
			// a client acts on, and the confirmation that carried it went out
			// days ago.
			BalanceAmount: notifications.BalanceAmount(b.Price, b.DepositAmount, b.CollectionStatus.String()),
			MapsURL:       booklink.MapsURL(b.ComplexName, b.ComplexAddress, b.ComplexCity, b.ComplexLatitude, b.ComplexLongitude),
			CancelURL:     cancelURL,
			ComplexPhone:  b.ComplexPhone,
			OwnerID:       ownerIDOf(b),
			BookingID:     b.ID.String(),
		}); err != nil {
			// Not queued, so not flagged: the next tick selects this booking again
			// and the reminder is attempted once more.
			s.logger.Error("cron_reminder_2h: failed to queue reminder, not flagged; the next tick retries",
				"error", err, "booking_id", b.ID)
			continue
		}

		s.logger.Info("cron_reminder_2h: sent reminder", "booking_id", b.ID)
		sent++

		// Flagged only once the reminder is queued (see Reminder2h). A failed flag
		// write leaves the booking selectable: the next tick queues it again, and
		// the dedup key makes that enqueue a no-op while the first job's row is
		// kept.
		if err := s.store.MarkReminderSent2h(ctx, b.ID); err != nil {
			s.logger.Error("cron_reminder_2h: failed to flag sent; the next tick queues it again and the dedup key absorbs the repeat",
				"error", err, "booking_id", b.ID)
		}
	}

	s.logger.Info("cron_reminder_2h: completed", "candidates", len(bookings), "sent", sent)
}

// ReleaseExpiredPayments cancels public bookings that are still unpaid past the
// payment window, freeing the slot they were holding.
//
// The checkout link is closed first, through the same helper the two
// cancellation paths use, so the client cannot pay for hours this sweep is
// about to put back on sale. That sharing is the point: this job used to carry
// its own copy of the expiry code, and the copy had already drifted — one
// attempt against the cancel path's two, and a credential it could not decrypt
// quietly became a call made as the platform.
//
// Before a booking is expired MercadoPago itself is asked whether its checkout
// was paid (reconcileBeforeExpiry): a notification that never arrived, or was
// refused, used to leave an approved payment unrecorded while this sweep
// cancelled the booking and sold its hours again. An approved payment is handed
// to reconciler, which processes it as the webhook would have, and the booking
// is left for that processing to confirm or refund.
//
// Expiring is not the end of the story either: a booking expired here or by
// a new booking landing on its stale hours (ReleaseStalePendingOverlaps) is
// asked about once more on a later run (reconcileExpiredHolds), since a
// payment can still be approved after the question above, and the inline
// release never asks at all.
func (s *Service) ReleaseExpiredPayments(ctx context.Context, reconciler PaymentReconciler) {
	s.releaseExpiredHolds(ctx)

	// Before this run expires anything, so a booking it expires is asked about
	// on the next run rather than straight after the question above.
	s.reconcileExpiredHolds(ctx, reconciler)

	bookings, err := s.store.GetExpiredPendingEnriched(ctx, s.cfg.PaymentExpiry)
	if err != nil {
		s.logger.Error("cron_release_expired_payments: failed to get bookings", "error", err)
		return
	}

	released := 0
	for _, b := range bookings {
		// Expire the MP preference FIRST so the client can't pay while we cancel.
		s.expireCheckoutPreference(ctx, &b.Booking, b)

		if s.reconcileBeforeExpiry(ctx, b, reconciler) {
			continue
		}

		if err := s.cancelUnpaidHold(ctx, b, bookingstore.HoldExpiryNote, true); err != nil {
			if errors.Is(err, data.ErrEditConflict) {
				// Cancelled by its client or paid since it was read: left as
				// it is, and nobody is told it expired.
				s.logger.Info("cron_release_expired_payments: booking changed since it was read, not expired", "booking_id", b.ID)
				continue
			}
			s.logger.Error("cron_release_expired_payments: failed to cancel booking", "error", err, "booking_id", b.ID)
			continue
		}
		s.logger.Info("cron_release_expired_payments: released expired booking", "booking_id", b.ID)
		released++
	}

	s.logger.Info("cron_release_expired_payments: completed", "candidates", len(bookings), "released", released)
}

// reconcileGiveUp is how long after its payment window an unpaid booking is
// kept while MercadoPago cannot be asked about it. Past it the booking is
// expired anyway: its hours already read as free to everyone else (the stale
// pending carve-out), the expired booking is asked about again by
// reconcileExpiredHolds, and a payment that surfaces later reopens it into
// hours still free or is refunded by the confirmation path.
const reconcileGiveUp = time.Hour

// reconcileBeforeExpiry asks MercadoPago for the payments made against b's
// checkout and reports whether b must be left pending rather than expired:
// b has not been expired for reconcileGiveUp yet, and either an approved
// payment was found (and handed to reconciler) or MercadoPago could not be
// asked.
//
// A booking whose seller credential is unreadable cannot be asked about, and
// is expired as before. One with no payment row is asked about all the same
// (see below); one that never reached MercadoPago costs a search that finds
// nothing.
func (s *Service) reconcileBeforeExpiry(ctx context.Context, b *bookingstore.CronBooking, reconciler PaymentReconciler) bool {
	if reconciler == nil {
		return false
	}
	// A missing payment row is no answer: the public path creates the
	// checkout link before it saves the row and only logs a failed save, and
	// MercadoPago is searched by the booking id alone. Only an unreadable row
	// keeps the booking unasked.
	if _, err := s.payments.GetByBookingID(ctx, b.ID); err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		s.logger.Error("cron_release_expired_payments: payment row unreadable, cannot ask MercadoPago; booking kept",
			"error", err, "booking_id", b.ID)
		return s.withinReconcileGrace(b)
	}

	// The same party closeCheckout addressed the link as: the seller whose
	// checkout it is. An unreadable credential has already been alerted on
	// there, and asking as anyone else would answer about the wrong account.
	caller, err := expiryCaller(b)
	if err != nil {
		return false
	}

	found, err := s.checkout.SearchPaymentsByReference(ctx, b.ID.String(), caller)
	if err != nil {
		s.logger.Error("cron_release_expired_payments: MercadoPago could not be asked about the checkout; booking kept",
			"error", err, "booking_id", b.ID)
		return s.withinReconcileGrace(b)
	}

	// An approved payment keeps the booking only for reconcileGiveUp, the same
	// bound the unreachable branch above uses. One the processing declines
	// without recording (a collector or amount mismatch) or keeps failing on
	// never confirms the booking; unbounded, every run kept it, wrote another
	// inbox row and raised another alert, forever. Past the window it is left
	// to a person, named in the error below, and the booking expires as it
	// did before this reconciliation existed.
	withinGrace := s.withinReconcileGrace(b)
	approved := false
	for _, p := range found {
		if p.Status != "approved" || p.ExternalReference != b.ID.String() {
			continue
		}
		mpPaymentID := strconv.FormatInt(p.ID, 10)
		if !withinGrace {
			s.logger.Error("cron_release_expired_payments: approved payment never confirmed the booking within the give-up window; booking expired, payment needs manual review",
				"booking_id", b.ID, "mp_payment_id", mpPaymentID)
			sentry.CaptureMessage(fmt.Sprintf("APPROVED PAYMENT NEVER CONFIRMED, BOOKING EXPIRED (manual review): booking_id=%s mp_payment_id=%s replay=%s",
				b.ID, mpPaymentID, paymentstore.ReplayCommand(mpPaymentID)))
			continue
		}
		approved = true
		if s.paymentAlreadyRecorded(ctx, b.ID, mpPaymentID) {
			// Recorded, yet the booking is still unpaid: the processing has
			// already decided about it and replaying it would only add inbox
			// rows. The money is captured, so the booking is not expired.
			s.logger.Error("cron_release_expired_payments: approved payment is recorded but the booking is unpaid; booking kept for review",
				"booking_id", b.ID, "mp_payment_id", mpPaymentID)
			continue
		}
		if err := reconciler.ReconcileApprovedPayment(ctx, mpPaymentID); err != nil {
			s.logger.Error("cron_release_expired_payments: an approved payment could not be recorded; booking kept for the next run",
				"error", err, "booking_id", b.ID, "mp_payment_id", mpPaymentID)
		}
	}
	if approved {
		s.logger.Warn("cron_release_expired_payments: booking was paid but never confirmed, reconciled instead of expired",
			"booking_id", b.ID)
		return true
	}
	return s.extendWhilePaymentInProcess(ctx, b, found)
}

// extendWhilePaymentInProcess keeps b's hold live while MercadoPago is still
// processing a payment on it (S8, slot-hold gaps). The window exists so nobody
// blocks a court without meaning to buy; a visitor whose payment is in process
// is buying. Keeping the row pending is not enough, because every reader frees
// the hours by the clock, so the hold itself is extended, inProcessHoldStep
// from now at a time, for at most inProcessHoldLimit after the attempt
// started. KeepPayingHolds runs it before the window ends; the expiry sweep
// runs it for one that ended before anybody asked. An attempt without
// a start time, or past the bound, extends nothing and the booking expires as
// before; a later approval is still reconciled by reconcileExpiredHolds.
func (s *Service) extendWhilePaymentInProcess(ctx context.Context, b *bookingstore.CronBooking, found []mp.Payment) bool {
	if s.holdExpiries == nil {
		return false
	}
	for _, p := range found {
		if p.ExternalReference != b.ID.String() || p.DateCreated.IsZero() ||
			time.Since(p.DateCreated) >= inProcessHoldLimit {
			continue
		}
		switch p.Status {
		case "pending", "in_process", "authorized":
		default:
			continue
		}
		if err := s.holdExpiries.ExtendHold(ctx, b.ID, b.ComplexID, inProcessHoldStep, s.cfg.PaymentExpiry); err != nil {
			s.logger.Error("payment in process: could not extend the hold",
				"error", err, "booking_id", b.ID, "mp_payment_id", p.ID)
			return false
		}
		s.logger.Info("payment in process: hold extended",
			"booking_id", b.ID, "mp_payment_id", p.ID, "status", p.Status)
		return true
	}
	return false
}

// paymentAlreadyRecorded reports whether the booking's ledger already holds a
// settled row for mpPaymentID. An unreadable ledger answers false, so the
// reconciliation goes ahead: its processing is idempotent.
func (s *Service) paymentAlreadyRecorded(ctx context.Context, bookingID uuid.UUID, mpPaymentID string) bool {
	rows, err := s.payments.ListByBookingID(ctx, bookingID)
	if err != nil {
		return false
	}
	for _, r := range rows {
		if r.MPPaymentID != nil && *r.MPPaymentID == mpPaymentID && r.Status != "unpaid" {
			return true
		}
	}
	return false
}

// expiredPaymentState reports what an expired booking's ledger says about the
// approved payment mpPaymentID. recorded is whether a row carries it with any
// status but unpaid. settled is whether nothing more is owed to that payment
// from the reconciliation: its row is refunded or its refund claimed
// (refunded, refund_pending), or it collected money on a booking it reopened,
// which is no longer cancelled. A row recorded as collected on a booking still
// cancelled is money owed back with no refund in motion, and is not settled.
// An unreadable ledger or booking answers not settled, so the booking is asked
// about again.
func (s *Service) expiredPaymentState(ctx context.Context, bookingID uuid.UUID, mpPaymentID string) (recorded, settled bool) {
	rows, err := s.payments.ListByBookingID(ctx, bookingID)
	if err != nil {
		return false, false
	}
	var row *paymentstore.Payment
	for _, r := range rows {
		if r.MPPaymentID != nil && *r.MPPaymentID == mpPaymentID && r.Status != "unpaid" {
			row = r
			break
		}
	}
	if row == nil {
		return false, false
	}
	if row.Status == "refunded" || row.Status == "refund_pending" {
		return true, true
	}
	current, err := s.store.GetByID(ctx, bookingID)
	if err != nil {
		s.logger.Error("cron_release_expired_payments: an expired booking could not be read back after its payment was recorded",
			"error", err, "booking_id", bookingID, "mp_payment_id", mpPaymentID)
		return true, false
	}
	return true, current.Status != bookingstore.BookingStatusCancelled
}

// withinReconcileGrace reports whether b's payment window closed less than
// reconcileGiveUp ago.
func (s *Service) withinReconcileGrace(b *bookingstore.CronBooking) bool {
	return time.Since(b.CreatedAt.Add(s.cfg.PaymentExpiry)) < reconcileGiveUp
}

// HoldExpiryStore is the record of storefront bookings cancelled only because
// their payment window ran out (bookingstore.Store, hold_expiry.go).
type HoldExpiryStore interface {
	// ExpireHold writes the cancelled booking and records it as expired, in
	// one transaction.
	ExpireHold(ctx context.Context, b *bookingstore.Booking) error
	// CancelReleasedCheckout writes a visitor's release of a booking it read
	// and records it to be asked about once more, never reopened, in one
	// transaction; a booking that changed since it was read is ErrEditConflict.
	CancelReleasedCheckout(ctx context.Context, b *bookingstore.Booking) error
	ExpiredHoldsToReconcile(ctx context.Context, lookback time.Duration, limit, maxFailures int) ([]*bookingstore.CronBooking, error)
	MarkHoldReconciled(ctx context.Context, bookingID uuid.UUID) error
	RecordHoldReconcileFailure(ctx context.Context, bookingID uuid.UUID) (int, error)
	// ExtendHold keeps a still pending, unpaid booking's hold live for
	// liveFor more, given its payment window.
	ExtendHold(ctx context.Context, bookingID, complexID uuid.UUID, liveFor, window time.Duration) error
	// PaymentsLapsingSoon reads the storefront bookings with a checkout whose
	// window ends within ahead or ended less than lookback ago.
	PaymentsLapsingSoon(ctx context.Context, window, ahead, lookback time.Duration, limit int) ([]*bookingstore.CronBooking, error)
}

// KeepPayingHoldsEvery is how often KeepPayingHolds runs (cmd/api schedules
// it with this value). The look-ahead and the extension below are sized on it.
const KeepPayingHoldsEvery = time.Minute

const (
	// inProcessHoldLimit bounds how long after a payment attempt started its
	// "in process" status keeps the court held (S8).
	inProcessHoldLimit = time.Hour
	// inProcessLookAhead is how far ahead of its window's end a booking is
	// asked about: more than one run, so a booking that would lapse before
	// the next run is asked about on this one.
	inProcessLookAhead = 2 * KeepPayingHoldsEvery
	// inProcessHoldStep is how long from now each extension keeps such a
	// hold: past the run after next, so one late or missed run still finds it
	// held, and the run before it lapses asks about it again (it is inside
	// the look-ahead by then).
	inProcessHoldStep = inProcessLookAhead + KeepPayingHoldsEvery
	// lapsingSoonBatch bounds the MercadoPago calls one look-ahead run makes.
	lapsingSoonBatch = 50
)

// KeepPayingHolds keeps a storefront booking's hours taken while its payment
// is in process (S8, slot-hold gaps): "los 15 minutos es para que no bloqueen
// las cancha sin ganas de comprar, se deberia de extender en caso de que la
// compra este en proceso".
//
// Every reader frees a booking's hours by the clock, the moment its window
// ends, and ReleaseExpiredPayments only sees a booking once that has happened
// — up to a whole run of that sweep later. In between, the hours read as free,
// another visitor's hold cancels the booking inline, and the person paying is
// refunded. So this runs every KeepPayingHoldsEvery and looks ahead: each
// booking with a checkout whose window ends within inProcessLookAhead (or
// ended without the expiry sweep having run yet) is asked about. A payment in
// process extends its hold (extendWhilePaymentInProcess); an approved one
// whose notification was lost is processed now. Nothing here expires a
// booking or closes its checkout: one nobody is paying lapses on time, and
// ReleaseExpiredPayments expires it as before.
func (s *Service) KeepPayingHolds(ctx context.Context, reconciler PaymentReconciler) {
	if s.holdExpiries == nil {
		return
	}
	candidates, err := s.holdExpiries.PaymentsLapsingSoon(ctx, s.cfg.PaymentExpiry, inProcessLookAhead, reconcileGiveUp, lapsingSoonBatch)
	if err != nil {
		s.logger.Error("cron_keep_paying_holds: failed to read bookings about to lapse", "error", err)
		return
	}
	extended := 0
	for _, b := range candidates {
		caller, err := expiryCaller(b)
		if err != nil {
			continue
		}
		found, err := s.checkout.SearchPaymentsByReference(ctx, b.ID.String(), caller)
		if err != nil {
			s.logger.Warn("cron_keep_paying_holds: MercadoPago could not be asked; left to the expiry sweep",
				"error", err, "booking_id", b.ID)
			continue
		}
		if s.processApprovedEarly(ctx, b, found, reconciler) {
			continue
		}
		if s.extendWhilePaymentInProcess(ctx, b, found) {
			extended++
		}
	}
	if len(candidates) > 0 {
		s.logger.Info("cron_keep_paying_holds: completed", "candidates", len(candidates), "extended", extended)
	}
}

// processApprovedEarly hands an approved payment on b, not yet recorded, to
// reconciler, and reports whether there was one: the booking is being paid
// for, and its processing confirms it.
//
// Only while b's window has not ended. Past it the expiry sweep owns the
// booking (reconcileBeforeExpiry): it replays the payment within
// reconcileGiveUp and raises one alert when it gives up. This job runs every
// minute, so replaying here too repeated a payment the processing declines
// (a collector or amount mismatch) sixty times an hour, each one another inbox
// row and alert. The answer is still true, so such a booking is not extended.
func (s *Service) processApprovedEarly(ctx context.Context, b *bookingstore.CronBooking, found []mp.Payment, reconciler PaymentReconciler) bool {
	live := time.Now().Before(b.CreatedAt.Add(s.cfg.PaymentExpiry))
	approved := false
	for _, p := range found {
		if p.Status != "approved" || p.ExternalReference != b.ID.String() {
			continue
		}
		approved = true
		mpPaymentID := strconv.FormatInt(p.ID, 10)
		if !live || reconciler == nil || s.paymentAlreadyRecorded(ctx, b.ID, mpPaymentID) {
			continue
		}
		if err := reconciler.ReconcileApprovedPayment(ctx, mpPaymentID); err != nil {
			s.logger.Error("cron_keep_paying_holds: an approved payment could not be recorded; left to the expiry sweep",
				"error", err, "booking_id", b.ID, "mp_payment_id", mpPaymentID)
		}
	}
	return approved
}

const (
	// expiredHoldLookback is how long after its expiry a booking is still
	// asked about. MercadoPago settles an in-process payment within hours;
	// past this a payment nobody found is a person's to chase.
	expiredHoldLookback = 48 * time.Hour
	// expiredHoldBatch bounds the MercadoPago calls one run makes.
	expiredHoldBatch = 50
	// expiredHoldMaxFailures is how many attempts may fail to settle one
	// booking (MercadoPago unreachable, the link not closable, a payment not
	// handed over) before the sweep stops and alerts. The store spaces the
	// attempts five minutes more apart after each failure, at most two hours
	// (RecordHoldReconcileFailure), so twenty span about sixteen hours: an
	// outage of MercadoPago lasting hours no longer abandons every booking
	// expired in it, as twelve runs at the bare five-minute cadence did after
	// one hour. Inside the 48-hour lookback.
	expiredHoldMaxFailures = 20
)

// expiredHoldOutcome is what one reconciliation of an expired booking ended in.
type expiredHoldOutcome int

const (
	// expiredHoldSettled: MercadoPago was asked, and every approved payment
	// was handed over or already recorded. Not asked again.
	expiredHoldSettled expiredHoldOutcome = iota
	// expiredHoldInFlight: a payment is still being processed by MercadoPago
	// (pending, in process, authorized). Asked again on the next run, without
	// counting a failure; the lookback bounds it.
	expiredHoldInFlight
	// expiredHoldFailed: the question could not be asked or answered. Asked
	// again once the store's backoff allows, counted against
	// expiredHoldMaxFailures.
	expiredHoldFailed
)

// reconcileExpiredHolds asks MercadoPago about the recently expired
// storefront bookings (S1, slot-hold gaps), with or without a saved payment
// row (ExpiredHoldsToReconcile says why). The inline release
// (bookingstore.ReleaseStalePendingOverlaps) cancels a booking without closing
// its link or asking anything, and even the sweep's own expiry asks before a
// payment in flight may settle; once cancelled, neither was ever looked at
// again, so an approved payment whose webhook was lost stayed captured with
// no payment row, no refund and no alert. Each one is now asked once: its
// link is closed (a no-op when the sweep already closed it), MercadoPago is
// asked for its payments, and an approved one goes to reconciler — the same
// processing the webhook runs, which reopens the booking into hours still
// free or refunds it.
func (s *Service) reconcileExpiredHolds(ctx context.Context, reconciler PaymentReconciler) {
	if reconciler == nil || s.holdExpiries == nil {
		return
	}
	expired, err := s.holdExpiries.ExpiredHoldsToReconcile(ctx, expiredHoldLookback, expiredHoldBatch, expiredHoldMaxFailures)
	if err != nil {
		s.logger.Error("cron_release_expired_payments: failed to read expired bookings to reconcile", "error", err)
		return
	}
	for _, b := range expired {
		outcome, unsettled := s.reconcileExpiredHold(ctx, b, reconciler)
		switch outcome {
		case expiredHoldSettled:
			if err := s.holdExpiries.MarkHoldReconciled(ctx, b.ID); err != nil {
				// Asked again next run; the processing is idempotent.
				s.logger.Error("cron_release_expired_payments: failed to mark an expired booking reconciled",
					"error", err, "booking_id", b.ID)
			}
		case expiredHoldInFlight:
		case expiredHoldFailed:
			failures, err := s.holdExpiries.RecordHoldReconcileFailure(ctx, b.ID)
			if err != nil {
				s.logger.Error("cron_release_expired_payments: failed to count a reconciliation failure",
					"error", err, "booking_id", b.ID)
				continue
			}
			if failures >= expiredHoldMaxFailures {
				replay := replayCommands(unsettled)
				s.logger.Error("cron_release_expired_payments: an expired booking could not be reconciled with MercadoPago; giving up, needs manual review",
					"booking_id", b.ID, "failures", failures, "replay", replay)
				sentry.CaptureMessage(fmt.Sprintf("EXPIRED BOOKING NOT RECONCILED WITH MERCADOPAGO (manual review): booking_id=%s failures=%d replay=%s",
					b.ID, failures, replay))
			}
		}
	}
}

// replayCommands is what a give-up alert carries for each approved payment it
// leaves behind: the command that has the API process it again once its cause
// is fixed (cmd/replaypayment). With none known, it says how to find them.
func replayCommands(mpPaymentIDs []string) string {
	if len(mpPaymentIDs) == 0 {
		return "none known (search the booking id as external_reference in MercadoPago, then " +
			paymentstore.ReplayCommand("<mp-payment-id>") + ")"
	}
	commands := make([]string, len(mpPaymentIDs))
	for i, id := range mpPaymentIDs {
		commands[i] = paymentstore.ReplayCommand(id)
	}
	return strings.Join(commands, " ; ")
}

// reconcileExpiredHold closes b's checkout link and asks MercadoPago about its
// payments, handing every approved one not yet recorded to reconciler, and
// returns the approved payments it could not settle. When none was ever
// approved and none is in flight, and the booking is still an unpaid expiry
// (expiredUnpaid), the client is told it expired (queueExpiredNotice).
//
// "Nobody paid" is only concluded from an answer that can say so. MercadoPago's
// search lags behind a payment approved in the last seconds of the hold, so a
// first empty answer is asked again on a later run, as the events sweep does
// (internal/events, checkPayment); it counts as a failed attempt, which is
// what spaces the second ask out. And a venue with no seller credential is
// searched as the platform, whose search does not answer for the seller's
// payments: an empty answer there is never trusted, and the booking is asked
// again until the sweep gives up and alerts.
func (s *Service) reconcileExpiredHold(ctx context.Context, b *bookingstore.CronBooking, reconciler PaymentReconciler) (expiredHoldOutcome, []string) {
	// Closed before asking, so nothing can be approved after the answer. The
	// inline release left it open; the sweep's own expiry already closed it,
	// and closing it again changes nothing. Quietly: a failure here is retried
	// every run, and reconcileExpiredHolds alerts once when it gives up.
	if err := s.closeCheckoutQuietly(ctx, &b.Booking, b); err != nil {
		return expiredHoldFailed, nil
	}
	caller, err := expiryCaller(b)
	if err != nil {
		return expiredHoldFailed, nil
	}
	found, err := s.checkout.SearchPaymentsByReference(ctx, b.ID.String(), caller)
	if err != nil {
		s.logger.Error("cron_release_expired_payments: MercadoPago could not be asked about an expired booking's checkout",
			"error", err, "booking_id", b.ID)
		return expiredHoldFailed, nil
	}

	outcome := expiredHoldSettled
	var unsettled []string
	// moneyMoved: a payment was approved on the checkout, whether it still
	// is or was refunded or charged back since.
	moneyMoved := false
	for _, p := range found {
		if p.ExternalReference != b.ID.String() {
			continue
		}
		switch p.Status {
		case "pending", "in_process", "authorized":
			if outcome == expiredHoldSettled {
				outcome = expiredHoldInFlight
			}
			continue
		case "refunded", "charged_back":
			moneyMoved = true
			continue
		case "approved":
			moneyMoved = true
		default:
			continue
		}
		mpPaymentID := strconv.FormatInt(p.ID, 10)
		if !s.reconcileExpiredPayment(ctx, b, mpPaymentID, reconciler) {
			outcome = expiredHoldFailed
			unsettled = append(unsettled, mpPaymentID)
		}
	}
	if outcome != expiredHoldSettled || moneyMoved {
		return outcome, unsettled
	}
	return s.settleNobodyPaid(b, caller), nil
}

// settleNobodyPaid decides an expired booking MercadoPago's answer holds no
// payment for, and tells its client it expired once that answer can be
// trusted: not when it came from the platform's search, which does not answer
// for the seller's payments, and not on the first ask, which is asked again
// (search lag). Both of those count as a failed attempt.
func (s *Service) settleNobodyPaid(b *bookingstore.CronBooking, caller mp.Caller) expiredHoldOutcome {
	if caller == mp.AsPlatform() {
		s.logger.Error("cron_release_expired_payments: an expired booking's venue has no seller credential, and the platform's empty search cannot prove nobody paid; asked again",
			"booking_id", b.ID, "complex_id", b.ComplexID)
		return expiredHoldFailed
	}
	if b.HoldReconcileFailures == 0 {
		s.logger.Info("cron_release_expired_payments: MercadoPago holds no payment for an expired booking on the first ask; asking once more before trusting it",
			"booking_id", b.ID)
		return expiredHoldFailed
	}
	if expiredUnpaid(b) {
		s.queueExpiredNotice(b)
	}
	return expiredHoldSettled
}

// reconcileExpiredPayment hands the approved payment mpPaymentID on the expired
// booking b to reconciler unless the ledger already records it, and reports
// whether it is settled (expiredPaymentState): false counts the run as a failed
// attempt, asked again and alerted at the bound.
func (s *Service) reconcileExpiredPayment(ctx context.Context, b *bookingstore.CronBooking, mpPaymentID string, reconciler PaymentReconciler) bool {
	recorded, settled := s.expiredPaymentState(ctx, b.ID, mpPaymentID)
	if settled {
		return true
	}
	if recorded {
		// Recorded, yet the booking is cancelled and its refund is not in
		// motion. Handing it over again is a no-op ("payment already
		// processed"), so nothing here can move it: it counts as a failed
		// attempt and alerts at the bound for a person.
		s.logger.Error("cron_release_expired_payments: an approved payment on an expired booking is recorded but neither refunded nor claimed; asked again",
			"booking_id", b.ID, "mp_payment_id", mpPaymentID)
		return false
	}
	s.logger.Warn("cron_release_expired_payments: an expired booking was paid and never processed, reconciling it",
		"booking_id", b.ID, "mp_payment_id", mpPaymentID)
	if err := reconciler.ReconcileApprovedPayment(ctx, mpPaymentID); err != nil {
		s.logger.Error("cron_release_expired_payments: an approved payment on an expired booking could not be handed over",
			"error", err, "booking_id", b.ID, "mp_payment_id", mpPaymentID)
		return false
	}
	// The hand-over records the event; its processing may still have failed
	// (a refund claim refused, the database away) and left the payment
	// unrecorded, or recorded on a booking still cancelled with its refund
	// neither claimed nor issued. Marking the booking reconciled then would
	// stop every further ask while that money stays captured. Only a payment
	// that reopened the booking, or whose refund is claimed or issued,
	// settles it.
	if _, settled := s.expiredPaymentState(ctx, b.ID, mpPaymentID); !settled {
		s.logger.Error("cron_release_expired_payments: an approved payment on an expired booking was handed over but is neither settling the booking nor being refunded; asked again",
			"booking_id", b.ID, "mp_payment_id", mpPaymentID)
		return false
	}
	return true
}

// expiredUnpaid reports whether b may be told "your booking expired, there is
// no payment to give back" once MercadoPago holds no payment for it: its hold
// expired (a booking its visitor released was let go on screen, by choice)
// and, as read by this run, nothing was collected on it and no refund is owed.
// An approved, refunded or charged-back payment is the processing's to tell
// (a reopened booking, or a refund), never this notice.
func expiredUnpaid(b *bookingstore.CronBooking) bool {
	return !b.ReleasedByClient &&
		b.CollectionStatus == bookingstore.CollectionStatusUnpaid &&
		b.RefundStatus == bookingstore.RefundStatusNone
}

// releaseExpiredHolds deletes the slot holds nobody converted within the
// payment window (see bookingstore.Store.ReleaseHold for why they are deleted)
// and tells each affected complex's grid once and the waitlist once per venue
// and day. There is nobody else to notify and no checkout to close: an
// untouched hold has neither.
func (s *Service) releaseExpiredHolds(ctx context.Context) {
	freed, err := s.store.DeleteExpiredHolds(ctx, s.cfg.PaymentExpiry)
	if err != nil {
		s.logger.Error("cron_release_expired_payments: failed to delete expired slot holds", "error", err)
		return
	}
	s.announceFreedHolds(ctx, freed)
	if len(freed) > 0 {
		s.logger.Info("cron_release_expired_payments: deleted expired slot holds", "count", len(freed))
	}
}

// announceFreedHolds tells each affected complex's grid once and the waitlist
// once per venue and day about the slots a sweep of holds freed.
func (s *Service) announceFreedHolds(ctx context.Context, freed []bookingstore.ReleasedHold) {
	type venueDay struct {
		complexID uuid.UUID
		day       string
	}
	told := make(map[uuid.UUID]struct{}, len(freed))
	handed := make(map[venueDay]struct{}, len(freed))
	for _, h := range freed {
		if _, seen := told[h.ComplexID]; !seen {
			told[h.ComplexID] = struct{}{}
			s.realtime.PublishBookingChanged(h.ComplexID)
		}
		key := venueDay{h.ComplexID, h.Day.Format(time.DateOnly)}
		if _, seen := handed[key]; !seen {
			handed[key] = struct{}{}
			s.freeHeldSlot(ctx, h)
		}
	}
}

// ReleaseUnpaidHolds cancels every storefront booking of one complex that is
// still waiting for its payment, closing its checkout link first. Disconnecting
// MercadoPago calls it before clearing the credentials: the webhook matches a
// payment to the venue by the seller identity those credentials carry, so a
// link paid after the disconnect would be neither confirmed nor refunded.
//
// A link that cannot be closed leaves its booking as it is and the call answers
// ErrCheckoutStillOpen, so the disconnect does not go ahead; the other holds are
// still released.
func (s *Service) ReleaseUnpaidHolds(ctx context.Context, complexID uuid.UUID) error {
	// Slot holds nobody has converted have no checkout link to close: they go
	// first, and the read below no longer sees them.
	if err := s.store.DeleteComplexHolds(ctx, complexID); err != nil {
		return fmt.Errorf("bookings: delete slot holds: %w", err)
	}

	holds, err := s.store.GetUnpaidPublicPendingEnriched(ctx, complexID)
	if err != nil {
		return fmt.Errorf("bookings: read unpaid holds: %w", err)
	}

	var stillOpen error
	for _, b := range holds {
		if s.checkoutBeingSetUp(ctx, b) {
			stillOpen = ErrCheckoutStillOpen
			continue
		}
		if err := s.closeCheckout(ctx, &b.Booking, b); err != nil {
			stillOpen = err
			continue
		}
		if err := s.cancelUnpaidHold(ctx, b, "Cancelado automáticamente: el complejo desconectó MercadoPago", false); err != nil {
			return fmt.Errorf("bookings: cancel unpaid hold %s: %w", b.ID, err)
		}
		s.logger.Info("mp_disconnect: released unpaid booking", "booking_id", b.ID, "complex_id", complexID)
	}
	return stillOpen
}

// checkoutSetupGrace is how long a storefront booking may go without a payment
// row before it counts as abandoned rather than still being set up. The public
// path inserts the booking, then asks MercadoPago for the checkout link, then
// writes the row; MercadoPago's call is bounded well under this.
const checkoutSetupGrace = 2 * time.Minute

// checkoutBeingSetUp reports a hold whose checkout link may be created right
// now: no payment row yet, and recent. Cancelling it would let the public path
// hand the visitor a live link for a booking that no longer exists.
func (s *Service) checkoutBeingSetUp(ctx context.Context, b *bookingstore.CronBooking) bool {
	_, err := s.payments.GetByBookingID(ctx, b.ID)
	return errors.Is(err, data.ErrRecordNotFound) && time.Since(b.CreatedAt) < checkoutSetupGrace
}

// cancelUnpaidHold cancels a storefront booking that was never paid, whose
// checkout link the caller has already closed, and tells the client why it went
// away. The owner is not told: they never had this booking.
//
// expired says the payment window ran out, which is recorded with the
// cancellation (HoldExpiryStore.ExpireHold): such a booking is asked about
// again by reconcileExpiredHolds, and reopened by a late payment into hours
// still free. A disconnect's release is not one: the seller can no longer be
// asked anything, and the booking stays cancelled.
func (s *Service) cancelUnpaidHold(ctx context.Context, b *bookingstore.CronBooking, note string, expired bool) error {
	b.Status = "cancelled"
	if b.Notes != nil {
		note = *b.Notes + " | " + note
	}
	b.Notes = &note

	write := s.store.Update
	if expired && s.holdExpiries != nil {
		write = s.holdExpiries.ExpireHold
	}
	if err := write(ctx, &b.Booking); err != nil {
		return err
	}

	s.realtime.PublishBookingChanged(b.ComplexID)

	s.freeWaitlistSlot(ctx, waitlist.Freed{
		ComplexID: b.ComplexID, ComplexName: b.ComplexName, ComplexSlug: b.ComplexSlug, Day: b.Date,
	})

	s.queueExpiredNotice(b)
	return nil
}

// queueExpiredNotice tells the client of a storefront booking that went away
// unpaid why it did. Its deduplication key carries the ":expired" subject
// (notifications.cancellationSubject), so queueing it again for the same
// booking is a no-op: the sweep's own expiry queues it when it cancels, and
// reconcileExpiredHold queues it for every recorded expiry once MercadoPago
// holds no payment, which is how a booking the inline release cancelled
// inside another visitor's request is told at all.
func (s *Service) queueExpiredNotice(b *bookingstore.CronBooking) {
	// A booking with no email has nobody to tell. The sweeps read no untouched
	// slot hold, so this is a guard rather than a path: such a booking is still
	// released, quietly, instead of queueing a notice with no address.
	if b.ClientEmail == "" {
		return
	}

	if err := s.notify.BookingCancelled(notifications.Cancellation{
		Email:       b.ClientEmail,
		ComplexName: b.ComplexName,
		CourtName:   b.CourtName,
		Date:        b.Date.Format("02/01"),
		StartTime:   timezone.HoursLabel(b.StartsAt, b.EndsAt),
		// This booking was never paid, so none of the six refund outcomes
		// applies and there is no amount. The client's question here is
		// why their booking vanished, which the generic cancellation copy
		// never answered.
		RefundLine: notifications.ExpiredUnpaidRefundLine,
		BookURL:    booklink.Book(s.cfg.PublicURL, b.ComplexSlug),
		// Never owner-facing: the owner was not told about this booking
		// (see SourceExpiredUnpaid), so there is nothing to take back.
		Source: notifications.SourceExpiredUnpaid,
		// No OwnerID: on a Cancellation it is also who the owner's notice
		// goes to, and the owner never had this booking. The client still
		// gets the complex's phone.
		ComplexPhone: b.ComplexPhone,
		BookingID:    b.ID.String(),
	}); err != nil {
		// The booking is already released; the notice is reported with it so
		// it can be resent, rather than failing the sweep for every other row.
		s.logger.Error("cron_release_expired_payments: the cancellation notice was not queued",
			"error", err, "booking_id", b.ID)
	}
}

// CompletePastBookings marks confirmed bookings whose hours have passed as
// completed.
func (s *Service) CompletePastBookings(ctx context.Context) {
	complexIDs, err := s.store.CompletePastBookings(ctx)
	if err != nil {
		s.logger.Error("cron_complete_bookings: failed", "error", err)
		return
	}
	// The completion is committed by the time the store returns, so every owner
	// whose calendar changed can be told now. A complex with several bookings
	// completed in this sweep gets one broadcast, not one per booking.
	told := make(map[uuid.UUID]struct{}, len(complexIDs))
	for _, complexID := range complexIDs {
		if _, seen := told[complexID]; seen {
			continue
		}
		told[complexID] = struct{}{}
		s.realtime.PublishBookingChanged(complexID)
	}
	s.logger.Info("cron_complete_bookings: completed", "count", len(complexIDs))
}

// LinkTokenRetention is how long past its stored expiry a terminal booking's
// link token is kept before CleanLinkTokens deletes it. Purely housekeeping —
// DeleteExpiredTerminal's terminal-status predicate already guarantees a live
// booking's token is never touched, so this window exists only to keep the
// table from growing without bound, not to protect a link still in use.
const LinkTokenRetention = 24 * time.Hour

// CleanLinkTokens deletes booking link tokens whose booking has reached a
// terminal state and whose stored expiry is safely in the past. See
// booklinkstore.Store.DeleteExpiredTerminal: a pending or confirmed booking's
// token is never touched here, however old, so a sweep can never turn a live
// link into a 404.
func (s *Service) CleanLinkTokens(ctx context.Context) {
	if err := s.linkTokens.DeleteExpiredTerminal(ctx, LinkTokenRetention); err != nil {
		s.logger.Error("cron_clean_booking_link_tokens: failed", "error", err)
		return
	}
	s.logger.Info("cron_clean_booking_link_tokens: completed")
}

// cronBookingCredentials is the compile-time check that the enriched booking
// the expiry sweep reads can stand in for a complex wherever a seller
// credential is all that is needed.
var _ sellerCredentials = (*bookingstore.CronBooking)(nil)

// ownerIDOf is the complex owner a client email replies to, or empty when the
// row carries none.
func ownerIDOf(b *bookingstore.CronBooking) string {
	if b.ComplexOwnerID == uuid.Nil {
		return ""
	}
	return b.ComplexOwnerID.String()
}
