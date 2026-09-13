package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// jobColumns is the projection every read below shares, so a column added to
// the table is added to scanJob and to one string rather than to four.
const jobColumns = `id, type, payload, status, run_at, attempts, max_attempts,
	COALESCE(last_error, ''), locked_at, COALESCE(locked_by, ''),
	COALESCE(dedup_key, ''), created_at, updated_at`

// Store is the jobs table.
//
// Backoff is the retry ladder Fail schedules on; an empty one means
// DefaultBackoff. It is a field rather than a constant so a test can make a
// retry observable inside a test's lifetime, and so a caller that knows its
// provider — a payment provider answering in seconds, a mail provider that
// rate-limits by the hour — can say so.
type Store struct {
	DB      *data.DB
	Backoff []time.Duration
}

// Enqueue records a job and reports whether it was recorded.
//
// A false return with a nil error is the dedup case and is not a failure: a
// job carrying this key is already in the table, so this call is the second
// delivery of work that is already queued, running, done or dead. The caller's
// move is to carry on.
//
// runAt in the past means "as soon as a worker picks it up", which is the
// ordinary case; maxAttempts and dedupKey may both be zero-valued, and the
// column defaults then apply.
func (s *Store) Enqueue(ctx context.Context, jobType string, payload any, runAt time.Time, maxAttempts int, dedupKey string) (uuid.UUID, bool, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("enqueue %s: marshal payload: %w", jobType, err)
	}

	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var id uuid.UUID
	// The payload column is jsonb and the parameter is a string, not a
	// []byte: the API pool runs in QueryExecModeExec, where pgx infers the
	// parameter type from the Go value and sends a []byte as bytea, which
	// Postgres refuses with "invalid input syntax for type json". Learned on
	// webhook_events; see the same note in its Insert before this table
	// absorbed it.
	err = s.DB.QueryRow(ctx, `
		INSERT INTO jobs (type, payload, run_at, max_attempts, dedup_key)
		VALUES ($1, $2, COALESCE($3, NOW()), COALESCE($4, 5), $5)
		-- The WHERE repeats idx_jobs_dedup_key's own predicate, which is how
		-- Postgres infers a PARTIAL unique index: without it the statement is
		-- refused outright with "no unique or exclusion constraint matching
		-- the ON CONFLICT specification", because a partial index only
		-- constrains the rows it covers.
		ON CONFLICT (dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING
		RETURNING id`,
		jobType, string(raw), nullTime(runAt), nullInt(maxAttempts), nullText(dedupKey),
	).Scan(&id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return uuid.Nil, false, nil
	case err != nil:
		return uuid.Nil, false, fmt.Errorf("enqueue %s: %w", jobType, err)
	}
	return id, true, nil
}

// Claim takes up to limit due jobs for worker and returns them.
//
// This is the statement the whole package exists for. The inner SELECT takes a
// row lock with FOR UPDATE SKIP LOCKED, so a row another instance is already
// claiming is passed over rather than waited on: every instance runs this on
// the same tick and they take disjoint sets, with no advisory lock, no lease
// table and no contention. The outer UPDATE and its RETURNING are the same
// statement, so there is no instant at which a row is neither pending nor
// claimed.
//
// The attempt is spent here rather than on failure. A handler that takes the
// process down with it has still consumed one, which is what stops a poison
// payload being read forever by each new instance in turn.
func (s *Store) Claim(ctx context.Context, worker string, limit int) ([]*Job, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := s.DB.Query(ctx, `
		UPDATE jobs SET
			status = 'processing',
			locked_at = NOW(),
			locked_by = $1,
			attempts = attempts + 1
		WHERE id IN (
			SELECT id FROM jobs
			WHERE status = 'pending' AND run_at <= NOW()
			ORDER BY run_at
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		)
		RETURNING `+jobColumns, worker, limit)
	if err != nil {
		return nil, fmt.Errorf("claim jobs: %w", err)
	}
	defer rows.Close()
	return scanJobs(rows)
}

// ClaimType is Claim narrowed to one job type.
//
// Claim's own contract is deliberately type-blind — every production worker
// drains the whole table, and an unregistered type simply drops back out
// through Pool's Release path (pool.go). This exists for a caller that must
// not depend on that: the integration suite shares one database across every
// package and test that touches this table, so a fixed-limit Claim can be
// filled entirely by another test's backlog before it ever reaches the rows
// this caller enqueued. Nothing in production calls this; Claim keeps its
// contract unchanged.
func (s *Store) ClaimType(ctx context.Context, worker, jobType string, limit int) ([]*Job, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := s.DB.Query(ctx, `
		UPDATE jobs SET
			status = 'processing',
			locked_at = NOW(),
			locked_by = $1,
			attempts = attempts + 1
		WHERE id IN (
			SELECT id FROM jobs
			WHERE status = 'pending' AND run_at <= NOW() AND type = $3
			ORDER BY run_at
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		)
		RETURNING `+jobColumns, worker, limit, jobType)
	if err != nil {
		return nil, fmt.Errorf("claim jobs of type %s: %w", jobType, err)
	}
	defer rows.Close()
	return scanJobs(rows)
}

