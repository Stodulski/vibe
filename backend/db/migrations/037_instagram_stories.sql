-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- INSTAGRAM STORIES — the stories Vibe publishes on a complex's own connected
-- Instagram account (instagram_connections, 036).
--
-- instagram_story_settings is the owner's choice, one row per complex, written
-- the first time the owner saves Configuración > Instagram. No row reads as the
-- defaults below: connecting Instagram publishes nothing by itself.
--
-- instagram_stories is the queue and the history. A row is queued as pending
-- (the daily story by the cron job, a freed story by a booking cancellation)
-- and the publisher moves it to published, skipped (nothing true to show, or
-- no longer allowed to publish) or failed (Meta refused, or the retries ran
-- out). dedup_key makes each story happen at most once ever per complex:
-- 'daily:<venue day>' or 'freed:<court>:<date>:<HH:MM>'.
--
-- container_id is the Instagram media container while it is being processed:
-- one cron run creates it, a later run checks it and publishes it, so a run
-- never waits on Meta for minutes. locked_until is the lease a run holds on
-- the rows it claimed.
--
-- Both tables are tenant-scoped like every table in 001_init.sql's ACCESS
-- section, with the same tenant_isolation + tenant_bypass pair. vibe_app's
-- grants come from the ALTER DEFAULT PRIVILEGES that 001_init.sql registered.
CREATE TABLE instagram_story_settings (
    complex_id      UUID PRIMARY KEY,
    daily_enabled   BOOLEAN NOT NULL DEFAULT false,
    -- Venue wall-clock time, HH:MM, from 08:00 to 22:00 (quiet hours are
    -- 23:00-08:00).
    daily_time      TEXT NOT NULL DEFAULT '10:00',
    freed_enabled   BOOLEAN NOT NULL DEFAULT false,
    freed_daily_cap INT NOT NULL DEFAULT 5,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT instagram_story_settings_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    CONSTRAINT instagram_story_settings_daily_time_check
        CHECK (daily_time ~ '^[0-2][0-9]:[0-5][0-9]$' AND daily_time BETWEEN '08:00' AND '22:00'),
    CONSTRAINT instagram_story_settings_freed_daily_cap_check
        CHECK (freed_daily_cap BETWEEN 1 AND 10)
);

CREATE TRIGGER set_updated_at BEFORE UPDATE ON instagram_story_settings
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

CREATE TABLE instagram_stories (
    id                   UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id           UUID NOT NULL,
    kind                 TEXT NOT NULL,
    dedup_key            TEXT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'pending',
    -- The public story image Instagram fetches (the landing's /historias route).
    image_url            TEXT NOT NULL,
    -- The venue day the story was queued on: the daily story's day, and the day
    -- a freed story counts against freed_daily_cap.
    queued_day           DATE NOT NULL,
    -- Freed stories only: when the freed slot starts. Under an hour away, the
    -- story is no longer published.
    slot_starts_at       TIMESTAMPTZ,
    scheduled_for        TIMESTAMPTZ NOT NULL,
    attempts             INT NOT NULL DEFAULT 0,
    next_attempt_at      TIMESTAMPTZ NOT NULL,
    locked_until         TIMESTAMPTZ,
    container_id         TEXT,
    container_created_at TIMESTAMPTZ,
    media_id             TEXT,
    published_at         TIMESTAMPTZ,
    reason               TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT instagram_stories_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    CONSTRAINT instagram_stories_dedup_key UNIQUE (complex_id, dedup_key),
    CONSTRAINT instagram_stories_kind_check CHECK (kind IN ('daily', 'freed')),
    CONSTRAINT instagram_stories_status_check
        CHECK (status IN ('pending', 'published', 'skipped', 'failed')),
    CONSTRAINT instagram_stories_freed_slot_check
        CHECK (kind <> 'freed' OR slot_starts_at IS NOT NULL)
);

-- The publisher claims the due pending rows.
CREATE INDEX idx_instagram_stories_due
    ON instagram_stories (next_attempt_at) WHERE status = 'pending';
-- The panel lists a complex's latest stories; the freed cap counts a day's.
CREATE INDEX idx_instagram_stories_complex_created
    ON instagram_stories (complex_id, created_at DESC);

CREATE TRIGGER set_updated_at BEFORE UPDATE ON instagram_stories
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

ALTER TABLE instagram_story_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE instagram_story_settings FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON instagram_story_settings
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON instagram_story_settings
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE instagram_stories ENABLE ROW LEVEL SECURITY;
ALTER TABLE instagram_stories FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON instagram_stories
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON instagram_stories
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- +goose Down

SET LOCAL lock_timeout = '3s';

DROP TABLE instagram_stories;
DROP TABLE instagram_story_settings;
