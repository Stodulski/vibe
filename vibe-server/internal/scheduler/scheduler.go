// Package scheduler runs the recurring background jobs: reminders, releasing
// unpaid bookings, completing played ones, and the various cleanups.
//
// Every instance of the service runs the same loops, so each job takes a
// PostgreSQL advisory lock before doing anything. Whoever gets it runs; the
// rest return immediately. That is what stops three instances sending the same
// reminder three times.
package scheduler

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"runtime/debug"
	"time"
)

// jobTimeout bounds a single run. A job that hangs would otherwise hold its
// lock and stop every later run of the same job.
const jobTimeout = 2 * time.Minute

// maxStartupJitter caps how long a job's first run is held back at boot.
//
// Every job used to fire the instant Start returned. Eleven of them at once, ten
// of which take a lock, is a burst of database work timed to collide with the
// slowest moment in the process's life — the one where the HTTP listener has not
// opened yet and a rolling deploy has two instances doing it at the same time.
// Spreading the first runs over a window costs a few seconds of freshness on a
// job that already runs every five minutes and removes the burst entirely.
//
// It is random rather than a fixed per-job offset on purpose: two instances
// started by the same deploy would compute the same fixed offsets and collide
// exactly as before, which is the case this is for.
//
// Thirty seconds is the ceiling because it is small next to every interval in
// cron.go (the shortest is two minutes) and large next to the burst it is
// smoothing. The delay is additionally capped at the job's own interval below,
// so a hypothetical ten-second job is never held back longer than one period.
const maxStartupJitter = 30 * time.Second

// Locker provides the cross-instance advisory lock.
type Locker interface {
	TryAdvisory(ctx context.Context, key string) (acquired bool, release func(), err error)
}

// Job is one recurring task.
type Job struct {
	// Name identifies the job in logs and as its lock key.
	Name string
	// Every is how often it runs.
	Every time.Duration
	// Run does the work. It is given a context bounded by jobTimeout and
	// cancelled at shutdown.
	Run func(ctx context.Context)
	// Local marks a job whose work is in-process rather than in the database,
	// so it needs no lock: every instance must run it on its own.
	Local bool
}

// Scheduler runs jobs on their intervals until shutdown.
type Scheduler struct {
	locks  Locker
	logger *slog.Logger
	run    func(func())
	// jitter returns how long to hold a first run back, given the span it may be
	// spread over. It is a field so tests can make the spread deterministic;
	// nothing outside this package sets it.
	jitter func(span time.Duration) time.Duration
}

// New returns a Scheduler. run is the application's tracked-goroutine helper,
// so the loops are waited on during a graceful shutdown.
func New(locks Locker, logger *slog.Logger, run func(func())) *Scheduler {
	return &Scheduler{
		locks:  locks,
		logger: logger,
		run:    run,
		//nolint:gosec // G404: this picks a start-up delay, not a secret.
		jitter: rand.N[time.Duration],
	}
}

// Start launches every job on its own loop, each with a short random delay
// before its first run so a boot does not fire all of them at once.
//
// The shutdown channel does two things. It stops the loops, as it always did,
// and it now also cancels the context every run is derived from: a job that is
// mid-flight when the signal arrives is told to stop rather than left to finish
// a two-minute budget while the process waits on it.
func (s *Scheduler) Start(shutdown <-chan struct{}, jobs ...Job) {
	ctx, cancel := context.WithCancel(context.Background())

	// Tracked like the loops themselves, so the wait group cannot report the
	// scheduler as drained while this watcher is still holding the cancel.
	s.run(func() {
		<-shutdown
		cancel()
	})

	for _, job := range jobs {
		s.run(func() { s.loop(ctx, job) })
	}
}

// loop runs one job forever, or until ctx is done.
func (s *Scheduler) loop(ctx context.Context, job Job) {
	if !wait(ctx, s.startupDelay(job)) {
		return
	}
	s.once(ctx, job)

	ticker := time.NewTicker(job.Every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.once(ctx, job)
		}
	}
}

// startupDelay is how long this job's first run is held back.
func (s *Scheduler) startupDelay(job Job) time.Duration {
	span := min(job.Every, maxStartupJitter)
	if span <= 0 {
		return 0
	}
	return s.jitter(span)
}

// wait sleeps for d, reporting whether it got there rather than being cancelled.
func wait(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// once runs a job a single time, holding its lock for the duration unless the
// job is local to this instance.
//
// The recover is here, around one run, rather than around the loop that calls
// it. The application's background helper does recover — but it wraps the whole
// loop, so a panic in a job body unwound the ticker along with it: the recover
// fired once, logged an anonymous "background task panic" with no job name, and
// that job never ran again for the life of the process while every other job
// carried on and the instance kept reporting healthy. A per-run recover keyed by
// job name turns that into one lost run and one log line that says which job.
func (s *Scheduler) once(ctx context.Context, job Job) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("cron: job panicked, its schedule is unaffected",
				"job", job.Name, "panic", r, "stack", string(debug.Stack()))
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()

	if job.Local {
		job.Run(ctx)
		return
	}

	acquired, release, err := s.locks.TryAdvisory(ctx, "cron:"+job.Name)
	if err != nil {
		s.logger.Error("cron: failed to take the job lock", "job", job.Name, "error", err)
		return
	}
	defer release()

	if !acquired {
		// Another instance is running this one.
		return
	}

	job.Run(ctx)
}
