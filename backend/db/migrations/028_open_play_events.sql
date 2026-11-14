-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- OPEN PLAY EVENTS ("canchas libres") — stage 1 of events
-- ============================================================================
-- Owner decision 2026-11-14: the owner publishes an event that takes courts
-- off sale for a window of one day, and people sign up one by one and pay
-- online. The courts are free for the participants to play on, so the event
-- only holds them: it blocks its courts and hours everywhere availability is
-- decided, through open_play_event_occupancy below, exactly as a fixed weekly
-- series does through booking_series_occurrences (027).
--
-- start_time + duration_minutes give the window; like a booking it may run past
-- midnight. price is centavos per person, the price the sign-up checks out at.
-- status 'cancelled' releases the courts at once.
--
-- Tenant-scoped like every table in 001_init.sql's ACCESS section, with the
-- same tenant_isolation + tenant_bypass pair (see 003_cashbox.sql). New tables
-- inherit vibe_app's grants from the ALTER DEFAULT PRIVILEGES that 001_init.sql
-- registered, so no GRANT statement is needed here.
CREATE TABLE open_play_events (
    id               UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id       UUID NOT NULL,
    name             TEXT NOT NULL,
    sport            sport_type NOT NULL DEFAULT 'padel',
    date             DATE NOT NULL,
    start_time       TIME NOT NULL,
    duration_minutes INTEGER NOT NULL,
    capacity         INTEGER NOT NULL,
    price            INTEGER NOT NULL,
    status           TEXT NOT NULL DEFAULT 'active',
    cancel_reason    TEXT,
    cancelled_at     TIMESTAMPTZ,
    cancelled_by     UUID,
    created_by       UUID NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Referenced by the composite keys below and by payments.
    CONSTRAINT open_play_events_id_complex_id_key UNIQUE (id, complex_id),
    CONSTRAINT open_play_events_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    CONSTRAINT open_play_events_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users (id),
    CONSTRAINT open_play_events_cancelled_by_fkey
        FOREIGN KEY (cancelled_by) REFERENCES users (id),
    CONSTRAINT open_play_events_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    CONSTRAINT open_play_events_duration_check CHECK (duration_minutes BETWEEN 30 AND 720),
    CONSTRAINT open_play_events_capacity_check CHECK (capacity BETWEEN 1 AND 500),
    CONSTRAINT open_play_events_price_check CHECK (price > 0),
    CONSTRAINT open_play_events_status_valid CHECK (status IN ('active', 'cancelled')),
    CONSTRAINT open_play_events_cancel_columns CHECK ((status = 'cancelled') = (cancelled_at IS NOT NULL))
);

CREATE TRIGGER set_updated_at BEFORE UPDATE ON open_play_events
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- The public list and the occupancy function read the live events of a complex
-- by date.
CREATE INDEX idx_open_play_events_complex_date ON open_play_events (complex_id, date) WHERE status = 'active';

ALTER TABLE open_play_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE open_play_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON open_play_events
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON open_play_events
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- ============================================================================
-- OPEN PLAY EVENT COURTS — the courts an event holds
-- ============================================================================
CREATE TABLE open_play_event_courts (
    event_id   UUID NOT NULL,
    complex_id UUID NOT NULL,
    court_id   UUID NOT NULL,
    PRIMARY KEY (event_id, court_id),
    CONSTRAINT open_play_event_courts_event_fkey
        FOREIGN KEY (event_id, complex_id) REFERENCES open_play_events (id, complex_id) ON DELETE CASCADE,
    -- The same cross-tenant guard bookings carries: an event cannot hold
    -- another tenant's court.
    CONSTRAINT open_play_event_courts_court_in_same_complex
        FOREIGN KEY (court_id, complex_id) REFERENCES courts (id, complex_id)
);

CREATE INDEX idx_open_play_event_courts_court ON open_play_event_courts (court_id);

