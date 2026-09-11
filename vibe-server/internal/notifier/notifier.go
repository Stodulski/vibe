// Package notifier is the durable task queue behind every email and WhatsApp
// message this service sends.
//
// It is at-least-once. A task is recoverable in Redis from the moment Enqueue
// returns until a worker acknowledges it, and every transition between those
// two points is a single atomic Redis script. The protocol is the one
// internal/data/webhook_events.go and internal/data/failed_refunds.go use
// against PostgreSQL, expressed in Redis primitives: a claim that moves the
// work to a processing set, a visibility timeout that reclaims a claim its
// owner never finished, a retry schedule with backoff, and a terminal
// dead-letter state. It is deliberately the same vocabulary — claim, reclaim,
// stale, exhausted, dead — because it is deliberately the same guarantee.
//
// The keys it owns:
//
//	vibe:notifications             list   pending tasks, LPUSH head, popped from the tail
//	vibe:notifications:processing  list   claimed tasks, one entry per in-flight attempt
//	vibe:notifications:claims      hash   task id -> claim time (unix millis)
//	vibe:notifications:delayed     zset   scheduled retries, score = due time (unix millis)
//	vibe:notifications:dead        list   tasks that will not be attempted again
package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

const (
	queueKey      = "vibe:notifications"
	processingKey = "vibe:notifications:processing"
	claimsKey     = "vibe:notifications:claims"
	delayedKey    = "vibe:notifications:delayed"
	deadKey       = "vibe:notifications:dead"

	// deadLetterCap bounds the dead-letter list. It is an operator's inbox, not
	// storage: past this many entries the oldest are dropped so a permanently
	// misconfigured provider cannot grow the list without limit.
	deadLetterCap = 1000

	// redisOpTimeout bounds every non-blocking Redis call below. The blocking
	// claim uses the poll interval instead.
	redisOpTimeout = 2 * time.Second

	// reclaimBatch is how many processing entries one sweep inspects.
	reclaimBatch = 200

	// scheduleBatch is how many due retries one scheduler tick releases.
	scheduleBatch = 100
)

// metrics is the queue's counter surface, published on /debug/vars. The two
// that matter operationally are enqueue_failed and dead_lettered: a non-zero
// value on either is mail that a client did not receive.
var metrics = expvar.NewMap("notifier")

// Errors a handler can return to steer the queue.
//
// Anything else is treated as a transient failure and retried with backoff.
var (
	// ErrNotAttempted says the work never reached the provider — an open
	// circuit breaker, a refused lease, a closed pool. The task is rescheduled
	// without consuming a retry, because nothing was tried.
	ErrNotAttempted = errors.New("notifier: task was not attempted")

	// ErrPermanent says the provider answered and refused this specific task.
	// Retrying re-sends the same bytes for the same answer, so the task goes
	// straight to the dead-letter list.
	ErrPermanent = errors.New("notifier: task permanently rejected")
)

// A handler error may answer these questions structurally instead of wrapping
// the sentinels above, so a package like internal/mailer can classify its own
// failures without importing this one. The shape is net.Error's.
type (
	notAttemptedError interface{ NotAttempted() bool }
	permanentError    interface{ Permanent() bool }
)

// outcome is what the queue does with a task after one attempt.
type outcome int

const (
	outcomeDone      outcome = iota // acknowledged, gone
	outcomeRetry                    // failed, consumes a retry
	outcomeNotTried                 // refused before the attempt, consumes nothing
	outcomePermanent                // refused for good, dead-letter now
)

