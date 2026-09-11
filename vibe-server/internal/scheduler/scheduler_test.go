package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubLocks struct {
	mu       sync.Mutex
	taken    bool
	err      error
	keys     []string
	released int
}

func (s *stubLocks) TryAdvisory(_ context.Context, key string) (bool, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.keys = append(s.keys, key)
	if s.err != nil {
		return false, func() {}, s.err
	}
	if s.taken {
		return false, func() {}, nil
	}
	return true, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.released++
	}, nil
}

func (s *stubLocks) releases() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.released
}

// newScheduler builds a Scheduler that runs its goroutines inline and adds no
// start-up jitter, so a test that calls once or loop directly is deterministic.
func newScheduler(locks Locker) *Scheduler {
	s := New(locks, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), func(fn func()) { fn() })
	s.jitter = func(time.Duration) time.Duration { return 0 }
	return s
}

// newTrackedScheduler builds a Scheduler whose goroutines really are goroutines,
// tracked in a wait group the way the application's background helper tracks
// them. Tests that go through Start need this; the inline runner above would
// deadlock on the first loop.
func newTrackedScheduler(t *testing.T, locks Locker) (*Scheduler, *bytes.Buffer, *sync.WaitGroup) {
	t.Helper()

	logs := &bytes.Buffer{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	s := New(locks, slog.New(slog.NewTextHandler(&lockedWriter{mu: &mu, buf: logs}, nil)), func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	})
	s.jitter = func(time.Duration) time.Duration { return 0 }
	return s, logs, &wg
}

