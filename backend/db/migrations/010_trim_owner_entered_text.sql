-- +goose Up

SET LOCAL lock_timeout = '3s';

-- This is a data step, so it opens with the tenant bypass: row-level security
-- is FORCEd, and without it these UPDATEs would touch zero rows and say nothing.
SET LOCAL app.bypass_tenant = 'on';

-- ============================================================================
-- TRIM OWNER- AND PLAYER-ENTERED TEXT (data only, no schema change)
--
-- internal/httpx ReadJSON now trims every string in a request body, so text
-- typed from this release on arrives without its edge whitespace. Rows saved
-- before it still carry it: a complex named "Afa " put "Afa  en Rosario" into
-- the storefront's SEO titles. This step trims those stored values once.
--
-- Trimmed: the free text a person typed. Names, addresses, cities, provinces,
-- phones, emails, descriptions, notes, categories, product names.
-- Not trimmed, on purpose:
--   * slugs, URLs, tokens and provider ids (machine values);
--   * country_code, currency, kind, category, reason, status (enum-like codes);
--   * password hashes;
--   * sale_items.product_name, a snapshot of the name as sold, kept for the
--     financial record;
--   * user_identities.email, supplied by Google rather than typed.
--
-- The set trimmed is space, tab, carriage return and newline. Written below as
-- E' \t\r\n' wherever it is used.
--
-- Guards, so no statement fails on the data it meets:
--   * A NOT NULL column whose trimmed value would be '' keeps its value.
--   * A nullable column whose trimmed value would be '' becomes NULL. Blank
--     means no value, which is how the handlers store an optional field.
--   * A row whose trimmed unique key would collide with another row is skipped
--     whole and left as it was: users.email (case-insensitive), courts
--     (complex, name) among live courts, clients (complex, phone), products
--     (complex, lower(name)) among active products.
--   * Collision checks compare trimmed values on both sides, so two dirty rows
--     that would collide with each other are both skipped, and no two rows are
--     ever set to one value by this step.
--
-- Side effect: UPDATE fires the set_updated_at and set_version triggers, so
-- complexes and courts bump their version. A form open at deploy time gets one
-- stale-version conflict on its next save, then reloads cleanly.
-- ============================================================================

-- complexes: name, address, city, province and phone are NOT NULL; email and
-- description are nullable. description has a length CHECK that refuses '', so
-- a blank one becomes NULL rather than ''.
UPDATE complexes AS c
SET name        = CASE WHEN btrim(c.name, E' \t\r\n') <> '' THEN btrim(c.name, E' \t\r\n') ELSE c.name END,
    address     = CASE WHEN btrim(c.address, E' \t\r\n') <> '' THEN btrim(c.address, E' \t\r\n') ELSE c.address END,
    city        = CASE WHEN btrim(c.city, E' \t\r\n') <> '' THEN btrim(c.city, E' \t\r\n') ELSE c.city END,
    province    = CASE WHEN btrim(c.province, E' \t\r\n') <> '' THEN btrim(c.province, E' \t\r\n') ELSE c.province END,
    phone       = CASE WHEN btrim(c.phone, E' \t\r\n') <> '' THEN btrim(c.phone, E' \t\r\n') ELSE c.phone END,
    email       = NULLIF(btrim(c.email::text, E' \t\r\n'), '')::citext,
    description = NULLIF(btrim(c.description, E' \t\r\n'), '')
WHERE (c.name <> btrim(c.name, E' \t\r\n') AND btrim(c.name, E' \t\r\n') <> '')
   OR (c.address <> btrim(c.address, E' \t\r\n') AND btrim(c.address, E' \t\r\n') <> '')
   OR (c.city <> btrim(c.city, E' \t\r\n') AND btrim(c.city, E' \t\r\n') <> '')
   OR (c.province <> btrim(c.province, E' \t\r\n') AND btrim(c.province, E' \t\r\n') <> '')
   OR (c.phone <> btrim(c.phone, E' \t\r\n') AND btrim(c.phone, E' \t\r\n') <> '')
   OR c.email IS DISTINCT FROM NULLIF(btrim(c.email::text, E' \t\r\n'), '')::citext
   OR c.description IS DISTINCT FROM NULLIF(btrim(c.description, E' \t\r\n'), '');

