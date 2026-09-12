-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ==================== ONE DURABLE WORK QUEUE ====================
--
-- There were three, and they said the same thing three ways (JOB-02):
--
--   * internal/notifier, a Redis list with five keys and five Lua scripts of
--     its own — a pending list, a processing list, a claims hash, a delayed
--     sorted set and a dead-letter list. Nothing about it was in Postgres, so
--     a Redis that lost its data lost every queued email.
--   * webhook_events and failed_refunds, two tables in this schema with the
--     same status vocabulary, the same retry_count/max_retries/next_retry_at
--     columns and the same backoff tables written out twice.
--
-- They shared a vocabulary because they shared a guarantee. They did not share
-- a line of code, so every fix to one of them was a fix to one of them: the
-- jitter this table's writers apply (OUT-02), the dedup key that stops a
-- redelivered webhook sending a second confirmation (JOB-04), the claim that
-- does not need a lease table.
--
-- THE CLAIM. A worker takes rows with
--
--     UPDATE jobs SET status = 'processing', ...
--     WHERE id IN (SELECT id FROM jobs WHERE status = 'pending'
--                    AND run_at <= NOW() ORDER BY run_at
--                  FOR UPDATE SKIP LOCKED LIMIT $n)
--     RETURNING ...
--
-- SKIP LOCKED is the whole design. Every instance runs that statement on the
-- same tick and they take disjoint rows without one of them ever waiting on
-- another: a row somebody else has locked is passed over, not queued behind.
-- The two tables it replaces used a conditional UPDATE per row instead, which
-- is correct but serialises every instance through the same row on its way to
-- finding out it lost, and needed a job_locks lease on top to stop the
-- convoy. job_locks stays — it is a cron mutex, not a queue.
--
-- NO TENANT COLUMN AND NO ROW-LEVEL SECURITY. This is the same posture as
-- job_locks and webhook_events (001_init.sql's "TABLES THAT GET NOTHING"): a
-- job is the platform's own deferred work, claimed by a background worker that
-- has no request and therefore no tenant. Anything a job needs to touch a
-- tenant's rows with is in its payload, and the store it reaches through is
-- under the policies as usual.
--
-- WHY IT IS SAFE TO BUILD THIS IN ONE STATEMENT. There is no production data:
-- this is pre-launch, so there is no backfill, no expand-and-contract and no
-- compatibility window for a queue in Redis that nothing will read again.

CREATE TABLE jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- The handler registry's key: 'email:booking_confirmation' and the rest of
    -- internal/notifications' task types. It is stored, so renaming one
    -- strands whatever is already queued under the old name.
    type         TEXT NOT NULL,
    -- The handler's whole argument. jsonb rather than json so a payload can be
    -- queried by an operator reading the dead letter without parsing it first.
    payload      JSONB NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    -- The earliest instant a worker may claim this row: now for an ordinary
    -- enqueue, and the backoff for every retry.
    run_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- attempts counts claims, not failures — Claim increments it — so a
    -- handler that takes the process down with it has still spent one. That is
    -- what bounds a payload that kills whichever instance reads it.
    attempts     INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    last_error   TEXT,
    -- The claim itself. locked_at is what the stale sweep measures the lease
    -- against; locked_by names the worker for the line that says which one
    -- went away.
    locked_at    TIMESTAMPTZ,
    locked_by    TEXT,
    -- The idempotency key, nullable because most work does not need one. See
    -- the unique index below for why it is not a UNIQUE constraint.
    dedup_key    TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT jobs_type_check CHECK (type <> ''),
    CONSTRAINT jobs_status_check
        CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    CONSTRAINT jobs_attempts_check CHECK (attempts >= 0),
    CONSTRAINT jobs_max_attempts_check CHECK (max_attempts > 0),
    -- A dedup key of '' is a caller that meant NULL and got the zero value.
    -- Every such caller would collide with every other, which is the worst
    -- possible reading of "no key".
    CONSTRAINT jobs_dedup_key_check CHECK (dedup_key IS NULL OR dedup_key <> '')
);

CREATE TRIGGER set_updated_at BEFORE UPDATE ON jobs
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- THE CLAIM INDEX. Exactly the predicate and the order of the inner SELECT
-- above, and partial on the one status a claim can match — 'done' is where
-- almost every row ends up, so keeping it out is what stops the index growing
-- with the table the way the two it replaces did.
CREATE INDEX idx_jobs_claim ON jobs (run_at) WHERE status = 'pending';

-- The stale sweep: rows claimed longer ago than the lease.
CREATE INDEX idx_jobs_stale ON jobs (locked_at) WHERE status = 'processing';

-- DEDUP. A partial unique index rather than a UNIQUE constraint, because the
-- column is nullable and most rows have no key: Postgres treats NULLs as
-- distinct, so a plain unique index would work, but it would also index every
-- keyless row for nothing. ON CONFLICT (dedup_key) DO NOTHING infers this
-- index, which is what makes a second Enqueue under the same key a no-op
-- rather than a second email.
CREATE UNIQUE INDEX idx_jobs_dedup_key ON jobs (dedup_key) WHERE dedup_key IS NOT NULL;

-- Retention deletes by the instant a row was closed out, which for a done row
-- is its last update.
CREATE INDEX idx_jobs_done ON jobs (updated_at) WHERE status = 'done';

COMMENT ON TABLE jobs IS
    'The durable work queue. Claimed with SELECT ... FOR UPDATE SKIP LOCKED; see internal/jobs.';
COMMENT ON COLUMN jobs.dedup_key IS
    'sha256 of the job type and what identifies the work (recipient, booking, event). A second enqueue under an existing key does nothing.';
COMMENT ON COLUMN jobs.attempts IS
    'Claims, not failures: incremented by the claim itself, so a handler that kills the process still spends one.';

-- +goose Down

DROP TABLE jobs;
