-- name: InsertRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens
WHERE token_hash = $1
  AND expires_at > NOW()
  AND used_at IS NULL;

-- name: GetUsedRefreshTokenByHash :one
SELECT * FROM refresh_tokens
WHERE token_hash = $1
  AND used_at IS NOT NULL;

-- name: MarkRefreshTokenUsed :exec
UPDATE refresh_tokens
SET used_at = NOW()
WHERE token_hash = $1;

-- name: DeleteRefreshToken :exec
DELETE FROM refresh_tokens
WHERE id = $1;

-- name: DeleteAllRefreshTokensByUser :exec
DELETE FROM refresh_tokens
WHERE user_id = $1;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens
WHERE expires_at <= NOW() OR (used_at IS NOT NULL AND used_at < NOW() - INTERVAL '7 days');
