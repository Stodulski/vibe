-- name: ClaimOwnerNudge :execrows
-- Records that a nudge is going out. The primary key makes each kind once-only
-- per owner: a second claim inserts nothing.
INSERT INTO owner_nudges (user_id, kind)
VALUES (sqlc.arg(user_id)::uuid, sqlc.arg(kind)::text)
ON CONFLICT (user_id, kind) DO NOTHING;

-- name: ReleaseOwnerNudge :exec
-- Undoes a claim whose email never reached the queue, so the next pass retries it.
DELETE FROM owner_nudges
WHERE user_id = sqlc.arg(user_id)::uuid
  AND kind = sqlc.arg(kind)::text;

-- name: ListVerifyEmailNudgeCandidates :many
SELECT u.id, u.email::text AS email, u.first_name, ''::text AS complex_name
FROM users u
WHERE u.role = 'owner'
  AND u.is_active
  AND NOT u.email_verified
  AND u.created_at <= NOW() - INTERVAL '24 hours'
  AND u.created_at > NOW() - INTERVAL '7 days'
  AND NOT EXISTS (SELECT 1 FROM owner_nudges n WHERE n.user_id = u.id AND n.kind = 'verify_email')
  AND NOT EXISTS (SELECT 1 FROM email_suppressions s WHERE s.email = lower(u.email::text))
ORDER BY u.created_at
LIMIT sqlc.arg(max_rows)::int;

-- name: ListCreateComplexNudgeCandidates :many
SELECT u.id, u.email::text AS email, u.first_name, ''::text AS complex_name
FROM users u
WHERE u.role = 'owner'
  AND u.is_active
  AND u.email_verified
  AND u.created_at <= NOW() - INTERVAL '24 hours'
  AND NOT EXISTS (SELECT 1 FROM complexes c WHERE c.owner_id = u.id AND c.deleted_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM owner_nudges n WHERE n.user_id = u.id AND n.kind = 'create_complex')
  AND NOT EXISTS (SELECT 1 FROM email_suppressions s WHERE s.email = lower(u.email::text))
ORDER BY u.created_at
LIMIT sqlc.arg(max_rows)::int;

-- name: ListAddCourtsNudgeCandidates :many
SELECT u.id, u.email::text AS email, u.first_name, c.name AS complex_name
FROM complexes c
JOIN users u ON u.id = c.owner_id
WHERE c.deleted_at IS NULL
  AND u.role = 'owner'
  AND u.is_active
  AND c.created_at <= NOW() - INTERVAL '48 hours'
  AND NOT EXISTS (SELECT 1 FROM courts k WHERE k.complex_id = c.id AND k.deleted_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM owner_nudges n WHERE n.user_id = u.id AND n.kind = 'add_courts')
  AND NOT EXISTS (SELECT 1 FROM email_suppressions s WHERE s.email = lower(u.email::text))
ORDER BY c.created_at
LIMIT sqlc.arg(max_rows)::int;

-- name: ListConnectMPNudgeCandidates :many
-- A complex with a live court and no MercadoPago account, a day after it was
-- created: it takes bookings but cannot take the deposit online.
SELECT u.id, u.email::text AS email, u.first_name, c.name AS complex_name
FROM complexes c
JOIN users u ON u.id = c.owner_id
WHERE c.deleted_at IS NULL
  AND u.role = 'owner'
  AND u.is_active
  AND c.mp_access_token IS NULL
  AND c.created_at <= NOW() - INTERVAL '24 hours'
  AND EXISTS (SELECT 1 FROM courts k WHERE k.complex_id = c.id AND k.deleted_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM owner_nudges n WHERE n.user_id = u.id AND n.kind = 'connect_mp')
  AND NOT EXISTS (SELECT 1 FROM email_suppressions s WHERE s.email = lower(u.email::text))
ORDER BY c.created_at
LIMIT sqlc.arg(max_rows)::int;

-- name: ListInactiveNudgeCandidates :many
-- An owner counts as active from the last request they made, or from account
-- creation when they have never made one. A booking from any source (the
-- public site, the owner's panel, the WhatsApp flow) keeps the complex alive.
SELECT u.id, u.email::text AS email, u.first_name, c.name AS complex_name
FROM complexes c
JOIN users u ON u.id = c.owner_id
WHERE c.deleted_at IS NULL
  AND u.role = 'owner'
  AND u.is_active
  AND COALESCE(u.last_active_at, u.created_at) < NOW() - INTERVAL '3 days'
  AND NOT EXISTS (SELECT 1 FROM bookings b WHERE b.complex_id = c.id AND b.created_at > NOW() - INTERVAL '3 days')
  AND NOT EXISTS (SELECT 1 FROM owner_nudges n WHERE n.user_id = u.id AND n.kind = 'inactive')
  AND NOT EXISTS (SELECT 1 FROM email_suppressions s WHERE s.email = lower(u.email::text))
ORDER BY COALESCE(u.last_active_at, u.created_at)
LIMIT sqlc.arg(max_rows)::int;
