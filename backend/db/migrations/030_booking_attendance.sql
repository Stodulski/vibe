-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- BOOKING ATTENDANCE — the owner confirms the client actually came
-- ============================================================================
-- Owner decision 2026-11-15: a booking whose hours are over reads "Turno
-- finalizado" until the owner opens it and marks "Se presentó" or "No se
-- presentó". 'completed' alone cannot say which: the completion sweep
-- (CompletePastBookings) moves every confirmed booking whose span ended to
-- 'completed' whether anybody came or not.
--
-- attended_at is when the owner marked the client as present, written by POST
-- /api/v1/complexes/{id}/bookings/{bookingID}/attended (internal/bookings/store
-- MarkAttended) together with status 'completed'. NULL means nobody confirmed
-- attendance, which is what every booking completed before this migration
-- reads: no data is rewritten.
--
-- bookings_attended_only_completed keeps the two facts from disagreeing: a
-- booking that is not 'completed' cannot carry an attendance mark, so the
-- owner's completed -> no_show correction must clear attended_at in the same
-- write (UpdateBooking / UpdateBookingIfUnchanged in db/queries/bookings.sql).
-- Every existing row has attended_at NULL and passes it untouched. It is added
-- NOT VALID, which is instant, and validated by 031 in a transaction of its
-- own, the way 006/007 do it: validating here would scan the whole bookings
-- table under the ACCESS EXCLUSIVE lock this ALTER TABLE holds until commit. The new
-- column inherits the table's grants and row-level security policy from
-- 001_init.sql's ACCESS section; nothing to add there.

ALTER TABLE bookings ADD COLUMN attended_at TIMESTAMPTZ;

ALTER TABLE bookings ADD CONSTRAINT bookings_attended_only_completed CHECK (
    attended_at IS NULL OR status = 'completed'
) NOT VALID;

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- Drops the attendance marks with the column: they cannot be stored anywhere
-- else, and the status they were recorded beside stays 'completed'.
ALTER TABLE bookings DROP CONSTRAINT bookings_attended_only_completed;
ALTER TABLE bookings DROP COLUMN attended_at;
