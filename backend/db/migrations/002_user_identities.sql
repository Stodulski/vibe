-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ==================== EXTERNAL IDENTITY LINKS ====================
--
-- One row per external identity linked to a local account — today, one
-- Google account behind Sign in with Google. Not tenant-scoped: like users
-- itself, an identity link belongs to the platform account, not to any one
-- complex, so it carries no row-level security policy — the same posture as
-- users, refresh_tokens, email_verification_tokens and
-- password_reset_tokens in 001_init.sql.
CREATE TABLE user_identities (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL,
    provider   TEXT NOT NULL CHECK (provider IN ('google')),
    subject    TEXT NOT NULL,
    email      CITEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_identities_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    -- One local account per external identity: the same Google account
    -- cannot silently take over a second local user.
    CONSTRAINT user_identities_provider_subject_key UNIQUE (provider, subject),
    -- One linked identity per provider per account: a user cannot end up
    -- signed into two different Google accounts under one local user.
    CONSTRAINT user_identities_user_id_provider_key UNIQUE (user_id, provider)
);

CREATE INDEX idx_user_identities_user_id ON user_identities (user_id);

-- The same DML the users table gets in 001_init.sql: SELECT/INSERT/UPDATE/
-- DELETE for vibe_app, nothing else. ALTER DEFAULT PRIVILEGES in
-- 001_init.sql already covers a table created by this migration's runner
-- (vibe_migrator, or the superuser applying it before the cutover) — this
-- grant is redundant with that mechanism and kept anyway so it is visible
-- beside the object it protects rather than a migration away.
GRANT SELECT, INSERT, UPDATE, DELETE ON user_identities TO vibe_app;

-- +goose Down

DROP TABLE IF EXISTS user_identities;
