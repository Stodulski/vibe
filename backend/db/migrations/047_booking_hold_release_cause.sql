-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- BOOKING HOLD RELEASES — a visitor's release is asked about again too, and is
-- never reopened by a late payment
-- ============================================================================
-- A visitor whose payment failed may give the slot back (ReleaseCheckout). The
-- release closes the checkout link, asks MercadoPago once for the booking's
-- payments and cancels the booking. MercadoPago's payment search can lag an
-- approval by a while, so a payment approved just before the link closed may
-- not be in that one answer yet; with its webhook lost, nobody ever found it.
--
-- The release now writes a booking_hold_expiries row too, in the transaction
-- that cancels the booking, so the release sweep asks MercadoPago once more a
-- while later (next_attempt_at, set by the writer) through the same
-- reconciliation an expiry gets. The row says why the booking was let go:
--
--   * cause = 'expired' — its payment window ran out (every row 044 wrote).
--     A late payment may still reopen it into hours still free (S5).
--   * cause = 'released' — its visitor let it go. That is a client
--     cancellation: a late payment found later is refunded, never confirmed,
--     and the visitor, who chose to release it, is not told it expired.
--
-- A column rather than a second table: the sweep's read, its backoff and its
-- bookkeeping are the same for both, and every reader that means "reopenable"
-- (bookingstore.ReopenableByLatePaymentSQL, the reversal trigger below) names
-- cause = 'expired' explicitly. Adding a NOT NULL column with a constant
-- default rewrites nothing; the existing rows are all expiries.
ALTER TABLE booking_hold_expiries
    ADD COLUMN cause TEXT NOT NULL DEFAULT 'expired',
    ADD CONSTRAINT booking_hold_expiries_cause_check CHECK (cause IN ('expired', 'released'));

-- The reversal trigger's exception (044) is for an expiry only: a booking its
-- visitor released stays cancelled whatever writes to it. Everything else about
-- the function is unchanged.
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
           AND EXISTS (SELECT 1 FROM booking_hold_expiries e
                       WHERE e.booking_id = OLD.id AND e.cause = 'expired')
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

-- Released rows would read as expiries once the column is gone, and a late
-- payment would then reopen a booking its visitor let go: they go first.
DELETE FROM booking_hold_expiries WHERE cause = 'released';

ALTER TABLE booking_hold_expiries DROP COLUMN cause;
