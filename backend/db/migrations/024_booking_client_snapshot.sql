-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- BOOKING CLIENT SNAPSHOT — a storefront booking names its client only once paid
-- ============================================================================
-- Owner decision 2026-11-10: a visitor who starts a checkout and never pays is
-- not a client of the venue, so POST /api/v1/book no longer inserts a clients
-- row. The pending booking carries who it is for in the four snapshot columns
-- below, with client_id NULL, and the transaction that records its payment
-- (internal/bookings/store AttachClientFromSnapshot) gets or creates the client
-- by (complex_id, phone), sets client_id and clears the snapshot.
--
-- So exactly one of the two says who the booking is for: client_id once a
-- client is attached, the snapshot until then. bookings_client_or_snapshot
-- holds that; every existing row has a client_id and passes it untouched.
--
-- Both foreign keys on client_id (bookings_client_id_fkey and the composite
-- bookings_client_in_same_complex) are MATCH SIMPLE, so a NULL client_id is
-- not checked against clients, which is what a pending storefront booking
-- needs. New columns inherit the table's grants and row-level security policy
-- from 001_init.sql's ACCESS section; nothing to add there.

ALTER TABLE bookings ALTER COLUMN client_id DROP NOT NULL;

ALTER TABLE bookings
    ADD COLUMN client_first_name TEXT,
    ADD COLUMN client_last_name  TEXT,
    ADD COLUMN client_phone      TEXT,
    ADD COLUMN client_email      CITEXT;

ALTER TABLE bookings ADD CONSTRAINT bookings_client_or_snapshot CHECK (
    client_id IS NOT NULL
    OR (client_first_name IS NOT NULL AND client_last_name IS NOT NULL AND client_phone IS NOT NULL)
);

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- Refuses, by design, while any booking still has no client: SET NOT NULL
-- fails on the first NULL rather than this file inventing a client for it.
ALTER TABLE bookings DROP CONSTRAINT bookings_client_or_snapshot;
ALTER TABLE bookings
    DROP COLUMN client_first_name,
    DROP COLUMN client_last_name,
    DROP COLUMN client_phone,
    DROP COLUMN client_email;
ALTER TABLE bookings ALTER COLUMN client_id SET NOT NULL;
