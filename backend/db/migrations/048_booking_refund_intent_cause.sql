-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- REFUND INTENT CAUSE — why a refund the sweep finishes was owed.
--
-- A payment that reaches a booking already cancelled is sent straight back by
-- the webhook, and the client is told why (failed_refunds.cause, 043). When
-- the claim for that refund fails for a moment, the refund-intent marker is
-- left standing and SweepOrphanedRefundIntents finishes it later. The marker
-- carried no reason, so the sweep claimed it as a cancellation and the client
-- got the plain notice, with no word that the money came back because the
-- payment arrived late.
--
-- refund_intent_cause is written next to a standing marker by that path only,
-- and every write that clears the marker clears it too. The sweep reads it and
-- claims the refund with it, so the attempt row carries the cause and the
-- notice says why, whether the sweep or the retry job records the refund.
-- NULL is a refund someone asked for (a cancellation), and every row written
-- before this column keeps it. Nothing is backfilled.
--
-- Adding a nullable column with no default rewrites nothing.
ALTER TABLE bookings ADD COLUMN refund_intent_cause TEXT;

ALTER TABLE bookings
    ADD CONSTRAINT bookings_refund_intent_cause_check
    CHECK (refund_intent_cause IS NULL OR refund_intent_cause IN (
        'late_payment',
        'slot_taken',
        'duplicate_payment'
    )) NOT VALID;
ALTER TABLE bookings VALIDATE CONSTRAINT bookings_refund_intent_cause_check;

-- +goose Down

SET LOCAL lock_timeout = '3s';

ALTER TABLE bookings DROP CONSTRAINT bookings_refund_intent_cause_check;
ALTER TABLE bookings DROP COLUMN refund_intent_cause;
