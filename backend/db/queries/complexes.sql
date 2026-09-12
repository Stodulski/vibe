-- name: InsertComplex :one
INSERT INTO complexes (
    owner_id, name, slug,
    address, city, province, country_code, currency,
    phone, email, logo_url, cover_url,
    deposit_percentage, cancellation_hours,
    latitude, longitude, amenities
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING *;

-- name: GetComplexByID :one
-- Reads active_complexes, not the table: this is the lookup behind the owner
-- guard, the public booking link and the payment paths, and none of them may
-- ever resolve a soft-deleted venue. See the soft-delete cascade in db/migrations/001_init.sql.
SELECT * FROM active_complexes
WHERE id = $1;

-- name: GetComplexBySlug :one
-- Reads active_complexes: the slug is the public URL, so this is the storefront.
-- A deleted venue keeps its slug (the name stays reserved) and must answer 404,
-- not serve a page. See the soft-delete cascade in db/migrations/001_init.sql.
SELECT * FROM active_complexes
WHERE slug = $1;

-- name: GetComplexesByOwner :many
--
-- Carries a court count, because the list is where an owner decides which club
-- to work on and a club with no courts can take no booking at all — it looked
-- identical to a finished one.
--
-- A COUNT, not a "complete" flag: complete is a policy that moves every time a
-- step is added to onboarding, and a flag would put that policy in the server
-- where the client cannot see it change. The server says how many courts there
-- are; what that means is the caller's business.
--
-- Soft-deleted courts do not count. Deactivated ones do: a venue that switched
-- a court off still has courts, and that is a different question. Neither do
-- the courts of a soft-deleted venue, but that is now true by construction:
-- active_courts cannot contain one.
--
-- A scalar subquery rather than a LEFT JOIN and a GROUP BY. Reading from the
-- views means `GROUP BY c.id` no longer covers the other columns: PostgreSQL's
-- functional-dependency shortcut applies to a base table's primary key, and a
-- view has none, so the join form needs all 26 columns spelled into the GROUP
-- BY. The subquery says the same thing in one line and cannot drift from the
-- column list.
SELECT c.*, (
    SELECT COUNT(*) FROM active_courts ct WHERE ct.complex_id = c.id
) AS court_count
FROM active_complexes c
WHERE c.owner_id = $1
ORDER BY c.created_at DESC;

-- name: UpdateComplex :one
-- H-14: an unconditional UPDATE let two concurrent edits race — each loads the
-- row, applies its own fields, and writes every column back, so whichever
-- request commits second silently overwrites the first's change with a value
-- it read before that first write existed. Both callers got 200 and neither
-- was told.
--
-- `updated_at = $17` is the optimistic-concurrency precondition: the caller
-- must echo back the `updated_at` it read when it loaded the row. If another
-- write landed in between, that timestamp has already moved and this UPDATE
-- matches zero rows instead of clobbering the other request's change.
-- complexstore.Store.Update turns zero rows into ErrRecordNotFound, which the
-- handler already maps to a 409 edit-conflict response (it previously existed
-- only to cover a row deleted out from under the request) — so a refused
-- update now reaches the caller as a conflict to retry, not as data loss.
UPDATE complexes
SET name = $1,
    slug = $2,
    address = $3,
    city = $4,
    province = $5,
    phone = $6,
    email = $7,
    logo_url = $8,
    cover_url = $9,
    deposit_percentage = $10,
    cancellation_hours = $11,
    is_active = $12,
    latitude = $13,
    longitude = $14,
    amenities = $15
--
-- `expected_version` is the CALLER's precondition, and it is optional (API-08).
-- NULL means "whatever it is now", which is the last-write-wins this endpoint
-- had before versions existed and is what a client that sends no If-Match
-- still gets. A client that does send one and finds the row has moved matches
-- zero rows, the same conflict the timestamp above produces.
WHERE id = $16
  AND deleted_at IS NULL
  AND updated_at = $17
  AND (sqlc.narg('expected_version')::int IS NULL
       OR version = sqlc.narg('expected_version')::int)
RETURNING *;

-- name: SoftDeleteComplex :execrows
-- :execrows, so the caller can tell "deleted it" from "it was already deleted"
-- without a separate SELECT. complexstore.Store.SoftDeleteCascade is the only caller
-- and runs this inside the transaction that also counts the courts the
-- soft-delete cascade trigger closes.
UPDATE complexes
SET deleted_at = NOW(),
    is_active = false
WHERE id = $1
  AND deleted_at IS NULL;

-- name: ListComplexesNeedingMPRefresh :many
-- Connected complexes whose OAuth token has no known expiry (never recorded
-- one, e.g. rows connected before this column existed) or expires within 30
-- days. cronRefreshMPTokens used to refresh every connected complex on every
-- 12h tick regardless of how recently its token was issued; this narrows
-- that to the complexes that actually need it.
SELECT * FROM active_complexes
WHERE mp_refresh_token IS NOT NULL
  AND is_active = true
  AND (mp_token_expires_at IS NULL OR mp_token_expires_at < now() + interval '30 days');
