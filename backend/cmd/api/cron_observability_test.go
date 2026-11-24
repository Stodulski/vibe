package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/bookings"
	"github.com/stodulski/vibe-server/internal/health"
	"github.com/stodulski/vibe-server/internal/jobs"
	"github.com/stodulski/vibe-server/internal/scheduler"
)

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

// cronFixture is a test application whose logger writes JSON into a buffer, so
// a test can assert on the fields of a cron line rather than on a substring.
type cronFixture struct {
	app  *application
	logs *bytes.Buffer
}

func newCronFixture(t *testing.T) *cronFixture {
	t.Helper()

	f := &cronFixture{logs: &bytes.Buffer{}}
	// The logger is named before the application is built, not assigned after:
	// the jobs that delegate to a domain service (the MercadoPago refresh
	// sweep) log through the logger that service captured at construction.
	logger := slog.New(slog.NewJSONHandler(f.logs, nil))
	f.app, _ = newTestApplicationWithLogger(t, logger)
	return f
}

// lines returns every log record whose message starts with prefix.
func (f *cronFixture) lines(t *testing.T, prefix string) []map[string]any {
	t.Helper()

	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(f.logs.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line is not JSON: %v\n%s", err, line)
		}
		if msg, _ := record["msg"].(string); strings.HasPrefix(msg, prefix) {
			out = append(out, record)
		}
	}
	return out
}

func (f *cronFixture) onlyLine(t *testing.T, prefix string) map[string]any {
	t.Helper()

	lines := f.lines(t, prefix)
	if len(lines) != 1 {
		t.Fatalf("want exactly one %q line; got %d\n%s", prefix, len(lines), f.logs.String())
	}
	return lines[0]
}

// field reads a field, failing when it is missing.
func field(t *testing.T, record map[string]any, key string) any {
	t.Helper()

	value, present := record[key]
	if !present {
		t.Fatalf("the log line has no %q field: %v", key, record)
	}
	return value
}

// ---------------------------------------------------------------------------
// The heartbeat
// ---------------------------------------------------------------------------

// The finding, stated exactly: a healthy deployment emitted roughly two cron
// log lines per day, and a completely dead one emitted zero. This is what
// makes them different.
func TestACronRunLeavesALineEvenWhenItDidNothing(t *testing.T) {
	f := newCronFixture(t)

	f.app.observed("clean_tokens", func(context.Context) {})(t.Context())

	line := f.onlyLine(t, "cron: run finished")
	if got := field(t, line, "job"); got != "clean_tokens" {
		t.Errorf("want the job named; got %v", got)
	}
	if _, present := line["duration_ms"]; !present {
		t.Error("want the run's duration on the line")
	}
}

// A duration field that is always zero passes any test for "the duration is
// logged". This asserts the number.
func TestTheHeartbeatCarriesTheTimeTheJobActuallyTook(t *testing.T) {
	f := newCronFixture(t)

	const slept = 20 * time.Millisecond
	f.app.observed("reminder_2h", func(context.Context) { time.Sleep(slept) })(t.Context())

	got, ok := field(t, f.onlyLine(t, "cron: run finished"), "duration_ms").(float64)
	if !ok {
		t.Fatalf("want duration_ms to be a number; got %T", field(t, f.onlyLine(t, "cron: run finished"), "duration_ms"))
	}
	if got < float64(slept.Milliseconds()) {
		t.Errorf("want at least %d ms, the time the job actually took; got %v", slept.Milliseconds(), got)
	}
}

