-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- REGISTRATION REFUND INTENT — a participant's cancellation that owes a refund.
--
-- A participant who cancels inside the complex's refund window is owed their
-- money back, and the cancel request refunds it right after the cancellation
-- commits. A crash (or a provider outage that outlives its retries) between the
-- two left the spot cancelled and the money collected, with nothing durable
-- saying it was owed: the refund sweep only looked at the owner's
-- cancellations, because a participant who cancels too late keeps no refund.
--
-- refund_intent_at is set in the cancellation's own write when the refund is
-- owed, as bookings.refund_intent_at is for a booking, and the sweep drives
-- every cancelled registration carrying it whose MercadoPago payment still
-- reads collected. A late cancellation leaves it NULL, so its payment is kept.
--
-- Row-level security is already on the table (028); this only adds a column.
ALTER TABLE open_play_registrations ADD COLUMN refund_intent_at TIMESTAMPTZ;

ALTER TABLE open_play_registrations
    ADD CONSTRAINT open_play_registrations_refund_intent_only_when_cancelled
    CHECK (refund_intent_at IS NULL OR status = 'cancelled') NOT VALID;
ALTER TABLE open_play_registrations
    VALIDATE CONSTRAINT open_play_registrations_refund_intent_only_when_cancelled;

-- +goose Down

SET LOCAL lock_timeout = '3s';

ALTER TABLE open_play_registrations DROP CONSTRAINT open_play_registrations_refund_intent_only_when_cancelled;
ALTER TABLE open_play_registrations DROP COLUMN refund_intent_at;
