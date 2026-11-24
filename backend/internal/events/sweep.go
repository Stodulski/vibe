package events

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/checkout"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/events/notice"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// refundSweepBatch is how many unrefunded cancellations one sweep drives: the
// sweep is sequential and a MercadoPago call takes seconds.
const refundSweepBatch = 12

// paymentCheckBatch is how many expired holds one sweep asks MercadoPago about,
// for the same reason.
const paymentCheckBatch = 12

// maxPaymentChecks is how many times the hold sweep tries to ask MercadoPago
// about one expired registration before it gives up. The store spaces the
// attempts five minutes more apart each time, at most two hours, so twenty
// span about sixteen hours.
const maxPaymentChecks = 20

// ExpireHolds closes the holds nobody paid in time. Their spot stays counted
// against capacity while their payment check is open, for at most fifteen
// minutes after their window ended (CountOpenPlaySpotsTaken): past the check's
// second answer, eight to eleven minutes on, so a payment approved in the
// hold's last seconds is not refunded because someone else filled the event
// before MercadoPago found it. Called by the scheduler, under the
// tenant bypass.
//
// Every hold opened a MercadoPago checkout, so closing it marks it due a
// payment check, and the person is told their spot expired only once
// MercadoPago was asked and holds no approved payment for it
// (checkExpiredPayments): a webhook that never arrived used to leave the money
// captured on an expired registration, with no payment row, no refund and no
// alert.
func (s *Service) ExpireHolds(ctx context.Context) {
	expired, err := s.store.ExpireHolds(ctx)
	if err != nil {
		s.logger.Error("event holds: failed to expire the holds that ran out", "error", err)
	}
	seen := map[string]bool{}
	for _, hold := range expired {
		if key := hold.ComplexID.String(); !seen[key] {
			seen[key] = true
			s.realtime.PublishBookingChanged(hold.ComplexID)
		}
	}
	if len(expired) > 0 {
		s.logger.Info("event holds: expired unpaid holds", "count", len(expired))
	}
	s.checkExpiredPayments(ctx)
}

// checkExpiredPayments asks MercadoPago about every expired hold whose payment
// check is due, the ones this run closed and the ones earlier runs deferred.
func (s *Service) checkExpiredPayments(ctx context.Context) {
	due, err := s.store.DuePaymentChecks(ctx, paymentCheckBatch)
	if err != nil {
		s.logger.Error("event holds: failed to read the due payment checks", "error", err)
		return
	}
	for _, check := range due {
		s.checkPayment(ctx, check)
	}
}

// checkPayment settles one expired hold's payment check. An approved payment of
// its checkout is handed to the payment processing, which confirms the spot
// while there is room and refunds it otherwise, and tells the person either
// way. No approved payment closes the check and tells the person their spot
// expired, when the check owes that notice — on the second answer, since a
// first empty one is asked again a few minutes later (search lag). A MercadoPago that cannot be asked,
// or a payment that cannot be handed over, defers the check to a later run,
// told nothing, until maxPaymentChecks attempts failed.
func (s *Service) checkPayment(ctx context.Context, check eventstore.PaymentCheck) {
	approvedIDs, err := s.approvedPayments(ctx, check)
	if err == nil && len(approvedIDs) > 0 {
		err = s.handOver(ctx, check, approvedIDs)
	}
	if err != nil {
		s.deferPaymentCheck(ctx, check, len(approvedIDs) > 0, err)
		return
	}
	if len(approvedIDs) == 0 && check.Failures == 0 {
		// The first answer, and it found nothing. MercadoPago's search lags
		// behind a payment approved in the last seconds of the hold, so it is
		// asked once more, a few minutes later, before the person is told the
		// spot expired. Pushed back like a failed attempt, which it counts as:
		// a later empty answer is trusted, as is one after a failed attempt,
		// which already came minutes later.
		if _, err := s.store.DeferPaymentCheck(ctx, check.RegistrationID); err != nil {
			s.logger.Error("event holds: failed to defer a payment check for its second ask", "error", err, "registration_id", check.RegistrationID)
		}
		return
	}
	closed, err := s.store.ClosePaymentCheck(ctx, check.RegistrationID)
	if err != nil {
		// Still open: the next run asks again, and the processing a payment was
		// handed to is idempotent.
		s.logger.Error("event holds: failed to close a payment check", "error", err, "registration_id", check.RegistrationID)
		return
	}
	if closed && len(approvedIDs) == 0 && check.Notify {
		s.notifyExpired(ctx, check.RegistrationID)
	}
}

