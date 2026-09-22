-- name: InsertCashSession :one
INSERT INTO cash_sessions (complex_id, opened_by, opening_cash, opening_note)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetOpenCashSessionForUpdate :one
-- Locks the row (if any) so a second open cannot race the check-then-insert:
-- the caller still relies on idx_cash_sessions_one_open for the actual
-- guarantee (this SELECT can only lock a row that already committed), but
-- taking the lock here means a concurrent close sees this transaction's
-- intent before either commits.
SELECT * FROM cash_sessions
WHERE complex_id = $1 AND closed_at IS NULL
FOR UPDATE;

-- name: GetOpenCashSessionByComplex :one
-- The read-only "current session" lookup — GET /current — never locks.
SELECT * FROM cash_sessions
WHERE complex_id = $1 AND closed_at IS NULL;

-- name: GetCashSessionByID :one
SELECT * FROM cash_sessions
WHERE id = $1 AND complex_id = $2;

-- name: GetCashSessionByIDForUpdate :one
-- Every write against a session (a movement, a void, the close itself) takes
-- this lock first, in the same transaction as the write, so a movement
-- cannot land in a session that is being closed concurrently and a session
-- cannot be closed twice.
SELECT * FROM cash_sessions
WHERE id = $1 AND complex_id = $2
FOR UPDATE;

-- name: ListCashSessionsByComplex :many
-- Keyset pagination, same shape as audit_log's listAuditLogsSQL: has_cursor
-- is false on the first page, so the OR's left side short-circuits instead of
-- comparing against a zero-value timestamp.
SELECT * FROM cash_sessions
WHERE complex_id = $1
  AND (NOT sqlc.arg(has_cursor)::bool
       OR (opened_at, id) < (sqlc.arg(cursor_opened_at)::timestamptz, sqlc.arg(cursor_id)::uuid))
ORDER BY opened_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::int;

-- name: CloseCashSession :one
-- closing_note is its own column: it never touches opening_note, so a
-- closing note can never erase what Open recorded.
UPDATE cash_sessions
SET closed_at = NOW(), closed_by = $1, counted_cash = $2, expected_cash = $3, closing_note = $4
WHERE id = $5 AND complex_id = $6
RETURNING *;

-- name: InsertCashMovement :one
-- Serves both an ordinary movement (voids_movement_id NULL) and a void
-- (voids_movement_id set): cash_movements_check_void (003_cashbox.sql)
-- enforces every void invariant that needs to read the original row, so this
-- query is deliberately the same one call for both.
INSERT INTO cash_movements (complex_id, session_id, kind, category, method, amount, note, voids_movement_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetCashMovementByID :one
SELECT * FROM cash_movements
WHERE id = $1 AND complex_id = $2;

-- name: ListCashMovementsBySession :many
-- Oldest first: a session's ledger reads top to bottom like a receipt tape.
-- Not paginated — a shift's movement count is bounded by a business day at
-- the counter, the same reasoning ListBlockedSlots' unpaginated call applies.
SELECT * FROM cash_movements
WHERE session_id = $1 AND complex_id = $2
ORDER BY created_at ASC, id ASC;

-- name: SumCashMovementsBySession :many
-- One row per (method, kind, category) combination actually used in the
-- session. The service derives two different things from this same result:
-- the summary's full breakdown (every row, every method), and the cash
-- reconciliation's own income/expense totals (the rows whose method is
-- 'cash' — "Cash reconciliation counts only cash" is a Go-side filter over
-- this query's rows, not a WHERE clause here, because the summary needs the
-- other methods' rows too).
SELECT method::text, kind, category, COALESCE(SUM(amount), 0)::bigint AS total, COUNT(*)::int AS count
FROM cash_movements
WHERE session_id = $1 AND complex_id = $2
GROUP BY method, kind, category;
