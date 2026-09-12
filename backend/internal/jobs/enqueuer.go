package jobs

import (
	"context"
	"expvar"
	"log/slog"
	"time"
)

// enqueueTimeout bounds one Enqueue. It is short because the caller is
// usually finishing a request.
const enqueueTimeout = 3 * time.Second

// Enqueuer is the publishing side of the queue, for callers that have work to
// defer and nothing to do about a failure to record it.
//
// It is deliberately detached from the caller's context and returns nothing.
// The job it records (an email, a WhatsApp message) must survive the
// cancellation of the request that triggered it and is delivered later by an
// independent pool, not synchronously here — so a caller must not be able to
// tie the durability of a confirmation to whether the browser stayed
// connected. Do not add a ctx parameter; thread a deadline through
// enqueueTimeout instead.
//
// A failure here is the one remaining way to lose a job. It is logged at error
// level and counted in enqueue_failed, so it is never silent.
type Enqueuer struct {
	Store   *Store
	Logger  *slog.Logger
	Metrics *expvar.Map
	// MaxAttempts is the budget every job this enqueuer records gets. Zero
	// leaves the column default.
	MaxAttempts int
}

// Enqueue records a job. An empty dedupKey means the job is not deduplicated
// and every call records another one.
func (e *Enqueuer) Enqueue(jobType string, payload any, dedupKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), enqueueTimeout)
	defer cancel()

	_, recorded, err := e.Store.Enqueue(ctx, jobType, payload, time.Time{}, e.MaxAttempts, dedupKey)
	switch {
	case err != nil:
		e.count("enqueue_failed", 1)
		e.Logger.Error("jobs: failed to enqueue, the work was dropped",
			"type", jobType, "error", err)
	case !recorded:
		// The second delivery of work already queued. This is the point of
		// the key, not a failure: it is what stops a redelivered webhook
		// sending the client a second confirmation.
		e.count("deduplicated", 1)
		e.Logger.Info("jobs: enqueue deduplicated, the work is already queued", "type", jobType)
	default:
		e.count("enqueued", 1)
	}
}

func (e *Enqueuer) count(name string, n int64) {
	if e.Metrics != nil {
		e.Metrics.Add(name, n)
	}
}
