package main

import (
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
	"github.com/stodulski/vibe-server/internal/health"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/middleware"
	"github.com/stodulski/vibe-server/internal/notifier"
	platformdb "github.com/stodulski/vibe-server/internal/platform/db"
	platformredis "github.com/stodulski/vibe-server/internal/platform/redis"
	"github.com/stodulski/vibe-server/internal/realtime"
	"github.com/stodulski/vibe-server/internal/scheduler"
)

// The adapters below bridge concrete infrastructure clients to the small
// interfaces the domain modules declare. They live here, in the composition
// root, because that is the only place that should know which database driver
// or cache client the application actually uses.

// dbProbe adapts the Postgres pool to health.Pinger and health.PoolReporter.
type dbProbe struct{ pool *platformdb.Pool }

func (p dbProbe) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

// PoolStats reports the figures that name pool exhaustion.
//
// empty_acquire is the one that matters and the one that was never read:
// pgxpool counts every acquire that had to wait because no connection was
// free. A pool of 25 in front of a database that has slowed down produces a
// service that is up, answering, and queueing — which looks like nothing at
// all from the outside. canceled_acquire counts the requests that gave up
// waiting, and acquire_wait_ms is how long the waiting cost in total.
func (p dbProbe) PoolStats() map[string]int64 {
	s := p.pool.Stat()
	return map[string]int64{
		"total":            int64(s.TotalConns()),
		"idle":             int64(s.IdleConns()),
		"in_use":           int64(s.AcquiredConns()),
		"max":              int64(s.MaxConns()),
		"constructing":     int64(s.ConstructingConns()),
		"acquire_count":    s.AcquireCount(),
		"empty_acquire":    s.EmptyAcquireCount(),
		"canceled_acquire": s.CanceledAcquireCount(),
		"acquire_wait_ms":  s.AcquireDuration().Milliseconds(),
	}
}

// redisProbe adapts the Redis client to the same two interfaces. go-redis
// returns a command rather than an error, which is why this cannot satisfy
// health.Pinger directly.
type redisProbe struct{ rdb *platformredis.Client }

func (p redisProbe) Ping(ctx context.Context) error { return p.rdb.Ping(ctx).Err() }

func (p redisProbe) PoolStats() map[string]int64 {
	s := p.rdb.PoolStats()
	return map[string]int64{
		"hits":     int64(s.Hits),
		"misses":   int64(s.Misses),
		"timeouts": int64(s.Timeouts),
		"total":    int64(s.TotalConns),
		"idle":     int64(s.IdleConns),
		"stale":    int64(s.StaleConns),
	}
}

// healthProbes returns the probes for whatever infrastructure is configured.
//
// Each is returned as a nil interface when its client is absent, rather than
// as an interface holding a nil pointer — the latter is non-nil to a nil check
// and would panic on the first probe instead of reporting "not configured".
//
// A free function rather than a method on *application: newApplication builds
// this before app.db/app.rdb are published, and a method would have to read
// those fields — the back-reference the composition root exists to remove.
func healthProbes(db *platformdb.Pool, rdb *platformredis.Client) (database, cache health.Pinger, dbPool, cachePool health.PoolReporter) {
	if db != nil {
		probe := dbProbe{pool: db}
		database, dbPool = probe, probe
	}
	if rdb != nil {
		probe := redisProbe{rdb: rdb}
		cache, cachePool = probe, probe
	}
	return database, cache, dbPool, cachePool
}

// userCache adapts the middleware's cached-user store to the narrow
// invalidation interface the admin module asks for.
//
// It holds *middleware.Middleware directly rather than *application: the
// former is what newApplication builds this from, and holding the latter
// would compile no matter which order middleware and admin were built in,
// defeating the point of building admin from a local.
type userCache struct{ mw *middleware.Middleware }

func (c userCache) InvalidateUser(ctx context.Context, id uuid.UUID) {
	c.mw.InvalidateUser(ctx, id)
}

// recordedNotification is one task published to a memoryQueue.
type recordedNotification struct {
	taskType string
	payload  any
}

// memoryQueue is the notifications.Queue newApplication builds when no Redis
// client is available: it records what was enqueued instead of publishing it.
//
// notifier.Enqueue is not nil-safe on a nil *platformredis.Client, so a notifier
// wrapped around one is not a usable fallback — this is a real, separate
// implementation, not notifier pointed at nothing. It is what a unit test
// needs to assert a handler actually published a notification, and what
// production would never construct: main() refuses to boot without Redis
// before newApplication is ever called.
type memoryQueue struct {
	mu    sync.Mutex
	tasks []recordedNotification
}

