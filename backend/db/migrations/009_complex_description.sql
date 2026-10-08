-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- COMPLEX DESCRIPTION — an optional, owner-written text for the storefront
--
-- The owner describes the venue in plain text, shown on the public page. NULL
-- means no description: the handler turns a blank value into NULL, so '' is
-- never stored (the CHECK below is the floor under that rule). The limit is
-- 600 characters, counted by char_length (characters, not UTF-8 bytes).
--
-- active_complexes freezes its column list when it is created (001_init.sql),
-- so it is replaced here with the new column last, in the same order as the
-- table. internal/complexes/store's complexRowFromActive conversion depends on
-- that order and stops compiling if it drifts.
-- ============================================================================

ALTER TABLE complexes ADD COLUMN description TEXT;

ALTER TABLE complexes ADD CONSTRAINT complexes_description_length CHECK (
    description IS NULL OR char_length(description) BETWEEN 1 AND 600
);

CREATE OR REPLACE VIEW active_complexes AS
SELECT id, owner_id, name, slug, address, city, province, country_code, currency,
       phone, email, logo_url, cover_url, deposit_percentage, cancellation_hours,
       latitude, longitude, is_active, mp_access_token, mp_refresh_token, mp_user_id,
       deleted_at, created_at, updated_at, amenities, mp_token_expires_at, version,
       description
FROM complexes
WHERE deleted_at IS NULL;

-- Restated, not inherited: CREATE OR REPLACE keeps the comment in PostgreSQL,
-- but sqlc copies it into internal/db from the migration text it reads.
COMMENT ON VIEW active_complexes IS
    'Complexes that are not soft-deleted. Read this, not the table, unless the caller needs deleted rows (admin, audit, slug reservation).';

ALTER VIEW active_complexes SET (security_invoker = true);

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- CREATE OR REPLACE VIEW cannot remove a column, so the view is dropped and
-- rebuilt without description before the column goes. Nothing else depends on
-- active_complexes. Its grants come back from the ALTER DEFAULT PRIVILEGES the
-- ACCESS section set for the migrator, which apply to any view it creates.
DROP VIEW active_complexes;

CREATE VIEW active_complexes AS
SELECT id, owner_id, name, slug, address, city, province, country_code, currency,
       phone, email, logo_url, cover_url, deposit_percentage, cancellation_hours,
       latitude, longitude, is_active, mp_access_token, mp_refresh_token, mp_user_id,
       deleted_at, created_at, updated_at, amenities, mp_token_expires_at, version
FROM complexes
WHERE deleted_at IS NULL;

COMMENT ON VIEW active_complexes IS
    'Complexes that are not soft-deleted. Read this, not the table, unless the caller needs deleted rows (admin, audit, slug reservation).';

ALTER VIEW active_complexes SET (security_invoker = true);

ALTER TABLE complexes DROP CONSTRAINT complexes_description_length;
ALTER TABLE complexes DROP COLUMN description;
