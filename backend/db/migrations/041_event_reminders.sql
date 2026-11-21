-- +goose Up

-- The day-before reminder of an event or tournament sign-up, and the two-hour
-- reminder of a scheduled tournament match, are sent once: the sweep that
-- queues one marks it here, as bookings.reminder_sent_2h does for a booking.
-- Scheduling a match again clears its mark, so the new time is reminded.
SET LOCAL lock_timeout = '3s';

ALTER TABLE open_play_registrations ADD COLUMN reminder_sent_at TIMESTAMPTZ;
ALTER TABLE tournament_matches ADD COLUMN reminder_sent_at TIMESTAMPTZ;

-- +goose Down

SET LOCAL lock_timeout = '3s';

ALTER TABLE tournament_matches DROP COLUMN reminder_sent_at;
ALTER TABLE open_play_registrations DROP COLUMN reminder_sent_at;
