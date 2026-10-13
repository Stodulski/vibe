package notifications

import "github.com/stodulski/vibe-server/internal/jobs"

// TaskEmailWelcome is the one welcome an owner gets when their account first
// becomes usable: the address is verified, or a Google sign-up creates it.
const TaskEmailWelcome = "email:welcome"

// WelcomeEmail is one welcome, resolved before it is queued. The mailer builds
// the links from its own configured origins.
type WelcomeEmail struct {
	To        string `json:"to"`
	FirstName string `json:"first_name"`
}

// Welcome queues the welcome for one owner. userID is the account. With the task
// type it forms the deduplication key, so the owner gets one welcome however
// often the trigger runs, including a verification retried or a task redelivered.
func (s *Service) Welcome(e WelcomeEmail, userID string) {
	key := jobs.DedupKey(TaskEmailWelcome, "welcome:"+userID)
	s.queue.Enqueue(TaskEmailWelcome, e, key)
}
