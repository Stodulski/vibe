-- name: InsertProduct :one
INSERT INTO products (complex_id, name, category, price, tracks_stock, low_stock_threshold)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetProductByID :one
SELECT * FROM products
WHERE id = $1 AND complex_id = $2;

-- name: GetProductByIDForUpdate :one
-- Every write against a product (an update, a restock, an adjustment) locks
-- the row first, in the same transaction as the write, so stock_on_hand can
-- never be read-modify-written by two requests at once.
SELECT * FROM products
WHERE id = $1 AND complex_id = $2
FOR UPDATE;

-- name: ListProductsByComplex :many
-- Not paginated — a shop's catalog is bounded the same way a complex's court
-- list is (internal/courts/store's GetByComplex). has_active_filter is false
-- when the caller asked for every product regardless of active state; the OR's
-- left side short-circuits instead of comparing against a meaningless
-- active_filter value.
SELECT * FROM products
WHERE complex_id = $1
  AND (NOT sqlc.arg(has_active_filter)::bool OR active = sqlc.arg(active_filter)::bool)
ORDER BY name ASC;

-- name: UpdateProduct :one
-- expected_version is the caller's optimistic-concurrency precondition and is
-- optional (API-08): NULL is last-write-wins. Zero rows means either the
-- product is gone or somebody else wrote it first; products/store.Store.Update
-- tells those apart the same way courtstore.Store.Update does (a prior read
-- already proved existence, so a zero-row result here can only be the second
-- case).
UPDATE products
SET name = $1,
    category = $2,
    price = $3,
    low_stock_threshold = $4,
    active = $5,
    tracks_stock = $6
WHERE id = $7
  AND complex_id = $8
  AND (sqlc.narg('expected_version')::int IS NULL
       OR version = sqlc.narg('expected_version')::int)
RETURNING *;

-- name: UpdateProductStock :one
-- Applies a stock movement's signed quantity to the product's own running
-- total, in the same transaction as the movement insert
-- (internal/products/store.Store.Restock / .Adjust), under the FOR UPDATE lock
-- GetProductByIDForUpdate already took. stock_on_hand may go negative — see
-- that column's own comment in db/migrations/004_pos_catalog_stock.sql.
UPDATE products
SET stock_on_hand = stock_on_hand + sqlc.arg(delta)::int
WHERE id = $1 AND complex_id = $2
RETURNING *;

-- name: InsertStockMovement :one
INSERT INTO stock_movements (complex_id, product_id, kind, quantity, reason, note, cash_movement_id, sale_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListStockMovementsByProduct :many
-- Keyset pagination, same shape as cash_sessions' own ListCashSessionsByComplex.
SELECT * FROM stock_movements
WHERE product_id = $1 AND complex_id = $2
  AND (NOT sqlc.arg(has_cursor)::bool
       OR (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::int;
