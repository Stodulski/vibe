-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- STOREFRONT BOOKING IP CAP — the per-address cap counts converted holds too
-- ============================================================================
-- 025 kept hold_ip only on an untouched hold, and the per-address cap
-- (bookingstore.Store.InsertHold) counted only those: converting a hold with
-- made-up details cleared the address and freed its place under the cap, so one
-- address could hold as many slots as it could convert. The address now stays
-- on a converted storefront booking until its payment attaches the client
-- (AttachBookingClient clears it), and POST /book without a hold token writes
-- it too, so the cap counts every unpaid storefront booking an address holds.
--
-- The cap's count therefore reads rows that no longer carry a hold token, which
-- 025's index (partial on hold_token_hash) does not cover. It is replaced by
-- one over every row that carries an address. Only storefront bookings within
-- their payment window, and expired unpaid ones, carry it.

DROP INDEX idx_bookings_hold_ip;
CREATE INDEX idx_bookings_hold_ip ON bookings (hold_ip, created_at)
    WHERE hold_ip IS NOT NULL;

-- +goose Down

SET LOCAL lock_timeout = '3s';

DROP INDEX idx_bookings_hold_ip;
CREATE INDEX idx_bookings_hold_ip ON bookings (hold_ip, created_at)
    WHERE hold_token_hash IS NOT NULL;
