package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testRedis returns a client against an in-process Redis.
//
// It used to dial localhost and t.Skipf when nothing answered, which meant
// every test in this file passed by not running — in CI, always. The queue's
// durability guarantees are exactly the kind that only hold while something
// checks them, so they are checked against miniredis, the same fake
// internal/auth/blacklist_redis_test.go uses.
func testRedis(t *testing.T) *redis.Client {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// fastConfig is the production protocol with every interval scaled down.
func fastConfig() Config {
	return Config{
		Workers:           1,
		TaskTimeout:       50 * time.Millisecond,
		MaxRetries:        2,
		RetryBackoff:      []time.Duration{20 * time.Millisecond},
		NotAttemptedDelay: 20 * time.Millisecond,
		VisibilityTimeout: 300 * time.Millisecond,
		ReclaimInterval:   20 * time.Millisecond,
		ScheduleInterval:  10 * time.Millisecond,
		PollInterval:      time.Second, // Redis block timeouts have one-second resolution.
		TaskTTL:           time.Hour,
	}
}

func waitFor(t *testing.T, desc string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", desc)
}

func llen(t *testing.T, rdb *redis.Client, key string) int64 {
	t.Helper()
	n, err := rdb.LLen(context.Background(), key).Result()
	if err != nil {
		t.Fatalf("LLen(%s): %v", key, err)
	}
	return n
}

func zcard(t *testing.T, rdb *redis.Client) int64 {
	t.Helper()
	n, err := rdb.ZCard(context.Background(), delayedKey).Result()
	if err != nil {
		t.Fatalf("ZCard(%s): %v", delayedKey, err)
	}
	return n
}

// scheduled returns the tasks currently parked in the delayed set.
func scheduled(t *testing.T, rdb *redis.Client) []task {
	t.Helper()

	raws, err := rdb.ZRange(context.Background(), delayedKey, 0, -1).Result()
	if err != nil {
		t.Fatalf("ZRange(%s): %v", delayedKey, err)
	}
	out := make([]task, 0, len(raws))
	for _, raw := range raws {
		var parsed task
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			t.Fatalf("unmarshal delayed task: %v", err)
		}
		out = append(out, parsed)
	}
	return out
}

// ---------------------------------------------------------------------------
// Baseline
// ---------------------------------------------------------------------------

func TestEnqueueAndProcess(t *testing.T) {
	rdb := testRedis(t)

	var count atomic.Int32
	cfg := fastConfig()
	cfg.Workers = 2

	n := New(rdb, testLogger(), cfg)
	n.RegisterHandler("test", func(_ context.Context, _ json.RawMessage) error {
		count.Add(1)
		return nil
	})
	n.Start()
	defer n.Shutdown()

	for range 5 {
		n.Enqueue("test", map[string]string{"key": "value"})
	}

	waitFor(t, "5 tasks processed", func() bool { return count.Load() == 5 })
	waitFor(t, "processing list drained", func() bool { return llen(t, rdb, processingKey) == 0 })

	if got := llen(t, rdb, queueKey); got != 0 {
		t.Fatalf("pending list = %d, want 0", got)
	}
	if got := llen(t, rdb, deadKey); got != 0 {
		t.Fatalf("dead-letter list = %d, want 0", got)
	}
}

func TestTasksSurviveRestart(t *testing.T) {
	rdb := testRedis(t)

	// Enqueue without starting workers — tasks sit in Redis.
	n1 := New(rdb, testLogger(), fastConfig())
	n1.Enqueue("persist", map[string]string{"val": "1"})
	n1.Enqueue("persist", map[string]string{"val": "2"})

	if got := llen(t, rdb, queueKey); got != 2 {
		t.Fatalf("pending list = %d, want 2", got)
	}

	var count atomic.Int32
	n2 := New(rdb, testLogger(), fastConfig())
	n2.RegisterHandler("persist", func(_ context.Context, _ json.RawMessage) error {
		count.Add(1)
		return nil
	})
	n2.Start()
	defer n2.Shutdown()

	waitFor(t, "2 tasks processed after restart", func() bool { return count.Load() == 2 })
}