// The instances that did not get the lock were silent, so a job that has not
// run on this instance for a week looked exactly like one that ran and found
// nothing.
func TestASkippedRunSaysWhyItWasSkipped(t *testing.T) {
	f := newCronFixture(t)
	locker := observedLocker{inner: heldLocks{}, logger: f.app.logger}

	acquired, release, err := locker.TryAdvisory(t.Context(), "cron:reminder_2h")
	release()

	if err != nil || acquired {
		t.Fatalf("setup: want the lock refused; got acquired=%v err=%v", acquired, err)
	}

	line := f.onlyLine(t, "cron: skipped")
	if got := field(t, line, "job"); got != "reminder_2h" {
		t.Errorf("want the job named without the lock-key prefix; got %v", got)
	}
	if msg, _ := line["msg"].(string); !strings.Contains(msg, "another instance holds the lock") {
		t.Errorf("want the reason stated; got %q", msg)
	}
}

// A lock that could not be taken at all is a different failure from one held
// elsewhere, and only one of the two is normal.
func TestALockFailureIsReportedAsAnError(t *testing.T) {
	f := newCronFixture(t)
	locker := observedLocker{inner: brokenLocks{}, logger: f.app.logger}

	_, release, err := locker.TryAdvisory(t.Context(), "cron:retry_refunds")
	release()

	if err == nil {
		t.Fatal("setup: want the lock attempt to fail")
	}
	line := f.onlyLine(t, "cron: could not take the job lock")
	if got := field(t, line, "level"); got != "ERROR" {
		t.Errorf("want the failure at ERROR; got %v", got)
	}
	if got := field(t, line, "job"); got != "retry_refunds" {
		t.Errorf("want the job named; got %v", got)
	}
}

