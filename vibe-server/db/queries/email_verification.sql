-- name: InsertEmailVerificationToken :one
INSERT INTO email_verification_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetEmailVerificationToken :one
SELECT * FROM email_verification_tokens
WHERE token_hash = $1
  AND expires_at > NOW();

-- name: DeleteEmailVerificationTokensByUser :exec
DELETE FROM email_verification_tokens
WHERE user_id = $1;

-- name: GetLatestEmailVerificationTokenByUser :one
SELECT * FROM email_verification_tokens
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: DeleteExpiredEmailVerificationTokens :exec
DELETE FROM email_verification_tokens
WHERE expires_at <= NOW();

-- name: InsertVerificationTokenWithCooldown :one
-- Atomic: deletes stale tokens (>3 min), inserts new only if none within 3 min.
WITH deleted AS (
    DELETE FROM email_verification_tokens
    WHERE user_id = $1
    AND created_at <= NOW() - INTERVAL '3 minutes'
)
INSERT INTO email_verification_tokens (user_id, token_hash, expires_at)
SELECT $1, $2, $3
WHERE NOT EXISTS (
    SELECT 1 FROM email_verification_tokens
    WHERE user_id = $1
    AND created_at > NOW() - INTERVAL '3 minutes'
)
RETURNING *;