-- courts: name is NOT NULL and unique among live courts of a complex.
UPDATE courts AS c
SET name        = CASE WHEN btrim(c.name, E' \t\r\n') <> '' THEN btrim(c.name, E' \t\r\n') ELSE c.name END,
    description = NULLIF(btrim(c.description, E' \t\r\n'), '')
WHERE ((c.name <> btrim(c.name, E' \t\r\n') AND btrim(c.name, E' \t\r\n') <> '')
       OR c.description IS DISTINCT FROM NULLIF(btrim(c.description, E' \t\r\n'), ''))
  AND NOT (
      c.deleted_at IS NULL
      AND c.name <> btrim(c.name, E' \t\r\n')
      AND EXISTS (
          SELECT 1
          FROM courts AS o
          WHERE o.complex_id = c.complex_id
            AND o.id <> c.id
            AND o.deleted_at IS NULL
            AND btrim(o.name, E' \t\r\n') = btrim(c.name, E' \t\r\n')
      )
  );

-- clients: first_name, last_name and phone are NOT NULL; phone is unique per
-- complex. email is citext and nullable; notes is nullable.
UPDATE clients AS c
SET first_name = CASE WHEN btrim(c.first_name, E' \t\r\n') <> '' THEN btrim(c.first_name, E' \t\r\n') ELSE c.first_name END,
    last_name  = CASE WHEN btrim(c.last_name, E' \t\r\n') <> '' THEN btrim(c.last_name, E' \t\r\n') ELSE c.last_name END,
    phone      = CASE WHEN btrim(c.phone, E' \t\r\n') <> '' THEN btrim(c.phone, E' \t\r\n') ELSE c.phone END,
    email      = NULLIF(btrim(c.email::text, E' \t\r\n'), '')::citext,
    notes      = NULLIF(btrim(c.notes, E' \t\r\n'), '')
WHERE ((c.first_name <> btrim(c.first_name, E' \t\r\n') AND btrim(c.first_name, E' \t\r\n') <> '')
    OR (c.last_name <> btrim(c.last_name, E' \t\r\n') AND btrim(c.last_name, E' \t\r\n') <> '')
    OR (c.phone <> btrim(c.phone, E' \t\r\n') AND btrim(c.phone, E' \t\r\n') <> '')
    OR c.email IS DISTINCT FROM NULLIF(btrim(c.email::text, E' \t\r\n'), '')::citext
    OR c.notes IS DISTINCT FROM NULLIF(btrim(c.notes, E' \t\r\n'), ''))
  AND NOT (
      c.phone <> btrim(c.phone, E' \t\r\n')
      AND EXISTS (
          SELECT 1
          FROM clients AS o
          WHERE o.complex_id = c.complex_id
            AND o.id <> c.id
            AND btrim(o.phone, E' \t\r\n') = btrim(c.phone, E' \t\r\n')
      )
  );

-- users: first_name, last_name, phone and email are NOT NULL. email is citext
-- and unique case-insensitively, so the collision check compares as citext.
UPDATE users AS u
SET first_name = CASE WHEN btrim(u.first_name, E' \t\r\n') <> '' THEN btrim(u.first_name, E' \t\r\n') ELSE u.first_name END,
    last_name  = CASE WHEN btrim(u.last_name, E' \t\r\n') <> '' THEN btrim(u.last_name, E' \t\r\n') ELSE u.last_name END,
    phone      = CASE WHEN btrim(u.phone, E' \t\r\n') <> '' THEN btrim(u.phone, E' \t\r\n') ELSE u.phone END,
    email      = CASE WHEN btrim(u.email::text, E' \t\r\n') <> '' THEN btrim(u.email::text, E' \t\r\n')::citext ELSE u.email END
