-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- BOOKING HOLD EXPIRIES — storefront bookings cancelled only because their
-- payment window ran out.
--
-- Two paths cancel an unpaid storefront booking whose window lapsed: the
-- release sweep (ReleaseExpiredPayments) and the inline release a new booking
-- runs over the stale rows it lands on (ReleaseStalePendingOverlaps). Both
-- wrote the same note and nothing else, so afterwards such a booking was
-- indistinguishable from one the owner or the client cancelled. That mattered
-- twice over:
--
--   * The inline release never asked MercadoPago anything and never closed the
--     checkout link. A payment approved on it whose webhook was lost left the
--     money captured with no payment row, no refund and no alert, because the
--     sweep only ever reads pending bookings.
--   * A late payment on such a booking was always refunded, even into hours
--     nobody else took, while the same payment one minute earlier (booking
--     still pending) confirmed it.
--
-- A row here says "this booking was cancelled by hold expiry, and nothing
-- else". Both expiry paths write it in the statement or transaction that
-- cancels the booking. The release sweep reads the recent ones back, closes
-- their checkout link and asks MercadoPago once for an approved payment
-- (reconciled_at). A run that cannot ask, or cannot hand a payment over,
-- counts it in failed_attempts and pushes next_attempt_at back five minutes
-- more for every failure, at most two hours apart, the backoff event
-- registrations use (045): a MercadoPago outage over an hour must not use up
-- every attempt of the bookings expired in it. The confirmation path reopens a
-- booking that has a row here when its hours are still free, deleting the row
-- in the same transaction so a later owner cancellation is final again.
--
-- Tenant-scoped like every table in 001_init.sql's ACCESS section, with the
-- same tenant_isolation + tenant_bypass pair. vibe_app's grants come from the
-- ALTER DEFAULT PRIVILEGES that 001_init.sql registered.
CREATE TABLE booking_hold_expiries (
    booking_id      UUID PRIMARY KEY,
    complex_id      UUID NOT NULL,
    expired_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reconciled_at   TIMESTAMPTZ,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT booking_hold_expiries_booking_id_fkey
        FOREIGN KEY (booking_id) REFERENCES bookings (id) ON DELETE CASCADE,
    CONSTRAINT booking_hold_expiries_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    CONSTRAINT booking_hold_expiries_failed_attempts_check CHECK (failed_attempts >= 0)
);

-- The sweep's read: the rows not yet reconciled whose next attempt is due.
CREATE INDEX idx_booking_hold_expiries_unreconciled
    ON booking_hold_expiries (next_attempt_at)
    WHERE reconciled_at IS NULL;

ALTER TABLE booking_hold_expiries ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_hold_expiries FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON booking_hold_expiries
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON booking_hold_expiries
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- The reversal trigger keeps refusing a terminal booking going live again,
-- with one exception: a booking cancelled only by hold expiry may be
-- confirmed by the payment it was waiting for. It must be getting money
-- (collection_status leaves 'unpaid'), owe no refund, and still carry its
-- hold-expiry row. Everything else about the function is unchanged.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION bookings_forbid_status_reversal() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
AS $$
BEGIN
    IF OLD.status IN ('cancelled', 'completed', 'no_show')
       AND NEW.status IN ('pending', 'confirmed')
       AND NOT (
           OLD.status = 'cancelled'
           AND NEW.status = 'confirmed'
           AND OLD.collection_status = 'unpaid'
           AND NEW.collection_status <> 'unpaid'
           AND OLD.refund_status = 'none'
           AND OLD.refund_intent_at IS NULL
           AND EXISTS (SELECT 1 FROM booking_hold_expiries e WHERE e.booking_id = OLD.id)
       ) THEN
        RAISE EXCEPTION
            'booking %: status cannot return to % from the terminal status %',
            OLD.id, NEW.status, OLD.status
            USING ERRCODE = '23514',
                  CONSTRAINT = 'bookings_status_no_terminal_reentry';
    END IF;

    IF OLD.collection_status <> 'unpaid' AND NEW.collection_status = 'unpaid' THEN
        RAISE EXCEPTION
            'booking %: collection_status cannot return to unpaid from %',
            OLD.id, OLD.collection_status
            USING ERRCODE = '23514',
                  CONSTRAINT = 'bookings_collection_status_no_return_to_unpaid';
    END IF;

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION bookings_forbid_status_reversal() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
AS $$
BEGIN
    IF OLD.status IN ('cancelled', 'completed', 'no_show')
       AND NEW.status IN ('pending', 'confirmed') THEN
        RAISE EXCEPTION
            'booking %: status cannot return to % from the terminal status %',
            OLD.id, NEW.status, OLD.status
            USING ERRCODE = '23514',
                  CONSTRAINT = 'bookings_status_no_terminal_reentry';
    END IF;

    IF OLD.collection_status <> 'unpaid' AND NEW.collection_status = 'unpaid' THEN
        RAISE EXCEPTION
            'booking %: collection_status cannot return to unpaid from %',
            OLD.id, OLD.collection_status
            USING ERRCODE = '23514',
                  CONSTRAINT = 'bookings_collection_status_no_return_to_unpaid';
    END IF;

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

DROP TABLE booking_hold_expiries;
