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
