package events

import (
	"context"

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
}

// SweepUnrefunded finds the participants an event cancellation ended whose money
// is still collected, past the grace the cancel request itself had to refund
// them, and drives each through the same refund path the cancellation uses. It
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
			s.notifyCancelled(reg, ev, complex, true, outcome)
		}
	}
}
