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