ALTER TABLE open_play_event_courts ENABLE ROW LEVEL SECURITY;
ALTER TABLE open_play_event_courts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON open_play_event_courts
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON open_play_event_courts
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- ============================================================================
-- OPEN PLAY REGISTRATIONS — one row per person signed up
-- ============================================================================
-- The person is a snapshot (name, phone, email), never a clients row: nobody is
-- a client until a booking succeeds (owner rule 2026-11-10). Phone is required;
-- email is optional everywhere (owner decision 2026-11-14).
--
-- A registration is born 'held': it holds one spot of the event's capacity
-- until held_until, like a pending storefront booking holds its slot. An
-- approved payment turns it 'confirmed'; a hold nobody paid stops counting once
-- held_until passes (the capacity count reads held_until, not a sweep) and the
-- sweep then marks it 'expired'. 'cancelled' is the participant's own
-- cancellation or the owner cancelling the event.
--
-- access_token_hash names the registration for the person who signed up: the
-- SHA-256 of the token the sign-up returned, the credential of the public
-- cancel routes, exactly as booking_slot_holds' hold token (025).
CREATE TABLE open_play_registrations (
    id                UUID PRIMARY KEY DEFAULT uuidv7(),
    event_id          UUID NOT NULL,
    complex_id        UUID NOT NULL,
    first_name        TEXT NOT NULL,
    last_name         TEXT NOT NULL,
    phone             TEXT NOT NULL,
    email             TEXT,
    status            TEXT NOT NULL DEFAULT 'held',
    -- What the person is asked to pay: the event's price at sign-up, and the
    -- flat service fee on top of it (the same rule bookings use).
    price             INTEGER NOT NULL,
    service_fee       INTEGER NOT NULL DEFAULT 0,
    held_until        TIMESTAMPTZ NOT NULL,
    access_token_hash BYTEA NOT NULL,
    hold_ip           TEXT,
    confirmed_at      TIMESTAMPTZ,
    cancelled_at      TIMESTAMPTZ,
    cancelled_by      TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Referenced by payments.
    CONSTRAINT open_play_registrations_id_complex_id_key UNIQUE (id, complex_id),
    CONSTRAINT open_play_registrations_event_fkey
        FOREIGN KEY (event_id, complex_id) REFERENCES open_play_events (id, complex_id) ON DELETE CASCADE,
    CONSTRAINT open_play_registrations_status_valid CHECK (status IN ('held', 'confirmed', 'cancelled', 'expired')),
    CONSTRAINT open_play_registrations_cancelled_by_valid CHECK (cancelled_by IS NULL OR cancelled_by IN ('participant', 'owner')),
    CONSTRAINT open_play_registrations_price_check CHECK (price > 0 AND service_fee >= 0)
);

CREATE TRIGGER set_updated_at BEFORE UPDATE ON open_play_registrations
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- One person (by phone) holds at most one live spot per event.
CREATE UNIQUE INDEX idx_open_play_registrations_one_live_per_phone
    ON open_play_registrations (event_id, phone) WHERE status IN ('held', 'confirmed');
CREATE UNIQUE INDEX idx_open_play_registrations_token ON open_play_registrations (access_token_hash);
CREATE INDEX idx_open_play_registrations_event ON open_play_registrations (event_id, created_at);
-- The expiry sweep reads the unpaid holds that ran out.
CREATE INDEX idx_open_play_registrations_held ON open_play_registrations (held_until) WHERE status = 'held';

