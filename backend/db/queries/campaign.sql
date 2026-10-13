-- One-off campaign (2026-10). Remove after it has been sent.

-- name: ClaimCampaign :execrows
-- Records that an owner is being sent the campaign. The primary key makes it
-- once-only per owner: a second claim inserts nothing.
INSERT INTO campaign_claims (campaign, user_id)
VALUES (sqlc.arg(campaign)::text, sqlc.arg(user_id)::uuid)
ON CONFLICT (campaign, user_id) DO NOTHING;

-- name: ListCampaignRecipients :many
-- The owners the campaign has not reached yet, oldest account first, at most
-- max_rows. Each carries the segment that picks the email's last line, and the
-- slug of their live complex (empty when they have none). complexes_owner_id_key
-- allows one live complex per owner, so the join yields at most one row per owner.
-- "Connected" means the live complex has its mp_user_id set.
SELECT u.id,
       u.email::text AS email,
       u.first_name,
       COALESCE(c.slug, '')::text AS slug,
       (CASE
            WHEN c.id IS NULL THEN 'create_complex'
            WHEN c.mp_user_id IS NULL THEN 'connect_mp'
            ELSE 'share_page'
        END)::text AS segment
FROM users u
LEFT JOIN complexes c
       ON c.owner_id = u.id
      AND c.deleted_at IS NULL
WHERE u.role = 'owner'
  AND u.is_active
  AND u.email_verified
  AND NOT EXISTS (SELECT 1 FROM email_suppressions s WHERE s.email = lower(u.email::text))
  AND NOT EXISTS (
      SELECT 1 FROM campaign_claims k
      WHERE k.campaign = sqlc.arg(campaign)::text AND k.user_id = u.id
  )
ORDER BY u.created_at, u.id
LIMIT sqlc.arg(max_rows)::int;