// Complete acknowledges a job. It is detached from the caller's cancellation
// for the reason every closing write in this repository is: the moment it
// matters most is the moment the caller's budget is spent, and a skipped
// acknowledgement leaves the row in 'processing' to be reclaimed and run a
// second time.
func (s *Store) Complete(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := data.DetachedQueryContext(ctx)
	defer cancel()

	_, err := s.DB.Exec(ctx, `
		UPDATE jobs
		SET status = 'done', locked_at = NULL, locked_by = NULL, last_error = NULL
		WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	return nil
}

// Fail spends an attempt and reports whether that was the last one.
//
// A true return means the job is in 'failed' — the dead letter. Nothing
// retries it and no retention sweep deletes it: it is the record of work this
// system accepted and could not do, which for a refund is a person's money.
//
// The budget is read back under FOR UPDATE rather than taken from the caller's
// copy of the row, because two workers that reclaimed the same abandoned
// attempt would otherwise compute the same next count from the same stale
// struct and spend one attempt between them.
func (s *Store) Fail(ctx context.Context, id uuid.UUID, cause string) (bool, error) {
	ctx, cancel := data.TxContext(context.WithoutCancel(ctx))
	defer cancel()

	var dead bool
	err := s.DB.WithTx(ctx, func(tx pgx.Tx, _ *db.Queries) error {
		var attempts, maxAttempts int
		err := tx.QueryRow(ctx,
			`SELECT attempts, max_attempts FROM jobs WHERE id = $1 FOR UPDATE`, id,
		).Scan(&attempts, &maxAttempts)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("lock job: %w", err)
		}

		dead = attempts >= maxAttempts
		status := StatusPending
		if dead {
			status = StatusFailed
		}

		if _, err = tx.Exec(ctx, `
		UPDATE jobs
		SET status = $2, last_error = $3, run_at = $4, locked_at = NULL, locked_by = NULL
		WHERE id = $1`,
			id, status, cause, time.Now().Add(Backoff(s.Backoff, attempts)),
		); err != nil {
			return fmt.Errorf("requeue job: %w", err)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return dead, nil
}

// Release returns a job to pending without spending its attempt, due at runAt.
//
// It is what an attempt that never happened settles as — an open circuit
// breaker, a refused lease. The budget bounds how many times a provider that
// is answering is asked; an attempt that got no answer is not evidence about
// this job, and charging it is how a single afternoon of provider downtime
// dead-lettered every queued row at once.
func (s *Store) Release(ctx context.Context, id uuid.UUID, runAt time.Time, cause string) error {
	ctx, cancel := data.DetachedQueryContext(ctx)
	defer cancel()

	_, err := s.DB.Exec(ctx, `
		UPDATE jobs
		SET status = 'pending', run_at = $2, last_error = $3,
		    attempts = GREATEST(attempts - 1, 0), locked_at = NULL, locked_by = NULL
		WHERE id = $1`, id, runAt, cause)
	if err != nil {
		return fmt.Errorf("release job: %w", err)
	}
	return nil
}

// Kill dead-letters a job now, whatever its remaining budget.
func (s *Store) Kill(ctx context.Context, id uuid.UUID, cause string) error {
	ctx, cancel := data.DetachedQueryContext(ctx)
	defer cancel()

	_, err := s.DB.Exec(ctx, `
		UPDATE jobs
		SET status = 'failed', last_error = $2, locked_at = NULL, locked_by = NULL
		WHERE id = $1`, id, cause)
	if err != nil {
		return fmt.Errorf("dead-letter job: %w", err)
	}
	return nil
}

// ReclaimStale returns claims older than lease to pending and reports how many
// it moved.
//
// This is what makes a worker that died mid-handle recoverable, and it runs on
// every instance, so recovery never waits for the dead process to come back.
// The attempt it spent stays spent.
func (s *Store) ReclaimStale(ctx context.Context, lease time.Duration) (int64, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	tag, err := s.DB.Exec(ctx, `
		UPDATE jobs
		SET status = 'pending', locked_at = NULL, locked_by = NULL
		WHERE status = 'processing' AND locked_at < NOW() - $1::interval`,
		lease.String())
	if err != nil {
		return 0, fmt.Errorf("reclaim stale jobs: %w", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteDone removes jobs finished longer ago than olderThan.
//
// Only 'done' rows are eligible. Pending, in-flight and failed rows all
// describe work that has not happened, and no timer may delete those — a
// 'failed' row in particular is the only record that something was accepted
// and never done.
//
// The retention window is also how long a DedupKey keeps protecting: the
// unique index is on live rows, so deleting a done job makes its key
// enqueueable again. It has to outlast any redelivery window a provider has.
func (s *Store) DeleteDone(ctx context.Context, olderThan time.Duration) (int64, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	tag, err := s.DB.Exec(ctx,
		`DELETE FROM jobs WHERE status = 'done' AND updated_at < NOW() - $1::interval`,
		olderThan.String())
	if err != nil {
		return 0, fmt.Errorf("delete done jobs: %w", err)
	}
	return tag.RowsAffected(), nil
}

// Get returns one job by id, for tests and for an operator reading the dead
// letter.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (*Job, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := s.DB.Query(ctx, `SELECT `+jobColumns+` FROM jobs WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	defer rows.Close()

	found, err := scanJobs(rows)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, data.ErrRecordNotFound
	}
	return found[0], nil
}

