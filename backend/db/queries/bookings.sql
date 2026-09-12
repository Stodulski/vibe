-- name: InsertBooking :one
INSERT INTO bookings (
    complex_id, court_id, client_id, date,
    start_time, duration_minutes,
    price, deposit_amount, status, collection_status, refund_status,
    notes, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetBookingByID :one
--
-- The tenant predicate is the explicit half of the isolation the row-level
-- security policies enforce (db/migrations/001_init.sql). RLS is the guarantee;
-- this is the statement saying out loud which tenant the row is supposed to
-- belong to, so a lookup by id cannot reach across tenants even if a policy is
-- ever relaxed, dropped, or bypassed for a path that did not need the bypass.
--
-- It is optional, and the NULL branch is not a loophole: it is exactly the set
-- of callers that legitimately have no tenant on the context — the cron sweeps,
-- the superadmin console, the MercadoPago webhook resolving its payment, the
-- public link resolving its booking. Those run under app.bypass_tenant, so RLS
-- lets them through anyway and a mandatory predicate here would only break
-- them. The store passes data.TenantFromContext's answer; a scoped caller gets
-- the filter, a bypassed one is exactly where it was.
SELECT * FROM bookings
WHERE id = $1
  AND (sqlc.narg('complex_id')::uuid IS NULL
       OR complex_id = sqlc.narg('complex_id')::uuid);

-- name: GetBookingsByComplex :many
SELECT * FROM bookings
WHERE complex_id = $1
  AND date BETWEEN $2 AND $3
  AND (date, id) > ($4, $5)
ORDER BY date ASC, id ASC
LIMIT $6;

-- name: GetBookingsByCourt :many
SELECT * FROM bookings
WHERE court_id = $1
  AND date BETWEEN $2 AND $3
ORDER BY date ASC, start_time ASC;

-- name: GetBookingByIDForUpdate :one
-- Locks the booking row for the duration of the enclosing transaction.
-- The refund recorder reads status, notes and deposit_amount back under this
-- lock and writes them straight through, so a concurrent booking update cannot
-- interleave with the money write or erase the deposit. This row lock is the
-- serialization on this table now that the `version` counter is gone: it is
-- held by the database for the whole transaction, which is what the counter
-- only pretended to be.
--
-- Tenant-scoped like GetBookingByID above, and optional for the same reason.
SELECT * FROM bookings
WHERE id = $1
  AND (sqlc.narg('complex_id')::uuid IS NULL
       OR complex_id = sqlc.narg('complex_id')::uuid)
FOR UPDATE;

-- name: UpdateBooking :one
-- Unconditional on the row's prior state. What refuses an illegitimate write is
-- the bookings_forbid_status_reversal trigger, which is enforced for every
-- writer rather than for the one query that remembered to check a counter —
-- see the bookings section of db/migrations/001_init.sql for why the counter was removed.
UPDATE bookings
SET status = $2,
    collection_status = $3,
    refund_status = $4,
    notes = $5,
    deposit_amount = $6,
    refund_intent_at = $7
WHERE id = $1
RETURNING *;

-- GetBookedSlots returns the hours already taken on the given courts and date.
--
-- Its status predicate must stay identical to releasedBookingStatuses in
-- internal/data/slotguard and to the WHERE clause on
-- bookings_no_overlapping_span (db/migrations/001_init.sql). Those three are one
-- predicate written three times, because a constraint inside Postgres cannot
-- read a Go constant and a file compiled by sqlc cannot concatenate one.
-- History for why the count matters: this query was named GetAvailableSlots and
-- excluded only 'cancelled' while slotTaken excluded 'cancelled' and 'no_show',
-- so a no_show slot was drawn as taken, passed the overlap check as free, and
-- was refused by the index as a duplicate — three answers to one question.
--
-- The fourth copy, idx_bookings_no_double, is gone: the exclusion constraint dropped
-- it once the exclusion constraint made it redundant.
-- Two instants, not a date and two times of day. A booking that runs past
-- midnight has no single time of day that says which day its end belongs to,
-- and the caller comparing "23:00" against "01:00" lexicographically is exactly
-- the shape that made a 23:00 booking invisible to the guard meant to see it
-- (the retired CHECK (start_time < end_time)). lower/upper come off the same span the exclusion
-- constraint enforces, so what the storefront greys out and what the write path
-- refuses are read from one value.
--
-- Matching on `span && local_day(day)` rather than `date = day`: a court
-- occupied until 01:00 is occupied on that morning's grid too. See
-- local_day() in db/migrations/001_init.sql for why "occupies day X" beat "started on day X".
-- name: GetBookedSlots :many
--
-- payment_expiry_seconds is the configured payment hold (Config.PaymentExpiry,
-- default 15 minutes — see internal/stores), passed as a parameter
-- rather than baked in as INTERVAL '15 minutes' for the same reason slotTaken
-- in internal/data/slotguard takes hold as an argument: this carve-out and
-- the insert-time collision guard have to agree on the same number, or a
-- pending booking between the two configured values shows as free here and
-- taken there (or the reverse). Both now read Config.PaymentExpiry once.
SELECT b.court_id, lower(b.span)::timestamptz AS starts_at, upper(b.span)::timestamptz AS ends_at
FROM bookings b
WHERE b.court_id = ANY($1::uuid[])
  AND b.span && local_day(sqlc.arg(day)::date)
  AND b.status NOT IN ('cancelled', 'no_show')
  AND NOT (
    b.status = 'pending'
    AND b.collection_status = 'unpaid'
    AND b.created_by IS NULL
    AND b.created_at < NOW() - make_interval(secs => sqlc.arg(payment_expiry_seconds)::float8)
  )
ORDER BY b.court_id, lower(b.span);

-- GetBookingsForReminder2h returns the confirmed bookings whose game starts
-- within the next two hours and that have not been reminded yet.
--
-- The window is expressed over booking_starts_at (db/migrations/001_init.sql), the one
-- place date and start_time are combined into an instant, and must stay that
-- way: the predicate this replaced compared times of day
-- (`start_time <= (now::time + INTERVAL '2 hours')`), which wraps at 23:00 into
-- `start_time <= 01:00 AND start_time > 23:00` and selects nothing, so no
-- booking in the last two hours of the day was ever reminded. Two timestamptz
-- values have no midnight between them.
--
-- `now` is a parameter rather than NOW() so the caller owns the clock — the
-- same reason GetForReminder2hEnriched in internal/bookings/store/bookings.go takes one.
-- A window keyed to whenever the test suite happened to run is a window nobody
-- can test at 23:00.
-- name: GetBookingsForReminder2h :many
SELECT * FROM bookings
WHERE booking_starts_at(date, start_time) > sqlc.arg(now)::timestamptz
  AND booking_starts_at(date, start_time) <= sqlc.arg(now)::timestamptz + INTERVAL '2 hours'
  AND status = 'confirmed'
  AND reminder_sent_2h = false;

-- name: MarkReminderSent2h :exec
UPDATE bookings
SET reminder_sent_2h = true, updated_at = NOW()
WHERE id = $1;

-- name: GetUnconfirmedAfterReminder :many
-- Kept for backwards compatibility with generated code. No longer used by business logic.
SELECT * FROM bookings WHERE false;

-- name: GetNextBookingByClientPhone :one
-- Kept for backwards compatibility with generated code. No longer used by business logic.
SELECT * FROM bookings WHERE false LIMIT 1;