func classify(err error) outcome {
	switch {
	case err == nil:
		return outcomeDone

	// Checked before the permanent and transient cases: an open breaker is not
	// a failed attempt, it is the absence of one. circuitbreaker.ErrOpen is
	// recognised directly so every caller that already follows the repo's
	// AllowRequest/RecordFailure pattern gets this for free.
	case errors.Is(err, ErrNotAttempted), errors.Is(err, circuitbreaker.ErrOpen), asNotAttempted(err):
		return outcomeNotTried

	case errors.Is(err, ErrPermanent), asPermanent(err):
		return outcomePermanent

	default:
		return outcomeRetry
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

// Handler processes a deserialized task payload.
type Handler func(ctx context.Context, data json.RawMessage) error

// task is the unit stored in Redis. Its JSON tags are a storage format:
// renaming one strands whatever is already queued.
type task struct {
	// ID identifies this task across every list it moves through. It is also
	// what makes two otherwise identical tasks distinct members of the delayed
	// sorted set, which is a set and would otherwise collapse them.
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
	// Retries counts failed attempts only. An attempt refused by a circuit
	// breaker does not increment it.
	Retries int `json:"retries,omitempty"`
	// Attempts counts every claim, including the refused ones, so a task
	// bouncing off an open breaker is visible in the logs.
	Attempts   int       `json:"attempts,omitempty"`
	EnqueuedAt time.Time `json:"enqueued_at,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
}

// age reports how long the task has been in the system. Tasks enqueued by an
// older build carry no timestamp; they are reported as ageless and never
// expire, rather than being dead-lettered on sight.
func (t task) age(now time.Time) (time.Duration, bool) {
	if t.EnqueuedAt.IsZero() {
		return 0, false
	}
	return now.Sub(t.EnqueuedAt), true
}

// Config configures a Notifier. Every zero value gets a production default.
type Config struct {
	Workers     int           // Worker goroutines. Default: 4.
	TaskTimeout time.Duration // Timeout per attempt. Default: 10s.
	MaxRetries  int           // Failed attempts before dead-lettering. Default: 3.

	// RetryBackoff is the delay before each retry, indexed by retry number.
	// Past the end of the table every further retry waits the last entry, the
	// same rule as data.retryBackoff. Default: defaultRetryBackoff.
	RetryBackoff []time.Duration

	// NotAttemptedDelay is how long a task waits after being refused without
	// being attempted. Default: 30s — half the mailer breaker's 60s reset
	// timeout, so a task rides out an open breaker in two hops instead of
	// spinning against it.
	NotAttemptedDelay time.Duration

	// VisibilityTimeout is how long a claim may go unfinished before another
	// worker may take the task back. Default: 60s, and never less than six
	// times TaskTimeout. Same reasoning as data.staleWebhookProcessing: it only
	// has to outlast one honest attempt, which TaskTimeout already bounds.
	VisibilityTimeout time.Duration

	// ReclaimInterval is how often the processing list is swept. Default: 15s.
	ReclaimInterval time.Duration

	// ScheduleInterval is how often due retries are released. Default: 1s.
	ScheduleInterval time.Duration

	// PollInterval is how long a worker blocks on one claim before re-checking
	// shutdown. Default: 1s.
	PollInterval time.Duration

	// TaskTTL dead-letters a task that has been in the system this long
	// whatever its retry count, so a task that only ever gets refused cannot
	// circulate forever. Default: 24h.
	TaskTTL time.Duration
}

// defaultRetryBackoff mirrors data.refundRetryBackoff: minutes, not seconds.
// The ladder it replaced was 1s+2s+4s, which burned every attempt inside a
// circuit breaker's 60s open window and discarded the task before the provider
// had a chance to recover.
var defaultRetryBackoff = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
	1 * time.Hour,
}

// Notifier is a durable, at-least-once task queue with a worker pool.
//
// RegisterHandler, Start and Shutdown are no-ops on a nil receiver, because
// server shutdown runs against an application that may never have finished
// booting. Enqueue is deliberately NOT: a nil queue that silently accepted
// tasks is exactly how a deployment without Redis served traffic normally
// while dropping every verification email, password reset and booking
// confirmation. See the same argument on notifications.Service. The guard
// belongs at boot — cmd/api refuses to start without Redis — not here.
type Notifier struct {
	rdb      *redis.Client
	logger   *slog.Logger
	handlers map[string]Handler

	taskTimeout       time.Duration
	workers           int
	maxRetries        int
	retryBackoff      []time.Duration
	notAttemptedDelay time.Duration
	visibilityTimeout time.Duration
	reclaimInterval   time.Duration
	scheduleInterval  time.Duration
	pollInterval      time.Duration
	taskTTL           time.Duration

	wg           sync.WaitGroup
	shutdown     chan struct{}
	shutdownOnce sync.Once
}

// New creates a Notifier. Call RegisterHandler for each task type, then Start.
func New(rdb *redis.Client, logger *slog.Logger, cfg Config) *Notifier {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.TaskTimeout <= 0 {
		cfg.TaskTimeout = 10 * time.Second
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if len(cfg.RetryBackoff) == 0 {
		cfg.RetryBackoff = defaultRetryBackoff
	}
	if cfg.NotAttemptedDelay <= 0 {
		cfg.NotAttemptedDelay = 30 * time.Second
	}
	if cfg.VisibilityTimeout <= 0 {
		cfg.VisibilityTimeout = 60 * time.Second
	}
	if cfg.VisibilityTimeout < 6*cfg.TaskTimeout {
		cfg.VisibilityTimeout = 6 * cfg.TaskTimeout
	}
	if cfg.ReclaimInterval <= 0 {
		cfg.ReclaimInterval = 15 * time.Second
	}
	if cfg.ScheduleInterval <= 0 {
		cfg.ScheduleInterval = 1 * time.Second
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 1 * time.Second
	}
	if cfg.TaskTTL <= 0 {
		cfg.TaskTTL = 24 * time.Hour
	}

	return &Notifier{
		rdb:               rdb,
		logger:            logger,
		handlers:          make(map[string]Handler),
		taskTimeout:       cfg.TaskTimeout,
		workers:           cfg.Workers,
		maxRetries:        cfg.MaxRetries,
		retryBackoff:      cfg.RetryBackoff,
		notAttemptedDelay: cfg.NotAttemptedDelay,
		visibilityTimeout: cfg.VisibilityTimeout,
		reclaimInterval:   cfg.ReclaimInterval,
		scheduleInterval:  cfg.ScheduleInterval,
		pollInterval:      cfg.PollInterval,
		taskTTL:           cfg.TaskTTL,
		shutdown:          make(chan struct{}),
	}
}

// RegisterHandler associates a task type with a handler function.
func (n *Notifier) RegisterHandler(taskType string, h Handler) {
	if n == nil {
		return
	}
	n.handlers[taskType] = h
}

// Start launches the worker pool, the reclaim sweeper and the retry scheduler.
// Call it after all handlers are registered.
func (n *Notifier) Start() {
	if n == nil {
		return
	}
	for range n.workers {
		n.wg.Go(n.worker)
	}
	n.wg.Go(n.reclaimLoop)
	n.wg.Go(n.scheduleLoop)
}

// Enqueue serializes the payload and pushes it onto the queue.
//
// It is intentionally detached from the caller's context: it derives its own
// short internal timeout instead of accepting one, because the durable task it
// enqueues (email, WhatsApp, etc.) must survive the triggering request's
// cancellation and is delivered later by an independent worker pool, not
// synchronously in this call. Do not add a ctx parameter here — thread a
// deadline through the internal timeout instead, and see the
// //nolint:contextcheck comments at call sites for the same reasoning.
//
// A failure here is the one remaining way to lose a task: Redis was reachable
// at boot and is not reachable now. It is logged at error level and counted in
// the notifier.enqueue_failed expvar, so it is never silent.
func (n *Notifier) Enqueue(taskType string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		metrics.Add("enqueue_failed", 1)
		n.logger.Error("notifier: failed to marshal payload", "type", taskType, "error", err)
		return
	}

	t := task{
		ID:         uuid.NewString(),
		Type:       taskType,
		Data:       data,
		EnqueuedAt: time.Now().UTC(),
	}
	raw, err := encode(t)
	if err != nil {
		metrics.Add("enqueue_failed", 1)
		n.logger.Error("notifier: failed to marshal task", "type", taskType, "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()
	if err := n.rdb.LPush(ctx, queueKey, raw).Err(); err != nil {
		metrics.Add("enqueue_failed", 1)
		n.logger.Error("notifier: failed to enqueue task, notification dropped",
			"type", taskType, "task_id", t.ID, "error", err)
		return
	}
	metrics.Add("enqueued", 1)
}

func encode(t task) (string, error) {
	raw, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ---------------------------------------------------------------------------
// The claim protocol
// ---------------------------------------------------------------------------
//
// Every terminal transition below is one script whose first statement is
// LREM on the processing list. That LREM is the ownership token: exactly one
// caller can remove a given entry, so exactly one caller may go on to
// acknowledge, reschedule, dead-letter or reclaim it. A worker whose LREM
// returns 0 has been reclaimed out from under itself and does nothing, which is
// what keeps a slow handler and the sweeper from both rescheduling the same
// task.

// ackScript drops an entry that completed.
var ackScript = redis.NewScript(`
if redis.call('LREM', KEYS[1], 1, ARGV[1]) == 1 then
	redis.call('HDEL', KEYS[2], ARGV[2])
	return 1
end
return 0
`)

// rescheduleScript moves an entry to the delayed set, due at ARGV[2].
var rescheduleScript = redis.NewScript(`
if redis.call('LREM', KEYS[2], 1, ARGV[3]) == 1 then
	redis.call('ZADD', KEYS[1], ARGV[2], ARGV[1])
	redis.call('HDEL', KEYS[3], ARGV[4])
	return 1
end
return 0
`)

// deadScript moves an entry to the dead-letter list and trims it.
var deadScript = redis.NewScript(`
if redis.call('LREM', KEYS[2], 1, ARGV[3]) == 1 then
	redis.call('LPUSH', KEYS[1], ARGV[1])
	redis.call('LTRIM', KEYS[1], 0, ARGV[2])
	redis.call('HDEL', KEYS[3], ARGV[4])
	return 1
end
return 0
`)

// reclaimScript takes an abandoned entry back to the tail of the pending list,
// where it is the next task claimed rather than the last.
var reclaimScript = redis.NewScript(`
if redis.call('LREM', KEYS[1], 1, ARGV[1]) == 1 then
	redis.call('RPUSH', KEYS[2], ARGV[1])
	redis.call('HDEL', KEYS[3], ARGV[2])
	return 1
end
return 0
`)

// releaseDueScript moves retries whose delay has elapsed back onto the pending
// list. ZREM is the ownership token here, and it is in the same script as the
// RPUSH so no instance can crash between them.
var releaseDueScript = redis.NewScript(`
local due = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, ARGV[2])
local moved = 0
for i = 1, #due do
	if redis.call('ZREM', KEYS[1], due[i]) == 1 then
		redis.call('RPUSH', KEYS[2], due[i])
		moved = moved + 1
	end
end
return moved
`)

// ---------------------------------------------------------------------------
// Workers
// ---------------------------------------------------------------------------

func (n *Notifier) worker() {
	var consecutiveErrors int

	for {
		select {
		case <-n.shutdown:
			return
		default:
		}

		// BLMOVE, not BRPOP: the task is written into the processing list in
		// the same round trip that removes it from the pending list, so there
		// is no instant at which the only copy is a local variable in this
		// goroutine. That instant is what made this queue at-most-once.
		raw, err := n.rdb.BLMove(context.Background(), queueKey, processingKey, "RIGHT", "LEFT", n.pollInterval).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				consecutiveErrors = 0
				continue
			}
			consecutiveErrors++
			backoff := min(time.Duration(100<<min(consecutiveErrors-1, 5))*time.Millisecond, 5*time.Second)
			n.logger.Error("notifier: claim failed", "error", err, "backoff", backoff)
			if n.sleep(backoff) {
				return
			}
			continue
		}
		consecutiveErrors = 0

		n.handleClaimed(raw)
	}
}

// handleClaimed runs one claimed entry to a terminal state.
func (n *Notifier) handleClaimed(raw string) {
	var t task
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		// Nothing will ever parse this. Retrying it would re-read the same
		// bytes forever, so it goes to the dead-letter list where an operator
		// can see it.
		n.logger.Error("notifier: undecodable task, dead-lettering", "error", err)
		n.toDeadLetter(raw, raw, "", "undecodable task")
		return
	}
	if t.ID == "" {
		// Enqueued by a build that predates task ids. Give it one now so the
		// claim, delayed-set and dead-letter bookkeeping below can address it.
		t.ID = uuid.NewString()
	}

	n.recordClaim(t.ID)

	t.Attempts++
	handler, ok := n.handlers[t.Type]
	if !ok {
		// Not a poison message: an instance running an older build does not
		// know a task type a newer one enqueued. Reschedule without consuming
		// a retry and let an instance that does know it pick it up. TaskTTL
		// stops a genuinely unknown type from circulating forever.
		n.logger.Warn("notifier: no handler for task type", "type", t.Type, "task_id", t.ID)
		n.settle(raw, t, outcomeNotTried, errors.New("no handler registered"))
		return
	}

	err := n.run(t, handler)
	n.settle(raw, t, classify(err), err)
}

// run executes one attempt, converting a panicking handler into an error so the
// task is retried rather than swallowed by the recover.
func (n *Notifier) run(t task, handler Handler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			n.logger.Error("notifier: task panic", "type", t.Type, "task_id", t.ID, "error", r, "retries", t.Retries)
			err = fmt.Errorf("notifier: handler panicked: %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), n.taskTimeout)
	defer cancel()

	return handler(ctx, t.Data)
}

// settle applies the outcome of one attempt. Every branch ends with the task
// either acknowledged, scheduled in Redis, or in the dead-letter list — never
// only in this goroutine.
func (n *Notifier) settle(raw string, t task, o outcome, err error) {
	if err != nil {
		t.LastError = err.Error()
	}
	now := time.Now().UTC()

	if age, known := t.age(now); known && age > n.taskTTL && o != outcomeDone {
		n.logger.Error("notifier: task exceeded its lifetime, dead-lettering",
			"type", t.Type, "task_id", t.ID, "age", age, "attempts", t.Attempts, "error", err)
		n.deadLetterTask(raw, t, "exceeded task ttl")
		return
	}

	switch o {
	case outcomeDone:
		n.ack(raw, t)
		metrics.Add("processed", 1)

	case outcomeNotTried:
		// The provider was never called, so this costs no retry. Only the
		// attempt counter moves.
		n.logger.Warn("notifier: task refused before it was attempted, rescheduling",
			"type", t.Type, "task_id", t.ID, "error", err,
			"attempts", t.Attempts, "retries", t.Retries, "delay", n.notAttemptedDelay)
		n.reschedule(raw, t, now.Add(n.notAttemptedDelay))
		metrics.Add("not_attempted", 1)

	case outcomeRetry:
		if t.Retries >= n.maxRetries {
			n.logger.Error("notifier: task exhausted all retries, dead-lettering",
				"type", t.Type, "task_id", t.ID, "error", err, "retries", t.Retries)
			n.deadLetterTask(raw, t, "retries exhausted")
			return
		}
		t.Retries++
		delay := n.retryDelay(t.Retries)
		n.logger.Warn("notifier: task failed, scheduling retry",
			"type", t.Type, "task_id", t.ID, "error", err,
			"retry", t.Retries, "max", n.maxRetries, "delay", delay)
		n.reschedule(raw, t, now.Add(delay))
		metrics.Add("retried", 1)

	case outcomePermanent:
		n.logger.Error("notifier: task permanently rejected, dead-lettering",
			"type", t.Type, "task_id", t.ID, "error", err, "attempts", t.Attempts)
		n.deadLetterTask(raw, t, "permanently rejected")
	}
}

// retryDelay returns how long a task waits before its retries-th attempt. Past
// the end of the table every further attempt waits the longest interval.
func (n *Notifier) retryDelay(retries int) time.Duration {
	i := retries - 1
	if i < 0 {
		i = 0
	}
	if i >= len(n.retryBackoff) {
		i = len(n.retryBackoff) - 1
	}
	return n.retryBackoff[i]
}

func (n *Notifier) ack(raw string, t task) {
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()

	if err := ackScript.Run(ctx, n.rdb, []string{processingKey, claimsKey}, raw, t.ID).Err(); err != nil {
		// The task stays in the processing list and the sweeper will hand it to
		// another worker, which delivers it a second time. That is the
		// at-least-once trade: a duplicate email beats a missing one.
		n.logger.Error("notifier: failed to acknowledge task, it may be redelivered",
			"type", t.Type, "task_id", t.ID, "error", err)
	}
}

func (n *Notifier) reschedule(raw string, t task, due time.Time) {
	next, err := encode(t)
	if err != nil {
		// Cannot re-encode: leave the entry in the processing list untouched so
		// the sweeper reclaims the original bytes.
		n.logger.Error("notifier: failed to marshal task for retry, leaving it claimed",
			"type", t.Type, "task_id", t.ID, "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()

	err = rescheduleScript.Run(ctx, n.rdb,
		[]string{delayedKey, processingKey, claimsKey},
		next, strconv.FormatInt(due.UnixMilli(), 10), raw, t.ID,
	).Err()
	if err != nil {
		n.logger.Error("notifier: failed to schedule retry, task stays claimed for reclaim",
			"type", t.Type, "task_id", t.ID, "error", err)
	}
}

func (n *Notifier) deadLetterTask(raw string, t task, reason string) {
	next, err := encode(t)
	if err != nil {
		next = raw
	}
	n.toDeadLetter(raw, next, t.Type, reason)
}

func (n *Notifier) toDeadLetter(raw, stored, taskType, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()

	err := deadScript.Run(ctx, n.rdb,
		[]string{deadKey, processingKey, claimsKey},
		stored, strconv.Itoa(deadLetterCap-1), raw, taskIDOf(stored),
	).Err()
	if err != nil {
		n.logger.Error("notifier: failed to dead-letter task", "type", taskType, "reason", reason, "error", err)
		return
	}
	metrics.Add("dead_lettered", 1)
}

func taskIDOf(raw string) string {
	var t task
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return ""
	}
	return t.ID
}

// ---------------------------------------------------------------------------
// Reclaim sweeper
// ---------------------------------------------------------------------------

// reclaimLoop returns tasks whose claim went stale to the pending list.
//
// This is what makes a worker that died mid-handle recoverable: the entry it
// claimed is still in the processing list, its claim timestamp stops advancing,
// and one visibility timeout later any instance may take it back. It runs on
// every instance, so recovery does not depend on the dead process ever coming
// back — the equivalent of the 'processing AND updated_at < NOW() - interval'
// clause in data.WebhookEventModel.GetPendingDue.
func (n *Notifier) reclaimLoop() {
	// Sweep once at startup so the clock on anything abandoned by the previous
	// process starts at boot rather than at the first tick.
	n.reclaimStale()

	ticker := time.NewTicker(n.reclaimInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.shutdown:
			return
		case <-ticker.C:
			n.reclaimStale()
		}
	}
}

func (n *Notifier) reclaimStale() {
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()

	entries, err := n.rdb.LRange(ctx, processingKey, 0, reclaimBatch-1).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			n.logger.Error("notifier: failed to read processing list", "error", err)
		}
		return
	}

	now := time.Now().UTC()
	for _, raw := range entries {
		id := taskIDOf(raw)
		if id == "" {
			// Undecodable entries cannot be tracked; hand it to a worker, which
			// dead-letters it with a log line.
			n.reclaim(ctx, raw, id)
			continue
		}

		claimed, ok, err := n.claimTime(ctx, id)
		if err != nil {
			return
		}
		if !ok {
			// Claimed between the BLMOVE and its HSET, or its claim record was
			// lost. Record one now: the entry becomes reclaimable a visibility
			// timeout from this sighting instead of immediately, so a task a
			// live worker just picked up is never stolen from it.
			n.recordClaim(id)
			continue
		}
		if now.Sub(claimed) < n.visibilityTimeout {
			continue
		}

		n.logger.Warn("notifier: reclaiming abandoned task",
			"task_id", id, "claimed_for", now.Sub(claimed))
		n.reclaim(ctx, raw, id)
	}
}

func (n *Notifier) reclaim(ctx context.Context, raw, id string) {
	moved, err := reclaimScript.Run(ctx, n.rdb, []string{processingKey, queueKey, claimsKey}, raw, id).Int64()
	if err != nil {
		n.logger.Error("notifier: failed to reclaim task", "task_id", id, "error", err)
		return
	}
	if moved == 1 {
		metrics.Add("reclaimed", 1)
	}
}

func (n *Notifier) recordClaim(id string) {
	if id == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()

	if err := n.rdb.HSet(ctx, claimsKey, id, time.Now().UTC().UnixMilli()).Err(); err != nil {
		// The sweeper writes a claim for any entry that has none, so the worst
		// case is a reclaim one sweep later, never a lost task.
		n.logger.Error("notifier: failed to record claim", "task_id", id, "error", err)
	}
}

func (n *Notifier) claimTime(ctx context.Context, id string) (time.Time, bool, error) {
	ms, err := n.rdb.HGet(ctx, claimsKey, id).Int64()
	switch {
	case errors.Is(err, redis.Nil):
		return time.Time{}, false, nil
	case err != nil:
		n.logger.Error("notifier: failed to read claim", "task_id", id, "error", err)
		return time.Time{}, false, err
	}
	return time.UnixMilli(ms).UTC(), true, nil
}

// ---------------------------------------------------------------------------
// Retry scheduler
// ---------------------------------------------------------------------------

// scheduleLoop releases retries whose backoff has elapsed.
//
// The backoff lives in a Redis sorted set rather than in a time.After inside a
// goroutine, which is what makes it survive SIGTERM: there is no in-process
// timer holding the only copy of a task, so a process that stops between a
// failure and its retry loses nothing.
func (n *Notifier) scheduleLoop() {
	ticker := time.NewTicker(n.scheduleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.shutdown:
			return
		case <-ticker.C:
			n.releaseDue()
		}
	}
}

func (n *Notifier) releaseDue() {
	ctx, cancel := context.WithTimeout(context.Background(), redisOpTimeout)
	defer cancel()

	moved, err := releaseDueScript.Run(ctx, n.rdb,
		[]string{delayedKey, queueKey},
		strconv.FormatInt(time.Now().UTC().UnixMilli(), 10), scheduleBatch,
	).Int64()
	if err != nil {
		n.logger.Error("notifier: failed to release due retries", "error", err)
		return
	}
	if moved > 0 {
		metrics.Add("released", moved)
	}
}

// sleep waits for d, or returns true if shutdown came first.
func (n *Notifier) sleep(d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-n.shutdown:
		return true
	case <-timer.C:
		return false
	}
}

// Shutdown signals workers to stop and waits for in-flight attempts to finish.
// It is safe to call more than once and from more than one goroutine.
//
// realtime.Hub.Shutdown expresses the same intent with a select/default guard;
// sync.Once is used here because two concurrent callers can both pass that
// guard and race into the same close.
//
// It returns once every worker has finished its current attempt and noticed the
// signal, which takes up to one PollInterval: a worker parked in a blocking
// claim runs that claim out, because Redis block timeouts have one-second
// resolution and there is nothing to gain by aborting the read.
//
// Anything still queued, claimed or scheduled stays in Redis. A task claimed by
// a worker that does not finish before the drain deadline is reclaimed by the
// next process a visibility timeout later.
func (n *Notifier) Shutdown() {
	if n == nil {
		return
	}
	n.shutdownOnce.Do(func() {
		close(n.shutdown)
	})
	n.wg.Wait()
}
