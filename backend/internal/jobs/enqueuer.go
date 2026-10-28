package jobs

import (
	"context"
	"expvar"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// enqueueTimeout bounds one Enqueue. It is short because the caller is
// usually finishing a request.
const enqueueTimeout = 3 * time.Second

// EnqueueStore is the write an Enqueuer records a job through. *Store satisfies
// it. It is an interface so a test can make the write fail or report a dedup
// hit without a database.
type EnqueueStore interface {
	Enqueue(ctx context.Context, jobType string, payload any, runAt time.Time, maxAttempts int, dedupKey string) (uuid.UUID, bool, error)
}

// Enqueuer is the publishing side of the queue, for callers that have work to
// defer.
//
// Enqueue and EnqueueAt are for callers with nothing to do about a failure to
// record the work: they return nothing, and a failure is logged at error level
// and counted in enqueue_failed, so it is never silent. TryEnqueueAt returns
// the failure instead, for a caller that can retry.
//
// It is deliberately detached from the caller's context. The job it records (an
// email, a WhatsApp message) must survive the cancellation of the request that
// triggered it and is delivered later by an independent pool, not synchronously
// here — so a caller must not be able to tie the durability of a confirmation to
// whether the browser stayed connected. Do not add a ctx parameter; thread a
// deadline through enqueueTimeout instead.
type Enqueuer struct {
	Store   EnqueueStore
	Logger  *slog.Logger
	Metrics *expvar.Map
	// MaxAttempts is the budget every job this enqueuer records gets. Zero
	// leaves the column default.
	MaxAttempts int
}

// Enqueue records a job that is due now. An empty dedupKey means the job is not
// deduplicated and every call records another one.
func (e *Enqueuer) Enqueue(jobType string, payload any, dedupKey string) {
	e.EnqueueAt(jobType, payload, time.Time{}, dedupKey)
}

// EnqueueAt records a job that becomes due at runAt, which is now when zero.
// The dedup rule is Enqueue's. A failure is logged and counted, not returned;
// see TryEnqueueAt.
func (e *Enqueuer) EnqueueAt(jobType string, payload any, runAt time.Time, dedupKey string) {
	if err := e.TryEnqueueAt(jobType, payload, runAt, dedupKey); err != nil {
		e.Logger.Error("jobs: failed to enqueue, the work was dropped",
			"type", jobType, "error", err)
	}
}

// TryEnqueueAt is EnqueueAt for a caller that can act on a failure. It returns
// the insert error and does not log it, because the caller decides what happens
// next. It does not retry: a caller that retries is safe, because the dedup key
// makes a second successful enqueue of the same work a no-op.
//
// A dedup hit is success, not an error. The work is already queued, and the key
// is what stops a redelivered webhook sending a second confirmation.
func (e *Enqueuer) TryEnqueueAt(jobType string, payload any, runAt time.Time, dedupKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), enqueueTimeout)
	defer cancel()

	_, recorded, err := e.Store.Enqueue(ctx, jobType, payload, runAt, e.MaxAttempts, dedupKey)
	switch {
	case err != nil:
		e.count("enqueue_failed", 1)
		return err
	case !recorded:
		// The second delivery of work already queued. This is the point of
		// the key, not a failure.
		e.count("deduplicated", 1)
		e.Logger.Info("jobs: enqueue deduplicated, the work is already queued", "type", jobType)
	default:
		e.count("enqueued", 1)
	}
	return nil
}

// TryEnqueue is TryEnqueueAt for work that is due now.
func (e *Enqueuer) TryEnqueue(jobType string, payload any, dedupKey string) error {
	return e.TryEnqueueAt(jobType, payload, time.Time{}, dedupKey)
}

func (e *Enqueuer) count(name string, n int64) {
	if e.Metrics != nil {
		e.Metrics.Add(name, n)
	}
}
