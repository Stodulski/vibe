-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- BOOKING HOLD START — when a storefront booking actually began holding hours
-- ============================================================================
-- An address may hold the same court hours for at most one payment window
-- before it waits (bookingstore.Store.InsertHold, slot-hold gaps S2). That is
-- a budget of time actually held, summed over the address's recent storefront
-- bookings on those hours, so letting a slot go and picking it again spends
-- only what is left, and the time a converted booking held while it waited for
-- its payment counts too.
--
-- created_at cannot say when holding began: it is the instant every reader
-- counts the window from, so it is moved forward when a hold is converted
-- (ConvertHold) or kept while its payment is in process (ExtendHold), and back
-- when a hold only gets what is left of the budget. hold_started_at is the
-- real instant the row started holding, written once by InsertHold for every
-- storefront booking that carries an address (hold_ip) and never moved.
--
-- Rows written before this column keep it NULL, and the budget reads their
-- created_at instead: they are at most a cooldown old by the time it matters.
-- Adding a nullable column with no default rewrites nothing.
ALTER TABLE bookings ADD COLUMN hold_started_at TIMESTAMPTZ;

-- +goose Down

SET LOCAL lock_timeout = '3s';

ALTER TABLE bookings DROP COLUMN hold_started_at;