ALTER TABLE open_play_registrations ENABLE ROW LEVEL SECURITY;
ALTER TABLE open_play_registrations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON open_play_registrations
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON open_play_registrations
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- ============================================================================
-- open_play_event_occupancy — the hours the events are holding, in one place
-- ============================================================================
-- Every question "is this court free then?" asks this function beside
-- booking_series_occurrences: the availability grid and the public booking
-- guards (GetBookedSlots, SlotTaken), the owner's blocked-slot guard
-- (SpanTaken), the owner's booking and series pre-checks, and the owner's
-- calendar. One expansion, so none of them can disagree about which hours an
-- event covers.
--
-- It returns one row per (active event, court) whose date is in [p_from, p_to],
-- with the instants the event holds, under the event's start date like a
-- booking: a caller asking about day D passes p_from = D - 1. p_complex_id and
-- p_court_ids narrow the rows; NULL means no narrowing.
--
-- It is SECURITY INVOKER: row-level security on the event tables applies to
-- whoever calls it.
-- +goose StatementBegin
CREATE FUNCTION open_play_event_occupancy(p_complex_id uuid, p_court_ids uuid[], p_from date, p_to date)
RETURNS TABLE (event_id uuid, complex_id uuid, court_id uuid, occurs_on date, span tstzrange)
    LANGUAGE sql
    STABLE
    PARALLEL SAFE
    SET search_path = pg_catalog, public
AS $$
    SELECT e.id, e.complex_id, c.court_id, e.date,
           tstzrange(
               booking_starts_at(e.date, e.start_time),
               booking_starts_at(e.date, e.start_time) + make_interval(mins => e.duration_minutes),
               '[)')
    FROM open_play_events e
    JOIN open_play_event_courts c ON c.event_id = e.id
    WHERE e.status = 'active'
      AND e.date BETWEEN p_from AND p_to
      AND (p_complex_id IS NULL OR e.complex_id = p_complex_id)
      AND (p_court_ids IS NULL OR c.court_id = ANY (p_court_ids))
$$;
-- +goose StatementEnd

COMMENT ON FUNCTION open_play_event_occupancy(uuid, uuid[], date, date) IS
    'The hours active open-play events hold on their courts: each holds them like a booking.';

-- ============================================================================
-- payments.registration_id — a payment is for a booking OR a registration
-- ============================================================================
-- An event sign-up is paid through the same MercadoPago flow as a booking and
-- lands in the same ledger, so the cash desk, the reports and the refund
-- lifecycle (claim, provider call, record) work on it unchanged. Every existing
-- row is a booking payment, so the CHECK holds for all of them; the constraints
-- are added NOT VALID and validated straight after, so the table is locked only
-- for the catalog change.
ALTER TABLE payments ALTER COLUMN booking_id DROP NOT NULL;
ALTER TABLE payments ADD COLUMN registration_id UUID;
ALTER TABLE payments ADD CONSTRAINT payments_registration_in_same_complex
    FOREIGN KEY (registration_id, complex_id) REFERENCES open_play_registrations (id, complex_id) NOT VALID;
ALTER TABLE payments VALIDATE CONSTRAINT payments_registration_in_same_complex;
ALTER TABLE payments ADD CONSTRAINT payments_exactly_one_subject
    CHECK ((booking_id IS NULL) <> (registration_id IS NULL)) NOT VALID;
ALTER TABLE payments VALIDATE CONSTRAINT payments_exactly_one_subject;
CREATE INDEX idx_payments_registration ON payments (registration_id) WHERE registration_id IS NOT NULL;

-- A refund attempt of a registration's payment names no booking.
ALTER TABLE failed_refunds ALTER COLUMN booking_id DROP NOT NULL;

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- A registration's payments and refund attempts cannot outlive the schema that
-- holds them: the booking column goes back to NOT NULL only without them.
DELETE FROM failed_refunds WHERE booking_id IS NULL;
DELETE FROM payments WHERE registration_id IS NOT NULL;
ALTER TABLE failed_refunds ALTER COLUMN booking_id SET NOT NULL;
DROP INDEX idx_payments_registration;
ALTER TABLE payments DROP CONSTRAINT payments_exactly_one_subject;
ALTER TABLE payments DROP CONSTRAINT payments_registration_in_same_complex;
ALTER TABLE payments DROP COLUMN registration_id;
ALTER TABLE payments ALTER COLUMN booking_id SET NOT NULL;
DROP FUNCTION open_play_event_occupancy(uuid, uuid[], date, date);
DROP TABLE open_play_registrations;
DROP TABLE open_play_event_courts;
DROP TABLE open_play_events;
