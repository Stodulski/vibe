-- name: UpsertInstagramConnection :exec
-- Connecting again (another account, or the same one after a reconnect)
-- replaces the row and clears needs_reconnect.
INSERT INTO instagram_connections (
    complex_id, instagram_user_id, username, account_type, access_token,
    expires_at, connected_at, refreshed_at, needs_reconnect
) VALUES (
    sqlc.arg(complex_id)::uuid, sqlc.arg(instagram_user_id)::text, sqlc.arg(username)::text,
    sqlc.arg(account_type)::text, sqlc.arg(access_token)::text, sqlc.arg(expires_at)::timestamptz,
    sqlc.arg(connected_at)::timestamptz, sqlc.arg(connected_at)::timestamptz, false
)
ON CONFLICT (complex_id) DO UPDATE SET
    instagram_user_id = EXCLUDED.instagram_user_id,
    username          = EXCLUDED.username,
    account_type      = EXCLUDED.account_type,
    access_token      = EXCLUDED.access_token,
    expires_at        = EXCLUDED.expires_at,
    connected_at      = EXCLUDED.connected_at,
    refreshed_at      = EXCLUDED.refreshed_at,
    needs_reconnect   = false;

-- name: GetInstagramConnection :one
-- The status read: everything but the token.
SELECT complex_id, instagram_user_id, username, account_type, expires_at,
       connected_at, refreshed_at, needs_reconnect
FROM instagram_connections
WHERE complex_id = sqlc.arg(complex_id)::uuid;

-- name: DeleteInstagramConnection :execrows
DELETE FROM instagram_connections WHERE complex_id = sqlc.arg(complex_id)::uuid;

-- name: ListInstagramConnectionsDueForRefresh :many
-- The live connections whose token expires before refresh_before and was last
-- refreshed before refreshed_before (Meta refuses a token younger than 24 h),
-- and that have not expired yet (an expired token cannot be refreshed).
SELECT complex_id, access_token, expires_at
FROM instagram_connections
WHERE NOT needs_reconnect
  AND expires_at > sqlc.arg(now)::timestamptz
  AND expires_at < sqlc.arg(refresh_before)::timestamptz
  AND refreshed_at < sqlc.arg(refreshed_before)::timestamptz
ORDER BY expires_at
LIMIT sqlc.arg(row_limit)::int;

-- name: UpdateInstagramToken :execrows
-- Stores a refreshed token. The old expiry is part of the match so a refresh
-- that raced a reconnect does not overwrite the newer token.
UPDATE instagram_connections
SET access_token = sqlc.arg(access_token)::text,
    expires_at   = sqlc.arg(expires_at)::timestamptz,
    refreshed_at = sqlc.arg(refreshed_at)::timestamptz
WHERE complex_id = sqlc.arg(complex_id)::uuid
  AND expires_at = sqlc.arg(old_expires_at)::timestamptz
  AND NOT needs_reconnect;

-- name: MarkExpiredInstagramConnections :execrows
-- A token past its expiry cannot be refreshed: the owner has to reconnect.
UPDATE instagram_connections
SET needs_reconnect = true
WHERE NOT needs_reconnect
  AND expires_at <= sqlc.arg(now)::timestamptz;
