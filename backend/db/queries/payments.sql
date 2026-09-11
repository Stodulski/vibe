-- name: InsertPayment :one
INSERT INTO payments (booking_id, complex_id, amount, method, status, service_fee, mp_payment_id, mp_preference_id, status_detail)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
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

-- name: GetPaymentByIDForUpdate :one
SELECT * FROM payments
WHERE id = $1
FOR UPDATE;
