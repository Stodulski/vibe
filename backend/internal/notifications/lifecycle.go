package notifications

import (
	"context"

	"github.com/stodulski/vibe-server/internal/jobs"
)

// TaskEmailLifecycleNudge is one owner or lead nudge: the reminder that an account
// stalled before taking bookings. The lifecycle package decides who is due; this
// task only delivers the email.
const TaskEmailLifecycleNudge = "email:lifecycle_nudge"

// LifecycleNudgeEmail is one nudge, fully resolved before it is queued: the
// recipient, the copy's variables and the two opt-out links. The links are signed
// at enqueue time, so a queued email still unsubscribes whatever the address
// happens to be called by delivery.
type LifecycleNudgeEmail struct {
	Kind        string `json:"kind"`
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	ComplexName string `json:"complex_name"`
	// CTAURL is the button's target: the verification link, the onboarding page,
	// the courts page, the panel, or the register page.
	CTAURL string `json:"cta_url"`
	// UnsubscribeURL is the API one-click endpoint, sent as the List-Unsubscribe
	// header.
	UnsubscribeURL string `json:"unsubscribe_url"`
	// OptOutURL is the /baja page, linked from the footer.
	OptOutURL string `json:"opt_out_url"`
}

// LifecycleNudge queues one nudge. subjectID is what the nudge is about, the owner
// id or a lead's address; with the kind it forms the deduplication key, so one
// owner gets one nudge of each kind even if the task is redelivered.
func (s *Service) LifecycleNudge(e LifecycleNudgeEmail, subjectID string) {
	key := jobs.DedupKey(TaskEmailLifecycleNudge, "nudge:"+e.Kind+":"+subjectID)
	s.queue.Enqueue(TaskEmailLifecycleNudge, e, key)
}

// registerLifecycleWorkers wires the delivery of a nudge. RegisterWorkers calls it.
func (s *Service) registerLifecycleWorkers() {
	s.queue.RegisterHandler(handle(TaskEmailLifecycleNudge, func(ctx context.Context, p LifecycleNudgeEmail) error {
		return s.mailer.SendLifecycleNudge(ctx, p.Email, p.Kind, p.FirstName, p.ComplexName,
			p.CTAURL, p.UnsubscribeURL, p.OptOutURL)
	}))
}
