-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- COMPLEX INSTAGRAM — the club's optional Instagram account
--
-- Stored as the bare handle: no '@', no URL. The handler reduces whatever the
-- owner pastes ("@club", "instagram.com/club", a full profile link) to that
-- handle and turns a blank value into NULL, so '' is never stored. The CHECK
-- below is the floor under that rule: Instagram handles are 1 to 30 letters,
-- digits, dots and underscores.
--
-- active_complexes freezes its column list when it is created, so it is
-- replaced here with the new column last, in the same order as the table (see
-- 009_complex_description.sql). internal/complexes/store's complexRowFromActive
-- conversion depends on that order and stops compiling if it drifts.
-- ============================================================================

ALTER TABLE complexes ADD COLUMN instagram TEXT;

ALTER TABLE complexes ADD CONSTRAINT complexes_instagram_handle CHECK (
    instagram IS NULL OR instagram ~ '^[A-Za-z0-9._]{1,30}$'
);

CREATE OR REPLACE VIEW active_complexes AS
SELECT id, owner_id, name, slug, address, city, province, country_code, currency,
       phone, email, logo_url, cover_url, deposit_percentage, cancellation_hours,
       latitude, longitude, is_active, mp_access_token, mp_refresh_token, mp_user_id,
       deleted_at, created_at, updated_at, amenities, mp_token_expires_at, version,
       description, instagram
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
-- rebuilt as 009 left it before the column goes.
DROP VIEW active_complexes;

CREATE VIEW active_complexes AS
SELECT id, owner_id, name, slug, address, city, province, country_code, currency,
       phone, email, logo_url, cover_url, deposit_percentage, cancellation_hours,
       latitude, longitude, is_active, mp_access_token, mp_refresh_token, mp_user_id,
       deleted_at, created_at, updated_at, amenities, mp_token_expires_at, version,
       description
FROM complexes
WHERE deleted_at IS NULL;

COMMENT ON VIEW active_complexes IS
    'Complexes that are not soft-deleted. Read this, not the table, unless the caller needs deleted rows (admin, audit, slug reservation).';

ALTER VIEW active_complexes SET (security_invoker = true);

ALTER TABLE complexes DROP CONSTRAINT complexes_instagram_handle;
ALTER TABLE complexes DROP COLUMN instagram;
