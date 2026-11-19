-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- INSTAGRAM CONNECTIONS — a complex's own Instagram professional account,
-- connected through Business Login for Instagram so Vibe can publish stories
-- for it. One row per complex; disconnecting deletes the row.
--
-- access_token is the long-lived Instagram user token (60 days), SEALED with
-- the credential keyring (internal/crypto) before it is written, bound by its
-- AAD to this complex and this column. It is never stored in plain text and
-- never returned by the API.
--
-- needs_reconnect is set when the token expired or was found unusable: the
-- owner has to go through the consent page again. A new connection clears it.
--
-- Tenant-scoped like every table in 001_init.sql's ACCESS section, with the
-- same tenant_isolation + tenant_bypass pair. vibe_app's grants come from the
-- ALTER DEFAULT PRIVILEGES that 001_init.sql registered.
CREATE TABLE instagram_connections (
    id                UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id        UUID NOT NULL,
    -- The Instagram professional account id (<IG_ID>), what publishing calls name.
    instagram_user_id TEXT NOT NULL,
    username          TEXT NOT NULL,
    -- As Meta reports it: 'BUSINESS' or 'MEDIA_CREATOR' today. Not constrained,
    -- so a new value from Meta cannot break a connection.
    account_type      TEXT NOT NULL DEFAULT '',
    access_token      TEXT NOT NULL,
    expires_at        TIMESTAMPTZ NOT NULL,
    connected_at      TIMESTAMPTZ NOT NULL,
    refreshed_at      TIMESTAMPTZ NOT NULL,
    needs_reconnect   BOOLEAN NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT instagram_connections_complex_id_key UNIQUE (complex_id),
    CONSTRAINT instagram_connections_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    CONSTRAINT instagram_connections_token_present CHECK (access_token <> '')
);

-- The daily refresh reads the live connections ordered by expiry.
CREATE INDEX idx_instagram_connections_expires_at
    ON instagram_connections (expires_at) WHERE NOT needs_reconnect;

CREATE TRIGGER set_updated_at BEFORE UPDATE ON instagram_connections
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

ALTER TABLE instagram_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE instagram_connections FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON instagram_connections
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON instagram_connections
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- +goose Down

SET LOCAL lock_timeout = '3s';

DROP TABLE instagram_connections;
