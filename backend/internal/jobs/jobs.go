// Package jobs is the one durable work queue behind every deferred task this
// service runs: the emails and WhatsApp messages a booking triggers, and the
// payment-provider work the webhook endpoint defers.
//
// It replaces three queues that said the same thing three ways — a Redis list
// with its own claim/reclaim scripts in internal/notifier, and two
// domain-shaped tables in the payments schema, each with its own conditional
// UPDATE, its own backoff table and its own retention sweep. They shared a
// vocabulary (claim, reclaim, stale, exhausted, dead) because they shared a
// guarantee; they did not share a line of code, so a fix to one of them was a
// fix to one of them.
//
// The guarantee is at-least-once, and the claim is the whole of it:
//
//	UPDATE jobs SET status = 'processing' ...
//	WHERE id IN (SELECT id FROM jobs WHERE ... FOR UPDATE SKIP LOCKED LIMIT n)
//	RETURNING ...
//
// SKIP LOCKED is what lets every instance run the same statement on the same
// tick and take disjoint rows without any of them waiting: a row another
// worker has locked is not contended, it is passed over. The row is durable
// from Enqueue until Complete, so a worker that dies mid-handle loses nothing
// — ReclaimStale hands its claim back once the lease has lapsed.
//
// At-least-once means a job can run twice. DedupKey is how a caller says that
// must not happen for this particular piece of work: an Enqueue carrying a key
// already in the table is a no-op, so a redelivered webhook cannot produce a
// second email for the same booking and event.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

// The states a job moves through. They are stored in the jobs.status column
// and pinned by its CHECK constraint, so the strings here and the ones in
// db/migrations/001_init.sql are one definition in two places.
const (
	// StatusPending is waiting for its run_at to come round.
	StatusPending = "pending"
	// StatusProcessing is claimed by a worker and not yet settled.
	StatusProcessing = "processing"
	// StatusDone is finished and kept only for the retention window.
	StatusDone = "done"
	// StatusFailed is the dead letter: the attempt budget is spent, or the
	// handler refused the job for good. Nothing retries it, and nothing
	// deletes it on a timer — it is an operator's inbox.
	StatusFailed = "failed"
)

// Job is one unit of deferred work.
type Job struct {
	ID   uuid.UUID
	Type string
	// Payload is the handler's argument, exactly as Enqueue serialised it.
	Payload json.RawMessage
	Status  string
	// RunAt is the earliest instant a worker may claim this job. Enqueue sets
	// it; every retry pushes it out by the backoff.
	RunAt time.Time
	// Attempts counts claims, not failures: it is incremented by Claim itself,
	// so a worker that dies mid-handle has still spent one. That is what
	// bounds a job whose handler panics the process every time it is read.
	Attempts    int
	MaxAttempts int
	LastError   string
	// LockedAt and LockedBy name the claim. LockedAt is what ReclaimStale
	// measures the lease against; LockedBy is for the log line that says which
	// worker went away.
	LockedAt *time.Time
	LockedBy string
	// DedupKey, when set, is unique across the table: a second Enqueue under
	// the same key does nothing. See DedupKey for how one is built.
	DedupKey  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Handler runs one job. The payload is the bytes Enqueue was given.
type Handler func(ctx context.Context, payload json.RawMessage) error

// Errors a handler returns to steer the queue. Anything else is a transient
// failure: it spends an attempt and comes back on the backoff.
var (
	// ErrNotAttempted says the work never reached its provider — an open
	// circuit breaker, a refused lease, a closed pool. The job is rescheduled
	// without spending an attempt, because nothing was tried.
	ErrNotAttempted = errors.New("jobs: job was not attempted")

	// ErrPermanent says the provider answered and refused this specific job.
	// Retrying re-sends the same bytes for the same answer, so it is
	// dead-lettered now rather than five attempts from now.
	ErrPermanent = errors.New("jobs: job permanently rejected")
)

// A handler error may answer these structurally instead of wrapping the
// sentinels above, so a package like internal/mailer can classify its own
// failures without importing this one. The shape is net.Error's.
type (
	notAttemptedError interface{ NotAttempted() bool }
	permanentError    interface{ Permanent() bool }
)

// Outcome is what the queue does with a job after one attempt.
type Outcome int

// The four settlements. Every attempt ends in exactly one of them.
const (
	// OutcomeDone acknowledges the job.
	OutcomeDone Outcome = iota
	// OutcomeRetry spends an attempt and reschedules on the backoff.
	OutcomeRetry
	// OutcomeNotAttempted reschedules without spending an attempt.
	OutcomeNotAttempted
	// OutcomePermanent dead-letters immediately.
	OutcomePermanent
)

// Classify maps a handler's error onto an outcome.
//
// The not-attempted case is checked first: an open circuit breaker is not a
// failed attempt, it is the absence of one, and it would otherwise match the
// transient default and spend a budget meant for answers.
func Classify(err error) Outcome {
	switch {
	case err == nil:
		return OutcomeDone
	case errors.Is(err, ErrNotAttempted), errors.Is(err, circuitbreaker.ErrOpen), asNotAttempted(err):
		return OutcomeNotAttempted
	case errors.Is(err, ErrPermanent), asPermanent(err):
		return OutcomePermanent
	default:
		return OutcomeRetry
	}
}

func asNotAttempted(err error) bool {
	var na notAttemptedError
	return errors.As(err, &na) && na.NotAttempted()
}

func asPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p) && p.Permanent()
}
