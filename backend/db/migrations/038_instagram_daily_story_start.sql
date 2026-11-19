-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- INSTAGRAM DAILY STORY START — the venue day the daily story starts on.
--
-- Turning the daily story on (daily_enabled false -> true) at or after the
-- day's daily_time must not publish today's story at once: it starts the next
-- venue day. daily_starts_on records that day, set by the save that turns the
-- switch on and kept by every later save while it stays on, so moving
-- daily_time later in the day does not make today's story fire either. The
-- cron job queues a complex's daily story only on or after this day.
--
-- NULL (every row saved before this migration, and a switch that was never
-- turned on) sets no start: the story is due from its time, as before.
--
-- Row-level security is already on the table (037); this only adds a column.
ALTER TABLE instagram_story_settings ADD COLUMN daily_starts_on DATE;

-- +goose Down

SET LOCAL lock_timeout = '3s';

ALTER TABLE instagram_story_settings DROP COLUMN daily_starts_on;