WHERE ((u.first_name <> btrim(u.first_name, E' \t\r\n') AND btrim(u.first_name, E' \t\r\n') <> '')
    OR (u.last_name <> btrim(u.last_name, E' \t\r\n') AND btrim(u.last_name, E' \t\r\n') <> '')
    OR (u.phone <> btrim(u.phone, E' \t\r\n') AND btrim(u.phone, E' \t\r\n') <> '')
    OR (u.email::text <> btrim(u.email::text, E' \t\r\n') AND btrim(u.email::text, E' \t\r\n') <> ''))
  AND NOT (
      u.email::text <> btrim(u.email::text, E' \t\r\n')
      AND EXISTS (
          SELECT 1
          FROM users AS o
          WHERE o.id <> u.id
            AND btrim(o.email::text, E' \t\r\n')::citext = btrim(u.email::text, E' \t\r\n')::citext
      )
  );

-- email_change_requests: new_email is citext and NOT NULL, with no unique
-- constraint on it.
UPDATE email_change_requests AS r
SET new_email = btrim(r.new_email::text, E' \t\r\n')::citext
WHERE r.new_email::text <> btrim(r.new_email::text, E' \t\r\n')
  AND btrim(r.new_email::text, E' \t\r\n') <> '';

-- products: name is NOT NULL and unique among active products of a complex by
-- lower(name). category is nullable and capped at 60 characters, which
-- trimming only shortens.
UPDATE products AS p
SET name     = CASE WHEN btrim(p.name, E' \t\r\n') <> '' THEN btrim(p.name, E' \t\r\n') ELSE p.name END,
    category = NULLIF(btrim(p.category, E' \t\r\n'), '')
WHERE ((p.name <> btrim(p.name, E' \t\r\n') AND btrim(p.name, E' \t\r\n') <> '')
       OR p.category IS DISTINCT FROM NULLIF(btrim(p.category, E' \t\r\n'), ''))
  AND NOT (
      p.active
      AND p.name <> btrim(p.name, E' \t\r\n')
      AND EXISTS (
          SELECT 1
          FROM products AS o
          WHERE o.complex_id = p.complex_id
            AND o.id <> p.id
            AND o.active
            AND lower(btrim(o.name, E' \t\r\n')) = lower(btrim(p.name, E' \t\r\n'))
      )
  );

-- Nullable free-text notes, where blank means no value.
UPDATE bookings
SET notes = NULLIF(btrim(notes, E' \t\r\n'), '')
WHERE notes IS DISTINCT FROM NULLIF(btrim(notes, E' \t\r\n'), '');

UPDATE blocked_slots
SET reason = NULLIF(btrim(reason, E' \t\r\n'), '')
WHERE reason IS DISTINCT FROM NULLIF(btrim(reason, E' \t\r\n'), '');

UPDATE cash_movements
SET note = NULLIF(btrim(note, E' \t\r\n'), '')
WHERE note IS DISTINCT FROM NULLIF(btrim(note, E' \t\r\n'), '');

UPDATE cash_sessions
SET opening_note = NULLIF(btrim(opening_note, E' \t\r\n'), ''),
    closing_note = NULLIF(btrim(closing_note, E' \t\r\n'), '')
WHERE opening_note IS DISTINCT FROM NULLIF(btrim(opening_note, E' \t\r\n'), '')
   OR closing_note IS DISTINCT FROM NULLIF(btrim(closing_note, E' \t\r\n'), '');

UPDATE stock_movements
SET note = NULLIF(btrim(note, E' \t\r\n'), '')
WHERE note IS DISTINCT FROM NULLIF(btrim(note, E' \t\r\n'), '');

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- Nothing to undo. Trimmed text is not restored to its old whitespace, and no
-- schema object changed, so this migration is a one-way data cleanup.
