-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- COURT SURFACE AND LIGHTING, AND SEVEN NEW AMENITIES
-- ============================================================================
-- Purely additive: two new columns on courts (one defaulted, one nullable) and
-- a wider amenity vocabulary. No existing row is rewritten, so it is safe on
-- production data.
--
-- surface is the playing floor: NULL means the owner has not said. The values
-- are the eight the product offers; which of them a sport may take lives in Go
-- (internal/courts/surfaces.go), because the handler must answer a mismatch
-- with a field error and one list is easier to keep right than two.
ALTER TABLE courts ADD COLUMN has_lighting BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE courts ADD COLUMN surface TEXT;
ALTER TABLE courts ADD CONSTRAINT courts_surface_known CHECK (
    surface IS NULL OR surface IN (
        'synthetic_grass',
        'natural_grass',
        'clay',
        'concrete',
        'parquet',
        'synthetic_floor',
        'sand',
        'rubber'
    )
);

-- The view lists its columns, so the new ones are added at the end, in the same
-- order as the table: courtRowFromActive converts one struct into the other.
CREATE OR REPLACE VIEW active_courts AS
SELECT c.id, c.complex_id, c.name, c.sport, c.court_type, c.is_active,
       c.deleted_at, c.created_at, c.updated_at, c.description, c.version,
       c.has_lighting, c.surface
FROM courts c
JOIN complexes cx ON cx.id = c.complex_id
WHERE c.deleted_at IS NULL
  AND cx.deleted_at IS NULL;

-- Restated, not inherited: sqlc copies the comment from the migration text.
COMMENT ON VIEW active_courts IS
    'Courts that are not soft-deleted and whose complex is not soft-deleted either. Read this, not the table, unless the caller needs deleted rows (admin, audit, the court name on a historical booking).';

ALTER VIEW active_courts SET (security_invoker = true);

-- The vocabulary only grows: every id of 001 stays, seven are added.
ALTER TABLE complexes DROP CONSTRAINT complexes_amenities_known;
ALTER TABLE complexes ADD CONSTRAINT complexes_amenities_known
    CHECK (amenities <@ ARRAY[
        'parking',
        'changing_rooms',
        'showers',
        'bar',
        'racket_rental',
        'pro_shop',
        'wifi',
        'lockers',
        'lessons',
        'tournaments',
        'accessible',
        'match_recording',
        'birthdays',
        'grill',
        'quincho',
        'medical_aid',
        'sports_school',
        'kiosk',
        'kids_area'
    ]::TEXT[]);

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- A venue that already picked one of the new amenities would fail the narrower
-- constraint, so they are removed from the rows first. Rollback loses them;
-- that is the price of going back.
SET LOCAL app.bypass_tenant = 'on';
UPDATE complexes
SET amenities = ARRAY(
    SELECT a FROM unnest(amenities) AS a
    WHERE a NOT IN ('birthdays', 'grill', 'quincho', 'medical_aid',
                    'sports_school', 'kiosk', 'kids_area')
)
WHERE amenities && ARRAY['birthdays', 'grill', 'quincho', 'medical_aid',
                         'sports_school', 'kiosk', 'kids_area']::TEXT[];

ALTER TABLE complexes DROP CONSTRAINT complexes_amenities_known;
ALTER TABLE complexes ADD CONSTRAINT complexes_amenities_known
    CHECK (amenities <@ ARRAY[
        'parking',
        'changing_rooms',
        'showers',
        'bar',
        'racket_rental',
        'pro_shop',
        'wifi',
        'lockers',
        'lessons',
        'tournaments',
        'accessible',
        'match_recording'
    ]::TEXT[]);

-- CREATE OR REPLACE VIEW cannot remove a column, so the view is dropped and
-- rebuilt as 001 left it before the columns go.
DROP VIEW active_courts;

CREATE VIEW active_courts AS
SELECT c.id, c.complex_id, c.name, c.sport, c.court_type, c.is_active,
       c.deleted_at, c.created_at, c.updated_at, c.description, c.version
FROM courts c
JOIN complexes cx ON cx.id = c.complex_id
WHERE c.deleted_at IS NULL
  AND cx.deleted_at IS NULL;

COMMENT ON VIEW active_courts IS
    'Courts that are not soft-deleted and whose complex is not soft-deleted either. Read this, not the table, unless the caller needs deleted rows (admin, audit, the court name on a historical booking).';

ALTER VIEW active_courts SET (security_invoker = true);

ALTER TABLE courts DROP CONSTRAINT courts_surface_known;
ALTER TABLE courts DROP COLUMN surface;
ALTER TABLE courts DROP COLUMN has_lighting;