func (q *memoryQueue) Enqueue(taskType string, payload any) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks = append(q.tasks, recordedNotification{taskType: taskType, payload: payload})
}

// RegisterHandler is the worker side, which a memoryQueue never runs.
func (q *memoryQueue) RegisterHandler(string, func(context.Context, json.RawMessage) error) {
}

// payloadsOf returns the payloads enqueued under taskType, in order.
func (q *memoryQueue) payloadsOf(taskType string) []any {
	q.mu.Lock()
	defer q.mu.Unlock()

	var out []any
	for _, task := range q.tasks {
		if task.taskType == taskType {
			out = append(out, task.payload)
		}
	}
	return out
}

// taskTypes returns every enqueued task type, in order, for failure messages.
func (q *memoryQueue) taskTypes() []string {
	q.mu.Lock()
	defer q.mu.Unlock()

	out := make([]string, 0, len(q.tasks))
	for _, task := range q.tasks {
		out = append(out, task.taskType)
	}
	return out
}

// taskQueue adapts the Redis-backed notifier to notifications.Queue.
//
// The adapter exists for one reason: notifier.RegisterHandler takes a named
// notifier.Handler type, and Go requires an exact signature match to satisfy an
// interface. Declaring notifications.Queue in terms of a plain func keeps that
// package from importing the queue implementation, and the conversion lands
// here instead.
type taskQueue struct{ n *notifier.Notifier }

func (q taskQueue) Enqueue(taskType string, payload any) {
	q.n.Enqueue(taskType, payload)
}

func (q taskQueue) RegisterHandler(taskType string, h func(ctx context.Context, payload json.RawMessage) error) {
	q.n.RegisterHandler(taskType, notifier.Handler(h))
}

// ---------------------------------------------------------------------------
// Observability adapters
// ---------------------------------------------------------------------------

// breakerProbe adapts a circuit breaker to health.Breaker.
//
// It exists so internal/health does not import internal/circuitbreaker, and so
// the decision that MercadoPago is the one whose failure stops the money —
// rather than WhatsApp or the mailer — is made here, in the composition root,
// next to where the three breakers are built.
type breakerProbe struct {
	cb *circuitbreaker.CircuitBreaker
	// blocksRevenue is true only for the payment provider. An open WhatsApp or
	// mailer circuit costs notifications; an open MercadoPago circuit means no
	// client on any tenant can pay.
	blocksRevenue bool
}

func (b breakerProbe) Name() string        { return b.cb.Name() }
func (b breakerProbe) State() string       { return b.cb.State().String() }
func (b breakerProbe) BlocksRevenue() bool { return b.blocksRevenue }

// processMetrics is the process's own counters, gathered for the detailed
// health endpoint.
//
// Three sources are merged because they were each unreachable on their own:
// the request middleware's counters (new), the runtime's goroutine and heap
// figures, and the expvar variables the notifier already publishes — which
// were computed on every deployment and served only when the environment was
// "development".
type processMetrics struct{ app *application }

