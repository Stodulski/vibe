//go:build integration

package main

import (
	"context"
	"testing"
	"time"

	platformdb "github.com/stodulski/vibe-server/internal/platform/db"

	"github.com/stodulski/vibe-server/internal/health"
)

// The queue-depth query is hand-written SQL against two tables, so a unit test
// with a stub proves nothing about it. These run it.

// clearWebhookEvents empties the queue table this file writes to. cleanupDB
// does not list it, and a leftover row from another test would move every
// count asserted below.
func clearWebhookEvents(t *testing.T, pool *platformdb.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := pool.Exec(ctx, "TRUNCATE webhook_events"); err != nil {
		t.Fatalf("truncating webhook_events: %v", err)
	}
}

// insertWebhookEvent writes one row in the given state, due at now+offset.
func insertWebhookEvent(t *testing.T, pool *platformdb.Pool, status string, offset time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := pool.Exec(ctx, `
		INSERT INTO webhook_events (provider, external_id, event_type, payload, status, next_retry_at)
		VALUES ('mercadopago', 'ext-'||gen_random_uuid(), 'payment', '{}'::jsonb, $1, now() + $2::interval)`,
		status, offset.String())
	if err != nil {
		t.Fatalf("inserting a %s webhook event: %v", status, err)
	}
}

// statsByName indexes the probe's answer.
func statsByName(t *testing.T, stats []health.QueueStats) map[string]health.QueueStats {
	t.Helper()

	out := make(map[string]health.QueueStats, len(stats))
	for _, s := range stats {
		out[s.Name] = s
	}
	return out
}

func TestQueueProbeCountsEachStateSeparately(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)
	clearWebhookEvents(t, pool)

	for range 3 {
		insertWebhookEvent(t, pool, "pending", -time.Minute)
	}
	insertWebhookEvent(t, pool, "processing", -time.Minute)
	for range 2 {
		insertWebhookEvent(t, pool, "exhausted", -time.Hour)
	}
	// Terminal work must not be reported as backlog: this table keeps 90 days
	// of processed rows, and counting them would make the number meaningless.
	insertWebhookEvent(t, pool, "processed", -time.Hour)

	stats, err := (queueProbe{pool: pool}).QueueStats(t.Context())
	if err != nil {
		t.Fatalf("QueueStats: %v", err)
	}

	byName := statsByName(t, stats)
	got, present := byName["webhook_events"]
	if !present {
		t.Fatalf("want a webhook_events row; got %+v", stats)
	}
	if got.Pending != 3 {
		t.Errorf("want 3 pending; got %d", got.Pending)
	}
	if got.Processing != 1 {
		t.Errorf("want 1 processing; got %d", got.Processing)
	}
	if got.Exhausted != 2 {
		t.Errorf("want 2 exhausted; got %d", got.Exhausted)
	}
}

// Depth alone cannot tell a queue that is draining steadily from one that has
// stopped: both can hold four rows. This can.
func TestQueueProbeReportsTheAgeOfTheOldestDueItem(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)
	clearWebhookEvents(t, pool)

	insertWebhookEvent(t, pool, "pending", -2*time.Hour)
	insertWebhookEvent(t, pool, "pending", -time.Minute)

	stats, err := (queueProbe{pool: pool}).QueueStats(t.Context())
	if err != nil {
		t.Fatalf("QueueStats: %v", err)
	}

	got := statsByName(t, stats)["webhook_events"]
	if got.OldestDueSeconds < 7100 || got.OldestDueSeconds > 7300 {
		t.Errorf("want roughly two hours (7200s) for the oldest due item; got %d", got.OldestDueSeconds)
	}
}

// A row scheduled for the future is not late, and reporting it as late would
// make the number fire on a queue that is behaving exactly as designed.
func TestAnItemNotYetDueIsNotReportedAsWaiting(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)
	clearWebhookEvents(t, pool)

	insertWebhookEvent(t, pool, "pending", time.Hour)

	stats, err := (queueProbe{pool: pool}).QueueStats(t.Context())
	if err != nil {
		t.Fatalf("QueueStats: %v", err)
	}

	got := statsByName(t, stats)["webhook_events"]
	if got.Pending != 1 {
		t.Errorf("want the row counted as pending; got %d", got.Pending)
	}
	if got.OldestDueSeconds != 0 {
		t.Errorf("want nothing reported as overdue; got %d seconds", got.OldestDueSeconds)
	}
}

// Both halves of the UNION have to execute. An empty queue must still report a
// row of zeros, or an operator cannot tell "nothing waiting" from "the query
// silently stopped covering this queue".
func TestQueueProbeReportsEveryQueueEvenWhenEmpty(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)
	clearWebhookEvents(t, pool)

	stats, err := (queueProbe{pool: pool}).QueueStats(t.Context())
	if err != nil {
		t.Fatalf("QueueStats: %v", err)
	}

	byName := statsByName(t, stats)
	for _, queue := range []string{"webhook_events", "failed_refunds"} {
		got, present := byName[queue]
		if !present {
			t.Errorf("want a %s row even with nothing in it; got %+v", queue, stats)
			continue
		}
		if got.Pending != 0 || got.Processing != 0 || got.Exhausted != 0 || got.OldestDueSeconds != 0 {
			t.Errorf("want %s all zeros; got %+v", queue, got)
		}
	}
}

// empty_acquire is the figure that names pool starvation, and nothing read it
// until now. It has to be present and it has to come from the pool.
func TestThePoolProbeReportsTheStarvationCounters(t *testing.T) {
	pool := setupTestDB(t)

	// One real acquire, so the counters are not all trivially zero.
	if err := pool.Ping(t.Context()); err != nil {
		t.Fatalf("ping: %v", err)
	}

	stats := dbProbe{pool: pool}.PoolStats()
	for _, key := range []string{"empty_acquire", "canceled_acquire", "acquire_count", "max", "in_use", "idle"} {
		if _, present := stats[key]; !present {
			t.Errorf("want %q in the pool figures; got %v", key, stats)
		}
	}
	if stats["acquire_count"] < 1 {
		t.Errorf("want the acquire the ping performed to be counted; got %d", stats["acquire_count"])
	}
	if stats["max"] != 5 {
		t.Errorf("want the configured ceiling reported, which is what in_use is read against; got %d", stats["max"])
	}
}
