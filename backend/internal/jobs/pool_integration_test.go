//go:build integration

package jobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/jobs"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// waitFor polls until cond holds, so a test does not sleep for a worker's poll
// interval it cannot see.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestThePoolRunsAJobAndAcknowledgesIt is the end-to-end shape: a job goes in
// through the enqueuer, a worker claims it, the registered handler runs with
// the payload it was given, and the row is closed out.
func TestThePoolRunsAJobAndAcknowledgesIt(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)

	var mu sync.Mutex
	var seen []string
	pool := jobs.NewPool(s, jobs.Config{
		Workers: 2, PollInterval: 10 * time.Millisecond, Logger: discardLogger(),
	})
	pool.RegisterHandler(jobType, func(_ context.Context, payload json.RawMessage) error {
		var p struct {
			To string `json:"to"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, p.To)
		return nil
	})

	id := enqueue(t, s, ctx, jobType, map[string]string{"to": "ana@example.com"}, time.Time{}, 5, "")

	pool.Start()
	t.Cleanup(pool.Shutdown)

	waitFor(t, "the job to be delivered", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(seen) == 1
	})
	if seen[0] != "ana@example.com" {
		t.Errorf("the handler was given %q", seen[0])
	}

	waitFor(t, "the job to be acknowledged", func() bool {
		got, err := s.Get(ctx, id)
		return err == nil && got.Status == jobs.StatusDone
	})
}

// TestShutdownFinishesTheJobInHand is JOB-05. A graceful stop that abandoned a
// claim would leave the row invisible for a whole lease, and the work — an
// email somebody is waiting for — would arrive a minute late at best.
func TestShutdownFinishesTheJobInHand(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)

	running := make(chan struct{})
	release := make(chan struct{})
	pool := jobs.NewPool(s, jobs.Config{
		Workers: 1, PollInterval: 10 * time.Millisecond, JobTimeout: 10 * time.Second,
		Logger: discardLogger(),
	})
	pool.RegisterHandler(jobType, func(context.Context, json.RawMessage) error {
		close(running)
		<-release
		return nil
	})

	id := enqueue(t, s, ctx, jobType, map[string]int{"n": 1}, time.Time{}, 5, "")
	pool.Start()

	<-running

	stopped := make(chan struct{})
	go func() {
		pool.Shutdown()
		close(stopped)
	}()

	// Shutdown must still be waiting: the handler has not returned.
	select {
	case <-stopped:
		t.Fatal("Shutdown returned while a job was still running; the claim was abandoned")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not return after the job finished")
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != jobs.StatusDone {
		t.Errorf("status after a graceful shutdown = %q, want %q; the job in hand was not finished", got.Status, jobs.StatusDone)
	}
}

// TestAPanickingHandlerIsARetryNotACrash: a panic in one job must cost that
// job an attempt, not the process and not the eleven other jobs the worker
// would have run.
func TestAPanickingHandlerIsARetryNotACrash(t *testing.T) {
	s, jobType := newStore(t)
	s.Backoff = []time.Duration{time.Nanosecond}
	ctx := bypass(t)

	pool := jobs.NewPool(s, jobs.Config{
		Workers: 1, PollInterval: 10 * time.Millisecond, Logger: discardLogger(),
	})
	pool.RegisterHandler(jobType, func(context.Context, json.RawMessage) error {
		panic("brevo client dereferenced a nil response")
	})

	id := enqueue(t, s, ctx, jobType, map[string]int{"n": 1}, time.Time{}, 2, "")
	pool.Start()
	t.Cleanup(pool.Shutdown)

	waitFor(t, "the panicking job to dead-letter", func() bool {
		got, err := s.Get(ctx, id)
		return err == nil && got.Status == jobs.StatusFailed
	})

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.LastError == "" {
		t.Error("the dead-lettered job records no reason, so nothing says it panicked")
	}
}

// TestAPermanentRefusalDoesNotSpendTheWholeBudget: a provider that answered
// "no" about this job answers the same way five times, so the queue stops
// asking.
func TestAPermanentRefusalDoesNotSpendTheWholeBudget(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)

	pool := jobs.NewPool(s, jobs.Config{
		Workers: 1, PollInterval: 10 * time.Millisecond, Logger: discardLogger(),
	})
	pool.RegisterHandler(jobType, func(context.Context, json.RawMessage) error {
		return errors.New("brevo: invalid recipient: " + jobs.ErrPermanent.Error())
	})
	pool.RegisterHandler(jobType+":sentinel", func(context.Context, json.RawMessage) error {
		return jobs.ErrPermanent
	})

	id := enqueue(t, s, ctx, jobType+":sentinel", map[string]int{"n": 1}, time.Time{}, 5, "")
	pool.Start()
	t.Cleanup(pool.Shutdown)

	waitFor(t, "the refused job to dead-letter", func() bool {
		got, err := s.Get(ctx, id)
		return err == nil && got.Status == jobs.StatusFailed
	})

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Attempts != 1 {
		t.Errorf("attempts before the dead letter = %d, want 1; a permanent refusal spent the whole budget", got.Attempts)
	}
}
