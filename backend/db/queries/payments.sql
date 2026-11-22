-- name: InsertPayment :one
INSERT INTO payments (booking_id, complex_id, amount, method, status, service_fee, mp_payment_id, mp_preference_id, status_detail, registration_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetPaymentByBookingID :one
-- A booking can legitimately have more than one payment row (the checkout row plus a
-- cash row inserted by ConfirmPayment), and idx_payments_booking is not unique, so
-- without an explicit order the driver returns an arbitrary row. Prefer the MercadoPago
-- row: it is the one carrying mp_payment_id, which the refund path needs.
SELECT * FROM payments
WHERE booking_id = $1
ORDER BY (mp_payment_id IS NOT NULL) DESC, created_at DESC
LIMIT 1;

-- name: ListPaymentsByBookingID :many
-- Every payment row of a booking, oldest first (the deposit before the balance),
-- for callers that need the whole ledger rather than one row — the refund path
-- sums money and the booking detail shows each payment; GetPaymentByBookingID's
-- single MercadoPago-preferred row is for callers that need that one row.
SELECT * FROM payments
WHERE booking_id = $1
ORDER BY created_at ASC;

-- name: GetPaymentByMPID :one
SELECT * FROM payments
WHERE mp_payment_id = $1;

-- name: UpdatePayment :one
UPDATE payments
SET status = $1,
    mp_payment_id = $2,
    mp_preference_id = $3,
    refund_amount = $4,
    status_detail = $5
WHERE id = $6
RETURNING *;

-- name: TakePaymentRow :one
-- UpdatePayment for an approved MercadoPago payment written onto a booking's
-- existing row, only while that row still takes it: a checkout no money has
-- reached (unpaid, nothing refunded; a rejected attempt's id may be on it), or
-- the row already carrying this very payment. The webhook decided from a read
-- taken before it wrote, and a second approved payment that settled the row
-- meanwhile owns it from then on: the write is refused (no row) instead of
-- replacing that payment's id.
UPDATE payments
SET status = $1,
    mp_payment_id = $2,
    mp_preference_id = $3,
    refund_amount = $4,
    status_detail = $5
WHERE id = $6
  AND ((status = 'unpaid' AND refund_amount = 0) OR mp_payment_id = $2)
RETURNING *;

-- name: RecordPaymentRejection :execrows
-- A rejected checkout attempt's MercadoPago id and decline reason, written
-- only onto a row that is still unpaid. The webhook read the row before it
-- asked MercadoPago; an approved payment that settled the same row meanwhile
-- owns it from then on, and a whole-row write from the stale read would revert
-- its status and repoint its refund at a payment that captured nothing.
UPDATE payments
SET mp_payment_id = sqlc.arg(mp_payment_id),
    status_detail = sqlc.arg(status_detail)
WHERE id = sqlc.arg(id)
  AND status = 'unpaid';

-- name: UpdatePaymentUnlessRefundPending :one
-- UpdatePayment for the webhook's record of a refund MercadoPago reports. The
-- webhook read the row before it asked MercadoPago, and a ClaimRefund that
-- committed meanwhile moved it to 'refund_pending': that claim owns the refund
-- from then on, so the write is refused (no row) instead of overwriting it.
UPDATE payments
SET status = $1,
    mp_payment_id = $2,
    mp_preference_id = $3,
    refund_amount = $4,
    status_detail = $5
WHERE id = $6
  AND status <> 'refund_pending'
RETURNING *;

-- name: MarkPaymentManuallyRefunded :one
-- Written by applyManualRefundRows (internal/payments/store/refunds.go),
-- once per unrefunded cash/transfer row a manual refund closes out, in the
-- same transaction as the booking's own move off refund_status 'partial'.
-- Separate from UpdatePayment (rather than widening it) so that
-- UpdatePayment's other callers — the automatic MercadoPago refund and
-- checkout paths — never have to pass manual_refund_amount/
-- manual_refunded_at at all.
--
-- refund_amount and manual_refund_amount are NOT the same number whenever
-- this row already carried a partial refund_amount: refund_amount is set to
-- the row's full amount + service_fee (the whole payment is now refunded,
-- same as an automatic full refund would record), while manual_refund_amount
-- is only the OWED difference the manual refund actually handed back — what
-- applyManualRefundRows already computed as `owed` before calling this query
-- — so the cashbox never double-subtracts money a previous refund already
-- took out of the till.
UPDATE payments
SET status = 'refunded',
    refund_amount = $1,
    manual_refund_amount = $2,
    manual_refunded_at = NOW()
WHERE id = $3
RETURNING *;

-- name: GetPaymentByIDForUpdate :one
-- Tenant-scoped: see the note on GetBookingByID in bookings.sql for why the
-- predicate is optional.
SELECT * FROM payments
WHERE id = $1
  AND (sqlc.narg('complex_id')::uuid IS NULL
       OR complex_id = sqlc.narg('complex_id')::uuid)
FOR UPDATE;

-- name: BlockedSlotOverlapsBooking :one
-- Whether an owner's blocked slot on the booking's court covers any of the hours
-- the booking is about to claim. It asks the same generated span column, and the
-- same half-open range, as the bookings exclusion constraint and SlotTaken, so a
-- block that starts exactly when the booking ends is not an overlap. The caller
-- holds the court-day lock, which the owner's block insert takes too.
SELECT EXISTS (
    SELECT 1 FROM blocked_slots
    WHERE court_id = sqlc.arg(court_id)
      AND span && tstzrange(
            booking_starts_at(sqlc.arg(booking_date)::date, sqlc.arg(start_time)::time),
            booking_starts_at(sqlc.arg(booking_date)::date, sqlc.arg(start_time)::time)
              + make_interval(mins => sqlc.arg(duration_minutes)::int),
            '[)')
) AS taken;

-- name: ListPaymentsByRegistrationID :many
-- Every payment row of an event registration, oldest first: the sign-up's
-- checkout row, and any other money that reached it.
SELECT * FROM payments
WHERE registration_id = $1
ORDER BY created_at ASC;