func (p processMetrics) Metrics() map[string]any {
	out := map[string]any{}

	if p.app.middleware != nil {
		for key, value := range p.app.middleware.Metrics() {
			out[key] = value
		}
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	out["goroutines"] = int64(runtime.NumGoroutine())
	// G115: heap size and object count are process facts bounded by the
	// machine's memory, many orders of magnitude below the int64 ceiling; the
	// conversion exists only because runtime reports them unsigned.
	out["heap_alloc_bytes"] = int64(mem.HeapAlloc) //nolint:gosec
	out["heap_objects"] = int64(mem.HeapObjects)   //nolint:gosec
	out["gc_cycles"] = int64(mem.NumGC)
	out["uptime_seconds"] = int64(time.Since(processStart).Seconds())

	// expvar's own "memstats" and "cmdline" are deliberately skipped: the first
	// duplicates the fields above at ten times the size, and the second is the
	// process command line, which on this deployment carries flag-supplied
	// secrets.
	expvar.Do(func(kv expvar.KeyValue) {
		switch kv.Key {
		case "memstats", "cmdline":
			return
		}
		out["expvar_"+kv.Key] = json.RawMessage(kv.Value.String())
	})

	return out
}

// processStart is when this process booted, so uptime_seconds can distinguish
// "the counters are zero because nothing is happening" from "the counters are
// zero because we restarted ninety seconds ago".
var processStart = time.Now()

// queueProbe reports the backlog of the two durable work queues.
//
// The SQL is here rather than in internal/data on purpose. This is an
// operational aggregate over two tables, read by one caller, that no domain
// handler ever wants; giving it a store method would put a health concern into
// the interface every booking handler depends on. It is the same reasoning
// that already lets dbProbe call pool.Stat() directly.
//
// One caveat worth knowing: webhook_events' partial index (idx_webhook_events_pending) is
// declared on 'pending' and 'processing' only, so the exhausted count is not
// index-covered and this is a scan of the table's non-processed rows. That is
// affordable because the retention sweep keeps the table to ninety days, and
// the query is bounded by the health handler's own deadline either way.
type queueProbe struct{ pool *platformdb.Pool }

// queueDepthSQL counts each queue's non-terminal rows and the age of the
// oldest one that is already due.
//
// Depth alone cannot tell a queue that is draining steadily from one that has
// stopped: both can hold fifty rows. The oldest due age can — a sweeper that
// is keeping up never leaves an item due for more than its own interval.
const queueDepthSQL = `
SELECT 'webhook_events' AS queue,
       count(*) FILTER (WHERE status = 'pending')    AS pending,
       count(*) FILTER (WHERE status = 'processing') AS processing,
       count(*) FILTER (WHERE status = 'exhausted')  AS exhausted,
       COALESCE(EXTRACT(EPOCH FROM now() - min(next_retry_at)
           FILTER (WHERE status IN ('pending', 'processing') AND next_retry_at <= now())), 0)::bigint
           AS oldest_due_seconds
FROM webhook_events
WHERE status IN ('pending', 'processing', 'exhausted')
UNION ALL
SELECT 'failed_refunds',
       count(*) FILTER (WHERE status = 'pending'),
       count(*) FILTER (WHERE status = 'processing'),
       count(*) FILTER (WHERE status = 'exhausted'),
       COALESCE(EXTRACT(EPOCH FROM now() - min(next_retry_at)
           FILTER (WHERE status IN ('pending', 'processing') AND next_retry_at <= now())), 0)::bigint
FROM failed_refunds
WHERE status IN ('pending', 'processing', 'exhausted')`

func (p queueProbe) QueueStats(ctx context.Context) ([]health.QueueStats, error) {
	rows, err := p.pool.Query(ctx, queueDepthSQL)
	if err != nil {
		return nil, fmt.Errorf("queue depth: %w", err)
	}
	defer rows.Close()

	var out []health.QueueStats
	for rows.Next() {
		var s health.QueueStats
		if err := rows.Scan(&s.Name, &s.Pending, &s.Processing, &s.Exhausted, &s.OldestDueSeconds); err != nil {
			return nil, fmt.Errorf("queue depth: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("queue depth: %w", err)
	}
	return out, nil
}

// dbPoolLogArgs returns the connection-pool figures as slog key/value pairs,
// for the line every cron run leaves.
//
// The pool is where a slow database becomes a dead service, and pgxpool has
// counted it all along: empty_acquire is every acquire that had to wait for a
// free connection. Nothing ever read it. Attaching it to the cron heartbeat
// means the figure is in the log at a bounded rate whether or not anyone is
// looking at a health endpoint at the time — which, during the night the pool
// filled up, nobody was.
func (app *application) dbPoolLogArgs() []any {
	if app.db == nil {
		return nil
	}

	stats := dbProbe{pool: app.db}.PoolStats()
	keys := slices.Sorted(maps.Keys(stats))

	args := make([]any, 0, len(keys)*2)
	for _, key := range keys {
		args = append(args, "db_"+key, stats[key])
	}
	return args
}

// observedLocker wraps the scheduler's advisory locker so that a run which
// never happened still leaves a line.
//
// The skip is invisible from inside the job: the scheduler calls Run only when
// the lock was taken, so a job that has not executed on this instance for a
// week is indistinguishable from one that ran and found nothing. The lock is
// the only place that knows, and the composition root is the only place that
// can wrap it without reaching into a package another concern owns.
type observedLocker struct {
	inner  scheduler.Locker
	logger *slog.Logger
}

func (l observedLocker) TryAdvisory(ctx context.Context, key string) (bool, func(), error) {
	acquired, release, err := l.inner.TryAdvisory(ctx, key)
	switch {
	case err != nil:
		l.logger.Error("cron: could not take the job lock", "job", jobOf(key), "error", err)
	case !acquired:
		l.logger.Info("cron: skipped, another instance holds the lock", "job", jobOf(key))
	}
	return acquired, release, err
}

// jobOf strips the scheduler's "cron:" lock-key prefix so the log line names
// the job the way every other cron line does.
func jobOf(key string) string { return strings.TrimPrefix(key, "cron:") }

// streamAuthorizer re-decides, on an open Server-Sent Events stream, whether
// the caller may still receive a complex's events.
//
// It answers by running the request through the real guard chain again —
// Authenticate, RequireAuth, RequireComplexOwner — rather than by
// re-implementing what those guards check. That matters more here than
// anywhere else: a second copy of the authorization rules would be the copy
// nobody updates, and it would be the one deciding whether a revoked session
// keeps a live feed of a venue's bookings.
type streamAuthorizer struct{ mw *middleware.Middleware }

// Authorize builds a request that carries the caller's credentials and nothing
// else, and reports whether the chain still lets it through.
//
// H-11: this used to build the probe on top of ctx, which is always a
// descendant of the connect-time request's own context — stream.go's re-check
// loop derives it from r.Context() read once at Stream()'s entry, then only
// wraps it with a deadline, so it still carries whatever RequireAuth and
// RequireComplexOwner put there when the stream was first authorized.
// Authenticate re-reads the cookie and re-consults the blacklist on every
// re-check, but on a blacklisted or otherwise invalid token its response is
// to call the next handler unchanged — it only ever *adds* a user to the
// context on success, it never removes one (chain.go). So a probe built on
// top of that stale context let RequireAuth find the leftover user and pass
// it through no matter what the blacklist said: a revoked session kept
// receiving live events until the stream's own 15-minute lifetime cap, not
// until the next re-check.
//
// The fix direction this finding preferred is clearing the user inside
// Authenticate itself, which would also protect any future caller that
// re-runs the chain — but the context key the user lives under
// (internal/httpx's unexported userContextKey) has no exported way to clear
// it, only to set it, and adding one is out of scope here. This is the
// fallback the finding names instead: probeCtx starts from
// context.Background(), which has no ancestor holding that key at all, and
// carries forward only the two things the chain actually needs that ctx
// would otherwise have supplied — the deadline stream.go's authorized()
// already put on ctx, and the request id, so a re-check that fails still logs
// under the stream's own correlation id. Neither of those is identity, so
// neither can resurrect the bug this fix closes.
//
// The route parameter the ownership guard reads is supplied from the
// stream's own subject rather than read back off the URL — a probe request
// built from r still carries the original request's path, not this
// authorizer's complexID argument.
func (a streamAuthorizer) Authorize(ctx context.Context, r *http.Request, complexID uuid.UUID) error {
	probeCtx := context.Background()
	if deadline, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		probeCtx, cancel = context.WithDeadline(probeCtx, deadline)
		defer cancel()
	}
	params := httprouter.Params{{Key: "id", Value: complexID.String()}}
	probeCtx = context.WithValue(probeCtx, httprouter.ParamsKey, params)

	//nolint:contextcheck // intentionally detached from ctx/r.Context(): see
	// the func comment above (H-11) — probeCtx starting from
	// context.Background() rather than an inherited context is the fix, not
	// an oversight.
	probe := r.Clone(probeCtx)
	probe = httpx.ContextSetRequestID(probe, httpx.ContextGetRequestID(r))

	var (
		sink    = &statusSink{header: make(http.Header)}
		allowed bool
	)
	chain := a.mw.Authenticate(a.mw.RequireAuth(a.mw.RequireComplexOwner(
		func(http.ResponseWriter, *http.Request) { allowed = true },
	)))
	chain.ServeHTTP(sink, probe)

	if allowed {
		return nil
	}
	// The chain's own status is what separates "this caller is out" from "we
	// could not tell": 401/403/404 are verdicts, a 5xx is the database or the
	// cache failing to answer. The stream treats the two differently, so
	// collapsing them here would either disconnect every dashboard on a blip
	// or keep a revoked one alive through an outage.
	if sink.status >= http.StatusInternalServerError {
		return fmt.Errorf("re-authorization could not be completed: status %d", sink.status)
	}
	return fmt.Errorf("%w: status %d", realtime.ErrStreamUnauthorized, sink.status)
}

// statusSink is the http.ResponseWriter the re-authorization probe writes to.
// Nothing it receives reaches a client; only the status code is read back.
type statusSink struct {
	header http.Header
	status int
}

func (s *statusSink) Header() http.Header { return s.header }

func (s *statusSink) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
}

func (s *statusSink) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return len(b), nil
}
