-- name: InsertCourt :one
-- Every court query names its columns instead of using *: sqlc expands * into
-- the column list at generation time, so a * silently binds the code to
-- whatever columns existed then (035 dropped courts.description this way).
INSERT INTO courts (complex_id, name, sport, court_type, has_lighting, surface)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, complex_id, name, sport, court_type, is_active, deleted_at, created_at, updated_at, version, has_lighting, surface;

-- name: GetCourtByID :one
-- Reads active_courts: a court whose complex was soft-deleted is not a court
-- anyone may book, edit or price, and filtering only on the court's own
-- deleted_at missed exactly that case. See the soft-delete cascade in db/migrations/001_init.sql.
-- Tenant-scoped: see the note on GetBookingByID in bookings.sql for why the
-- predicate is optional.
SELECT id, complex_id, name, sport, court_type, is_active, deleted_at, created_at, updated_at, version, has_lighting, surface
FROM active_courts
WHERE id = $1
  AND (sqlc.narg('complex_id')::uuid IS NULL
       OR complex_id = sqlc.narg('complex_id')::uuid);

-- name: GetCourtsByComplex :many
-- Ordered naturally rather than lexically: courts are almost always named
-- "Cancha 1", "Cancha 2", ... "Cancha 10", and a plain ORDER BY name sorts
-- "Cancha 10" before "Cancha 2". There is no explicit display-order column,
-- so the numeric part of the name (if any) is extracted and sorted as an
-- integer first, with non-numeric names falling back to plain name order
-- after every numbered court.
SELECT id, complex_id, name, sport, court_type, is_active, deleted_at, created_at, updated_at, version, has_lighting, surface
FROM active_courts
WHERE complex_id = $1
ORDER BY NULLIF(regexp_replace(name, '\D', '', 'g'), '')::int NULLS LAST, name;

-- name: UpdateCourt :one
-- expected_version is the caller's optimistic-concurrency precondition and is
-- optional (API-08): NULL is the last-write-wins this endpoint had before
-- versions existed. Zero rows means either the court is gone or somebody else
-- wrote it first; courtstore.Store.Update tells those apart by re-reading.
UPDATE courts
SET name = $1,
    sport = $2,
    court_type = $3,
    is_active = $4,
    has_lighting = $5,
    surface = $6
WHERE id = $7
  AND deleted_at IS NULL
  AND (sqlc.narg('expected_version')::int IS NULL
       OR version = sqlc.narg('expected_version')::int)
RETURNING id, complex_id, name, sport, court_type, is_active, deleted_at, created_at, updated_at, version, has_lighting, surface;

-- name: BumpCourtVersion :one
-- Bumps the court's version without changing any of its own columns, so that
-- replacing its price bands moves a counter a client can hold.
--
-- The price rows are replaced wholesale (courtstore.Store.ReplacePrices deletes
-- them and inserts the new set), so a version on an individual band is gone the
-- moment the set is written and cannot be anybody's precondition. The court is
-- the thing that persists, so the court's version is the price set's version.
--
-- The UPDATE names deleted_at as its assignment precisely because it changes
-- nothing: the trigger on this table fires on any UPDATE, which is the whole
-- effect wanted here.
UPDATE courts
SET deleted_at = deleted_at
WHERE id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('expected_version')::int IS NULL
       OR version = sqlc.narg('expected_version')::int)
RETURNING version;

-- name: SoftDeleteCourt :exec
UPDATE courts
SET deleted_at = NOW(),
    is_active = false
WHERE id = $1
  AND deleted_at IS NULL;
