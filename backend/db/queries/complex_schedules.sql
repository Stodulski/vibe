-- name: UpsertSchedule :one
INSERT INTO complex_schedules (complex_id, day, open_time, close_time, is_closed)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (complex_id, day)
DO UPDATE SET
    open_time = EXCLUDED.open_time,
    close_time = EXCLUDED.close_time,
    is_closed = EXCLUDED.is_closed
RETURNING *;

-- name: GetSchedulesByComplex :many
SELECT * FROM complex_schedules
WHERE complex_id = $1
ORDER BY day;
