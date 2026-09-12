//go:build integration

package jobs_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/jobs"
)

// newStore opens a store against the E2E database.
//
// Every test below invents its own job type — named after the test itself
// plus a UUID, so a row is traceable back to the test that left it and never
// collides with another run's — so the rows one test writes are invisible to
// the claim of another. That still is not enough on its own: Store.Claim and
// Pool.work both take the oldest *due* rows off the whole table with no type
// filter, so a row a test leaves behind in 'pending' (Release's open-breaker
// case, or a backoff that later elapses) keeps competing for every later
// test's Claim budget once its run_at arrives. The Cleanup below deletes only
// this test's own rows, by type prefix, so nothing outlives the test that
// created it — which is what lets this file run beside the rest of the
// integration suite, sharing one database, under any run order or repeat
// count, without truncating it.
func newStore(t *testing.T) (*jobs.Store, string) {
	t.Helper()
	pool := datatest.SetupTestDB(t)
	s := &jobs.Store{DB: data.NewDB(pool)}

	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	jobType := "test:" + name + ":" + uuid.NewString()

	// Prefix match, not equality: a pool test registers handlers (and
	// enqueues) under jobType plus a suffix such as ":sentinel" or ":fails",
	// and every one of those rows belongs to this test alone.
	t.Cleanup(func() {
		ctx := data.ContextWithTenantBypass(context.Background())
		if _, err := s.DB.Exec(ctx, `DELETE FROM jobs WHERE type LIKE $1`, jobType+"%"); err != nil {
			t.Logf("cleanup: deleting jobs of type %q: %v", jobType, err)
		}
	})

	return s, jobType
}

// bypass is the tenant scope every background worker runs under. jobs carries
// no tenant policy, but the pool's checkout hook stamps a scope on every
// connection and a context with none set is rejected before the statement.
func bypass(t *testing.T) context.Context {
	t.Helper()
	return data.ContextWithTenantBypass(t.Context())
}

func enqueue(t *testing.T, s *jobs.Store, ctx context.Context, jobType string, payload any, runAt time.Time, maxAttempts int, dedupKey string) uuid.UUID {
	t.Helper()
	id, recorded, err := s.Enqueue(ctx, jobType, payload, runAt, maxAttempts, dedupKey)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if !recorded {
		t.Fatalf("Enqueue: the job was deduplicated away; this test expected it to be recorded")
	}
	return id
}

// TestOnlyOneWorkerClaimsAJob is the whole reason this table exists. Two
// workers claim at the same instant against the same row; SKIP LOCKED has to
// give it to exactly one of them, and — this is the part a conditional UPDATE
// does not give you — the loser must not block waiting to find out.
func TestOnlyOneWorkerClaimsAJob(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)
	id := enqueue(t, s, ctx, jobType, map[string]string{"to": "ana@example.com"}, time.Time{}, 5, "")

	var wg sync.WaitGroup
	claims := make([][]*jobs.Job, 2)
	errs := make([]error, 2)
	start := make(chan struct{})

	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			claims[i], errs[i] = s.ClaimType(bypass(t), "worker-"+string(rune('a'+i)), jobType, 10)
		}()
	}
	close(start)
	wg.Wait()

	winners := 0
	for i := range 2 {
		if errs[i] != nil {
			t.Fatalf("worker %d: Claim: %v", i, errs[i])
		}
		for _, claimed := range claims[i] {
			if claimed.ID == id {
				winners++
			}
		}
	}
	if winners != 1 {
		t.Fatalf("%d workers claimed the same job; want exactly 1", winners)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != jobs.StatusProcessing {
		t.Errorf("status after the claim = %q, want %q", got.Status, jobs.StatusProcessing)
	}
	if got.Attempts != 1 {
		t.Errorf("attempts after one claim = %d, want 1; the claim is what spends the budget", got.Attempts)
	}
	if got.LockedBy == "" || got.LockedAt == nil {
		t.Error("the claim left no owner on the row, so the stale sweep has nothing to measure")
	}
}

