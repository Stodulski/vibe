package db

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// SlowQueryTracer logs one warn line per query that took longer than its
// threshold.
//
// Before it, a slow query was only ever visible second-hand: as a request that
// crossed the HTTP logger's own one-second line, with nothing saying which
// statement spent the time — and never at all for the queries the cron jobs
// and the notification workers run, which serve no request.
//
// It never logs arguments. Every argument this API binds is somebody's email
// address, phone number, name or payment identifier, and a log line is the
// last place any of them should turn up.
type SlowQueryTracer struct {
	threshold time.Duration
	logger    *slog.Logger
	// now is the clock, so a test can measure a query without waiting for one.
	now func() time.Time
}

// NewSlowQueryTracer returns a tracer, or nil when tracing is not wanted — a
// zero threshold or no logger. A nil tracer is not assigned to the pool, so
// the untraced path stays exactly as fast as it was.
func NewSlowQueryTracer(threshold time.Duration, logger *slog.Logger) *SlowQueryTracer {
	if threshold <= 0 || logger == nil {
		return nil
	}
	return &SlowQueryTracer{threshold: threshold, logger: logger, now: time.Now}
}

// traceKey carries the start time and the statement from TraceQueryStart to
// TraceQueryEnd. pgx guarantees the two run on the same context chain.
type traceKey struct{}

type queryTrace struct {
	start time.Time
	sql   string
}

// TraceQueryStart records when the query began.
func (t *SlowQueryTracer) TraceQueryStart(
	ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData,
) context.Context {
	return context.WithValue(ctx, traceKey{}, queryTrace{start: t.now(), sql: data.SQL})
}

// TraceQueryEnd logs the query when it was slower than the threshold.
func (t *SlowQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	trace, ok := ctx.Value(traceKey{}).(queryTrace)
	if !ok {
		return
	}
	elapsed := t.now().Sub(trace.start)
	if elapsed < t.threshold {
		return
	}

	attrs := []any{
		"duration_ms", float64(elapsed.Microseconds()) / 1000,
		"threshold_ms", float64(t.threshold.Microseconds()) / 1000,
		"query", QueryName(trace.sql),
		"rows", data.CommandTag.RowsAffected(),
	}
	if data.Err != nil {
		attrs = append(attrs, "error", data.Err.Error())
	}
	t.logger.LogAttrs(ctx, slog.LevelWarn, "slow query", argsToAttrs(attrs)...)
}

// argsToAttrs turns the key/value pairs above into slog attributes. LogAttrs
// is used rather than Warn so the line costs nothing to build when the level
// is off.
func argsToAttrs(args []any) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(args)/2)
	for i := 0; i+1 < len(args); i += 2 {
		key, _ := args[i].(string)
		attrs = append(attrs, slog.Any(key, args[i+1]))
	}
	return attrs
}

// QueryName is the sqlc query's name when the statement carries one.
//
// sqlc keeps the `-- name: GetUser :one` header it generated from as the first
// line of every query constant, which is the only handle on "which query was
// this" that does not involve logging the SQL — and the SQL is what carries
// the table and column names a log aggregator has no business holding.
// Anything else (a hand-written statement, a transaction control command)
// falls back to the first few words.
func QueryName(sql string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(sql), "\n")
	if name, ok := strings.CutPrefix(strings.TrimSpace(line), "-- name:"); ok {
		name = strings.TrimSpace(name)
		if verb := strings.IndexByte(name, ' '); verb > 0 {
			return name[:verb]
		}
		return name
	}
	return summarize(line)
}

// summarize is the fallback label: the leading keywords of a statement, capped,
// so a hand-written query is still identifiable without its predicates — the
// half of a statement that can carry a literal value.
func summarize(line string) string {
	const words = 3
	fields := strings.Fields(line)
	if len(fields) > words {
		fields = fields[:words]
	}
	return strings.Join(fields, " ")
}
