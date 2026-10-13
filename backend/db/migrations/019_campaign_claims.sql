-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- ONE-OFF CAMPAIGN CLAIMS (2026-10). One row per owner per campaign: the durable
-- record that the "beneficios de Vibe" email was claimed for them.
--
-- The job queue dedups by key, but a finished job row is swept after its
-- retention window (7 days), which frees its key. A key alone would therefore
-- send the email again to every owner on the next pass after the sweep. This
-- table does not expire.
--
-- Remove this table together with the campaign once it has been sent.
-- ============================================================================
CREATE TABLE campaign_claims (
    campaign   TEXT        NOT NULL,
    user_id    UUID        NOT NULL,
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (campaign, user_id),
    CONSTRAINT campaign_claims_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- +goose Down

DROP TABLE campaign_claims;
