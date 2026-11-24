-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- BOOKING HOLD RELEASE REQUEST — leaving the confirm page lets the hold go a
-- minute later, not at once
-- ============================================================================
-- The storefront's confirm page used to release its slot hold the moment the
-- page went away (a `pagehide` beacon). A reload, or a phone discarding the
-- background tab while the visitor checks another app, lost the slot and sent
-- the visitor back to the hours. Owner decision 2026-11-24: "liberar con un
-- margen de 1 minuto".
--
-- The beacon now records when the visitor left (POST /book/holds/{token}/leave,
-- bookingstore.Store.RequestHoldRelease). A visitor back within the grace takes
-- the hold again (POST /book/holds/{token}/reclaim), which clears it and leaves
-- created_at, and so the hold's window, untouched. A request older than the
-- grace is released by the release_left_holds job through the same cancel an
-- immediate release runs (bookingstore.Store.ReleaseLeftHolds). Converting the
-- hold clears it too.
--
-- NULL means nobody asked. Nothing new needs an index: the sweep reads only
-- untouched holds, which bookings_hold_token_hash_key (025) already covers as a
-- partial index. Adding a nullable column with no default rewrites nothing.
ALTER TABLE bookings ADD COLUMN hold_release_requested_at TIMESTAMPTZ;

-- +goose Down

SET LOCAL lock_timeout = '3s';

ALTER TABLE bookings DROP COLUMN hold_release_requested_at;