// approvedPayments asks MercadoPago, as the seller whose checkout it was, for
// the approved payments made against the registration's checkout, whose
// reference is the registration's id (mp.CreatePreferenceInput.RegistrationID).
func (s *Service) approvedPayments(ctx context.Context, check eventstore.PaymentCheck) ([]string, error) {
	complex, err := s.complexes.GetByID(ctx, check.ComplexID)
	if err != nil {
		return nil, fmt.Errorf("read the complex: %w", err)
	}
	seller, err := checkout.Seller(complex)
	if err != nil {
		return nil, fmt.Errorf("the seller's credential: %w", err)
	}
	reference := check.RegistrationID.String()
	found, err := s.checkout.SearchPaymentsByReference(ctx, reference, seller)
	if err != nil {
		return nil, fmt.Errorf("search the checkout's payments: %w", err)
	}
	var ids []string
	for _, p := range found {
		if p.Status == "approved" && p.ExternalReference == reference {
			ids = append(ids, strconv.FormatInt(p.ID, 10))
		}
	}
	return ids, nil
}

// handOver gives each approved payment to the payment processing, exactly as
// its webhook would have been processed. The first that cannot be recorded
// fails the hand-over, so the check is asked again; one already handed over is
// a no-op the second time.
func (s *Service) handOver(ctx context.Context, check eventstore.PaymentCheck, approvedIDs []string) error {
	for _, id := range approvedIDs {
		s.logger.Warn("event holds: approved payment found on an expired hold, processing it",
			"registration_id", check.RegistrationID, "mp_payment_id", id)
		if err := s.reconciler.ReconcileApprovedPayment(ctx, id); err != nil {
			return fmt.Errorf("hand over payment %s: %w", id, err)
		}
		s.alertUnlessRecorded(ctx, check, id)
	}
	return nil
}

// alertUnlessRecorded raises an alert for an approved payment the processing
// was handed and recorded nothing for. A hand-over succeeds once the event is
// recorded, whatever the processing then decides: a payment it refuses (the
// collector or the amount does not check out) leaves no ledger row, and the
// check closed as answered with the payer's money captured and their spot
// lost. The processing's own alert names the refusal; this one names what it
// cost — the registration — and the command that replays the payment once the
// cause is fixed. Handing it over again on every run would only repeat the
// refusal, so the check still closes.
func (s *Service) alertUnlessRecorded(ctx context.Context, check eventstore.PaymentCheck, mpPaymentID string) {
	_, err := s.payments.GetByMPPaymentID(ctx, mpPaymentID)
	if err == nil {
		return
	}
	if !errors.Is(err, data.ErrRecordNotFound) {
		s.logger.Error("event holds: cannot read whether a handed-over payment was recorded",
			"error", err, "registration_id", check.RegistrationID, "mp_payment_id", mpPaymentID)
		return
	}
	replay := paymentstore.ReplayCommand(mpPaymentID)
	s.logger.Error("event holds: an approved payment on an expired hold was handed over and recorded by nothing; the spot is lost, manual review",
		"registration_id", check.RegistrationID, "mp_payment_id", mpPaymentID, "replay", replay)
	sentry.CaptureMessage(fmt.Sprintf("PAID REGISTRATION NOT RECORDED (manual review): registration_id=%s mp_payment_id=%s replay=%s",
		check.RegistrationID, mpPaymentID, replay))
}

