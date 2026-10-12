-- name: InsertUser :one
INSERT INTO users (email, password_hash, first_name, last_name, phone, role)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: UpdateUser :one
UPDATE users
SET email = $1,
    first_name = $2,
    last_name = $3,
    phone = $4,
    is_active = $5,
    email_verified = $6
WHERE id = $7
RETURNING *;

-- name: SetEmailVerified :exec
UPDATE users SET email_verified = true WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: DeleteUnverifiedStaleUsers :exec
DELETE FROM users
WHERE email_verified = false
AND created_at < NOW() - INTERVAL '7 days'
AND NOT EXISTS (SELECT 1 FROM complexes WHERE owner_id = users.id);

-- name: TouchOwnerLastActive :execrows
-- The WHERE clause is the throttle: an owner's activity is written at most once
-- an hour, however many requests they make. Callers ignore a zero row count.
UPDATE users
SET last_active_at = NOW()
WHERE id = sqlc.arg(id)::uuid
  AND role = 'owner'
  AND (last_active_at IS NULL OR last_active_at < NOW() - INTERVAL '1 hour');