// ---------------------------------------------------------------------------
// Durability: the four ways a task used to disappear
// ---------------------------------------------------------------------------

// TestTaskSurvivesWorkerDeathMidHandle is the direct inverse of the measured
// symptom that started this: while a handler was in flight, LLEN was 0 and
// KEYS vibe:* was empty, because BRPOP had destroyed the only copy.
func TestTaskSurvivesWorkerDeathMidHandle(t *testing.T) {
	rdb := testRedis(t)

	release := make(chan struct{})
	started := make(chan struct{}, 1)

	// The dying process. Its own sweeper is pushed out of the way so the
	// recovery under test is another instance's, not its own.
	dyingCfg := fastConfig()
	dyingCfg.ReclaimInterval = time.Hour

	dying := New(rdb, testLogger(), dyingCfg)
	dying.RegisterHandler("mail", func(_ context.Context, _ json.RawMessage) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return nil
	})
	dying.Start()
	dying.Enqueue("mail", map[string]string{"to": "client@example.com"})

	<-started

	// The task is not nowhere: it is claimed, and the claim is in Redis.
	waitFor(t, "task visible in the processing list", func() bool {
		return llen(t, rdb, processingKey) == 1
	})

	// A surviving instance takes the abandoned claim back after the visibility
	// timeout and delivers the task.
	var delivered atomic.Int32
	survivor := New(rdb, testLogger(), fastConfig())
	survivor.RegisterHandler("mail", func(_ context.Context, _ json.RawMessage) error {
		delivered.Add(1)
		return nil
	})
	survivor.Start()

	waitFor(t, "abandoned task reclaimed and delivered", func() bool { return delivered.Load() == 1 })

	survivor.Shutdown()
	close(release)
	dying.Shutdown()

	if got := llen(t, rdb, deadKey); got != 0 {
		t.Fatalf("dead-letter list = %d, want 0", got)
	}
}

// TestBreakerRejectionDoesNotConsumeRetry pins the rule that
// circuitbreaker.ErrOpen means "not attempted", not "failed".
func TestBreakerRejectionDoesNotConsumeRetry(t *testing.T) {
	rdb := testRedis(t)

	cfg := fastConfig()
	// Long enough that the refused task is still parked when asserted on.
	cfg.NotAttemptedDelay = 3 * time.Second

	var calls atomic.Int32
	n := New(rdb, testLogger(), cfg)
	n.RegisterHandler("mail", func(_ context.Context, _ json.RawMessage) error {
		calls.Add(1)
		return fmt.Errorf("mailer: %w", circuitbreaker.ErrOpen)
	})
	n.Start()
	defer n.Shutdown()

	n.Enqueue("mail", map[string]string{"to": "client@example.com"})

	waitFor(t, "task rescheduled after breaker rejection", func() bool { return zcard(t, rdb) == 1 })

	parked := scheduled(t, rdb)
	if got := parked[0].Retries; got != 0 {
		t.Errorf("retries = %d after a breaker rejection, want 0: an open breaker never attempted the send", got)
	}
	if got := parked[0].Attempts; got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
	if got := llen(t, rdb, deadKey); got != 0 {
		t.Fatalf("dead-letter list = %d, want 0: a refused task must never be discarded", got)
	}
	if got := llen(t, rdb, processingKey); got != 0 {
		t.Fatalf("processing list = %d, want 0", got)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("handler calls = %d, want 1", got)
	}
}

