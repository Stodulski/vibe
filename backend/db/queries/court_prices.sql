-- name: InsertCourtPrice :one
INSERT INTO court_prices (court_id, price, day_type, time_from, time_to)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetCourtPrices :many
SELECT * FROM court_prices
WHERE court_id = $1
ORDER BY day_type, time_from;

-- name: UpdateCourtPrice :one
UPDATE court_prices
SET price = $1,
    day_type = $2,
    time_from = $3,
    time_to = $4
WHERE id = $5
RETURNING *;

-- name: DeleteCourtPrice :exec
DELETE FROM court_prices
WHERE id = $1;
