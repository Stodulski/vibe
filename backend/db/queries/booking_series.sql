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

-- name: ListActiveBookingSeries :many
-- Every active series across tenants, for the daily extension job (which runs
-- under the tenant bypass). Series whose end is already behind them are
-- included on purpose: the job is what marks them ended.
SELECT * FROM booking_series
WHERE status = 'active'
ORDER BY created_at, id;

-- name: EndBookingSeries :one
UPDATE booking_series
SET ends_on = $2, status = $3
WHERE id = $1
RETURNING *;

-- name: ListBookingSeriesDates :many
-- The dates of one series already generated (booked or skipped) inside a
-- window, which is what stops the job from generating a date twice.
SELECT series_id, complex_id, date, outcome, reason, created_at
FROM booking_series_dates
WHERE series_id = $1 AND date >= $2 AND date <= $3
ORDER BY date;

-- name: RecordBookingSeriesSkip :execrows
-- A date that could not be booked. DO NOTHING keeps the first answer: a date
-- already booked or already skipped is never rewritten.
INSERT INTO booking_series_dates (series_id, complex_id, date, outcome, reason)
VALUES ($1, $2, $3, 'skipped', $4)
ON CONFLICT (series_id, date) DO NOTHING;

-- name: ListBookingSeriesBookingsFrom :many
SELECT * FROM bookings
WHERE series_id = $1 AND date >= $2
ORDER BY date, start_time;