// deferPaymentCheck counts a failed attempt and asks again later, or gives up
// once maxPaymentChecks attempts failed. Giving up on a payment seen approved
// leaves it to a person and tells its payer nothing; giving up on a MercadoPago
// that never answered tells the person their spot expired, which is all that
// is known.
func (s *Service) deferPaymentCheck(ctx context.Context, check eventstore.PaymentCheck, sawApproved bool, cause error) {
	failures, err := s.store.DeferPaymentCheck(ctx, check.RegistrationID)
	if err != nil {
		s.logger.Error("event holds: failed to defer a payment check", "error", err, "cause", cause, "registration_id", check.RegistrationID)
		return
	}
	if failures < maxPaymentChecks {
		s.logger.Warn("event holds: payment check failed, asking again later",
			"error", cause, "registration_id", check.RegistrationID, "failures", failures)
		return
	}
	closed, err := s.store.ClosePaymentCheck(ctx, check.RegistrationID)
	if err != nil {
		s.logger.Error("event holds: failed to give up a payment check", "error", err, "registration_id", check.RegistrationID)
		return
	}
	if !closed {
		return
	}
	s.logger.Error("event holds: GAVE UP asking MercadoPago about an expired hold, manual review",
		"error", cause, "registration_id", check.RegistrationID, "saw_approved_payment", sawApproved)
	sentry.CaptureMessage(fmt.Sprintf("REGISTRATION PAYMENT CHECK GAVE UP (manual review): registration_id=%s saw_approved_payment=%t error=%v",
		check.RegistrationID, sawApproved, cause))
	if !sawApproved && check.Notify {
		s.notifyExpired(ctx, check.RegistrationID)
	}
}

// notifyExpired tells the person whose hold ran out that their spot was
// released, as an expired storefront booking is told (S10). Only a person who
// left an email hears of it: there is no other channel. The registration is
// read again first: a late payment that confirmed it meanwhile wins, and the
// person is not told a spot they paid for expired. A notice that cannot be
// built is logged; the hold is already closed either way.
func (s *Service) notifyExpired(ctx context.Context, regID uuid.UUID) {
	reg, err := s.store.GetRegistration(ctx, regID)
	if err != nil {
		s.logger.Error("event holds: cannot read an expired hold to notify it", "error", err, "registration_id", regID)
		return
	}
	if reg.Status != eventstore.RegistrationExpired || reg.Email == nil || *reg.Email == "" {
		return
	}
	ev, complex, err := s.eventAndComplex(ctx, reg)
	if err != nil {
		s.logger.Error("event holds: cannot build the expiry notice", "error", err, "registration_id", regID)
		return
	}
	s.notify.EventCancelled(notice.Expired(reg, ev, complex, s.cfg.PublicURL))
}

// eventAndComplex reads the event and the complex a registration's notice names.
func (s *Service) eventAndComplex(ctx context.Context, reg *eventstore.Registration) (*eventstore.Event, *complexstore.Complex, error) {
	ev, err := s.store.GetEvent(ctx, reg.EventID)
	if err != nil {
		return nil, nil, err
	}
	complex, err := s.complexes.GetByID(ctx, reg.ComplexID)
	if err != nil {
		return nil, nil, err
	}
	return ev, complex, nil
}

// RemindUpcoming queues the day-before reminder of every paid spot whose event
// or tournament category starts within a day, to the person and, for a
// tournament pair, to their partner (S12). Like the booking reminder, a spot is
// queued first and marked second: a refused enqueue leaves it unmarked and the
// next tick tries again, and a repeat after a failed mark is absorbed by the
// dedup key. Called by the scheduler, under the tenant bypass.
func (s *Service) RemindUpcoming(ctx context.Context) {
	now := time.Now()
	due, err := s.store.DueReminders(ctx, now)
	if err != nil {
		s.logger.Error("event reminders: failed to read the due reminders", "error", err)
		return
	}
	sent := 0
	for _, d := range due {
		if s.remind(ctx, d, now) {
			sent++
		}
	}
	s.logger.Info("event reminders: completed", "candidates", len(due), "sent", sent)
}

