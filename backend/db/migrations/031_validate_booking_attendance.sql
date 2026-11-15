-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- VALIDATE bookings_attended_only_completed
--
-- 030_booking_attendance.sql adds bookings_attended_only_completed as NOT
-- VALID, which does not scan existing rows. The scan happens here, in its own
-- migration, for the reason 007_validate_payment_manual_refunds.sql gives:
-- goose runs each file in one transaction, so validating inside 030 would scan
-- every booking under the ACCESS EXCLUSIVE lock that file's ALTER TABLE holds
-- until commit, blocking every booking read and write meanwhile. In a
-- transaction of its own, VALIDATE CONSTRAINT takes only SHARE UPDATE
-- EXCLUSIVE: it blocks other DDL on bookings, not ordinary reads and writes.
ALTER TABLE bookings
    VALIDATE CONSTRAINT bookings_attended_only_completed;

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- Nothing to undo: VALIDATE CONSTRAINT only records that a constraint 030
-- already added holds for every row. Rolling back 030 is what removes it.
