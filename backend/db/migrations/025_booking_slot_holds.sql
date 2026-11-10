-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- BOOKING SLOT HOLDS — the storefront holds a slot when the visitor picks a court
-- ============================================================================
-- Owner decision 2026-11-10 (storefront slot hold): picking a court on the
-- public page holds the slot for the payment window, before the visitor has
-- typed who they are. The hold IS the pending storefront booking POST /book
-- used to create on pay, created earlier: it already blocks the grid through
-- bookings_no_overlapping_span, is hidden from the owner's reads
-- (slotguard.UnpaidPublicBooking) and stops holding its slot once
-- BOOKING_PAYMENT_EXPIRY has passed since created_at.
--
-- What it does not have yet is anybody to name, so it carries neither a client
-- nor the snapshot of 024 until POST /book converts it. bookings_client_or_snapshot
-- is relaxed for exactly that row and nothing else: a hold (hold_token_hash set),
-- made by a visitor (created_by NULL), with nothing collected, still pending or
-- cancelled. Any other booking still names its client or carries the snapshot.
--
-- hold_token_hash is the SHA-256 of the hold token the visitor was handed (the
-- plaintext is never stored), on the row itself rather than in
-- booking_link_tokens: a hold token has exactly one booking and lives exactly as
-- long as the booking is an untouched hold, so it needs no expiry or table of
-- its own, and it must not authorize the link routes (status, cancel) the link
-- token does. hold_ip is the address that took it, for the per-address cap on
-- live holds. Converting the hold clears both, so a booking that names somebody
-- carries neither the token nor the address.
--
-- New columns inherit the table's grants and row-level security policy from
-- 001_init.sql's ACCESS section; nothing to add there.

ALTER TABLE bookings
    ADD COLUMN hold_token_hash BYTEA,
    ADD COLUMN hold_ip         TEXT;

CREATE UNIQUE INDEX bookings_hold_token_hash_key ON bookings (hold_token_hash)
    WHERE hold_token_hash IS NOT NULL;
CREATE INDEX idx_bookings_hold_ip ON bookings (hold_ip, created_at)
    WHERE hold_token_hash IS NOT NULL;

ALTER TABLE bookings DROP CONSTRAINT bookings_client_or_snapshot;
ALTER TABLE bookings ADD CONSTRAINT bookings_client_or_snapshot CHECK (
    client_id IS NOT NULL
    OR (client_first_name IS NOT NULL AND client_last_name IS NOT NULL AND client_phone IS NOT NULL)
    OR (hold_token_hash IS NOT NULL
        AND created_by IS NULL
        AND collection_status = 'unpaid'
        AND status IN ('pending', 'cancelled'))
);

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- An untouched hold names nobody, so 024's constraint cannot hold while one
-- exists. Holds are minutes-long and worth nothing to the owner, so the
-- rollback deletes them rather than inventing a client for them. Row-level
-- security is FORCEd, so the delete has to say it crosses tenants.
SET LOCAL app.bypass_tenant = 'on';
DELETE FROM bookings
WHERE hold_token_hash IS NOT NULL
  AND client_id IS NULL
  AND client_phone IS NULL;

ALTER TABLE bookings DROP CONSTRAINT bookings_client_or_snapshot;
ALTER TABLE bookings ADD CONSTRAINT bookings_client_or_snapshot CHECK (
    client_id IS NOT NULL
    OR (client_first_name IS NOT NULL AND client_last_name IS NOT NULL AND client_phone IS NOT NULL)
);

DROP INDEX idx_bookings_hold_ip;
DROP INDEX bookings_hold_token_hash_key;
ALTER TABLE bookings
    DROP COLUMN hold_token_hash,
    DROP COLUMN hold_ip;
