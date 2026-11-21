-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- MP REFRESH FAILURE NOTICES — the last MercadoPago refresh-failure episode
-- the owner of a complex was emailed about.
--
-- When MercadoPago refuses to renew a complex's connection (invalid_grant),
-- online payments stop until the owner reconnects, and the owner is told. The
-- 12-hour sweep and every checkout keep meeting the same refused refresh token
-- until then, so the email is sent once per episode: episode is a SHA-256
-- digest of the refused refresh token (never the token), and a reconnect or a
-- successful refresh stores a new token, so the next refusal is a new episode.
-- This lives in the database because the job queue's dedup key is freed a
-- week after its job is done, and an owner who never reconnects would be
-- emailed again every week.
--
-- Tenant-scoped like every table in 001_init.sql's ACCESS section, with the
-- same tenant_isolation + tenant_bypass pair. vibe_app's grants come from the
-- ALTER DEFAULT PRIVILEGES that 001_init.sql registered.
CREATE TABLE mp_refresh_failure_notices (
    complex_id  UUID PRIMARY KEY,
    episode     TEXT NOT NULL,
    notified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT mp_refresh_failure_notices_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    CONSTRAINT mp_refresh_failure_notices_episode_check CHECK (episode <> '')
);

ALTER TABLE mp_refresh_failure_notices ENABLE ROW LEVEL SECURITY;
ALTER TABLE mp_refresh_failure_notices FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON mp_refresh_failure_notices
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON mp_refresh_failure_notices
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- +goose Down

SET LOCAL lock_timeout = '3s';

DROP TABLE mp_refresh_failure_notices;
