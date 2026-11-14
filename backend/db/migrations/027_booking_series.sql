-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- BOOKING SERIES — fixed weekly bookings ("turnos fijos")
-- ============================================================================
-- Owner decision 2026-11-13: the owner books a court every week at the same
-- hour for the same person. The series is the standing order; the bookings it
-- produces are ordinary rows in `bookings`, materialized ahead (eight weeks, and
-- a daily job extends them), so the grid, the cash desk, the reports and the
-- storefront's availability work on them unchanged.
--
-- weekday is 0 = Sunday .. 6 = Saturday, the numbering Go's time.Weekday and
-- PostgreSQL's EXTRACT(DOW ...) share. It is stored beside starts_on, and a CHECK
-- keeps the two agreeing, so the pattern cannot drift from its first date.
--
-- price is the owner's manual price for every occurrence; NULL means each one is
-- priced like any other owner booking at the moment it is generated.
--
-- ends_on is inclusive and NULL while the series has no end. status 'ended' is
-- set once an end-from-date leaves nothing ahead to generate or keep.
--
-- The client is resolved (found or created by phone) when the series is
-- created, so the series names a clients row, never a snapshot.
--
-- Tenant-scoped like every table in 001_init.sql's ACCESS section, with the same
-- tenant_isolation + tenant_bypass pair (see 003_cashbox.sql). New tables
-- inherit vibe_app's grants from the ALTER DEFAULT PRIVILEGES that 001_init.sql
-- registered, so no GRANT statement is needed here.
CREATE TABLE booking_series (
    id               UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id       UUID NOT NULL,
    court_id         UUID NOT NULL,
    client_id        UUID NOT NULL,
    weekday          SMALLINT NOT NULL,
    start_time       TIME NOT NULL,
    duration_minutes INTEGER NOT NULL,
    price            INTEGER,
    notes            TEXT,
    starts_on        DATE NOT NULL,
    ends_on          DATE,
    status           TEXT NOT NULL DEFAULT 'active',
    created_by       UUID NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Referenced by the composite keys below and by bookings.
    CONSTRAINT booking_series_id_complex_id_key UNIQUE (id, complex_id),
    CONSTRAINT booking_series_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    -- Same cross-tenant guard bookings carries: a series cannot name another
    -- tenant's court or client.
    CONSTRAINT booking_series_court_in_same_complex
        FOREIGN KEY (court_id, complex_id) REFERENCES courts (id, complex_id),
    CONSTRAINT booking_series_client_in_same_complex
        FOREIGN KEY (client_id, complex_id) REFERENCES clients (id, complex_id),
    CONSTRAINT booking_series_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users (id),
    CONSTRAINT booking_series_weekday_check CHECK (weekday BETWEEN 0 AND 6),
    CONSTRAINT booking_series_weekday_matches_start
        CHECK (weekday = EXTRACT(DOW FROM starts_on)),
    CONSTRAINT booking_series_duration_minutes_permitted CHECK (duration_minutes IN (60, 90, 120)),
    CONSTRAINT booking_series_price_check CHECK (price IS NULL OR price >= 0),
    -- One day before starts_on is the empty series: ended from its very first
    -- date, so none of its occurrences stand.
    CONSTRAINT booking_series_ends_after_start CHECK (ends_on IS NULL OR ends_on >= starts_on - 1),
    CONSTRAINT booking_series_status_valid CHECK (status IN ('active', 'ended'))
);

CREATE TRIGGER set_updated_at BEFORE UPDATE ON booking_series
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- The daily extension job reads every active series across tenants.
CREATE INDEX idx_booking_series_active ON booking_series (complex_id) WHERE status = 'active';

ALTER TABLE booking_series ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_series FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON booking_series
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON booking_series
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- ============================================================================
-- BOOKING SERIES DATES — every date a series has ever been generated for
-- ============================================================================
-- One row per (series, local date), written once and never deleted: the primary
-- key IS the rule "a date is generated at most once ever". A booking that was
-- cancelled afterwards keeps its row, so the daily job does not book it again;
-- a date that could not be booked (a clash, a blocked hour, a closed day) is
-- recorded as 'skipped' with its reason, so the job does not retry it forever.
--
-- The 'booked' row is inserted in the same transaction as the booking itself
-- (internal/bookings/store insertSafe), so a crash can leave neither half.
CREATE TABLE booking_series_dates (
    series_id  UUID NOT NULL,
    complex_id UUID NOT NULL,
    date       DATE NOT NULL,
    outcome    TEXT NOT NULL,
    reason     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (series_id, date),
    CONSTRAINT booking_series_dates_series_fkey
        FOREIGN KEY (series_id, complex_id) REFERENCES booking_series (id, complex_id) ON DELETE CASCADE,
    CONSTRAINT booking_series_dates_outcome_valid CHECK (outcome IN ('booked', 'skipped')),
    CONSTRAINT booking_series_dates_reason_only_when_skipped
        CHECK ((outcome = 'skipped') = (reason IS NOT NULL))
);

ALTER TABLE booking_series_dates ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_series_dates FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON booking_series_dates
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON booking_series_dates
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- ============================================================================
-- bookings.series_id — which series an occurrence belongs to
-- ============================================================================
-- NULL for every booking that is not an occurrence, which is every existing
-- row. The composite key is MATCH SIMPLE, so a NULL is not checked; a value
-- must name a series of the booking's own complex. It is added NOT VALID and
-- validated straight after, so the table is locked only for the catalog change.
ALTER TABLE bookings ADD COLUMN series_id UUID;
ALTER TABLE bookings ADD CONSTRAINT bookings_series_in_same_complex
    FOREIGN KEY (series_id, complex_id) REFERENCES booking_series (id, complex_id) NOT VALID;
ALTER TABLE bookings VALIDATE CONSTRAINT bookings_series_in_same_complex;

-- The owner's "end from this date" reads a series' bookings by date.
CREATE INDEX idx_bookings_series_date ON bookings (series_id, date) WHERE series_id IS NOT NULL;

-- +goose Down

SET LOCAL lock_timeout = '3s';

DROP INDEX idx_bookings_series_date;
ALTER TABLE bookings DROP CONSTRAINT bookings_series_in_same_complex;
ALTER TABLE bookings DROP COLUMN series_id;
DROP TABLE booking_series_dates;
DROP TABLE booking_series;