// lockedWriter serialises writes to a buffer a test reads from another goroutine.
type lockedWriter struct {
	mu  *sync.Mutex
	buf *bytes.Buffer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func TestAJobRunsWhenItHoldsTheLock(t *testing.T) {
	locks := &stubLocks{}
	s := newScheduler(locks)

	var ran int
	s.once(context.Background(), Job{Name: "reminders", Run: func(context.Context) { ran++ }})

	if ran != 1 {
		t.Errorf("want the job to run once; got %d", ran)
	}
	if len(locks.keys) != 1 || locks.keys[0] != "cron:reminders" {
		t.Errorf("the lock must be keyed on the job name; got %v", locks.keys)
	}
}

// Every instance runs the same loops, so the one that does not get the lock
// must do nothing — otherwise three instances send the same reminder three
// times.
func TestAJobIsSkippedWhenAnotherInstanceHoldsTheLock(t *testing.T) {
	locks := &stubLocks{taken: true}
	s := newScheduler(locks)

	var ran int
	s.once(context.Background(), Job{Name: "reminders", Run: func(context.Context) { ran++ }})

	if ran != 0 {
		t.Errorf("the job must not run without the lock; ran %d times", ran)
	}
}

// The lock has to come back whatever the job did, or every later run of that
// job is skipped.
func TestTheLockIsReleasedAfterTheJobRuns(t *testing.T) {
	locks := &stubLocks{}
	s := newScheduler(locks)

	s.once(context.Background(), Job{Name: "reminders", Run: func(context.Context) {}})

	if locks.releases() != 1 {
		t.Errorf("want the lock released once; got %d", locks.releases())
	}
}

func TestTheLockIsReleasedWhenTheJobPanics(t *testing.T) {
	locks := &stubLocks{}
	s := newScheduler(locks)

	s.once(context.Background(), Job{Name: "reminders", Run: func(context.Context) { panic("boom") }})

	if locks.releases() != 1 {
		t.Errorf("a panicking job must still release its lock; released %d", locks.releases())
	}
}

func TestAFailedLockAttemptDoesNotRunTheJob(t *testing.T) {
	locks := &stubLocks{err: errors.New("database unreachable")}
	s := newScheduler(locks)

	var ran int
	s.once(context.Background(), Job{Name: "reminders", Run: func(context.Context) { ran++ }})

	if ran != 0 {
		t.Error("a job must not run when the lock could not be attempted")
	}
}

// A job whose work is in-process — clearing an in-memory cache, say — has to
// run on every instance, so it takes no lock.
func TestALocalJobRunsWithoutTakingTheLock(t *testing.T) {
	locks := &stubLocks{taken: true}
	s := newScheduler(locks)

	var ran int
	s.once(context.Background(), Job{Name: "blacklist-cleanup", Local: true, Run: func(context.Context) { ran++ }})

	if ran != 1 {
		t.Errorf("a local job must run regardless of the lock; ran %d", ran)
	}
	if len(locks.keys) != 0 {
		t.Errorf("a local job must not take a lock; attempted %v", locks.keys)
	}
}

// The job gets a bounded context: one that hangs would hold its lock and stop
// every later run.
func TestAJobRunsUnderADeadline(t *testing.T) {
	s := newScheduler(&stubLocks{})

	var hasDeadline bool
	s.once(context.Background(), Job{Name: "reminders", Run: func(ctx context.Context) {
		_, hasDeadline = ctx.Deadline()
	}})

	if !hasDeadline {
		t.Error("the job context must carry a deadline")
	}
}

// FINDING 1. A panic inside a job body must cost that job one run and nothing
// more.
//
// The recover used to live outside the loop — in the application's background
// helper, which wraps the whole loop — so the first panic unwound the ticker
// with it. The job was then dead for the life of the process while every other
// job kept running, so nothing looked wrong: two runs, then eighteen ticks
// producing nothing.
func TestAPanicCostsOneRunAndNotTheSchedule(t *testing.T) {
	s := newScheduler(&stubLocks{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runs := make(chan struct{}, 8)
	go s.loop(ctx, Job{Name: "reminders", Every: 5 * time.Millisecond, Run: func(context.Context) {
		select {
		case runs <- struct{}{}:
		default:
		}
		panic("boom")
	}})

	// Three runs is past the first panic twice over: the second proves the
	// ticker survived, the third that it keeps surviving.
	deadline := time.After(2 * time.Second)
	for i := range 3 {
		select {
		case <-runs:
		case <-deadline:
			t.Fatalf("the job stopped running after a panic; got %d runs, want 3", i)
		}
	}
}

// The log line has to name the job. An anonymous "background task panic" is
// what made this invisible: it did not say which of the eleven loops had died.
func TestAPanicIsLoggedWithTheJobName(t *testing.T) {
	locks := &stubLocks{}
	logs := &bytes.Buffer{}
	s := New(locks, slog.New(slog.NewTextHandler(logs, nil)), func(fn func()) { fn() })
	s.jitter = func(time.Duration) time.Duration { return 0 }

	s.once(context.Background(), Job{Name: "retry_refunds", Run: func(context.Context) { panic("boom") }})

	if !strings.Contains(logs.String(), "retry_refunds") {
		t.Errorf("the panic log must name the job; got %s", logs.String())
	}
}

// FINDING 2. A job that is running when the stop signal arrives must be told,
// rather than left to spend its whole two-minute budget issuing provider calls
// and writes while the process waits on it.
func TestShutdownCancelsAJobThatIsAlreadyRunning(t *testing.T) {
	s, _, wg := newTrackedScheduler(t, &stubLocks{})
	shutdown := make(chan struct{})

	started := make(chan struct{})
	stopped := make(chan error, 1)

	s.Start(shutdown, Job{Name: "sweep_webhook_events", Every: time.Hour, Run: func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		stopped <- ctx.Err()
	}})

	<-started
	close(shutdown)

	select {
	case err := <-stopped:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("want the run cancelled; got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the running job was never told to stop, so the shutdown wait blocks behind it for the whole job budget")
	}

	drained := make(chan struct{})
	go func() { wg.Wait(); close(drained) }()
	select {
	case <-drained:
	case <-time.After(3 * time.Second):
		t.Fatal("the scheduler's goroutines did not finish after shutdown")
	}
}

// FINDING 3. Nothing runs before its jitter has elapsed, so eleven jobs do not
// all hit the database at the moment the process boots.
func TestTheFirstRunWaitsOutItsJitter(t *testing.T) {
	s := newScheduler(&stubLocks{})
	s.jitter = func(time.Duration) time.Duration { return time.Hour }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ran := make(chan struct{}, 1)
	go s.loop(ctx, Job{Name: "reminders", Every: time.Minute, Run: func(context.Context) {
		ran <- struct{}{}
	}})

	select {
	case <-ran:
		t.Fatal("the job ran before its start-up jitter had elapsed, so a boot still fires every job at once")
	case <-time.After(150 * time.Millisecond):
	}
}

// The delay is bounded by the job's own interval and by maxStartupJitter, so a
// deploy never leaves a job unrun for anything like a full period.
func TestTheStartupDelayIsBoundedByTheIntervalAndTheCeiling(t *testing.T) {
	s := newScheduler(&stubLocks{})
	s.jitter = func(span time.Duration) time.Duration { return span }

	if got := s.startupDelay(Job{Every: 10 * time.Second}); got != 10*time.Second {
		t.Errorf("a job that runs every 10s must not be held back longer than that; got %v", got)
	}
	if got := s.startupDelay(Job{Every: 24 * time.Hour}); got != maxStartupJitter {
		t.Errorf("want the delay capped at %v; got %v", maxStartupJitter, got)
	}
}

// The jitter is drawn per job rather than shared, or every loop would be
// delayed by the same amount and the burst would simply move.
func TestEachJobDrawsItsOwnJitter(t *testing.T) {
	s := newScheduler(&stubLocks{})

	var spans []time.Duration
	s.jitter = func(span time.Duration) time.Duration {
		spans = append(spans, span)
		return 0
	}

	s.startupDelay(Job{Every: time.Minute})
	s.startupDelay(Job{Every: time.Minute})

	if len(spans) != 2 {
		t.Errorf("want a draw per job; got %d", len(spans))
	}
}

// The real jitter has to land inside the span it is given, and vary.
func TestTheDefaultJitterIsSpreadAcrossTheSpan(t *testing.T) {
	s := New(&stubLocks{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), func(fn func()) { fn() })

	seen := map[time.Duration]bool{}
	for range 50 {
		d := s.startupDelay(Job{Every: 5 * time.Minute})
		if d < 0 || d >= maxStartupJitter {
			t.Fatalf("jitter %v is outside [0, %v)", d, maxStartupJitter)
		}
		seen[d] = true
	}
	if len(seen) < 2 {
		t.Error("the jitter is constant, so two instances started together would still collide")
	}
}