// remind queues one spot's reminders and marks it, reporting whether it did.
func (s *Service) remind(ctx context.Context, d eventstore.DueReminder, now time.Time) bool {
	reg, err := s.store.GetRegistration(ctx, d.RegistrationID)
	if err != nil {
		s.logger.Error("event reminders: cannot read the registration", "error", err, "registration_id", d.RegistrationID)
		return false
	}
	ev, complex, err := s.eventAndComplex(ctx, reg)
	if err != nil {
		s.logger.Error("event reminders: cannot build the reminder", "error", err, "registration_id", d.RegistrationID)
		return false
	}
	type recipient struct{ email, name string }
	var to []recipient
	if reg.Email != nil && *reg.Email != "" {
		to = append(to, recipient{*reg.Email, reg.FirstName})
	}
	if d.PartnerEmail != nil && *d.PartnerEmail != "" {
		to = append(to, recipient{*d.PartnerEmail, d.PartnerFirstName})
	}
	for _, r := range to {
		if err := s.notify.EventReminderDue(notice.Reminder(reg, ev, complex, s.cfg.PublicURL, r.email, r.name, d.StartsAt, d.HasTime, now)); err != nil {
			// Not queued, so not marked: the next tick tries the whole spot again,
			// and the dedup key absorbs the half already queued.
			s.logger.Error("event reminders: failed to queue, not marked; the next tick retries", "error", err, "registration_id", reg.ID)
			return false
		}
	}
	if err := s.store.MarkReminded(ctx, reg.ID); err != nil {
		s.logger.Error("event reminders: failed to mark sent; the dedup key absorbs the repeat", "error", err, "registration_id", reg.ID)
	}
	return true
}

// SweepUnrefunded finds the cancelled registrations that owe their money back —
// the owner's cancellations, and a participant's inside the refund window —
// whose money is still collected, past the grace the cancel request itself had
// to refund them, and drives each through the same refund path the cancellation uses. It
// recovers the crash window between the cancellation's commit and the claim of
// each refund; a refund already claimed is the retry job's, and answers
// already-in-flight here. Called by the scheduler, under the tenant bypass.
func (s *Service) SweepUnrefunded(ctx context.Context) {
	pending, err := s.store.OwnerCancelledStillPaid(ctx, s.cfg.RefundGrace, refundSweepBatch)
	if err != nil {
		s.logger.Error("event refund sweep: failed to read the unrefunded cancellations", "error", err)
		return
	}
	for _, p := range pending {
		reg, err := s.store.GetRegistration(ctx, p.RegistrationID)
		if err != nil {
			s.logger.Error("event refund sweep: cannot read the registration", "error", err, "registration_id", p.RegistrationID)
			continue
		}
		ev, err := s.store.GetEvent(ctx, reg.EventID)
		if err != nil {
			s.logger.Error("event refund sweep: cannot read the event", "error", err, "registration_id", p.RegistrationID)
			continue
		}
		complex, err := s.complexes.GetByID(ctx, reg.ComplexID)
		if err != nil {
			s.logger.Error("event refund sweep: cannot read the complex", "error", err, "registration_id", p.RegistrationID)
			continue
		}
		outcome := s.payments.RefundRegistration(ctx, reg.ID)
		s.logger.Info("event refund sweep: drove an unrefunded cancellation", "registration_id", reg.ID, "result", outcome.Result)
		if outcome.Result == paymentstore.RefundIssued {
			byOwner := reg.CancelledBy != nil && *reg.CancelledBy == eventstore.CancelledByOwner
			s.notifyCancelled(reg, ev, complex, byOwner, outcome)
		}
	}
}
