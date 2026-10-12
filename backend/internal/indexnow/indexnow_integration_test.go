//go:build integration

package indexnow_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/indexnow"
	"github.com/stodulski/vibe-server/internal/jobs"
)

// TestABurstOfSavesIsOneQueuedSubmissionDueAtTheWindowsEnd runs the notifier
// against the real jobs table: the dedup key and the due time are what the
// database stores, not what the fake queue of the unit tests records.
func TestABurstOfSavesIsOneQueuedSubmissionDueAtTheWindowsEnd(t *testing.T) {
	pool := datatest.SetupTestDB(t)
	store := &jobs.Store{DB: data.NewDB(pool)}
	ctx := data.ContextWithTenantBypass(t.Context())

	// A slug only this test uses, so its rows are found and removed without
	// touching any other submission.
	slug := "club-" + uuid.NewString()
	match := "%" + slug + "%"
	t.Cleanup(func() {
		// t.Context is already cancelled when cleanup runs, so cleanup gets its own context.
		cleanup := data.ContextWithTenantBypass(context.Background())
		if _, err := store.DB.Exec(cleanup, `DELETE FROM jobs WHERE type = $1 AND payload::text LIKE $2`, indexnow.TaskSubmit, match); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})

	enq := &jobs.Enqueuer{Store: store, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	svc := indexnow.NewService("production", indexnow.ProductionOrigin, enq,
		indexnow.NewClient(indexnow.ProductionOrigin, nil), slog.New(slog.NewTextHandler(io.Discard, nil)))

	id := uuid.New()
	save := indexnow.Change{
		ComplexID: id,
		Before:    indexnow.Snapshot{Slug: slug, City: "Rosario", Visible: true},
		After:     indexnow.Snapshot{Slug: slug, City: "Rosario", Visible: true},
	}
	rename := indexnow.Change{
		ComplexID: id,
		Before:    save.After,
		After:     indexnow.Snapshot{Slug: slug + "-nuevo", City: "Rosario", Visible: true},
	}

	svc.ComplexChanged(save)
	svc.ComplexChanged(save)
	svc.ComplexChanged(save)
	if got := countJobs(t, ctx, store, match); got != 1 {
		t.Fatalf("three saves in one window queued %d rows, want 1", got)
	}

	svc.ComplexChanged(rename)
	if got := countJobs(t, ctx, store, match); got != 2 {
		t.Fatalf("a rename in the same window queued %d rows in total, want 2", got)
	}

	var runAt time.Time
	if err := store.DB.QueryRow(ctx, `SELECT min(run_at) FROM jobs WHERE type = $1 AND payload::text LIKE $2`, indexnow.TaskSubmit, match).Scan(&runAt); err != nil {
		t.Fatalf("reading run_at: %v", err)
	}
	if !runAt.After(time.Now()) {
		t.Fatalf("run_at %s is not in the future: the submission must wait for its window to close", runAt)
	}
	if runAt.Unix()%600 != 0 {
		t.Fatalf("run_at %s is not a window boundary", runAt)
	}

	// Not due yet, so no worker can claim it before its window has closed.
	claimed, err := store.ClaimType(ctx, "indexnow-test", indexnow.TaskSubmit, 10)
	if err != nil {
		t.Fatalf("ClaimType: %v", err)
	}
	for _, j := range claimed {
		if strings.Contains(string(j.Payload), slug) {
			t.Fatalf("a submission was claimed before its window closed: %s", j.Payload)
		}
	}
}

func countJobs(t *testing.T, ctx context.Context, store *jobs.Store, match string) int {
	t.Helper()
	var n int
	if err := store.DB.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE type = $1 AND payload::text LIKE $2`, indexnow.TaskSubmit, match).Scan(&n); err != nil {
		t.Fatalf("counting jobs: %v", err)
	}
	return n
}
