-- name: GetInstagramStorySettings :one
-- No row means the defaults (everything off), which the store answers itself.
SELECT daily_enabled, daily_time, freed_enabled, freed_daily_cap
FROM instagram_story_settings
WHERE complex_id = sqlc.arg(complex_id)::uuid;

-- name: UpsertInstagramStorySettings :one
INSERT INTO instagram_story_settings (complex_id, daily_enabled, daily_time, freed_enabled, freed_daily_cap)
VALUES (
    sqlc.arg(complex_id)::uuid, sqlc.arg(daily_enabled)::boolean, sqlc.arg(daily_time)::text,
    sqlc.arg(freed_enabled)::boolean, sqlc.arg(freed_daily_cap)::int
)
ON CONFLICT (complex_id) DO UPDATE SET
    daily_enabled   = EXCLUDED.daily_enabled,
    daily_time      = EXCLUDED.daily_time,
    freed_enabled   = EXCLUDED.freed_enabled,
    freed_daily_cap = EXCLUDED.freed_daily_cap
RETURNING daily_enabled, daily_time, freed_enabled, freed_daily_cap;

-- name: LockInstagramStorySettings :one
-- Taken inside the freed-story transaction: two cancellations of the same
-- complex queue one after the other, so the daily cap is never overshot.
SELECT freed_enabled, freed_daily_cap
FROM instagram_story_settings
WHERE complex_id = sqlc.arg(complex_id)::uuid
FOR UPDATE;

-- name: CountInstagramStoriesQueuedOnDay :one
SELECT count(*)::int AS queued
FROM instagram_stories
WHERE complex_id = sqlc.arg(complex_id)::uuid
  AND kind = sqlc.arg(kind)::text
  AND queued_day = sqlc.arg(queued_day)::date;

-- name: InsertInstagramStory :execrows
-- A story already queued under the same dedup key is left alone: it happens
-- once ever.
INSERT INTO instagram_stories (
    complex_id, kind, dedup_key, image_url, queued_day, slot_starts_at,
    scheduled_for, next_attempt_at
) VALUES (
    sqlc.arg(complex_id)::uuid, sqlc.arg(kind)::text, sqlc.arg(dedup_key)::text,
    sqlc.arg(image_url)::text, sqlc.arg(queued_day)::date, sqlc.narg(slot_starts_at)::timestamptz,
    sqlc.arg(scheduled_for)::timestamptz, sqlc.arg(scheduled_for)::timestamptz
)
ON CONFLICT (complex_id, dedup_key) DO NOTHING;

-- name: ListComplexesDueForDailyStory :many
-- The live complexes whose daily story is on, whose time has come (daily_time
-- is HH:MM, so text order is time order), whose Instagram connection is
-- usable, and that have no daily story for this venue day yet.
SELECT c.id AS complex_id, c.slug
FROM instagram_story_settings s
JOIN complexes c ON c.id = s.complex_id AND c.deleted_at IS NULL
JOIN instagram_connections ic ON ic.complex_id = s.complex_id
WHERE s.daily_enabled
  AND s.daily_time <= sqlc.arg(now_hhmm)::text
  AND NOT ic.needs_reconnect
  AND ic.expires_at > sqlc.arg(now)::timestamptz
  AND NOT EXISTS (
      SELECT 1 FROM instagram_stories st
      WHERE st.complex_id = s.complex_id
        AND st.dedup_key = sqlc.arg(dedup_key)::text
  )
ORDER BY c.id
LIMIT sqlc.arg(row_limit)::int;

-- name: ClaimDueInstagramStories :many
-- Leases the due pending stories to this run. A run that dies leaves the lease
-- to expire, and the next run picks the story up again.
UPDATE instagram_stories
SET locked_until = sqlc.arg(locked_until)::timestamptz
WHERE id IN (
    SELECT id FROM instagram_stories
    WHERE status = 'pending'
      AND next_attempt_at <= sqlc.arg(now)::timestamptz
      AND (locked_until IS NULL OR locked_until < sqlc.arg(now)::timestamptz)
    ORDER BY next_attempt_at, id
    LIMIT sqlc.arg(row_limit)::int
    FOR UPDATE SKIP LOCKED
)
RETURNING id, complex_id, kind, image_url, queued_day, slot_starts_at, scheduled_for,
          attempts, container_id, container_created_at;

-- name: SaveInstagramStoryState :exec
-- Writes back what a run did to a story and releases its lease.
UPDATE instagram_stories
SET status               = sqlc.arg(status)::text,
    attempts             = sqlc.arg(attempts)::int,
    next_attempt_at      = sqlc.arg(next_attempt_at)::timestamptz,
    container_id         = sqlc.narg(container_id)::text,
    container_created_at = sqlc.narg(container_created_at)::timestamptz,
    media_id             = sqlc.narg(media_id)::text,
    published_at         = sqlc.narg(published_at)::timestamptz,
    reason               = sqlc.narg(reason)::text,
    locked_until         = NULL
WHERE id = sqlc.arg(id)::uuid;

-- name: ListRecentInstagramStories :many
SELECT kind, status, scheduled_for, published_at, reason
FROM instagram_stories
WHERE complex_id = sqlc.arg(complex_id)::uuid
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: GetInstagramStoryAccount :one
-- The connection the publisher publishes with, token included (sealed).
SELECT instagram_user_id, access_token, expires_at, needs_reconnect
FROM instagram_connections
WHERE complex_id = sqlc.arg(complex_id)::uuid;

-- name: MarkInstagramStoryAccountNeedsReconnect :execrows
-- Meta refused the token while publishing (code 190).
UPDATE instagram_connections
SET needs_reconnect = true
WHERE complex_id = sqlc.arg(complex_id)::uuid
  AND NOT needs_reconnect;
