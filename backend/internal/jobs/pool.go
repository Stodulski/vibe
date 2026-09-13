package jobs

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Config configures a Pool. Every zero value gets a production default.
type Config struct {
	// Workers is how many jobs run at once. Default: 4.
	Workers int
	// ClaimBatch is how many jobs one worker takes per round trip. Default: 1
	// — a worker holds what it claimed, so a larger batch is a larger amount
	// of work one dying process takes down with it for a lease.
	ClaimBatch int
	// JobTimeout bounds one attempt. Default: 10s. A type named in Timeouts
	// uses its own bound instead.
	JobTimeout time.Duration
	// Timeouts overrides JobTimeout for specific job types, keyed by job
	// type. A type absent from the map — or mapped to a non-positive value —
	// uses JobTimeout. This exists because one pool serves every job type
	// (see RegisterHandler: a second pool would claim with no type filter
	// and run jobs it has no handler for), so a type whose work genuinely
	// needs longer than the shared default — the payments export, which
	// also uploads the workbook after building it — gets its own ceiling
	// without widening JobTimeout, and the lease that comes with it, for
	// every other type.
	Timeouts map[string]time.Duration
	// PollInterval is how long a worker waits after finding nothing before it
	// asks again. Default: 1s.
	PollInterval time.Duration
	// Lease is how long a claim may go unfinished before another worker may
	// take the job back. Default: 60s, and never less than six times the
	// longest attempt timeout in effect — JobTimeout, or a type's own entry
	// in Timeouts when that is longer — because it only has to outlast one
	// honest attempt, which that timeout already bounds.
	Lease time.Duration
	// ReclaimInterval is how often stale claims are swept. Default: 15s.
	ReclaimInterval time.Duration
	// NotAttemptedDelay is how long a job waits after being refused without
	// being attempted. Default: 30s — half the mailer breaker's 60s reset, so
	// a job rides out an open breaker in two hops instead of spinning on it.
	NotAttemptedDelay time.Duration
	// MaxAge bounds how long a job may circulate without ever completing an
	// attempt. Default: 24h — the old notifier's TaskTTL. Past it, an
	// outcome that would otherwise be released (an unknown type, an open
	// breaker) kills the job instead, and a claim past MaxAge is killed
	// rather than run.
	MaxAge time.Duration
	// Retention is how long a finished job is kept before cmd/api's
	// clean_jobs cron deletes it. Default: 7 days — also how long DedupKey
	// keeps protecting.
	Retention time.Duration

	// Metrics is the counter surface, published by whoever registered the map.
	//
	// It is injected rather than created here (CON-07). The queue used to call
	// expvar.NewMap in a package-level var, which registers into the process's
	// global namespace as an import side effect: a second construction in one
	// process panics on the duplicate name, so nothing could build two queues,
	// and a test binary that merely imported the package published counters.
	// A nil map is valid and means no counters.
	Metrics *expvar.Map
	// Logger is required. A queue that cannot say what it did is worse than
	// no queue.
	Logger *slog.Logger
	// Worker names this process in the locked_by column. Default: a random id.
	Worker string
}

func (c *Config) applyDefaults() {
	if c.Workers <= 0 {
		c.Workers = 4
	}
	if c.ClaimBatch <= 0 {
		c.ClaimBatch = 1
	}
	if c.JobTimeout <= 0 {
		c.JobTimeout = 10 * time.Second
	}
	if c.PollInterval <= 0 {
		c.PollInterval = time.Second
	}
	if c.Lease <= 0 {
		c.Lease = 60 * time.Second
	}
	if floor := 6 * c.longestTimeout(); c.Lease < floor {
		c.Lease = floor
	}
	if c.ReclaimInterval <= 0 {
		c.ReclaimInterval = 15 * time.Second
	}
	if c.NotAttemptedDelay <= 0 {
		c.NotAttemptedDelay = 30 * time.Second
	}
	if c.MaxAge <= 0 {
		c.MaxAge = 24 * time.Hour
	}
	if c.Retention <= 0 {
		c.Retention = 7 * 24 * time.Hour
	}
	if c.Worker == "" {
		c.Worker = uuid.NewString()
	}
}

// longestTimeout returns the longest attempt timeout in effect: JobTimeout,
// or any per-type override in Timeouts that exceeds it. applyDefaults derives
// the Lease floor from it, so a type given a longer timeout via Timeouts
// never leaves the lease too short to outlast it.
func (c *Config) longestTimeout() time.Duration {
	longest := c.JobTimeout
	for _, t := range c.Timeouts {
		if t > longest {
			longest = t
		}
	}
	return longest
}

