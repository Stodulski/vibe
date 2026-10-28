//go:build integration

package jobs_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/jobs"
)

// secretPayload stands in for a notification payload: a single-use cancel link
// is exactly what must not outlive the attempt that needed it.
type secretPayload struct {
	CancelURL string `json:"cancel_url"`
}

// isEmptyObject reports whether raw is the JSON object {}. It never fails the
// test, so it can sit inside a waitFor condition.
func isEmptyObject(raw json.RawMessage) bool {
	var m map[string]any
	return json.Unmarshal(raw, &m) == nil && len(m) == 0
}

// TestClearPayloadEmptiesAFinishedJobAndKeepsItsDedupKey pins the half of the
// fix that lives in the store. The row stays for the retention window, and so
// does its DedupKey: deleting the row or the key would let a redelivery run the
// same notification twice.
func TestClearPayloadEmptiesAFinishedJobAndKeepsItsDedupKey(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)
	key := "clear-" + jobType

	id := enqueue(t, s, ctx, jobType, secretPayload{CancelURL: "https://vibe.example/cancel/SECRET"}, time.Time{}, 5, key)
	if _, err := s.ClaimType(ctx, "worker", jobType, 10); err != nil {
		t.Fatalf("ClaimType: %v", err)
	}
	if err := s.Complete(ctx, id); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if err := s.ClearPayload(ctx, id); err != nil {
		t.Fatalf("ClearPayload: %v", err)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != jobs.StatusDone {
		t.Errorf("status = %q, want %q", got.Status, jobs.StatusDone)
	}
	if !isEmptyObject(got.Payload) {
		t.Errorf("a finished job kept its payload: %s", got.Payload)
	}

	_, recorded, err := s.Enqueue(ctx, jobType, secretPayload{}, time.Time{}, 5, key)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if recorded {
		t.Error("clearing the payload freed the dedup key: the same work could be queued again while its row still exists")
	}
}

// TestClearPayloadEmptiesADeadLetter covers the other terminal state. A failed
// row is never deleted by the retention sweep, so without this its cancel link
// would stay readable for as long as the database does.
func TestClearPayloadEmptiesADeadLetter(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)

	id := enqueue(t, s, ctx, jobType, secretPayload{CancelURL: "https://vibe.example/cancel/DEAD"}, time.Time{}, 1, "")
	if _, err := s.ClaimType(ctx, "worker", jobType, 10); err != nil {
		t.Fatalf("ClaimType: %v", err)
	}
	dead, err := s.Fail(ctx, id, "provider unavailable")
	if err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if !dead {
		t.Fatal("a budget of one should dead-letter the first failure")
	}
	if err := s.ClearPayload(ctx, id); err != nil {
		t.Fatalf("ClearPayload: %v", err)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != jobs.StatusFailed {
		t.Errorf("status = %q, want %q", got.Status, jobs.StatusFailed)
	}
	if !isEmptyObject(got.Payload) {
		t.Errorf("a dead-lettered job kept its payload: %s", got.Payload)
	}
}

// TestClearPayloadLeavesWorkThatHasNotFinished is the guard on the status
// predicate. A pending or claimed job still needs its payload: a retry or a
// reclaim runs the handler again with exactly those bytes.
func TestClearPayloadLeavesWorkThatHasNotFinished(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)

	pending := enqueue(t, s, ctx, jobType, secretPayload{CancelURL: "https://vibe.example/cancel/PENDING"}, time.Now().Add(time.Hour), 5, "")
	claimed := enqueue(t, s, ctx, jobType+":claimed", secretPayload{CancelURL: "https://vibe.example/cancel/CLAIMED"}, time.Time{}, 5, "")
	if _, err := s.ClaimType(ctx, "worker", jobType+":claimed", 10); err != nil {
		t.Fatalf("ClaimType: %v", err)
	}

	if err := s.ClearPayload(ctx, pending); err != nil {
		t.Fatalf("ClearPayload(pending): %v", err)
	}
	if err := s.ClearPayload(ctx, claimed); err != nil {
		t.Fatalf("ClearPayload(claimed): %v", err)
	}

	gotPending, err := s.Get(ctx, pending)
	if err != nil {
		t.Fatalf("Get(pending): %v", err)
	}
	if !strings.Contains(string(gotPending.Payload), "PENDING") {
		t.Errorf("a pending job lost the payload its run still needs: %s", gotPending.Payload)
	}
	gotClaimed, err := s.Get(ctx, claimed)
	if err != nil {
		t.Fatalf("Get(claimed): %v", err)
	}
	if !strings.Contains(string(gotClaimed.Payload), "CLAIMED") {
		t.Errorf("a claimed job lost the payload its run still needs: %s", gotClaimed.Payload)
	}
}

// TestThePoolEmptiesThePayloadsOfTheTypesItIsTold runs the policy end to end.
// Only the configured types are scrubbed, once they finish done or dead, and
// the other type keeps its payload untouched.
func TestThePoolEmptiesThePayloadsOfTheTypesItIsTold(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)

	scrubbedType := jobType
	deadType := jobType + ":dead"
	keptType := jobType + ":kept"

	scrubbed := enqueue(t, s, ctx, scrubbedType, secretPayload{CancelURL: "https://vibe.example/cancel/OK"}, time.Time{}, 5, "")
	dead := enqueue(t, s, ctx, deadType, secretPayload{CancelURL: "https://vibe.example/cancel/DEAD"}, time.Time{}, 5, "")
	kept := enqueue(t, s, ctx, keptType, secretPayload{CancelURL: "https://vibe.example/cancel/KEPT"}, time.Time{}, 5, "")

	pool := jobs.NewPool(s, jobs.Config{
		Workers: 2, PollInterval: 10 * time.Millisecond, Logger: discardLogger(),
		ScrubPayload: func(taskType string) bool {
			return taskType == scrubbedType || taskType == deadType
		},
	})
	pool.RegisterHandler(scrubbedType, func(_ context.Context, _ json.RawMessage) error { return nil })
	pool.RegisterHandler(deadType, func(_ context.Context, _ json.RawMessage) error { return jobs.ErrPermanent })
	pool.RegisterHandler(keptType, func(_ context.Context, _ json.RawMessage) error { return nil })
	pool.Start()
	defer pool.Shutdown()

	waitFor(t, "the scrubbed payloads to be cleared and the kept job to finish", func() bool {
		a, errA := s.Get(ctx, scrubbed)
		b, errB := s.Get(ctx, dead)
		k, errK := s.Get(ctx, kept)
		if errA != nil || errB != nil || errK != nil {
			return false
		}
		return a.Status == jobs.StatusDone && isEmptyObject(a.Payload) &&
			b.Status == jobs.StatusFailed && isEmptyObject(b.Payload) &&
			k.Status == jobs.StatusDone
	})

	got, err := s.Get(ctx, kept)
	if err != nil {
		t.Fatalf("Get(kept): %v", err)
	}
	if !strings.Contains(string(got.Payload), "KEPT") {
		t.Errorf("a type the policy does not name lost its payload: %s", got.Payload)
	}
}