// TestBreakerRejectionOutlastsTheRetryBudget is the production shape: the
// breaker stays open for longer than the whole retry ladder. The old code
// consumed all four attempts against it in about eight seconds and discarded
// the task; the task must instead still be delivered once the breaker closes.
func TestBreakerRejectionOutlastsTheRetryBudget(t *testing.T) {
	rdb := testRedis(t)

	cfg := fastConfig()
	cfg.MaxRetries = 2

	var open atomic.Bool
	open.Store(true)

	var delivered atomic.Int32
	n := New(rdb, testLogger(), cfg)
	n.RegisterHandler("mail", func(_ context.Context, _ json.RawMessage) error {
		if open.Load() {
			return fmt.Errorf("mailer: %w", circuitbreaker.ErrOpen)
		}
		delivered.Add(1)
		return nil
	})
	n.Start()
	defer n.Shutdown()

	n.Enqueue("mail", map[string]string{"to": "client@example.com"})

	// Far more refusals than MaxRetries.
	waitFor(t, "task refused repeatedly", func() bool { return zcard(t, rdb) == 1 })
	time.Sleep(200 * time.Millisecond)

	if got := llen(t, rdb, deadKey); got != 0 {
		t.Fatalf("dead-letter list = %d, want 0 while the breaker is open", got)
	}

	open.Store(false)
	waitFor(t, "task delivered once the breaker closed", func() bool { return delivered.Load() == 1 })
}

// TestShutdownDuringBackoffPreservesTask covers SIGTERM landing between a
// failed attempt and its retry. The retry used to be a time.After inside a
// goroutine that returned on shutdown, holding the only copy of the task.
func TestShutdownDuringBackoffPreservesTask(t *testing.T) {
	rdb := testRedis(t)

	cfg := fastConfig()
	cfg.RetryBackoff = []time.Duration{2 * time.Second}

	var calls atomic.Int32
	failing := New(rdb, testLogger(), cfg)
	failing.RegisterHandler("mail", func(_ context.Context, _ json.RawMessage) error {
		calls.Add(1)
		return errors.New("smtp unavailable")
	})
	failing.Start()
	failing.Enqueue("mail", map[string]string{"to": "client@example.com"})

	// Stop the process as soon as the first attempt has failed — that is, while
	// the two-second backoff before the retry is still running. The assertion
	// below is deliberately made after Shutdown has returned, so it reads what
	// survived the process, not what a goroutine was still holding.
	waitFor(t, "first attempt failed", func() bool { return calls.Load() >= 1 })
	failing.Shutdown()

	if got := zcard(t, rdb); got != 1 {
		t.Fatalf("delayed set = %d after shutdown, want 1: the retry must outlive the process", got)
	}
	parked := scheduled(t, rdb)
	if got := parked[0].Retries; got != 1 {
		t.Errorf("retries = %d, want 1", got)
	}

	// The next process finds it and delivers it.
	restartCfg := fastConfig()
	var delivered atomic.Int32
	restarted := New(rdb, testLogger(), restartCfg)
	restarted.RegisterHandler("mail", func(_ context.Context, _ json.RawMessage) error {
		delivered.Add(1)
		return nil
	})
	restarted.Start()
	defer restarted.Shutdown()

	waitFor(t, "task delivered after restart", func() bool { return delivered.Load() == 1 })
}