// DefaultConfig applies every default to a zero Config — for a caller that
// needs one field's default (cmd/api's clean_jobs cron and Retention)
// without duplicating the numbers here.
func DefaultConfig() Config {
	var c Config
	c.applyDefaults()
	return c
}

// Pool is a fixed set of workers draining the jobs table, plus the sweeper
// that hands abandoned claims back.
//
// RegisterHandler, Start and Shutdown are no-ops on a nil receiver, because
// shutdown runs against an application that may never have finished booting.
type Pool struct {
	store    *Store
	cfg      Config
	handlers map[string]Handler

	wg           sync.WaitGroup
	shutdown     chan struct{}
	shutdownOnce sync.Once
}

// NewPool returns a Pool. Register every handler, then Start.
func NewPool(store *Store, cfg Config) *Pool {
	cfg.applyDefaults()
	return &Pool{
		store:    store,
		cfg:      cfg,
		handlers: make(map[string]Handler),
		shutdown: make(chan struct{}),
	}
}

// RegisterHandler associates a job type with the function that runs it.
func (p *Pool) RegisterHandler(jobType string, h Handler) {
	if p == nil {
		return
	}
	p.handlers[jobType] = h
}

// Start launches the workers and the reclaim sweeper.
func (p *Pool) Start() {
	if p == nil {
		return
	}
	for range p.cfg.Workers {
		p.wg.Go(p.work)
	}
	p.wg.Go(p.reclaimLoop)
}

// Shutdown stops the workers and waits for the attempts in flight.
//
// A worker that has claimed a job finishes it (JOB-05): the signal is checked
// between jobs, never inside one, so a graceful stop never abandons a claim it
// would then have to wait a whole lease to reclaim. It is safe to call more
// than once and from more than one goroutine.
func (p *Pool) Shutdown() {
	if p == nil {
		return
	}
	p.shutdownOnce.Do(func() { close(p.shutdown) })
	p.wg.Wait()
}

// stopping reports whether Shutdown has been called.
func (p *Pool) stopping() bool {
	select {
	case <-p.shutdown:
		return true
	default:
		return false
	}
}

// sleep waits for d, returning true if shutdown came first.
func (p *Pool) sleep(d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-p.shutdown:
		return true
	case <-timer.C:
		return false
	}
}

// work is one worker: claim, run, settle, repeat.
func (p *Pool) work() {
	for {
		if p.stopping() {
			return
		}

		claimed, err := p.store.Claim(context.Background(), p.cfg.Worker, p.cfg.ClaimBatch)
		if err != nil {
			p.cfg.Logger.Error("jobs: claim failed", "error", err)
			if p.sleep(p.cfg.PollInterval) {
				return
			}
			continue
		}
		if len(claimed) == 0 {
			if p.sleep(p.cfg.PollInterval) {
				return
			}
			continue
		}

		// Not interrupted by shutdown: a claimed job is finished, never
		// abandoned. See Shutdown.
		for _, job := range claimed {
			p.handle(job)
		}
	}
}

// handle runs one claimed job to a terminal state and leaves one line saying
// which.
func (p *Pool) handle(job *Job) {
	start := time.Now()

	// Checked before the handler lookup: a type a rollback restored to the
	// registry must still not run once the job is this old.
	if p.expired(job) {
		p.expire(job, "claimed past MaxAge without an attempt", start)
		return
	}

	handler, known := p.handlers[job.Type]
	if !known {
		// Not a poison payload: an instance on an older build does not know a
		// type a newer one enqueued. Give it back without spending the
		// attempt and let an instance that knows it pick it up.
		p.settle(job, OutcomeNotAttempted, fmt.Errorf("no handler registered for %q", job.Type), start)
		return
	}

	err := p.run(job, handler)
	p.settle(job, Classify(err), err, start)
}

