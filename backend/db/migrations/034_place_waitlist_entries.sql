-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- PLACE WAITLIST ENTRIES — "Avisarme": a player asks to be emailed when one
-- place frees up on something that is full: a spot in an open-play event, a
-- pair's spot in a tournament category, or one taken court slot.
--
-- It sits next to waitlist_entries (012), which waits for a whole day, and does
-- not replace it. Written by a public, account-less endpoint, so the row carries
-- only what the email needs. Tenant-scoped like every table in 001_init.sql's
-- ACCESS section, with the same tenant_isolation + tenant_bypass pair. New tables
-- inherit vibe_app's grants from the ALTER DEFAULT PRIVILEGES that 001_init.sql
-- registered, so no GRANT statement is needed here.
--
-- kind 'open_play' waits for a spot of one event. A tournament category is
-- stored by its sign-up container, tournament_categories.event_id, so it is an
-- 'open_play' row too. kind 'slot' waits for one court position on one day.
--
-- notified_at is set once, by the sweep that claims the row and enqueues the
-- email (one-shot): every waiting address is told at once and the first to pay
-- wins. A row whose place has passed is deleted by the daily cleanup.
CREATE TABLE place_waitlist_entries (
    id               UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id       UUID NOT NULL,
    email            TEXT NOT NULL,
    kind             TEXT NOT NULL,
    event_id         UUID,
    court_id         UUID,
    -- The storefront grid's day and time of day, in the product's wall-clock
    -- (America/Argentina/Buenos_Aires), never an instant.
    date             DATE,
    start_time       TIME,
    duration_minutes INTEGER,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    notified_at      TIMESTAMPTZ,
    CONSTRAINT place_waitlist_entries_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    -- Composite keys, as 030 does: an entry cannot name another tenant's event
    -- or court.
    CONSTRAINT place_waitlist_entries_event_fkey
        FOREIGN KEY (event_id, complex_id) REFERENCES open_play_events (id, complex_id) ON DELETE CASCADE,
    CONSTRAINT place_waitlist_entries_court_fkey
        FOREIGN KEY (court_id, complex_id) REFERENCES courts (id, complex_id) ON DELETE CASCADE,
    CONSTRAINT place_waitlist_entries_kind_valid CHECK (kind IN ('open_play', 'slot')),
    -- Each kind names exactly its own place and nothing of the other's.
    CONSTRAINT place_waitlist_entries_target CHECK (
        (kind = 'open_play' AND event_id IS NOT NULL
            AND court_id IS NULL AND date IS NULL AND start_time IS NULL AND duration_minutes IS NULL)
        OR
        (kind = 'slot' AND event_id IS NULL
            AND court_id IS NOT NULL AND date IS NOT NULL AND start_time IS NOT NULL
            AND duration_minutes IS NOT NULL AND duration_minutes > 0)
    ),
    -- Trimmed and lowercased is a schema invariant, as in waitlist_entries: the
    -- unique indexes below would otherwise see "Ana@x.com" and "ana@x.com" as two people.
    CONSTRAINT place_waitlist_entries_email_normalized
        CHECK (email = lower(btrim(email)) AND char_length(email) BETWEEN 3 AND 254)
);

-- One waiting entry per address per place; a second join while still waiting is
-- a no-op. Notified rows are left out, so an address that was told and missed
-- the place can wait for it again: the new row gets its own notice.
CREATE UNIQUE INDEX idx_place_waitlist_entries_open_play_email
    ON place_waitlist_entries (event_id, email) WHERE kind = 'open_play' AND notified_at IS NULL;
CREATE UNIQUE INDEX idx_place_waitlist_entries_slot_email
    ON place_waitlist_entries (court_id, date, start_time, duration_minutes, email)
    WHERE kind = 'slot' AND notified_at IS NULL;

-- The per-address cap on the join counts the waiting entries of one address.
CREATE INDEX idx_place_waitlist_entries_waiting_email
    ON place_waitlist_entries (complex_id, email) WHERE notified_at IS NULL;

-- The every-minute sweep reads only the rows still waiting: the event entries,
-- and the distinct grids (complex, day, duration) the slot entries wait on.
CREATE INDEX idx_place_waitlist_entries_unnotified_events
    ON place_waitlist_entries (created_at) WHERE kind = 'open_play' AND notified_at IS NULL;
CREATE INDEX idx_place_waitlist_entries_unnotified_slots
    ON place_waitlist_entries (date, complex_id, duration_minutes) WHERE kind = 'slot' AND notified_at IS NULL;

ALTER TABLE place_waitlist_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE place_waitlist_entries FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON place_waitlist_entries
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON place_waitlist_entries
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- +goose Down

SET LOCAL lock_timeout = '3s';

DROP TABLE place_waitlist_entries;
