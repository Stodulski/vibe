package events

import (
	"context"
	"time"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/events/notice"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// refundSweepBatch is how many unrefunded cancellations one sweep drives: the
// sweep is sequential and a MercadoPago call takes seconds.
const refundSweepBatch = 12

// ExpireHolds closes the holds nobody paid in time. They stopped counting against
// capacity the moment their window ended, so a sweep running late changes
// nothing a visitor sees; this keeps the rows honest and the owner's dashboards
// current. Called by the scheduler, under the tenant bypass.
func (s *Service) ExpireHolds(ctx context.Context) {
	expired, err := s.store.ExpireHolds(ctx)
	if err != nil {
		s.logger.Error("event holds: failed to expire the holds that ran out", "error", err)
		return
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
	s.notifyExpired(ctx, expired)
}

// notifyExpired tells each person whose hold ran out that their spot was
// released, as an expired storefront booking is told (S10). Only a person who
// left an email hears of it: there is no other channel. A notice that cannot be
// built is logged; the holds are already closed either way.
func (s *Service) notifyExpired(ctx context.Context, expired []eventstore.ExpiredHold) {
	for _, hold := range expired {
		reg, err := s.store.GetRegistration(ctx, hold.ID)
		if err != nil {
			s.logger.Error("event holds: cannot read an expired hold to notify it", "error", err, "registration_id", hold.ID)
			continue
		}
		if reg.Email == nil || *reg.Email == "" {
			continue
		}
		ev, complex, err := s.eventAndComplex(ctx, reg)
		if err != nil {
			s.logger.Error("event holds: cannot build the expiry notice", "error", err, "registration_id", hold.ID)
			continue
		}
		s.notify.EventCancelled(notice.Expired(reg, ev, complex, s.cfg.PublicURL))
	}
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
