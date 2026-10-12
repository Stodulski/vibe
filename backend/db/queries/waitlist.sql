-- name: InsertWaitlistEntry :exec
INSERT INTO waitlist_entries (complex_id, date, email)
VALUES (sqlc.arg(complex_id)::uuid, sqlc.arg(date)::date, sqlc.arg(email)::text)
ON CONFLICT (complex_id, date, email) DO NOTHING;

-- name: ClaimWaitlistEntries :many
-- Marks every entry still waiting for this day as notified and returns them, in
-- one statement, so two frees of the same day cannot both claim the same row.
UPDATE waitlist_entries
SET notified_at = NOW()
WHERE complex_id = sqlc.arg(complex_id)::uuid
  AND date = sqlc.arg(date)::date
  AND notified_at IS NULL
RETURNING id, email;

-- name: DeleteWaitlistEntriesBefore :execrows
DELETE FROM waitlist_entries
WHERE date < sqlc.arg(today)::date;
