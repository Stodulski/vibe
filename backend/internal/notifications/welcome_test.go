package notifications

import (
	"encoding/json"
	"testing"

	"github.com/stodulski/vibe-server/internal/jobs"
)

// The welcome is one delivery per owner, however many times its trigger runs.
// The key is the owner's id under the welcome task, the same shape the
// lifecycle nudges use, so a second verification or a redelivered task is the
// same job to the queue.
func TestWelcomeIsQueuedUnderTheOwnersDedupKey(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)
	const userID = "7d9a4f5e-2c1b-4e8a-9f3d-1a2b3c4d5e6f"
	email := WelcomeEmail{To: "ana@example.com", FirstName: "Ana"}

	s.Welcome(email, userID)
	if len(q.tasks) != 1 || q.tasks[0].taskType != TaskEmailWelcome {
		t.Fatalf("want one %s task; got %v", TaskEmailWelcome, q.types())
	}
	if got, want := q.tasks[0].dedupKey, jobs.DedupKey(TaskEmailWelcome, "welcome:"+userID); got != want {
		t.Errorf("dedup key = %q, want the owner key %q", got, want)
	}

	s.Welcome(email, userID)
	if len(q.tasks) != 2 || q.tasks[1].dedupKey != q.tasks[0].dedupKey {
		t.Error("a second welcome for the same owner has a different key; the queue would send it twice")
	}
}

func TestWelcomeWorkerDeliversTheWelcome(t *testing.T) {
	s, q, mailer, _, _ := newTestService(t, true)
	s.RegisterWorkers()

	payload, _ := json.Marshal(WelcomeEmail{To: "ana@example.com", FirstName: "Ana"})
	if err := q.handlers[TaskEmailWelcome](t.Context(), payload); err != nil {
		t.Fatalf("delivering the welcome: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].kind != "welcome" || mailer.sent[0].to != "ana@example.com" {
		t.Errorf("the welcome was not delivered; got %v", mailer.sent)
	}
}
