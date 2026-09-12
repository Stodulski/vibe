package db

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// clock is the tracer's time source, advanced by hand so a test can measure a
// query that took a second without taking a second.
type clock struct {
	now  time.Time
	step time.Duration
}

func (c *clock) next() time.Time {
	t := c.now
	c.now = c.now.Add(c.step)
	return t
}

// trace runs one query through the tracer and returns what it logged.
func trace(t *testing.T, threshold, elapsed time.Duration, sql string, tag pgconn.CommandTag, queryErr error) string {
	t.Helper()

	var out strings.Builder
	logger := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelWarn}))

	tracer := NewSlowQueryTracer(threshold, logger)
	if tracer == nil {
		t.Fatal("NewSlowQueryTracer: nil for an armed threshold")
	}
	c := &clock{now: time.Unix(0, 0), step: elapsed}
	tracer.now = c.next

	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: sql})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{CommandTag: tag, Err: queryErr})

	return out.String()
}

func TestASlowQueryIsLogged(t *testing.T) {
	got := trace(t, 500*time.Millisecond, 750*time.Millisecond,
		"-- name: GetBookingsByComplex :many\nSELECT id FROM bookings WHERE complex_id = $1",
		pgconn.NewCommandTag("SELECT 3"), nil)

	for _, want := range []string{"slow query", "GetBookingsByComplex", "duration_ms=750", "rows=3", "level=WARN"} {
		if !strings.Contains(got, want) {
			t.Errorf("log line %q does not contain %q", got, want)
		}
	}
}

func TestAQueryUnderTheThresholdIsSilent(t *testing.T) {
	got := trace(t, 500*time.Millisecond, 499*time.Millisecond,
		"-- name: GetUser :one\nSELECT id FROM users WHERE id = $1",
		pgconn.NewCommandTag("SELECT 1"), nil)

	if got != "" {
		t.Errorf("a fast query logged %q", got)
	}
}

// TestTheArgumentsAreNeverLogged is the reason this tracer names the query
// instead of printing the statement: every argument this API binds is somebody's
// email address, phone number or payment identifier.
func TestTheArgumentsAreNeverLogged(t *testing.T) {
	got := trace(t, time.Millisecond, time.Second,
		"-- name: GetUserByEmail :one\nSELECT id FROM users WHERE email = $1",
		pgconn.NewCommandTag("SELECT 1"), nil)

	for _, secret := range []string{"users", "email = $1", "SELECT id"} {
		if strings.Contains(got, secret) {
			t.Errorf("log line %q leaks %q", got, secret)
		}
	}
}

func TestTheQueryErrorIsCarried(t *testing.T) {
	got := trace(t, time.Millisecond, time.Second,
		"-- name: InsertBooking :one\nINSERT INTO bookings DEFAULT VALUES",
		pgconn.CommandTag{}, context.DeadlineExceeded)

	if !strings.Contains(got, "context deadline exceeded") {
		t.Errorf("log line %q does not carry the error", got)
	}
}

func TestAnUnarmedTracerIsNil(t *testing.T) {
	if NewSlowQueryTracer(0, slog.Default()) != nil {
		t.Error("a zero threshold arms the tracer")
	}
	if NewSlowQueryTracer(time.Second, nil) != nil {
		t.Error("a nil logger arms the tracer")
	}
}

func TestQueryName(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{"sqlc header", "-- name: GetUser :one\nSELECT 1", "GetUser"},
		{"sqlc header, no annotation", "-- name: GetUser\nSELECT 1", "GetUser"},
		{"leading whitespace", "\n  -- name: ListCourts :many\nSELECT 1", "ListCourts"},
		{"hand-written statement", "SELECT id FROM bookings WHERE complex_id = $1", "SELECT id FROM"},
		{"transaction control", "BEGIN", "BEGIN"},
		{"empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := QueryName(tc.sql); got != tc.want {
				t.Errorf("QueryName = %q, want %q", got, tc.want)
			}
		})
	}
}
