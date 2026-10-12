-- name: UpsertRegistrationLead :exec
-- A repeat capture refreshes the names it has and keeps the ones it does not:
-- a later capture with an empty field never erases an earlier, fuller one.
INSERT INTO registration_leads (email, first_name, last_name, phone)
VALUES (sqlc.arg(email)::text, sqlc.arg(first_name)::text, sqlc.arg(last_name)::text, sqlc.arg(phone)::text)
ON CONFLICT (email) DO UPDATE SET
    first_name = COALESCE(NULLIF(EXCLUDED.first_name, ''), registration_leads.first_name),
    last_name  = COALESCE(NULLIF(EXCLUDED.last_name, ''), registration_leads.last_name),
    phone      = COALESCE(NULLIF(EXCLUDED.phone, ''), registration_leads.phone),
    updated_at = NOW();

-- name: ListFinishSignupNudgeCandidates :many
SELECT l.email, l.first_name
FROM registration_leads l
WHERE l.nudged_at IS NULL
  AND l.created_at <= NOW() - INTERVAL '2 hours'
  AND NOT EXISTS (SELECT 1 FROM users u WHERE lower(u.email::text) = l.email)
  AND NOT EXISTS (SELECT 1 FROM email_suppressions s WHERE s.email = l.email)
ORDER BY l.created_at
LIMIT sqlc.arg(max_rows)::int;

-- name: ClaimRegistrationLeadNudge :execrows
-- Sets nudged_at once. A lead already claimed matches no row.
UPDATE registration_leads
SET nudged_at = NOW()
WHERE email = sqlc.arg(email)::text
  AND nudged_at IS NULL;

-- name: ReleaseRegistrationLeadNudge :exec
UPDATE registration_leads
SET nudged_at = NULL
WHERE email = sqlc.arg(email)::text;

-- name: DeleteRegistrationLeadsOlderThan30Days :execrows
DELETE FROM registration_leads
WHERE created_at < NOW() - INTERVAL '30 days';
