-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- BOOKING SERIES — fixed weekly bookings ("turnos fijos")
-- ============================================================================
-- Owner decision 2026-11-13: the owner books a court every week at the same
-- hour for the same person, with no end date and no horizon. The series is a
-- RULE, not a pile of rows: nothing is booked ahead. Every date the rule
-- matches counts as taken for its court and hours (see
-- booking_series_occurrences below) until that date is settled in
-- booking_series_dates, and on the venue's own day a job turns the date into an
-- ordinary row in `bookings`, so the cash desk, the reports and the reminders
-- work on it unchanged.
--
-- weekday is 0 = Sunday .. 6 = Saturday, the numbering Go's time.Weekday and
-- PostgreSQL's EXTRACT(DOW ...) share. It is stored beside starts_on, and a CHECK
-- keeps the two agreeing, so the pattern cannot drift from its first date.
--
-- price is the owner's manual price for every occurrence; NULL means each one is
-- priced like any other owner booking on the day it is created.
--
-- ends_on is inclusive and NULL while the series has no end. status 'ended' is
-- set when the owner ends the series, or when its last day has passed.
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

-- booking_series_occurrences reads the active series of one court (the
-- availability grid, the write guards) or of one complex (the owner's calendar).
-- A venue has a handful of series per court, so a partial index per key is all
-- the expansion needs.
CREATE INDEX idx_booking_series_active_court ON booking_series (court_id) WHERE status = 'active';
CREATE INDEX idx_booking_series_active_complex ON booking_series (complex_id) WHERE status = 'active';

ALTER TABLE booking_series ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_series FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON booking_series
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON booking_series
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- ============================================================================
-- BOOKING SERIES DATES — the ledger of settled dates
-- ============================================================================
-- One row per (series, local date), written once and never deleted: the primary
-- key IS the rule "a date is settled at most once ever". A date without a row
-- is still the rule's — it blocks its court and hours, and the daily job will
-- turn it into a booking. A date with a row no longer blocks anything on its own:
--   'booked'     a booking was created for it. If that booking is cancelled
--                afterwards the row stays, so the job never books it again and
--                the hours go back on sale.
--   'exception'  the date is not part of the series: the owner cancelled that
--                one date ('cancelled_by_owner'), or it could not be booked and
--                the reason says why. Its hours are free.
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
    CONSTRAINT booking_series_dates_outcome_valid CHECK (outcome IN ('booked', 'exception')),
    CONSTRAINT booking_series_dates_reason_only_for_exceptions
        CHECK ((outcome = 'exception') = (reason IS NOT NULL)),
    CONSTRAINT booking_series_dates_reason_valid CHECK (reason IS NULL OR reason IN
        ('cancelled_by_owner', 'overlap', 'blocked', 'closed_day', 'no_price', 'client_blocked', 'missed'))
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
-- booking_series_occurrences — the dates a series is holding, in one place
-- ============================================================================
-- Every question "is this court free then?" that must see a fixed booking asks
-- this function: the availability grid and the public booking guards
-- (GetBookedSlots, SlotTaken), the owner's blocked-slot guard (SpanTaken), the
-- owner's booking and series pre-checks, and the owner's calendar. One
-- expansion of the rule, so none of them can disagree about which dates a series
-- covers.
--
-- It returns one row per matching date in [p_from, p_to] of an ACTIVE series
-- (never before starts_on, never after ends_on) that has no row in
-- booking_series_dates, with the instants the occurrence holds. p_complex_id and
-- p_court_ids narrow the series; NULL means no narrowing (the daily job reads
-- every tenant under the bypass). An occurrence that starts on one day and runs
-- past midnight is returned under its start date, like a booking, so a caller
-- asking about day D passes p_from = D - 1.
--
-- It is SECURITY INVOKER: row-level security on booking_series and
-- booking_series_dates applies to whoever calls it.
-- +goose StatementBegin
CREATE FUNCTION booking_series_occurrences(p_complex_id uuid, p_court_ids uuid[], p_from date, p_to date)
RETURNS TABLE (series_id uuid, complex_id uuid, court_id uuid, occurs_on date, span tstzrange)
    LANGUAGE sql
    STABLE
    PARALLEL SAFE
    SET search_path = pg_catalog, public
AS $$
    SELECT s.id, s.complex_id, s.court_id, g.d::date,
           tstzrange(
               booking_starts_at(g.d::date, s.start_time),
               booking_starts_at(g.d::date, s.start_time) + make_interval(mins => s.duration_minutes),
               '[)')
    FROM booking_series s
    CROSS JOIN LATERAL generate_series(
        -- The first date on or after the window's start that falls on the weekday.
        greatest(p_from, s.starts_on)
            + ((s.weekday - extract(dow FROM greatest(p_from, s.starts_on))::int + 7) % 7),
        least(p_to, coalesce(s.ends_on, p_to)),
        interval '7 days') AS g(d)
    WHERE s.status = 'active'
      AND (p_complex_id IS NULL OR s.complex_id = p_complex_id)
      AND (p_court_ids IS NULL OR s.court_id = ANY (p_court_ids))
      AND NOT EXISTS (
          SELECT 1 FROM booking_series_dates x
          WHERE x.series_id = s.id AND x.date = g.d::date)
$$;
-- +goose StatementEnd

COMMENT ON FUNCTION booking_series_occurrences(uuid, uuid[], date, date) IS
    'The unsettled dates of active fixed weekly series: each holds its court and hours like a booking.';

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

-- Ending a series and the series detail read its bookings by date.
CREATE INDEX idx_bookings_series_date ON bookings (series_id, date) WHERE series_id IS NOT NULL;

-- +goose Down

SET LOCAL lock_timeout = '3s';

DROP INDEX idx_bookings_series_date;
ALTER TABLE bookings DROP CONSTRAINT bookings_series_in_same_complex;
ALTER TABLE bookings DROP COLUMN series_id;
DROP FUNCTION booking_series_occurrences(uuid, uuid[], date, date);
DROP TABLE booking_series_dates;
DROP TABLE booking_series;