// TestAClaimTakesOnlyDueJobs pins the two filters a claim applies: a job
// scheduled for later is not work yet, and one already claimed is not work
// twice.
func TestAClaimTakesOnlyDueJobs(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)

	due := enqueue(t, s, ctx, jobType, map[string]int{"n": 1}, time.Time{}, 5, "")
	later := enqueue(t, s, ctx, jobType, map[string]int{"n": 2}, time.Now().Add(time.Hour), 5, "")

	claimed, err := s.ClaimType(ctx, "worker", jobType, 10)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}

	taken := map[uuid.UUID]bool{}
	for _, j := range claimed {
		taken[j.ID] = true
	}
	if !taken[due] {
		t.Error("the due job was not claimed")
	}
	if taken[later] {
		t.Error("a job scheduled for an hour from now was claimed; run_at is not being honoured")
	}
}

// TestAFailedAttemptComesBackOnItsBackoff covers the retry half of a failure:
// the job returns to pending, its error is recorded, and it is not due again
// immediately.
func TestAFailedAttemptComesBackOnItsBackoff(t *testing.T) {
	s, jobType := newStore(t)
	s.Backoff = []time.Duration{30 * time.Second}
	ctx := bypass(t)
	id := enqueue(t, s, ctx, jobType, map[string]string{"to": "ana@example.com"}, time.Time{}, 5, "")

	if _, err := s.ClaimType(ctx, "worker", jobType, 10); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	dead, err := s.Fail(ctx, id, "brevo: 502 bad gateway")
	if err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if dead {
		t.Fatal("the first of five attempts dead-lettered the job")
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != jobs.StatusPending {
		t.Errorf("status after one failure = %q, want %q", got.Status, jobs.StatusPending)
	}
	if got.LastError == "" {
		t.Error("the failure recorded no reason; the dead letter would say nothing about why")
	}
	if got.LockedBy != "" || got.LockedAt != nil {
		t.Error("a failed attempt left its claim on the row, so the stale sweep would reclaim a job that is already pending")
	}
	if !got.RunAt.After(time.Now().Add(20 * time.Second)) {
		t.Errorf("run_at after a failure is %v, which is sooner than the backoff; the retry would fire at once", got.RunAt)
	}

	// A second claim on the same tick must find nothing: the backoff is what
	// stops a failing provider being hammered.
	claimed, err := s.ClaimType(ctx, "worker", jobType, 10)
	if err != nil {
		t.Fatalf("Claim after the failure: %v", err)
	}
	for _, j := range claimed {
		if j.ID == id {
			t.Error("the failed job was claimable again immediately; the backoff is not applied")
		}
	}
}

// TestAJobDeadLettersWhenItsBudgetIsSpent is the terminal case. The row has to
// stop being claimable and stay readable: it is the only record that this
// system accepted work and could not do it.
func TestAJobDeadLettersWhenItsBudgetIsSpent(t *testing.T) {
	s, jobType := newStore(t)
	s.Backoff = []time.Duration{time.Nanosecond}
	ctx := bypass(t)
	id := enqueue(t, s, ctx, jobType, map[string]string{"to": "ana@example.com"}, time.Time{}, 2, "")

	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := s.ClaimType(ctx, "worker", jobType, 10); err != nil {
			t.Fatalf("attempt %d: Claim: %v", attempt, err)
		}
		dead, err := s.Fail(ctx, id, "brevo: 502 bad gateway")
		if err != nil {
			t.Fatalf("attempt %d: Fail: %v", attempt, err)
		}
		if dead != (attempt == 2) {
			t.Fatalf("attempt %d of 2: Fail reported dead = %v", attempt, dead)
		}
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != jobs.StatusFailed {
		t.Fatalf("status after the budget was spent = %q, want %q", got.Status, jobs.StatusFailed)
	}

	claimed, err := s.ClaimType(ctx, "worker", jobType, 10)
	if err != nil {
		t.Fatalf("Claim after the dead letter: %v", err)
	}
	for _, j := range claimed {
		if j.ID == id {
			t.Error("a dead-lettered job was claimed again; 'failed' is meant to be terminal")
		}
	}

	// Retention must not touch it either: a failed row is money or mail that
	// never went out, and no timer may delete that.
	if _, err := s.DeleteDone(ctx, 0); err != nil {
		t.Fatalf("DeleteDone: %v", err)
	}
	if _, err := s.Get(ctx, id); err != nil {
		t.Errorf("the retention sweep deleted a dead-lettered job: %v", err)
	}
}

// TestARefusedAttemptCostsNoAttempt is the outage rule (the queue's half of
// OUT-02's reasoning): the budget bounds how many times a provider that is
// answering is asked, and an attempt that never reached one is not evidence
// about this job.
func TestARefusedAttemptCostsNoAttempt(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)
	id := enqueue(t, s, ctx, jobType, map[string]string{"to": "ana@example.com"}, time.Time{}, 5, "")

	if _, err := s.ClaimType(ctx, "worker", jobType, 10); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := s.Release(ctx, id, time.Now().Add(-time.Minute), "mp: circuit breaker is open"); err != nil {
		t.Fatalf("Release: %v", err)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Attempts != 0 {
		t.Errorf("attempts after a refused attempt = %d, want 0; an open breaker spent part of the budget", got.Attempts)
	}
	if got.Status != jobs.StatusPending {
		t.Errorf("status after a refused attempt = %q, want %q", got.Status, jobs.StatusPending)
	}
}

// TestADedupKeyMakesASecondEnqueueANoOp is JOB-04 at the table. The queue is
// at-least-once, so this is the only thing between a redelivered webhook and a
// second confirmation email.
func TestADedupKeyMakesASecondEnqueueANoOp(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)
	key := jobs.DedupKey(jobType, "ana@example.com", uuid.NewString())

	first := enqueue(t, s, ctx, jobType, map[string]string{"to": "ana@example.com"}, time.Time{}, 5, key)

	_, recorded, err := s.Enqueue(ctx, jobType, map[string]string{"to": "ana@example.com"}, time.Time{}, 5, key)
	if err != nil {
		t.Fatalf("the second Enqueue failed instead of being a no-op: %v", err)
	}
	if recorded {
		t.Fatal("the second Enqueue under the same key recorded another job; the client gets two emails")
	}

	claimed, err := s.ClaimType(ctx, "worker", jobType, 10)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	mine := 0
	for _, j := range claimed {
		if j.Type == jobType {
			mine++
		}
	}
	if mine != 1 {
		t.Errorf("%d jobs of this type are queued; want 1", mine)
	}

	// The key protects for as long as the row lives, in every state — a job
	// already running is exactly the redelivery window that matters.
	if _, recorded, err = s.Enqueue(ctx, jobType, map[string]string{"to": "ana@example.com"}, time.Time{}, 5, key); err != nil {
		t.Fatalf("Enqueue against a claimed job: %v", err)
	}
	if recorded {
		t.Error("the key stopped protecting once the job was claimed, which is when a redelivery arrives")
	}

	if err := s.Complete(ctx, first); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if _, recorded, err = s.Enqueue(ctx, jobType, map[string]string{"to": "ana@example.com"}, time.Time{}, 5, key); err != nil {
		t.Fatalf("Enqueue against a finished job: %v", err)
	}
	if recorded {
		t.Error("the key stopped protecting once the job was done; a redelivery would send a second email")
	}
}