// GetByDedupKey returns the job holding key.
//
// Enqueue reports a deduplicated call as (uuid.Nil, false, nil): it says the
// work is already queued but not which row is doing it, because none of its
// callers cared — a second confirmation email is simply not sent. An export
// does care. The caller is an owner who clicked the button twice and is owed
// the id of the export that is already running, not a second one.
//
// Answers data.ErrRecordNotFound when no live row holds the key, which is the
// ordinary state once the retention sweep has deleted the done job that did.
func (s *Store) GetByDedupKey(ctx context.Context, key string) (*Job, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := s.DB.Query(ctx, `SELECT `+jobColumns+` FROM jobs WHERE dedup_key = $1`, key)
	if err != nil {
		return nil, fmt.Errorf("get job by dedup key: %w", err)
	}
	defer rows.Close()

	found, err := scanJobs(rows)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, data.ErrRecordNotFound
	}
	return found[0], nil
}

// ReleaseDedupKey frees a dead job's key so the same work can be queued again,
// and reports whether it freed one.
//
// The status predicate is the whole of it. A dedup key protects for as long as
// its row lives, which is right for a notification — the email either went or
// is still going — but wrong for work an owner can legitimately ask for again
// after it failed. Without `status = 'failed'` this would also free the key of
// a job that is pending or in flight, and the retry would enqueue a second
// copy of work already running.
//
// A 'done' job keeps its key too: re-running it writes the same object from
// the same month, so the second request is answered with the first export
// rather than by rebuilding it.
func (s *Store) ReleaseDedupKey(ctx context.Context, id uuid.UUID) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	tag, err := s.DB.Exec(ctx,
		`UPDATE jobs SET dedup_key = NULL WHERE id = $1 AND status = 'failed'`, id)
	if err != nil {
		return false, fmt.Errorf("release dedup key: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// GetExport returns one job by id, but only if it is a jobType job whose
// payload names complexID.
//
// This table deliberately carries no complex_id column and no row-level
// security (db/migrations/004_jobs.sql): it is a queue every domain shares,
// and a tenant policy on it would have to be satisfied by every background
// sweep that legitimately spans tenants. So the tenant lives in the payload,
// and the predicate here is the readable half of the isolation — the other
// half being RequireComplexOwner, which has already proved the caller owns
// the complex named in the path.
//
// jobType is required alongside id: this table holds every job type this
// process enqueues, not only exports, and a payload shape is a convention,
// not a schema — nothing stops some other job type from also carrying a
// complex_id field whose value collides with the id the caller asked for.
// Without the type filter, that job would be handed back as if it were the
// export the caller polled for. jobs stays generic and does not know what
// "export" means, so the caller — reporting, via TaskExportPayments — passes
// the type it wants rather than this package hardcoding one.
//
// A mismatch answers data.ErrRecordNotFound rather than a distinguishable
// refusal, matching internal/clients' convention: a 403 would confirm the
// export exists and turn this route into a probe for another tenant's ids.
//
// complexID is text because the payload is jsonb and `payload->>'complex_id'`
// is text; binding a uuid here would compare text to uuid and fail.
func (s *Store) GetExport(ctx context.Context, id uuid.UUID, complexID, jobType string) (*Job, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := s.DB.Query(ctx,
		`SELECT `+jobColumns+` FROM jobs WHERE id = $1 AND payload->>'complex_id' = $2 AND type = $3`,
		id, complexID, jobType)
	if err != nil {
		return nil, fmt.Errorf("get export job: %w", err)
	}
	defer rows.Close()

	found, err := scanJobs(rows)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, data.ErrRecordNotFound
	}
	return found[0], nil
}

func scanJobs(rows pgx.Rows) ([]*Job, error) {
	var out []*Job
	for rows.Next() {
		var j Job
		err := rows.Scan(&j.ID, &j.Type, &j.Payload, &j.Status, &j.RunAt,
			&j.Attempts, &j.MaxAttempts, &j.LastError, &j.LockedAt, &j.LockedBy,
			&j.DedupKey, &j.CreatedAt, &j.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		out = append(out, &j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan jobs: %w", err)
	}
	return out, nil
}

// The three helpers below turn a Go zero value into a SQL NULL, so the
// column's own default applies rather than a zero the caller did not mean.
func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func nullInt(n int) *int {
	if n <= 0 {
		return nil
	}
	return &n
}

func nullText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
