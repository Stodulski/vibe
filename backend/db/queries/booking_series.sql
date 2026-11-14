-- name: InsertBookingSeries :one
INSERT INTO booking_series (
    complex_id, court_id, client_id, weekday, start_time, duration_minutes,
    price, notes, starts_on, ends_on, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetBookingSeries :one
-- The tenant predicate is the explicit half of the isolation the row-level
-- security policies enforce; see GetBookingByID for why it is optional.
SELECT * FROM booking_series
WHERE id = $1
  AND (sqlc.narg('complex_id')::uuid IS NULL
       OR complex_id = sqlc.narg('complex_id')::uuid);

-- name: EndExpiredBookingSeries :execrows
-- Marks ended the active series whose last day is behind the given date; only
-- the materialization job calls it, under the tenant bypass. Such a series no
-- longer holds any date (booking_series_occurrences stops at ends_on), so this
-- changes what the owner reads, not what is sold.
UPDATE booking_series
SET status = 'ended'
WHERE status = 'active' AND ends_on IS NOT NULL AND ends_on < $1;

-- name: EndBookingSeries :one
UPDATE booking_series
SET ends_on = $2, status = $3
WHERE id = $1
RETURNING *;

-- name: ListBookingSeriesDates :many
-- The ledger rows of one series inside a window: the dates already settled
-- (booked or excepted).
SELECT series_id, complex_id, date, outcome, reason, created_at
FROM booking_series_dates
WHERE series_id = $1 AND date >= $2 AND date <= $3
ORDER BY date;

-- name: RecordBookingSeriesException :execrows
-- A date that is not part of the series: cancelled by the owner, or one that
-- could not be booked. DO NOTHING keeps the first answer: a date already booked
-- or excepted is never rewritten, and the row count says whether this call was
-- the one that settled it.
INSERT INTO booking_series_dates (series_id, complex_id, date, outcome, reason)
VALUES ($1, $2, $3, 'exception', $4)
ON CONFLICT (series_id, date) DO NOTHING;

-- name: ListBookingSeriesBookingsFrom :many
SELECT * FROM bookings
WHERE series_id = $1 AND date >= $2
ORDER BY date, start_time;
