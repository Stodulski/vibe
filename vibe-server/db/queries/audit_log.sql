-- name: InsertAuditLog :one
INSERT INTO audit_log (user_id, complex_id, action, entity_type, entity_id, old_value, new_value, ip_address)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetAuditLogByComplex :many
SELECT * FROM audit_log
WHERE complex_id = $1
  AND (created_at, id) < ($2, $3)
ORDER BY created_at DESC, id DESC
LIMIT $4;
