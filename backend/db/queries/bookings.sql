-- name: InsertBooking :one
INSERT INTO bookings (
    complex_id, court_id, client_id, date,
    start_time, duration_minutes,
    price, deposit_amount, status, collection_status, refund_status,
    notes, created_by,
    client_first_name, client_last_name, client_phone, client_email
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING *;

-- name: GetUnattachedBookingClientSnapshot :one
-- Who a storefront booking is for, while it has no client yet: the snapshot
-- POST /book stored instead of creating a clients row (024). No row means the
-- booking already names its client, which is what makes attaching idempotent.
SELECT complex_id, client_first_name, client_last_name, client_phone, client_email
FROM bookings
WHERE id = $1 AND client_id IS NULL;

-- name: AttachBookingClient :one
-- Names the booking's client and drops the snapshot it was held under, so the
-- person is recorded in one place (clients) once the booking succeeded.
--
-- client_email is kept: it is the address typed for THIS booking, and its
-- notices go there (bookingstore.ContactOf). The client row keeps its own
-- email; an unauthenticated storefront request never overwrites it, so
-- clearing this column sent a returning client's notices to an address they
-- did not give for this booking.
--
-- hold_ip goes too: a paid booking is nobody's hold and stops counting against
-- the address that took it (bookingstore.Store.InsertHold).
UPDATE bookings
SET client_id = $2,
    client_first_name = NULL,
    client_last_name = NULL,
    client_phone = NULL,
    hold_ip = NULL
WHERE id = $1 AND client_id IS NULL
RETURNING client_id;

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
-- Overlapping the days asked for, not starting on them: a booking that runs
-- 23:00 to 01:00 occupies both days, and an owner looking at the next morning
-- has to see the hours it is still holding. Same predicate as the owner list in
-- internal/bookings/store (GetByComplex); see local_day() in
-- db/migrations/001_init.sql.
SELECT * FROM bookings
WHERE complex_id = $1
  AND span && tstzrange(lower(local_day($2)), upper(local_day($3)), '[)')
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
    refund_intent_at = $7,
    -- An attendance mark belongs to a completed booking only
    -- (bookings_attended_only_completed): the owner's completed -> no_show
    -- correction drops it in the same write instead of tripping the CHECK.
    attended_at = CASE WHEN $2 = 'completed'::booking_status THEN attended_at END
WHERE id = $1
RETURNING *;

-- name: UpdateBookingIfUnchanged :one
-- The owner's edit, applied only while the row is still the one the owner read.
-- updated_at is stamped by the set_updated_at trigger on every write, so an
-- equal value means nothing has written the row since. The whole-row write
-- below is built from the owner's snapshot, and the trigger does not refuse
-- every stale one: a cancellation reverted to no_show, or a deposit reverted
-- from fully_paid, is a legal-looking transition. A row that moved matches
-- nothing and comes back as no rows, which the store answers as an edit
-- conflict instead of writing the stale snapshot over the other writer's change.
--
-- UpdateBooking stays unconditional for every other writer.
UPDATE bookings
SET status = $2,
    collection_status = $3,
    refund_status = $4,
    notes = $5,
    deposit_amount = $6,
    refund_intent_at = $7,
    -- An attendance mark belongs to a completed booking only
    -- (bookings_attended_only_completed): the owner's completed -> no_show
    -- correction drops it in the same write instead of tripping the CHECK.
    attended_at = CASE WHEN $2 = 'completed'::booking_status THEN attended_at END
WHERE id = $1
  AND updated_at = sqlc.arg(expected_updated_at)
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
-- The window is the local days day_from..day_to inclusive (the same day twice
-- for the storefront's single-day grid). Real bookings only: the dates fixed
-- weekly series are holding are read beside it by Store.SeriesOccurrences, so
-- GetBookedSlotsByCourtIDs can hand both to every caller as one answer.
--
-- payment_expiry_seconds is the configured payment hold (Config.PaymentExpiry,
-- default 10 minutes — see internal/stores), passed as a parameter
-- rather than baked in as INTERVAL '15 minutes' for the same reason slotTaken
-- in internal/data/slotguard takes hold as an argument: this carve-out and
-- the insert-time collision guard have to agree on the same number, or a
-- pending booking between the two configured values shows as free here and
-- taken there (or the reverse). Both now read Config.PaymentExpiry once.
-- online_hold marks a visitor booking the hours online right now: the stale
-- ones are filtered out below, so an unpaid storefront pending row that is
-- left is inside its window (slotguard.LiveOnlineHold).
SELECT b.court_id, lower(b.span)::timestamptz AS starts_at, upper(b.span)::timestamptz AS ends_at,
       (b.status = 'pending' AND b.collection_status = 'unpaid' AND b.created_by IS NULL)::bool AS online_hold
FROM bookings b
WHERE b.court_id = ANY($1::uuid[])
  AND b.span && tstzrange(lower(local_day(sqlc.arg(day_from)::date)), upper(local_day(sqlc.arg(day_to)::date)), '[)')
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

-- name: MarkBookingManualBalanceOwed :execrows
-- Marks a cancelled booking's cash or transfer balance as owed back by hand, so
-- the owner's manual-refund action becomes available. Only refund_status moves,
-- and only from 'none': the refund path runs after its own read, and a
-- whole-row write from that snapshot would revert whatever another writer (an
-- owner edit) changed in between. A row already past 'none' is left alone.
UPDATE bookings
SET refund_status = 'partial'
WHERE id = $1 AND refund_status = 'none';

-- name: CancelUnpaidSeriesOccurrence :one
-- Ending a series cancels one of its occurrences, but only while it is still
-- the occurrence the owner meant: active, nothing collected, and its hours
-- still ahead of now. A counter payment, a cancellation or the start of the
-- game that commits first leaves the row out (no rows), and the booking
-- stands. Run inside the transaction that also moves the series' end.
UPDATE bookings
SET status = 'cancelled',
    notes = CASE WHEN notes IS NULL THEN sqlc.arg(note)::text
                 ELSE notes || ' | ' || sqlc.arg(note)::text END
WHERE id = sqlc.arg(id)
  AND series_id = sqlc.arg(series_id)
  AND status IN ('pending', 'confirmed')
  AND collection_status = 'unpaid'
  AND lower(span) > sqlc.arg(now)::timestamptz
RETURNING *;

-- name: ExpireUnpaidBooking :one
-- The release sweep's cancellation of a storefront booking whose payment
-- window ran out. Conditional, unlike UpdateBooking: the sweep decides from a
-- read taken before it asked MercadoPago, and a client cancellation or a
-- payment committed in between must not be overwritten into an expiry. No row
-- means the booking is no longer pending and unpaid.
UPDATE bookings
SET status = 'cancelled',
    notes = $2
WHERE id = $1
  AND status = 'pending'
  AND collection_status = 'unpaid'
RETURNING *;

-- name: InsertBookingHoldExpiry :exec
-- Records that a storefront booking was cancelled by hold expiry and nothing
-- else (044_booking_hold_expiries.sql). Written in the transaction that
-- cancels it; a second write for the same booking changes nothing.
INSERT INTO booking_hold_expiries (booking_id, complex_id)
VALUES ($1, $2)
ON CONFLICT (booking_id) DO NOTHING;

-- name: MarkBookingHoldReconciled :exec
-- The release sweep asked MercadoPago about this expired booking and handed
-- any approved payment over: it is not asked again.
UPDATE booking_hold_expiries
SET reconciled_at = NOW()
WHERE booking_id = $1;

-- name: RecordBookingHoldReconcileFailure :one
-- MercadoPago could not be asked, or an approved payment could not be handed
-- over: counted, so the sweep gives up after a bounded number of attempts, and
-- asked again later, five minutes more for every failed attempt, at most two
-- hours apart (the backoff DeferRegistrationPaymentCheck uses).
UPDATE booking_hold_expiries
SET failed_attempts = failed_attempts + 1,
    next_attempt_at = NOW() + make_interval(mins => LEAST(5 * (failed_attempts + 1), 120))
WHERE booking_id = $1
RETURNING failed_attempts;

-- name: DeleteBookingHoldExpiry :exec
-- The booking was reopened by the payment it was waiting for, in the same
-- transaction: from here it is a live booking, and a later cancellation of it
-- is final like any other.
DELETE FROM booking_hold_expiries
WHERE booking_id = $1;
