package notifications

import "context"

// TaskEmailWaitlistPlaceFreed tells someone who asked to be told that one place
// they waited for has opened up: a spot in an open-play event, a pair's spot in
// a tournament category, or a taken court slot.
const TaskEmailWaitlistPlaceFreed = "email:waitlist_place_freed"

// WaitlistPlaceFreedEmail is one place-waitlist entry's notice. EntryID is the
// place_waitlist_entries row and NoticeAt the instant the sweep told it; the two
// together are the deduplication subject, so one notice yields one email even if
// its task is redelivered, while a row put back on the list after someone else
// took the place gets a new email the next time it frees. PlaceLabel says what freed, in the
// words the email shows ("Cancha 1 el miércoles 18/11 a las 19:00").
type WaitlistPlaceFreedEmail struct {
	EntryID     string `json:"entry_id"`
	NoticeAt    string `json:"notice_at"`
	Email       string `json:"email"`
	ComplexName string `json:"complex_name"`
	PlaceLabel  string `json:"place_label"`
	BookURL     string `json:"book_url"`
}

// WaitlistPlaceFreed queues the notice for one place-waitlist entry.
func (s *Service) WaitlistPlaceFreed(e WaitlistPlaceFreedEmail) {
	s.queue.Enqueue(TaskEmailWaitlistPlaceFreed, e, dedup(TaskEmailWaitlistPlaceFreed, e.Email, e.EntryID+"@"+e.NoticeAt))
}

// registerWaitlistPlaceWorkers wires the delivery of the notice. RegisterWorkers calls it.
func (s *Service) registerWaitlistPlaceWorkers() {
	s.queue.RegisterHandler(handle(TaskEmailWaitlistPlaceFreed, func(ctx context.Context, p WaitlistPlaceFreedEmail) error {
		return s.mailer.SendWaitlistPlaceFreed(ctx, p.Email, p.ComplexName, p.PlaceLabel, p.BookURL)
	}))
}
