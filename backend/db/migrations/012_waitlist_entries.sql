-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- WAITLIST ENTRIES — "Avisame si se libera": a player asks to be emailed when
-- a slot on a day that is fully booked is freed again.
--
-- Written by a public, account-less endpoint, so the row carries only what the
-- email needs. Tenant-scoped like every table in 001_init.sql's ACCESS section,
-- with the same tenant_isolation + tenant_bypass pair (see 003_cashbox.sql).
-- New tables inherit vibe_app's grants from the ALTER DEFAULT PRIVILEGES that
-- 001_init.sql registered, so no GRANT statement is needed here.
--
-- notified_at is set once, by the claim that enqueues the email (one-shot).
-- A row whose date has passed is deleted by the daily cleanup job: an address
-- is kept only as long as the day it asked for is still ahead.
CREATE TABLE waitlist_entries (
    id          UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id  UUID NOT NULL,
    -- The calendar day in the product's wall-clock (America/Argentina/Buenos_Aires),
    -- not an instant, so a DATE and never a TIMESTAMPTZ.
    date        DATE NOT NULL,
    email       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    notified_at TIMESTAMPTZ,
    CONSTRAINT waitlist_entries_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    -- One entry per address per day per complex; a second join is a no-op.
    CONSTRAINT waitlist_entries_complex_id_date_email_key UNIQUE (complex_id, date, email),
    -- "Trimmed and lowercased" is a schema invariant, not only a handler courtesy:
    -- the unique key above would otherwise see "Ana@x.com" and "ana@x.com" as two people.
    CONSTRAINT waitlist_entries_email_normalized
        CHECK (email = lower(btrim(email)) AND char_length(email) BETWEEN 3 AND 254)
);

-- The daily cleanup deletes by date across every complex.
CREATE INDEX idx_waitlist_entries_date ON waitlist_entries (date);

ALTER TABLE waitlist_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE waitlist_entries FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON waitlist_entries
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON waitlist_entries
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- +goose Down

SET LOCAL lock_timeout = '3s';

DROP TABLE waitlist_entries;
