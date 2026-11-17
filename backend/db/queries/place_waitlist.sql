-- name: InsertPlaceWaitlistOpenPlay :exec
-- A second join of the same address for the same event is a no-op.
INSERT INTO place_waitlist_entries (complex_id, email, kind, event_id)
VALUES (sqlc.arg(complex_id)::uuid, sqlc.arg(email)::text, 'open_play', sqlc.arg(event_id)::uuid)
ON CONFLICT DO NOTHING;

-- name: InsertPlaceWaitlistSlot :exec
-- A second join of the same address for the same court slot is a no-op.
INSERT INTO place_waitlist_entries (complex_id, email, kind, court_id, date, start_time, duration_minutes)
VALUES (sqlc.arg(complex_id)::uuid, sqlc.arg(email)::text, 'slot', sqlc.arg(court_id)::uuid,
        sqlc.arg(date)::date, sqlc.arg(start_time)::time, sqlc.arg(duration_minutes)::int)
ON CONFLICT DO NOTHING;

-- name: GetWaitlistableOpenPlayEvent :one
-- An open-play event a person can wait for: live, of this complex, not a
-- tournament's sign-up container, and not started yet (now_* is the product's
-- wall-clock).
SELECT e.id FROM open_play_events e
WHERE e.id = sqlc.arg(event_id)::uuid
  AND e.complex_id = sqlc.arg(complex_id)::uuid
  AND e.status = 'active'
  AND e.tournament_id IS NULL
  AND e.date + e.start_time > sqlc.arg(now_date)::date + sqlc.arg(now_time)::time;

-- name: GetWaitlistableCategoryEvent :one
-- The sign-up container of a tournament category a person can wait for: live,
-- of this complex, sign-up still open (no draw), and its day not yet begun.
SELECT e.id FROM tournament_categories c
JOIN open_play_events e ON e.id = c.event_id
WHERE c.id = sqlc.arg(category_id)::uuid
  AND c.complex_id = sqlc.arg(complex_id)::uuid
  AND e.status = 'active'
  AND c.draw_generated_at IS NULL
  AND COALESCE(c.starts_on + COALESCE(c.starts_at, '00:00'::time), e.date + e.start_time)
      > sqlc.arg(now_date)::date + sqlc.arg(now_time)::time;

-- name: GetWaitlistableCourt :one
-- An active court of this complex.
SELECT id FROM courts
WHERE id = sqlc.arg(court_id)::uuid
  AND complex_id = sqlc.arg(complex_id)::uuid
  AND is_active;

-- name: ClaimFreedOpenPlayWaitlist :many
-- Claims, in one statement, every entry still waiting for an event that has a
-- free spot now and has not started, and returns what the email needs. A spot
-- is free when capacity exceeds the confirmed registrations plus the holds still
-- inside their window (the CountOpenPlaySpotsTaken rule). A tournament's
-- category is only waited for while its sign-up is open (no draw yet). SKIP
-- LOCKED and the notified_at IS NULL guard make the claim one-shot even with two
-- sweeps running at once.
WITH freed AS (
    SELECT w.id, w.event_id
    FROM place_waitlist_entries w
    JOIN open_play_events e ON e.id = w.event_id
    JOIN complexes cx ON cx.id = e.complex_id
    LEFT JOIN tournament_categories tc ON tc.event_id = e.id
    WHERE w.kind = 'open_play'
      AND w.notified_at IS NULL
      AND cx.is_active
      AND e.status = 'active'
      AND (tc.id IS NULL OR tc.draw_generated_at IS NULL)
      AND COALESCE(tc.starts_on + COALESCE(tc.starts_at, '00:00'::time), e.date + e.start_time)
          > sqlc.arg(now_date)::date + sqlc.arg(now_time)::time
      AND e.capacity > (
          SELECT count(*) FROM open_play_registrations r
          WHERE r.event_id = e.id
            AND (r.status = 'confirmed' OR (r.status = 'held' AND r.held_until > NOW())))
    ORDER BY w.created_at
    LIMIT sqlc.arg(batch_limit)::int
    FOR UPDATE OF w SKIP LOCKED
)
UPDATE place_waitlist_entries w
SET notified_at = NOW()
FROM freed f
JOIN open_play_events e ON e.id = f.event_id
JOIN complexes cx ON cx.id = e.complex_id
LEFT JOIN tournament_categories tc ON tc.event_id = e.id
WHERE w.id = f.id
  AND w.notified_at IS NULL
RETURNING w.id, w.email, e.id AS event_id, e.name AS event_name, e.date AS event_date,
          e.start_time AS event_start_time, e.tournament_id, COALESCE(tc.name, '')::text AS category_name,
          cx.name AS complex_name, cx.slug AS complex_slug;

-- name: ListWaitingSlotEntries :many
-- The slot entries still waiting whose day may still be ahead: from yesterday,
-- because a venue trading past midnight files tonight's 00:30 under yesterday's
-- grid. Whether each slot is free and still in the future is the courts grid's
-- call, not this query's.
SELECT w.id, w.email, w.court_id, w.date, w.start_time, w.duration_minutes,
       ct.name AS court_name, cx.name AS complex_name, cx.slug AS complex_slug
FROM place_waitlist_entries w
JOIN courts ct ON ct.id = w.court_id
JOIN complexes cx ON cx.id = w.complex_id
WHERE w.kind = 'slot'
  AND w.notified_at IS NULL
  AND cx.is_active
  AND w.date >= sqlc.arg(from_date)::date
ORDER BY w.created_at
LIMIT sqlc.arg(batch_limit)::int;

-- name: ClaimPlaceWaitlistEntries :many
-- Marks the given entries notified and returns the ones this call claimed; an
-- entry already claimed is left out, so it is never emailed twice.
UPDATE place_waitlist_entries
SET notified_at = NOW()
WHERE id = ANY(sqlc.arg(ids)::uuid[])
  AND notified_at IS NULL
RETURNING id;

-- name: DeletePastPlaceWaitlistEntries :execrows
-- The daily cleanup: slot entries whose day is gone (kept one extra day for the
-- hours past midnight), and event entries whose event was cancelled or whose day
-- has passed.
DELETE FROM place_waitlist_entries w
WHERE (w.kind = 'slot' AND w.date < sqlc.arg(slot_before)::date)
   OR (w.kind = 'open_play' AND EXISTS (
        SELECT 1 FROM open_play_events e
        LEFT JOIN tournament_categories tc ON tc.event_id = e.id
        WHERE e.id = w.event_id
          AND (e.status <> 'active' OR COALESCE(tc.starts_on, e.date) < sqlc.arg(today)::date)));
