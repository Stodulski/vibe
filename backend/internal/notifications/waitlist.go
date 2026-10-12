package notifications

import "context"

// TaskEmailWaitlistSlotFreed tells someone who asked to be told that a slot on
// the day they waited for has opened up again.
const TaskEmailWaitlistSlotFreed = "email:waitlist_slot_freed"

// WaitlistSlotFreedEmail is one waitlist entry's notice. EntryID is the waitlist
// row; it is the deduplication subject, so one row yields one email even if its
// task is redelivered.
type WaitlistSlotFreedEmail struct {
	EntryID     string `json:"entry_id"`
	Email       string `json:"email"`
	ComplexName string `json:"complex_name"`
	DateLabel   string `json:"date_label"`
	BookURL     string `json:"book_url"`
}

// WaitlistSlotFreed queues the notice for one waitlist entry.
func (s *Service) WaitlistSlotFreed(e WaitlistSlotFreedEmail) {
	s.queue.Enqueue(TaskEmailWaitlistSlotFreed, e, dedup(TaskEmailWaitlistSlotFreed, e.Email, e.EntryID))
}

// registerWaitlistWorkers wires the delivery of the notice. RegisterWorkers calls it.
func (s *Service) registerWaitlistWorkers() {
	s.queue.RegisterHandler(handle(TaskEmailWaitlistSlotFreed, func(ctx context.Context, p WaitlistSlotFreedEmail) error {
		return s.mailer.SendWaitlistSlotFreed(ctx, p.Email, p.ComplexName, p.DateLabel, p.BookURL)
	}))
}
