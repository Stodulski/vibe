-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- A DELETED COMPLEX FREES ITS SLUG
--
-- complexes_slug_key was a UNIQUE over the whole table, deleted rows included,
-- so a soft-deleted venue kept its public URL forever and no new venue could
-- take it. The product now says otherwise: once a complex is deleted, its slug
-- is free for a new complex. Its old page answers 410 Gone, naming the deleted
-- venue's name and city, for as long as no live complex holds the slug.
--
-- The uniqueness that matters is among live complexes. It becomes a partial
-- unique index, the same predicate complexes_owner_id_key already uses, and
-- idx_complexes_slug (a plain partial index on the same column and predicate)
-- is subsumed by it, so it goes too.
--
-- Down restores the whole-table constraint. It can only succeed while no two
-- rows, deleted ones included, share a slug; once a new complex has taken a
-- deleted complex's slug, Down fails with a unique violation and rolls back
-- whole. Choosing which venue keeps the URL is a data decision this migration
-- does not make for you.
-- ============================================================================

ALTER TABLE complexes DROP CONSTRAINT complexes_slug_key;
DROP INDEX idx_complexes_slug;

CREATE UNIQUE INDEX complexes_slug_live_key ON complexes (slug) WHERE deleted_at IS NULL;

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- Restores the whole-table UNIQUE and the plain partial index it replaced. The
-- constraint is added first: if a slug is now held by two rows (a new complex
-- on a deleted one's slug), it fails here and the transaction rolls back whole.
ALTER TABLE complexes ADD CONSTRAINT complexes_slug_key UNIQUE (slug);
CREATE INDEX idx_complexes_slug ON complexes (slug) WHERE deleted_at IS NULL;

DROP INDEX complexes_slug_live_key;
