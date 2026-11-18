-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- COURT DESCRIPTION, GONE — the owner decided a court is described by its
-- sport, roof type, surface and lighting, all structured columns; the free
-- text was removed from the product. The release before this one stopped every
-- read and write of courts.description, so no running binary queries it when
-- this migration drops it. The stored texts are deleted for good, as the owner
-- asked.
--
-- active_courts names the column (026_court_surface_lighting.sql) and CREATE OR
-- REPLACE VIEW cannot remove one, so the view is dropped and rebuilt without
-- it, with the same columns in the same order otherwise. Nothing else depends
-- on active_courts. Its grants come back from the ALTER DEFAULT PRIVILEGES the
-- ACCESS section set for the migrator, which apply to any view it creates, and
-- security_invoker is set again: without it the view would read as its owner
-- and bypass row-level security (001_init.sql, ACCESS section).
-- ============================================================================

DROP VIEW active_courts;

CREATE VIEW active_courts AS
SELECT c.id, c.complex_id, c.name, c.sport, c.court_type, c.is_active,
       c.deleted_at, c.created_at, c.updated_at, c.version,
       c.has_lighting, c.surface
FROM courts c
JOIN complexes cx ON cx.id = c.complex_id
WHERE c.deleted_at IS NULL
  AND cx.deleted_at IS NULL;

-- Restated, not inherited: sqlc copies the comment from the migration text.
COMMENT ON VIEW active_courts IS
    'Courts that are not soft-deleted and whose complex is not soft-deleted either. Read this, not the table, unless the caller needs deleted rows (admin, audit, the court name on a historical booking).';

ALTER VIEW active_courts SET (security_invoker = true);

ALTER TABLE courts DROP CONSTRAINT courts_description_length;
ALTER TABLE courts DROP COLUMN description;

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- The view goes first, as in Up: readers lock the view and then courts, so
-- taking courts first could deadlock with them.
DROP VIEW active_courts;

-- The column comes back empty: the texts dropped above are not recoverable.
ALTER TABLE courts ADD COLUMN description TEXT;
ALTER TABLE courts ADD CONSTRAINT courts_description_length
    CHECK (description IS NULL OR char_length(description) <= 200);

CREATE VIEW active_courts AS
SELECT c.id, c.complex_id, c.name, c.sport, c.court_type, c.is_active,
       c.deleted_at, c.created_at, c.updated_at, c.description, c.version,
       c.has_lighting, c.surface
FROM courts c
JOIN complexes cx ON cx.id = c.complex_id
WHERE c.deleted_at IS NULL
  AND cx.deleted_at IS NULL;

COMMENT ON VIEW active_courts IS
    'Courts that are not soft-deleted and whose complex is not soft-deleted either. Read this, not the table, unless the caller needs deleted rows (admin, audit, the court name on a historical booking).';

ALTER VIEW active_courts SET (security_invoker = true);