// A run that took its lock must not also produce a skip line.
func TestATakenLockIsNotReportedAsASkip(t *testing.T) {
	f := newCronFixture(t)
	locker := observedLocker{inner: freeLocks{}, logger: f.app.logger}

	acquired, release, _ := locker.TryAdvisory(t.Context(), "cron:reminder_2h")
	release()

	if !acquired {
		t.Fatal("setup: want the lock taken")
	}
	if got := len(f.lines(t, "cron: skipped")); got != 0 {
		t.Errorf("want no skip line when the lock was taken; got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Each job's own outcome
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Retention: the resolved-refund sweep
// ---------------------------------------------------------------------------

// paymentstore.FailedRefunds.DeleteResolved was written and covered by an integration
// test, but no cron job called it, so in production nothing ever deleted a
// resolved refund attempt. Since refunds became claim-first the table takes a
// row per refund rather than per failure, so it grew for the life of the
// deployment and the sweeper's query — which runs every two minutes — grew with
// it. A job that is never registered is invisible from every other angle: the
// method compiles and its own test passes.
func TestTheResolvedRefundSweepIsScheduled(t *testing.T) {
	f := newCronFixture(t)

	jobs := f.app.cronJobs()

	var found *scheduler.Job
	for i := range jobs {
		if jobs[i].Name == "clean_failed_refunds" {
			found = &jobs[i]
		}
	}
	if found == nil {
		t.Fatal("nothing prunes failed_refunds, so the table grows forever and the sweeper's query with it")
	}
	if found.Run == nil {
		t.Error("a registered job with no work to run is the same as no job at all")
	}
	if found.Local {
		t.Error("a DELETE against the shared database must take the cross-instance lock like every other pruner")
	}
}

// The sweep has to reach the store through the interface and hand it the
// retention window, not merely log a line.
func TestTheResolvedRefundSweepPrunesThroughTheStore(t *testing.T) {
	f := newCronFixture(t)

	var got time.Duration
	calls := 0
	f.app.models.FailedRefunds = &mockFailedRefundStore{
		DeleteResolvedFn: func(_ context.Context, olderThan time.Duration) (int64, error) {
			calls++
			got = olderThan
			return 12, nil
		},
	}

	f.app.cronCleanFailedRefunds(t.Context())

	if calls != 1 {
		t.Fatalf("want the store asked to prune exactly once; got %d calls", calls)
	}
	if got != webhookEventRetention {
		t.Errorf("want the retention window the sibling forensic table uses (%v); got %v", webhookEventRetention, got)
	}
	if count := field(t, f.onlyLine(t, "cron_clean_failed_refunds: completed"), "count"); count != float64(12) {
		t.Errorf("want the number of rows deleted on the line, as the sibling job reports it; got %v", count)
	}
}

// A failed DELETE must say so rather than report a silent zero.
func TestAFailedResolvedRefundSweepIsReported(t *testing.T) {
	f := newCronFixture(t)
	f.app.models.FailedRefunds = &mockFailedRefundStore{
		DeleteResolvedFn: func(context.Context, time.Duration) (int64, error) {
			return 0, errors.New("statement timeout")
		},
	}

	f.app.cronCleanFailedRefunds(t.Context())

	line := f.onlyLine(t, "cron_clean_failed_refunds: failed")
	if got := field(t, line, "error"); got != "statement timeout" {
		t.Errorf("want the cause on the line; got %v", got)
	}
	if got := len(f.lines(t, "cron_clean_failed_refunds: completed")); got != 0 {
		t.Errorf("a sweep that failed must not also report completion; got %d lines", got)
	}
}

// fakeJobRetentionStore lets a test fake the one method clean_jobs needs
// from jobs.Store, which is concrete and otherwise needs a database.
type fakeJobRetentionStore struct {
	fn func(ctx context.Context, olderThan time.Duration) (int64, error)
}

func (f fakeJobRetentionStore) DeleteDone(ctx context.Context, olderThan time.Duration) (int64, error) {
	return f.fn(ctx, olderThan)
}

// TestTheFinishedJobsSweepPrunesThroughTheStore covers registration, the
// call and its retention window, and the completion log in one job: a sweep
// unregistered or uncalled is invisible from every other angle.
func TestTheFinishedJobsSweepPrunesThroughTheStore(t *testing.T) {
	f := newCronFixture(t)

	registered := f.app.cronJobs()
	var found *scheduler.Job
	for i := range registered {
		if registered[i].Name == "clean_jobs" {
			found = &registered[i]
		}
	}
	if found == nil {
		t.Fatal("want clean_jobs registered")
	}

	var got time.Duration
	calls := 0
	f.app.jobRetention = fakeJobRetentionStore{fn: func(_ context.Context, olderThan time.Duration) (int64, error) {
		calls++
		got = olderThan
		return 9, nil
	}}

	f.app.cronCleanJobs(t.Context())

	if calls != 1 {
		t.Fatalf("want DeleteDone called exactly once; got %d", calls)
	}
	if want := jobs.DefaultConfig().Retention; got != want {
		t.Errorf("want the default retention window (%v); got %v", want, got)
	}
	if count := field(t, f.onlyLine(t, "cron_clean_jobs: completed"), "count"); count != float64(9) {
		t.Errorf("want the deleted count on the line; got %v", count)
	}
}

// ---------------------------------------------------------------------------
// Queue depth
// ---------------------------------------------------------------------------

type stubQueues struct {
	stats []health.QueueStats
	err   error
}

func (s stubQueues) QueueStats(context.Context) ([]health.QueueStats, error) {
	return s.stats, s.err
}

// The sweepers log a completion count bounded by their own batch limit, so a
// backlog of fifty and a backlog of fifty thousand produce the same line.
func TestTheQueueDepthJobReportsTheBacklogAndTheOldestDueAge(t *testing.T) {
	f := newCronFixture(t)
	f.app.queues = stubQueues{stats: []health.QueueStats{
		{Name: "webhook_events", Pending: 51234, Processing: 3, Exhausted: 0, OldestDueSeconds: 7200},
	}}

	f.app.cronReportQueueDepth(t.Context())

	line := f.onlyLine(t, "cron_report_queue_depth: backlog")
	for key, want := range map[string]float64{
		"pending":            51234,
		"processing":         3,
		"oldest_due_seconds": 7200,
	} {
		if got := field(t, line, key); got != want {
			t.Errorf("want %s=%v; got %v", key, want, got)
		}
	}
	if got := field(t, line, "queue"); got != "webhook_events" {
		t.Errorf("want the queue named; got %v", got)
	}
}

// Exhaustion was reported one Sentry message per row, so during an incident an
// operator got N alerts and no way to see N. This is N, on one line.
func TestExhaustedItemsAreReportedAsOneAggregate(t *testing.T) {
	f := newCronFixture(t)
	f.app.queues = stubQueues{stats: []health.QueueStats{
		{Name: "failed_refunds", Pending: 0, Exhausted: 47},
	}}

	f.app.cronReportQueueDepth(t.Context())

	line := f.onlyLine(t, "cron_report_queue_depth: items have exhausted")
	if got := field(t, line, "exhausted"); got != float64(47) {
		t.Errorf("want all 47 on one line; got %v", got)
	}
	if got := field(t, line, "level"); got != "ERROR" {
		t.Errorf("money nobody will retry must be reported at ERROR; got %v", got)
	}
}

// A queue with nothing exhausted must not produce the alarm line, or the alarm
// stops meaning anything.
func TestAQueueWithNothingExhaustedRaisesNoAlarm(t *testing.T) {
	f := newCronFixture(t)
	f.app.queues = stubQueues{stats: []health.QueueStats{
		{Name: "webhook_events", Pending: 4, Exhausted: 0},
	}}

	f.app.cronReportQueueDepth(t.Context())

	if got := len(f.lines(t, "cron_report_queue_depth: items have exhausted")); got != 0 {
		t.Errorf("want no exhaustion alarm; got %d lines", got)
	}
}

func TestAFailedQueueDepthQueryIsReported(t *testing.T) {
	f := newCronFixture(t)
	f.app.queues = stubQueues{err: errors.New("statement timeout")}

	f.app.cronReportQueueDepth(t.Context())

	line := f.onlyLine(t, "cron_report_queue_depth: failed")
	if got := field(t, line, "error"); got != "statement timeout" {
		t.Errorf("want the cause on the line; got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Lock stubs
// ---------------------------------------------------------------------------

type freeLocks struct{}

func (freeLocks) TryAdvisory(context.Context, string) (bool, func(), error) {
	return true, func() {}, nil
}

type heldLocks struct{}

func (heldLocks) TryAdvisory(context.Context, string) (bool, func(), error) {
	return false, func() {}, nil
}

type brokenLocks struct{}

func (brokenLocks) TryAdvisory(context.Context, string) (bool, func(), error) {
	return false, func() {}, errors.New("connection refused")
}

// S8: the in-process look-ahead only keeps a paying visitor's hold if it runs
// at the cadence its look-ahead and extension are sized on.
func TestTheKeepPayingHoldsSweepIsScheduledAtItsCadence(t *testing.T) {
	f := newCronFixture(t)
	for _, j := range f.app.cronJobs() {
		if j.Name == "keep_paying_holds" {
			if j.Every != bookings.KeepPayingHoldsEvery {
				t.Errorf("want keep_paying_holds every %v; got %v", bookings.KeepPayingHoldsEvery, j.Every)
			}
			return
		}
	}
	t.Fatal("want keep_paying_holds registered")
}

// The grace a visitor who left the confirm page has to come back (owner
// decision 2026-11-24) only holds if the sweep that ends it runs at its cadence.
func TestTheReleaseLeftHoldsSweepIsScheduledAtItsCadence(t *testing.T) {
	f := newCronFixture(t)
	for _, j := range f.app.cronJobs() {
		if j.Name == "release_left_holds" {
			if j.Every != bookings.ReleaseLeftHoldsEvery {
				t.Errorf("want release_left_holds every %v; got %v", bookings.ReleaseLeftHoldsEvery, j.Every)
			}
			return
		}
	}
	t.Fatal("want release_left_holds registered")
}
