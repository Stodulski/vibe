-- name: InsertSale :one
INSERT INTO sales (complex_id, session_id, method, total, cash_movement_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: InsertSaleItem :one
INSERT INTO sale_items (complex_id, sale_id, product_id, product_name, unit_price, quantity, line_total)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSaleByID :one
SELECT * FROM sales
WHERE id = $1 AND complex_id = $2;

-- name: GetSaleByIDForUpdate :one
-- Void locks the row first, in the same transaction as the write, so two
-- concurrent void attempts on the same sale cannot both read voided_at IS
-- NULL and both proceed — the same reasoning
-- GetCashSessionByIDForUpdate's own comment gives for cash_sessions.
SELECT * FROM sales
WHERE id = $1 AND complex_id = $2
FOR UPDATE;

-- name: VoidSale :one
-- WHERE voided_at IS NULL is a second, cheap guard against the same race
-- GetSaleByIDForUpdate's lock already closes — see that query's comment and
-- sales_forbid_update_after_void (db/migrations/005_pos_sales.sql) for the
-- third, database-level guard. Zero rows means either the sale is gone (it
-- never is, once inserted) or it was already voided; Store.Void tells those
-- apart the same way cashboxstore.Store.Close does for cash_sessions,
-- because the lock above already proved the row exists moments earlier.
UPDATE sales
SET voided_at = $1, voided_by = $2, void_cash_movement_id = $3
WHERE id = $4 AND complex_id = $5 AND voided_at IS NULL
RETURNING *;

-- name: ListSalesByComplex :many
-- Keyset pagination, same shape as cash_sessions' own ListCashSessionsByComplex.
-- has_session_filter is false when the caller asked for every session's
-- sales; the OR's left side short-circuits instead of comparing against a
-- zero-value uuid.
SELECT * FROM sales
WHERE complex_id = $1
  AND (NOT sqlc.arg(has_session_filter)::bool OR session_id = sqlc.arg(session_filter)::uuid)
  AND (NOT sqlc.arg(has_cursor)::bool
       OR (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::int;

-- name: ListSaleItemsBySale :many
-- One sale's own line items, in the order they were inserted — used by Get
-- and by Void (to know which tracked products to restore stock for is a
-- separate query, ListSaleStockMovements, below; this one is for the
-- response shape).
SELECT * FROM sale_items
WHERE sale_id = $1 AND complex_id = $2
ORDER BY created_at ASC, id ASC;

-- name: ListSaleItemsBySaleIDs :many
-- The batched counterpart to ListSaleItemsBySale, for a page of sales at
-- once (List): one query instead of one per sale on the page.
SELECT * FROM sale_items
WHERE sale_id = ANY(sqlc.arg(sale_ids)::uuid[]) AND complex_id = sqlc.arg(complex_id)::uuid
ORDER BY sale_id, created_at ASC, id ASC;

-- name: ListSaleStockMovements :many
-- Every stock-tracked line item's original debit for one sale — exactly the
-- set Store.Void must reverse with a 'sale_void' row and a stock_on_hand
-- update. Filtered to kind = 'sale' deliberately: a void's own 'sale_void'
-- rows are inserted, in the same call, against this exact sale_id, and must
-- never be read back by this query as something still needing reversal.
SELECT * FROM stock_movements
WHERE sale_id = $1 AND complex_id = $2 AND kind = 'sale'
ORDER BY product_id;

-- name: InsertSaleBookingPayment :one
INSERT INTO sale_booking_payments (complex_id, sale_id, booking_id, payment_id, amount)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListSaleBookingPaymentsBySaleIDs :many
-- The booking lines a page of sales collected, for the receipt. Joined to the
-- booking, court and client so the response can name them without a second
-- round trip per sale.
SELECT sbp.sale_id, sbp.payment_id, sbp.booking_id, sbp.amount,
       b.date AS booking_date,
       lower(b.span)::timestamptz AS starts_at,
       upper(b.span)::timestamptz AS ends_at,
       c.name AS court_name,
       cl.first_name AS client_first_name,
       cl.last_name AS client_last_name
FROM sale_booking_payments sbp
JOIN bookings b ON b.id = sbp.booking_id AND b.complex_id = sbp.complex_id
JOIN courts c ON c.id = b.court_id
JOIN clients cl ON cl.id = b.client_id
WHERE sbp.sale_id = ANY(sqlc.arg(sale_ids)::uuid[]) AND sbp.complex_id = sqlc.arg(complex_id)::uuid
ORDER BY sbp.created_at ASC, sbp.id ASC;

-- name: ListBookingPaymentLinesByPaymentIDs :many
-- The same booking lines as ListSaleBookingPaymentsBySaleIDs, read from the
-- payments rows directly: a booking-only checkout has no sale to hang them on.
SELECT p.id AS payment_id, p.booking_id, p.amount,
       b.date AS booking_date,
       lower(b.span)::timestamptz AS starts_at,
       upper(b.span)::timestamptz AS ends_at,
       c.name AS court_name,
       cl.first_name AS client_first_name,
       cl.last_name AS client_last_name
FROM payments p
JOIN bookings b ON b.id = p.booking_id AND b.complex_id = p.complex_id
JOIN courts c ON c.id = b.court_id
JOIN clients cl ON cl.id = b.client_id
WHERE p.id = ANY(sqlc.arg(payment_ids)::uuid[]) AND p.complex_id = sqlc.arg(complex_id)::uuid
ORDER BY p.created_at ASC, p.id ASC;

-- name: CountSaleBookingPayments :one
SELECT COUNT(*)::bigint FROM sale_booking_payments
WHERE sale_id = $1 AND complex_id = $2;

-- name: ListBookingBalancesDue :many
-- The sell page's list: one day's bookings of a complex that still owe money.
-- A balance is price less what the booking has collected (deposit_amount when
-- deposit_paid, the whole price when fully_paid). Only confirmed and completed
-- bookings qualify: cancelled, no-show and pending bookings are never sold
-- against here. A pending booking is confirmed from the booking panel, which
-- re-checks its slot, and that is the only path that may confirm one.
SELECT b.id, b.date AS booking_date,
       lower(b.span)::timestamptz AS starts_at,
       upper(b.span)::timestamptz AS ends_at,
       b.price,
       b.deposit_amount,
       b.collection_status,
       b.status,
       c.name AS court_name,
       cl.first_name AS client_first_name,
       cl.last_name AS client_last_name
FROM bookings b
JOIN courts c ON c.id = b.court_id
JOIN clients cl ON cl.id = b.client_id
WHERE b.complex_id = sqlc.arg(complex_id)::uuid
  AND b.date = sqlc.arg(booking_date)::date
  AND b.status IN ('confirmed', 'completed')
  AND b.collection_status <> 'fully_paid'
  AND b.price > CASE WHEN b.collection_status = 'deposit_paid' THEN b.deposit_amount ELSE 0 END
ORDER BY lower(b.span) ASC, c.name ASC, b.id ASC;