// TestHandlerPanicDoesNotConsumeTask covers the recover that used to log and
// return, leaving the task acknowledged by nobody and queued nowhere.
func TestHandlerPanicDoesNotConsumeTask(t *testing.T) {
	rdb := testRedis(t)

	var calls atomic.Int32
	n := New(rdb, testLogger(), fastConfig())
	n.RegisterHandler("mail", func(_ context.Context, _ json.RawMessage) error {
		if calls.Add(1) == 1 {
			panic("nil map write in the delivery path")
		}
		return nil
	})
	n.Start()
	defer n.Shutdown()

	n.Enqueue("mail", map[string]string{"to": "client@example.com"})

	waitFor(t, "task retried after the panic", func() bool { return calls.Load() == 2 })
	waitFor(t, "processing list drained", func() bool { return llen(t, rdb, processingKey) == 0 })

	if got := llen(t, rdb, deadKey); got != 0 {
		t.Fatalf("dead-letter list = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// Terminal states
// ---------------------------------------------------------------------------

func TestPermanentErrorDeadLettersImmediately(t *testing.T) {
	rdb := testRedis(t)

	var calls atomic.Int32
	n := New(rdb, testLogger(), fastConfig())
	n.RegisterHandler("mail", func(_ context.Context, _ json.RawMessage) error {
		calls.Add(1)
		return fmt.Errorf("recipient refused: %w", ErrPermanent)
	})
	n.Start()
	defer n.Shutdown()

	n.Enqueue("mail", map[string]string{"to": "not-an-address"})

	waitFor(t, "task dead-lettered", func() bool { return llen(t, rdb, deadKey) == 1 })
	time.Sleep(80 * time.Millisecond)

	if got := calls.Load(); got != 1 {
		t.Errorf("handler calls = %d, want 1: a permanent rejection must not be retried", got)
	}
	if got := zcard(t, rdb); got != 0 {
		t.Errorf("delayed set = %d, want 0", got)
	}
}

func TestStructuralPermanentErrorDeadLetters(t *testing.T) {
	rdb := testRedis(t)

	n := New(rdb, testLogger(), fastConfig())
	n.RegisterHandler("mail", func(_ context.Context, _ json.RawMessage) error {
		return refusedError{}
	})
	n.Start()
	defer n.Shutdown()

	n.Enqueue("mail", map[string]string{"to": "not-an-address"})
	waitFor(t, "structurally permanent error dead-lettered", func() bool { return llen(t, rdb, deadKey) == 1 })
}

// refusedError answers Permanent() without importing this package's sentinel,
// the way internal/mailer's brevoError does.
type refusedError struct{}

func (refusedError) Error() string   { return "provider refused the message" }
func (refusedError) Permanent() bool { return true }

func TestExhaustedRetriesDeadLetter(t *testing.T) {
	rdb := testRedis(t)

	cfg := fastConfig()
	cfg.MaxRetries = 2

	var calls atomic.Int32
	n := New(rdb, testLogger(), cfg)
	n.RegisterHandler("mail", func(_ context.Context, _ json.RawMessage) error {
		calls.Add(1)
		return errors.New("provider unavailable")
	})
	n.Start()
	defer n.Shutdown()

	n.Enqueue("mail", map[string]string{"to": "client@example.com"})

	waitFor(t, "task dead-lettered after its retries", func() bool { return llen(t, rdb, deadKey) == 1 })

	// One initial attempt plus MaxRetries.
	if got := calls.Load(); got != 3 {
		t.Errorf("handler calls = %d, want 3 (1 attempt + 2 retries)", got)
	}
}

func TestUndecodableTaskDeadLetters(t *testing.T) {
	rdb := testRedis(t)

	if err := rdb.LPush(context.Background(), queueKey, "{not json").Err(); err != nil {
		t.Fatalf("LPush: %v", err)
	}

	n := New(rdb, testLogger(), fastConfig())
	n.Start()
	defer n.Shutdown()

	waitFor(t, "poison task dead-lettered", func() bool { return llen(t, rdb, deadKey) == 1 })
	if got := llen(t, rdb, processingKey); got != 0 {
		t.Fatalf("processing list = %d, want 0", got)
	}
}

// TestUnknownTaskTypeIsRescheduled covers a rolling deploy: an old instance
// claims a task type only the new build knows. Dropping it would lose the mail.
func TestUnknownTaskTypeIsRescheduled(t *testing.T) {
	rdb := testRedis(t)

	cfg := fastConfig()
	cfg.NotAttemptedDelay = 3 * time.Second

	n := New(rdb, testLogger(), cfg)
	n.Start()
	defer n.Shutdown()

	n.Enqueue("mail:from_the_future", map[string]string{"to": "client@example.com"})

	waitFor(t, "unknown task type rescheduled", func() bool { return zcard(t, rdb) == 1 })
	if got := llen(t, rdb, deadKey); got != 0 {
		t.Fatalf("dead-letter list = %d, want 0", got)
	}
}

func TestTaskTTLDeadLettersAStaleTask(t *testing.T) {
	rdb := testRedis(t)

	cfg := fastConfig()
	cfg.TaskTTL = time.Nanosecond

	n := New(rdb, testLogger(), cfg)
	n.RegisterHandler("mail", func(_ context.Context, _ json.RawMessage) error {
		return errors.New("provider unavailable")
	})
	n.Start()
	defer n.Shutdown()

	n.Enqueue("mail", map[string]string{"to": "client@example.com"})
	waitFor(t, "expired task dead-lettered", func() bool { return llen(t, rdb, deadKey) == 1 })
}

// ---------------------------------------------------------------------------
// Classification and lifecycle
// ---------------------------------------------------------------------------

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want outcome
	}{
		{"nil is done", nil, outcomeDone},
		{"plain error retries", errors.New("boom"), outcomeRetry},
		{"sentinel not attempted", fmt.Errorf("wrapped: %w", ErrNotAttempted), outcomeNotTried},
		{"open breaker is not attempted", fmt.Errorf("mailer: %w", circuitbreaker.ErrOpen), outcomeNotTried},
		{"sentinel permanent", fmt.Errorf("wrapped: %w", ErrPermanent), outcomePermanent},
		{"structural permanent", refusedError{}, outcomePermanent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(tt.err); got != tt.want {
				t.Errorf("classify(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestRetryDelayClampsPastTheTable(t *testing.T) {
	n := New(nil, testLogger(), Config{
		RetryBackoff: []time.Duration{time.Second, 2 * time.Second},
	})

	if got := n.retryDelay(1); got != time.Second {
		t.Errorf("retryDelay(1) = %v, want 1s", got)
	}
	if got := n.retryDelay(2); got != 2*time.Second {
		t.Errorf("retryDelay(2) = %v, want 2s", got)
	}
	if got := n.retryDelay(9); got != 2*time.Second {
		t.Errorf("retryDelay(9) = %v, want 2s (clamped to the last entry)", got)
	}
}

func TestVisibilityTimeoutOutlastsATaskAttempt(t *testing.T) {
	n := New(nil, testLogger(), Config{TaskTimeout: 30 * time.Second, VisibilityTimeout: time.Second})

	if n.visibilityTimeout < 6*n.taskTimeout {
		t.Fatalf("visibilityTimeout = %v, want at least 6x taskTimeout (%v)", n.visibilityTimeout, n.taskTimeout)
	}
}

// TestShutdownIsIdempotent covers the unguarded close that panicked the
// process on a second call. realtime.Hub.Shutdown has had the guard all along.
func TestShutdownIsIdempotent(t *testing.T) {
	rdb := testRedis(t)

	n := New(rdb, testLogger(), fastConfig())
	n.Start()

	n.Shutdown()
	n.Shutdown()
	n.Shutdown()
}

func TestConcurrentShutdownIsSafe(t *testing.T) {
	rdb := testRedis(t)

	n := New(rdb, testLogger(), fastConfig())
	n.Start()

	done := make(chan struct{})
	for range 8 {
		go func() {
			n.Shutdown()
			done <- struct{}{}
		}()
	}
	for range 8 {
		<-done
	}
}

func TestNilNotifierLifecycleIsNoOp(t *testing.T) {
	var n *Notifier

	// Shutdown runs against an application that may never have finished
	// booting, so these stay nil-safe. Enqueue deliberately does not: a queue
	// that silently accepts tasks it cannot store is the failure being removed.
	n.RegisterHandler("test", nil)
	n.Start()
	n.Shutdown()
}
