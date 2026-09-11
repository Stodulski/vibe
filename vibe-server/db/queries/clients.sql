-- name: InsertClient :one
INSERT INTO clients (complex_id, first_name, last_name, phone, email, notes)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetClientByPhone :one
SELECT
  c.id, c.complex_id, c.first_name, c.last_name, c.phone, c.email, c.notes,
  c.is_blocked, c.no_shows, c.created_at, c.updated_at,
  COALESCE((
    SELECT COUNT(*)::int FROM bookings b
    WHERE b.client_id = c.id AND b.status IN ('confirmed', 'completed', 'no_show')
  ), 0)::int AS total_bookings
FROM clients c
WHERE c.complex_id = $1
  AND c.phone = $2;

-- name: GetClientByID :one
SELECT
  c.id, c.complex_id, c.first_name, c.last_name, c.phone, c.email, c.notes,
  c.is_blocked, c.no_shows, c.created_at, c.updated_at,
  COALESCE((
    SELECT COUNT(*)::int FROM bookings b
    WHERE b.client_id = c.id AND b.status IN ('confirmed', 'completed', 'no_show')
  ), 0)::int AS total_bookings
FROM clients c
WHERE c.id = $1;

-- name: GetClientsByComplex :many
SELECT * FROM clients
WHERE complex_id = $1
  AND (created_at, id) > ($2, $3)
ORDER BY created_at ASC, id ASC
LIMIT $4;

-- name: UpdateClient :one
UPDATE clients
SET first_name = $1,
    last_name = $2,
    phone = $3,
    email = $4,
    notes = $5,
    is_blocked = $6
WHERE id = $7
RETURNING *;

-- name: IncrementNoShowCount :exec
UPDATE clients
SET no_shows = no_shows + 1
WHERE id = $1;
