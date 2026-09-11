-- name: InsertUserIdentity :exec
-- Idempotent: a repeated Google sign-in re-links the same (provider,
-- subject) or (user_id, provider) pair and this is a silent no-op — see the
-- unique constraints in db/migrations/002_user_identities.sql.
INSERT INTO user_identities (user_id, provider, subject, email)
VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: GetUserIdentityByProviderSubject :one
SELECT * FROM user_identities
WHERE provider = $1 AND subject = $2;

-- name: GetUserIdentitiesByUser :many
SELECT * FROM user_identities
WHERE user_id = $1
ORDER BY created_at;