// TestKeylessJobsDoNotDeduplicateEachOther is the other half: most work has no
// key, and a NULL key must not collide with the next NULL key.
func TestKeylessJobsDoNotDeduplicateEachOther(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)

	enqueue(t, s, ctx, jobType, map[string]int{"n": 1}, time.Time{}, 5, "")
	enqueue(t, s, ctx, jobType, map[string]int{"n": 2}, time.Time{}, 5, "")

	claimed, err := s.ClaimType(ctx, "worker", jobType, 10)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	mine := 0
	for _, j := range claimed {
		if j.Type == jobType {
			mine++
		}
	}
	if mine != 2 {
		t.Errorf("%d keyless jobs are queued; want 2 — a NULL dedup key is collapsing rows", mine)
	}
}

// TestAnAbandonedClaimIsReclaimed is what makes a worker that died mid-handle
// recoverable without waiting for that process to come back.
func TestAnAbandonedClaimIsReclaimed(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)
	id := enqueue(t, s, ctx, jobType, map[string]string{"to": "ana@example.com"}, time.Time{}, 5, "")

	if _, err := s.ClaimType(ctx, "worker-that-died", jobType, 10); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	// A lease longer than the claim is old: nothing moves.
	if _, err := s.ReclaimStale(ctx, time.Hour); err != nil {
		t.Fatalf("ReclaimStale: %v", err)
	}
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != jobs.StatusProcessing {
		t.Fatalf("an honest in-flight attempt was reclaimed after %q; status = %q", time.Hour, got.Status)
	}

	// A lease already elapsed: the claim comes back.
	moved, err := s.ReclaimStale(ctx, 0)
	if err != nil {
		t.Fatalf("ReclaimStale: %v", err)
	}
	if moved < 1 {
		t.Fatal("ReclaimStale moved nothing; an abandoned claim would be invisible forever")
	}

	got, err = s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != jobs.StatusPending {
		t.Errorf("status after the reclaim = %q, want %q", got.Status, jobs.StatusPending)
	}
	if got.LockedBy != "" || got.LockedAt != nil {
		t.Error("the reclaim left the dead worker's claim on the row")
	}
	if got.Attempts != 1 {
		t.Errorf("attempts after a reclaim = %d, want 1; the attempt the dead worker spent has to stay spent, "+
			"or a payload that kills whichever instance reads it circulates forever", got.Attempts)
	}
}