// run executes one attempt, turning a panicking handler into an error so the
// job is retried rather than swallowed by the recover.
func (p *Pool) run(job *Job, handler Handler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("jobs: handler panicked: %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), p.timeoutFor(job.Type))
	defer cancel()

	return handler(ctx, job.Payload)
}

// timeoutFor returns the attempt timeout jobType runs under: its own entry in
// cfg.Timeouts when it has one, otherwise cfg.JobTimeout.
func (p *Pool) timeoutFor(jobType string) time.Duration {
	if t, ok := p.cfg.Timeouts[jobType]; ok && t > 0 {
		return t
	}
	return p.cfg.JobTimeout
}

// settle applies the outcome of one attempt and logs it.
//
// Every branch ends with the row acknowledged, rescheduled or dead-lettered —
// never only in this goroutine.
func (p *Pool) settle(job *Job, outcome Outcome, cause error, start time.Time) {
	ctx := context.Background()
	text := ""
	if cause != nil {
		text = cause.Error()
	}

	if outcome == OutcomeNotAttempted && p.expired(job) {
		p.expire(job, text, start)
		return
	}

	var result string
	var err error
	switch outcome {
	case OutcomeDone:
		result, err = "done", p.store.Complete(ctx, job.ID)
		p.count("processed", 1)
	case OutcomeNotAttempted:
		result = "not_attempted"
		err = p.store.Release(ctx, job.ID, time.Now().Add(p.cfg.NotAttemptedDelay), text)
		p.count("not_attempted", 1)
	case OutcomePermanent:
		result, err = "dead_lettered", p.store.Kill(ctx, job.ID, text)
		p.count("dead_lettered", 1)
	case OutcomeRetry:
		result, err = p.retry(ctx, job, text)
	}

	p.log(job, result, cause, err, start)
}

// expired reports whether job has sat past MaxAge without completing an
// attempt — the state that would otherwise circulate forever.
func (p *Pool) expired(job *Job) bool {
	return time.Since(job.CreatedAt) >= p.cfg.MaxAge
}

// expire kills a job that ran out its MaxAge without a completed attempt.
// See jobs.Config.MaxAge.
func (p *Pool) expire(job *Job, reason string, start time.Time) {
	text := fmt.Sprintf("expired after %s without an attempt: %s", p.cfg.MaxAge, reason)
	err := p.store.Kill(context.Background(), job.ID, text)
	p.count("expired", 1)
	p.log(job, "expired", errors.New(reason), err, start)
}

// retry spends the attempt, reporting whether that was the last one.
func (p *Pool) retry(ctx context.Context, job *Job, cause string) (string, error) {
	dead, err := p.store.Fail(ctx, job.ID, cause)
	switch {
	case err != nil:
		return "retried", err
	case dead:
		p.count("dead_lettered", 1)
		return "dead_lettered", nil
	default:
		p.count("retried", 1)
		return "retried", nil
	}
}

// log is the one line per job outcome.
func (p *Pool) log(job *Job, result string, cause, settleErr error, start time.Time) {
	args := []any{
		"job_id", job.ID, "type", job.Type, "outcome", result,
		"attempts", job.Attempts, "max_attempts", job.MaxAttempts,
		"duration_ms", time.Since(start).Milliseconds(),
	}
	if cause != nil {
		args = append(args, "error", cause.Error())
	}
	if settleErr != nil {
		// The settle itself failed, so the row is still claimed and the
		// sweeper will hand it to another worker — which runs it a second
		// time. That is the at-least-once trade, and it is never silent.
		args = append(args, "settle_error", settleErr.Error())
		p.cfg.Logger.Error("jobs: job settled badly, it may be redelivered", args...)
		return
	}
	if result == "done" {
		p.cfg.Logger.Info("jobs: job finished", args...)
		return
	}
	p.cfg.Logger.Warn("jobs: job did not finish", args...)
}

// reclaimLoop hands claims whose lease lapsed back to the queue.
//
// It runs on every instance, so recovery from a process that died mid-handle
// never waits for that process to come back.
func (p *Pool) reclaimLoop() {
	// Sweep once at startup, so the clock on anything the previous process
	// abandoned starts at boot rather than at the first tick.
	p.reclaim()

	ticker := time.NewTicker(p.cfg.ReclaimInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.shutdown:
			return
		case <-ticker.C:
			p.reclaim()
		}
	}
}

func (p *Pool) reclaim() {
	moved, err := p.store.ReclaimStale(context.Background(), p.cfg.Lease)
	if err != nil {
		p.cfg.Logger.Error("jobs: reclaim sweep failed", "error", err)
		return
	}
	if moved > 0 {
		p.cfg.Logger.Warn("jobs: reclaimed abandoned jobs", "count", moved, "lease", p.cfg.Lease)
		p.count("reclaimed", moved)
	}
}

func (p *Pool) count(name string, n int64) {
	if p.cfg.Metrics != nil {
		p.cfg.Metrics.Add(name, n)
	}
}
