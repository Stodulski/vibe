-- name: InsertCourt :one
INSERT INTO courts (complex_id, name, sport, court_type, description)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetCourtByID :one
-- Reads active_courts: a court whose complex was soft-deleted is not a court
-- anyone may book, edit or price, and filtering only on the court's own
-- deleted_at missed exactly that case. See the soft-delete cascade in db/migrations/001_init.sql.
SELECT * FROM active_courts
WHERE id = $1;

-- name: GetCourtsByComplex :many
-- Ordered naturally rather than lexically: courts are almost always named
-- "Cancha 1", "Cancha 2", ... "Cancha 10", and a plain ORDER BY name sorts
-- "Cancha 10" before "Cancha 2". There is no explicit display-order column,
-- so the numeric part of the name (if any) is extracted and sorted as an
-- integer first, with non-numeric names falling back to plain name order
-- after every numbered court.
SELECT * FROM active_courts
WHERE complex_id = $1
ORDER BY NULLIF(regexp_replace(name, '\D', '', 'g'), '')::int NULLS LAST, name;

-- name: UpdateCourt :one
UPDATE courts
SET name = $1,
    sport = $2,
    court_type = $3,
    is_active = $4,
    description = $5
WHERE id = $6
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteCourt :exec
UPDATE courts
SET deleted_at = NOW(),
    is_active = false
WHERE id = $1
  AND deleted_at IS NULL;