// TestRetentionDeletesFinishedJobsAndNothingElse pins which rows a timer may
// remove. It is also how long a dedup key protects for, which is why it is
// worth being explicit about.
func TestRetentionDeletesFinishedJobsAndNothingElse(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)

	done := enqueue(t, s, ctx, jobType, map[string]int{"n": 1}, time.Time{}, 5, "")
	pending := enqueue(t, s, ctx, jobType, map[string]int{"n": 2}, time.Now().Add(time.Hour), 5, "")

	if _, err := s.ClaimType(ctx, "worker", jobType, 10); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := s.Complete(ctx, done); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if _, err := s.DeleteDone(ctx, 0); err != nil {
		t.Fatalf("DeleteDone: %v", err)
	}
	if _, err := s.Get(ctx, done); err == nil {
		t.Error("a finished job survived the retention sweep")
	}
	if _, err := s.Get(ctx, pending); err != nil {
		t.Errorf("the retention sweep deleted a job that has not run: %v", err)
	}
}

// TestThePayloadSurvivesTheRoundTrip is the jsonb parameter trap, pinned. The
// API pool runs in QueryExecModeExec, where a []byte bound to a jsonb column
// is sent as bytea and refused — which is how every MercadoPago delivery
// started answering 500 once, against a suite that ran in a mode that hid it.
func TestThePayloadSurvivesTheRoundTrip(t *testing.T) {
	s, jobType := newStore(t)
	ctx := bypass(t)

	type payload struct {
		To      string `json:"to"`
		Attempt int    `json:"attempt"`
	}
	id := enqueue(t, s, ctx, jobType, payload{To: "ana@example.com", Attempt: 3}, time.Time{}, 5, "")

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	var back payload
	if err := json.Unmarshal(got.Payload, &back); err != nil {
		t.Fatalf("the stored payload does not decode: %v", err)
	}
	if back.To != "ana@example.com" || back.Attempt != 3 {
		t.Errorf("payload round-tripped as %+v", back)
	}
}
