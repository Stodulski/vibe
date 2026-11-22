-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- REGISTRATION PAYMENT CHECK — asking MercadoPago before telling a person
-- their spot expired.
--
-- An event or tournament sign-up holds its spot for a window and opens a
-- MercadoPago checkout that expires with it. The sweep closed every hold whose
-- window ran out, and nothing ever asked MercadoPago whether it was paid: a
-- webhook that never arrived left the money captured on an expired
-- registration, with no payment row, no refund and no alert.
--
-- payment_check_due_at is set in the expiry's own write, for every hold (each
-- one reached checkout creation: a registration always has a price), and says
-- when the sweep should next ask MercadoPago about it. payment_check_notify
-- says whether the person is told their spot expired once MercadoPago finds
-- no payment: the sweep's expiry tells them, while the expiry a new sign-up of
-- the same phone makes (the person is already signing up again) does not.
-- An approved payment is handed to the same processing the webhook runs, which
-- confirms the spot when there is still room and refunds it otherwise. A
-- MercadoPago that cannot be asked pushes the check back, counted in
-- payment_checks, so a registration is asked a bounded number of times. The
-- column is cleared once MercadoPago answered or the sweep gave up.
--
-- Every row written before this column keeps it NULL: a hold that expired
-- before this deploy already had its expiry notice, and nothing is backfilled.
--
-- Row-level security is already on the table (028); this only adds columns,
-- and the table-level grants of 001's ACCESS section cover them.
ALTER TABLE open_play_registrations
    ADD COLUMN payment_check_due_at TIMESTAMPTZ,
    ADD COLUMN payment_checks SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN payment_check_notify BOOLEAN NOT NULL DEFAULT false;

-- The sweep reads the checks that are due.
CREATE INDEX idx_open_play_registrations_payment_check
    ON open_play_registrations (payment_check_due_at)
    WHERE payment_check_due_at IS NOT NULL;

-- +goose Down

SET LOCAL lock_timeout = '3s';

DROP INDEX idx_open_play_registrations_payment_check;
ALTER TABLE open_play_registrations
    DROP COLUMN payment_check_notify,
    DROP COLUMN payment_checks,
    DROP COLUMN payment_check_due_at;
