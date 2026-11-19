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
-- wall-clock). spots_left is the CountOpenPlaySpotsTaken rule: capacity minus
-- the confirmed registrations and the holds still inside their window.
SELECT e.id, (e.capacity - (
           SELECT count(*) FROM open_play_registrations r
           WHERE r.event_id = e.id
             AND (r.status = 'confirmed' OR (r.status = 'held' AND r.held_until > NOW()))))::int AS spots_left
FROM open_play_events e
WHERE e.id = sqlc.arg(event_id)::uuid
  AND e.complex_id = sqlc.arg(complex_id)::uuid
  AND e.status = 'active'
  AND e.tournament_id IS NULL
  AND e.date + e.start_time > sqlc.arg(now_date)::date + sqlc.arg(now_time)::time;

-- name: GetWaitlistableCategoryEvent :one
-- The sign-up container of a tournament category a person can wait for: live,
-- of this complex, sign-up still open (no draw), and its day not yet begun, with
-- its pair spots left (the same rule as GetWaitlistableOpenPlayEvent).
SELECT e.id, (e.capacity - (
           SELECT count(*) FROM open_play_registrations r
           WHERE r.event_id = e.id
             AND (r.status = 'confirmed' OR (r.status = 'held' AND r.held_until > NOW()))))::int AS spots_left
FROM tournament_categories c
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

-- name: CountWaitingPlaceEntriesOfEmail :one
-- How many places one address is still waiting for at one complex; the join
-- caps it.
SELECT count(*)::int FROM place_waitlist_entries
WHERE complex_id = sqlc.arg(complex_id)::uuid
  AND email = sqlc.arg(email)::text
  AND notified_at IS NULL;

-- name: ListWaitingSlotGrids :many
-- The distinct grids (complex, day, duration) that slot entries still wait on,
-- from from_date, in key order after the cursor (NULL: from the start). The
-- sweep reads each grid once and walks the keys in pages, wrapping round, so
-- every waiting entry is inspected however many are still taken: an entry is
-- never starved behind older ones.
SELECT DISTINCT w.complex_id, cx.slug AS complex_slug, w.date, w.duration_minutes
FROM place_waitlist_entries w
JOIN complexes cx ON cx.id = w.complex_id
WHERE w.kind = 'slot'
  AND w.notified_at IS NULL
  AND cx.is_active
  AND w.date >= sqlc.arg(from_date)::date
  AND (sqlc.narg(after_date)::date IS NULL
       OR (w.date, w.complex_id, w.duration_minutes)
          > (sqlc.narg(after_date)::date, sqlc.narg(after_complex_id)::uuid, sqlc.narg(after_duration)::int))
ORDER BY w.date, w.complex_id, w.duration_minutes
LIMIT sqlc.arg(batch_limit)::int;

-- name: ClaimFreeSlotEntries :many
-- Marks notified, and returns with what the email needs, every entry still
-- waiting on one grid whose position is among the free ones given, each written
-- "<court id> HH:MM". The notified_at IS NULL guard makes it one-shot.
UPDATE place_waitlist_entries w
SET notified_at = NOW()
FROM courts ct, complexes cx
WHERE ct.id = w.court_id
  AND cx.id = w.complex_id
  AND w.kind = 'slot'
  AND w.notified_at IS NULL
  AND w.complex_id = sqlc.arg(complex_id)::uuid
  AND w.date = sqlc.arg(date)::date
  AND w.duration_minutes = sqlc.arg(duration_minutes)::int
  AND (w.court_id::text || ' ' || to_char(w.start_time, 'HH24:MI')) = ANY(sqlc.arg(positions)::text[])
RETURNING w.id, w.email, w.court_id, ct.name AS court_name, ct.sport::text AS court_sport, w.date, w.start_time,
          cx.name AS complex_name, cx.slug AS complex_slug;

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
