-- name: InsertOpenPlayEvent :one
INSERT INTO open_play_events (
    complex_id, name, sport, date, start_time, duration_minutes, capacity, price, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: InsertOpenPlayEventCourt :exec
INSERT INTO open_play_event_courts (event_id, complex_id, court_id)
VALUES ($1, $2, $3);

-- name: GetOpenPlayEvent :one
-- The tenant predicate is the explicit half of the isolation the row-level
-- security policies enforce; see GetBookingByID for why it is optional.
SELECT * FROM open_play_events
WHERE id = $1
  AND (sqlc.narg('complex_id')::uuid IS NULL
       OR complex_id = sqlc.narg('complex_id')::uuid);

-- name: LockOpenPlayEvent :one
-- Every write that changes how many spots are taken (a sign-up, a payment
-- confirmed late, the owner cancelling) takes this lock first, so two of them
-- can never both read the same count and both fit into the last spot.
SELECT * FROM open_play_events
WHERE id = $1
FOR UPDATE;

-- name: ListOpenPlayEventCourts :many
SELECT oc.court_id, c.name AS court_name
FROM open_play_event_courts oc
JOIN courts c ON c.id = oc.court_id
WHERE oc.event_id = $1
ORDER BY c.name, oc.court_id;

-- name: ListOpenPlayEvents :many
-- The events of one complex whose date is in the window, soonest first, with
-- how many spots are taken: confirmed registrations plus the holds still
-- running, the count capacity is checked against (CountOpenPlaySpotsTaken).
-- include_cancelled is what the owner's list sets and the public one does not.
SELECT e.*,
       (SELECT count(*) FROM open_play_registrations r
         WHERE r.event_id = e.id AND r.status = 'confirmed')::int AS confirmed_count,
       (SELECT count(*) FROM open_play_registrations r
         WHERE r.event_id = e.id AND r.status = 'held' AND r.held_until > NOW())::int AS held_count
FROM open_play_events e
WHERE e.complex_id = sqlc.arg(complex_id)
  AND e.date BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date
  AND (sqlc.arg(include_cancelled)::bool OR e.status = 'active')
ORDER BY e.date, e.start_time, e.id
LIMIT sqlc.arg(page_limit)::int;

-- name: CancelOpenPlayEvent :one
-- Only a live event can be cancelled; a second cancel finds nothing.
UPDATE open_play_events
SET status = 'cancelled', cancelled_at = NOW(), cancelled_by = $2, cancel_reason = $3
WHERE id = $1 AND status = 'active'
RETURNING *;

-- name: CountOpenPlaySpotsTaken :one
-- The spots of an event that are not free: registrations paid for, and holds
-- still inside their window. except is the registration being decided about,
-- which never counts against itself (NULL excludes nothing).
SELECT count(*)::int FROM open_play_registrations
WHERE event_id = sqlc.arg(event_id)
  AND (status = 'confirmed' OR (status = 'held' AND held_until > NOW()))
  AND (sqlc.narg('except')::uuid IS NULL OR id <> sqlc.narg('except')::uuid);

-- name: ExpireOpenPlayHoldsOfPhone :exec
-- Lets a person whose own earlier hold ran out sign up again: the one-live-
-- spot-per-phone index would otherwise still count the stale row.
UPDATE open_play_registrations
SET status = 'expired'
WHERE event_id = $1 AND phone = $2 AND status = 'held' AND held_until <= NOW();

-- name: InsertOpenPlayRegistration :one
INSERT INTO open_play_registrations (
    event_id, complex_id, first_name, last_name, phone, email,
    price, service_fee, held_until, access_token_hash, hold_ip
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetOpenPlayRegistration :one
SELECT * FROM open_play_registrations
WHERE id = $1
  AND (sqlc.narg('complex_id')::uuid IS NULL
       OR complex_id = sqlc.narg('complex_id')::uuid);

-- name: GetOpenPlayRegistrationByTokenHash :one
SELECT * FROM open_play_registrations
WHERE access_token_hash = $1;

-- name: ListOpenPlayRegistrations :many
-- Every registration of an event with the payment that carries its money (the
-- MercadoPago row first, as GetPaymentByBookingID prefers it), oldest first.
-- Registrations that never held a spot (expired holds) are the caller's to
-- leave out with live_only.
SELECT r.*,
       p.id AS payment_id,
       COALESCE(p.status::text, '')::text AS payment_status,
       COALESCE(p.amount, 0)::int AS payment_amount,
       COALESCE(p.service_fee, 0)::int AS payment_service_fee,
       COALESCE(p.refund_amount, 0)::int AS payment_refund_amount,
       p.mp_payment_id AS payment_mp_payment_id
FROM open_play_registrations r
LEFT JOIN LATERAL (
    SELECT * FROM payments pp
    WHERE pp.registration_id = r.id
    ORDER BY (pp.mp_payment_id IS NOT NULL) DESC, pp.created_at DESC
    LIMIT 1
) p ON TRUE
WHERE r.event_id = sqlc.arg(event_id)
  AND (NOT sqlc.arg(live_only)::bool OR r.status IN ('held', 'confirmed') OR (r.status = 'cancelled' AND p.id IS NOT NULL))
ORDER BY r.created_at, r.id;

-- name: ConfirmOpenPlayRegistration :one
-- A hold that ran out and was swept can still be confirmed by a payment that
-- arrives late, while the event has room (the caller checks, under the event
-- lock).
UPDATE open_play_registrations
SET status = 'confirmed', confirmed_at = NOW()
WHERE id = $1 AND status IN ('held', 'expired')
RETURNING *;

-- name: CancelOpenPlayRegistration :one
UPDATE open_play_registrations
SET status = 'cancelled', cancelled_at = NOW(), cancelled_by = $2
WHERE id = $1 AND status IN ('held', 'confirmed')
RETURNING *;

-- name: CancelOpenPlayEventRegistrations :many
-- The owner cancelling an event ends every registration of it at once. The rows
-- come back with their previous status, so the caller knows which were paid.
UPDATE open_play_registrations r
SET status = 'cancelled', cancelled_at = NOW(), cancelled_by = 'owner'
FROM open_play_registrations old
WHERE r.event_id = $1 AND r.status IN ('held', 'confirmed') AND old.id = r.id
RETURNING r.id, old.status AS previous_status;

-- name: ExpireOpenPlayHolds :many
-- The sweep: marks expired the holds nobody paid in time. They stopped counting
-- against capacity the moment held_until passed; this only closes the rows.
UPDATE open_play_registrations
SET status = 'expired'
WHERE status = 'held' AND held_until <= NOW()
RETURNING id, complex_id, event_id;

-- name: ListOwnerCancelledRegistrationsStillPaid :many
-- Registrations the owner's cancellation ended whose money has not been sent
-- back, once the cancellation is older than the grace: the refund the cancel
-- request itself never finished (a crash, a provider outage that outlived its
-- retries). The sweep drives each through the same refund path the cancel uses.
SELECT r.id, r.complex_id
FROM open_play_registrations r
JOIN payments p ON p.registration_id = r.id
WHERE r.status = 'cancelled' AND r.cancelled_by = 'owner'
  AND r.cancelled_at < sqlc.arg(older_than)::timestamptz
  AND p.method = 'mercadopago'
  AND p.status IN ('deposit_paid', 'fully_paid', 'partial_refund')
ORDER BY r.cancelled_at
LIMIT sqlc.arg(page_limit)::int;
