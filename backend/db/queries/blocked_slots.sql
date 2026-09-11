-- name: GetBlockedSlots :many
SELECT * FROM blocked_slots
WHERE court_id = $1
  AND date BETWEEN $2 AND $3
ORDER BY date, start_time;

-- name: GetBlockedSlotByID :one
SELECT * FROM blocked_slots
WHERE id = $1;

-- name: DeleteBlockedSlot :exec
DELETE FROM blocked_slots
WHERE id = $1;
