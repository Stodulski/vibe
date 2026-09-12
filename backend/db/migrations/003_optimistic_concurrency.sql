-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ==================== OPTIMISTIC CONCURRENCY ON THE ADMIN TABLES ====================
--
-- Two staff members editing the same venue is last-write-wins today. Each one
-- loads the row, changes the one field they came for, and PUTs every column
-- back; whoever saves second overwrites the other's change with a value read
-- before that change existed. Both get 200 and neither is told (API-08).
--
-- `version` is what lets the second one be told. A client that read version 3
-- sends it back with its write; if anything moved in between, the row is at 4
-- and the UPDATE matches nothing, which the stores turn into
-- data.ErrEditConflict and the handlers into 409. A client that sends no
-- version keeps the old behaviour exactly — the frontend adopts this on its own
-- schedule, and there are no other clients.
--
-- WHY THESE THREE. They are the tables an owner and their staff edit through a
-- form, from two browsers, over the same minute. Bookings are deliberately not
-- here: the race that matters there is two people claiming the same hours,
-- which a version counter cannot see and the EXCLUDE constraint already
-- refuses. 001_init.sql says the same thing about the counter it removed from
-- bookings — "exactly one of four writers ever checked it, so it was not a
-- lock" — and that argument is about bookings, not about a form.
--
-- WHY A TRIGGER RATHER THAN `version = version + 1` IN EACH UPDATE. Every
-- writer has to bump it, including the ones that do not check it: the
-- MercadoPago credential writes on complexes, the soft-delete, a backfill run
-- by hand. One of those forgetting is a version that stands still while the row
-- changes, which is worse than no version at all — it is a lock that reports
-- success. The trigger cannot be forgotten.
--
-- DEFAULT 1, NOT NULL: every existing row starts at 1 rather than at NULL, so
-- there is no "not versioned yet" state for a reader to handle. No expand and
-- contract, no backfill in batches: there is no production data (pre-launch),
-- and the column is filled by its default in one statement.

ALTER TABLE complexes    ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE courts       ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE court_prices ADD COLUMN version INTEGER NOT NULL DEFAULT 1;

-- +goose StatementBegin
CREATE FUNCTION trigger_bump_version() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
AS $$
BEGIN
    -- OLD.version, not NEW.version: a writer that sends the whole row back
    -- would otherwise be able to write the counter itself, which is the one
    -- thing a version counter must not allow.
    NEW.version = OLD.version + 1;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- BEFORE UPDATE, and after set_updated_at in name order so that both fire on
-- the same row without either depending on the other: they touch different
-- columns.
CREATE TRIGGER set_version BEFORE UPDATE ON complexes
    FOR EACH ROW EXECUTE FUNCTION trigger_bump_version();
CREATE TRIGGER set_version BEFORE UPDATE ON courts
    FOR EACH ROW EXECUTE FUNCTION trigger_bump_version();
CREATE TRIGGER set_version BEFORE UPDATE ON court_prices
    FOR EACH ROW EXECUTE FUNCTION trigger_bump_version();

-- Both views name their columns one by one (001_init.sql), so a new column on
-- the table behind them does not reach them until they are rebuilt. CREATE OR
-- REPLACE VIEW can append a column at the end of the list, which is what these
-- two are.
--
-- security_invoker is restated rather than relied on. It is what makes every
-- read through these views run as the caller and therefore inside the
-- tenant_isolation policy (001_init.sql calls it "not optional"), and a replace
-- that resets it would turn most of the read surface into a cross-tenant leak
-- silently. Restating it costs a line and cannot be wrong. The comments are
-- restated for the same reason: a replace is not required to keep them.
CREATE OR REPLACE VIEW active_complexes WITH (security_invoker = true) AS
SELECT id, owner_id, name, slug, address, city, province, country_code, currency,
       phone, email, logo_url, cover_url, deposit_percentage, cancellation_hours,
       latitude, longitude, is_active, mp_access_token, mp_refresh_token, mp_user_id,
       deleted_at, created_at, updated_at, amenities, mp_token_expires_at, version
FROM complexes
WHERE deleted_at IS NULL;

CREATE OR REPLACE VIEW active_courts WITH (security_invoker = true) AS
SELECT c.id, c.complex_id, c.name, c.sport, c.court_type, c.is_active,
       c.deleted_at, c.created_at, c.updated_at, c.description, c.version
FROM courts c
JOIN complexes cx ON cx.id = c.complex_id
WHERE c.deleted_at IS NULL
  AND cx.deleted_at IS NULL;

COMMENT ON VIEW active_complexes IS
    'Complexes that are not soft-deleted. Read this, not the table, unless the caller needs deleted rows (admin, audit, slug reservation).';
COMMENT ON VIEW active_courts IS
    'Courts that are not soft-deleted and whose complex is not soft-deleted either. Read this, not the table, unless the caller needs deleted rows (admin, audit, the court name on a historical booking).';

-- +goose Down

DROP TRIGGER IF EXISTS set_version ON court_prices;
DROP TRIGGER IF EXISTS set_version ON courts;
DROP TRIGGER IF EXISTS set_version ON complexes;
DROP FUNCTION IF EXISTS trigger_bump_version();

-- The Up rebuilt both views with `version` in their column list, so a bare
-- ALTER TABLE ... DROP COLUMN on complexes/courts fails on that view
-- dependency. Drop both views first, drop the columns, then recreate the
-- views exactly as 001_init.sql defines them (same column list without
-- version, same JOIN/WHERE, same security_invoker).
DROP VIEW active_courts;
DROP VIEW active_complexes;

ALTER TABLE court_prices DROP COLUMN IF EXISTS version;
ALTER TABLE courts       DROP COLUMN IF EXISTS version;
ALTER TABLE complexes    DROP COLUMN IF EXISTS version;

CREATE VIEW active_complexes WITH (security_invoker = true) AS
SELECT id, owner_id, name, slug, address, city, province, country_code, currency,
       phone, email, logo_url, cover_url, deposit_percentage, cancellation_hours,
       latitude, longitude, is_active, mp_access_token, mp_refresh_token, mp_user_id,
       deleted_at, created_at, updated_at, amenities, mp_token_expires_at
FROM complexes
WHERE deleted_at IS NULL;

CREATE VIEW active_courts WITH (security_invoker = true) AS
SELECT c.id, c.complex_id, c.name, c.sport, c.court_type, c.is_active,
       c.deleted_at, c.created_at, c.updated_at, c.description
FROM courts c
JOIN complexes cx ON cx.id = c.complex_id
WHERE c.deleted_at IS NULL
  AND cx.deleted_at IS NULL;

COMMENT ON VIEW active_complexes IS
    'Complexes that are not soft-deleted. Read this, not the table, unless the caller needs deleted rows (admin, audit, slug reservation).';
COMMENT ON VIEW active_courts IS
    'Courts that are not soft-deleted and whose complex is not soft-deleted either. Read this, not the table, unless the caller needs deleted rows (admin, audit, the court name on a historical booking).';
