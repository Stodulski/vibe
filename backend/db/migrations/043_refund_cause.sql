-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- REFUND CAUSE — why a refund nobody asked for was made.
--
-- The webhook sends some payments straight back on its own: a payment that
-- arrived for a booking already cancelled, one whose hours went to somebody
-- else, a second payment on a booking or a sign-up already paid, a payment for
-- a spot that is gone. When MercadoPago takes the refund at once, the person is
-- told it came back and why. When it does not, the claim's attempt row is what
-- the retry job finishes later, and that row said nothing about why, so the
-- notice it sent had no reason line and the owner of a booking paid twice was
-- never told.
--
-- cause is written in the claim's own transaction, with the attempt row, and
-- the retry job reads it to send the same notice the immediate path sends.
-- NULL is a refund someone asked for (a cancellation), and every row written
-- before this column keeps it: the retry job sends those the plain notice, as
-- before. Nothing is backfilled.
--
-- Row-level security is already on the table (001); this only adds a column,
-- and the table-level grants of 001's ACCESS section cover it.
ALTER TABLE failed_refunds ADD COLUMN cause TEXT;

ALTER TABLE failed_refunds
    ADD CONSTRAINT failed_refunds_cause_check
    CHECK (cause IS NULL OR cause IN (
        'late_payment',
        'slot_taken',
        'duplicate_payment',
        'registration_spot_gone',
        'registration_extra_payment'
    )) NOT VALID;
ALTER TABLE failed_refunds VALIDATE CONSTRAINT failed_refunds_cause_check;

-- +goose Down

SET LOCAL lock_timeout = '3s';

ALTER TABLE failed_refunds DROP CONSTRAINT failed_refunds_cause_check;
ALTER TABLE failed_refunds DROP COLUMN cause;
