-- name: ListActiveComplexesForHubs :many
-- The public city hubs and the sitemap read this. It reads active_complexes, so
-- a soft-deleted venue is absent, and keeps only the switched-on ones. Each row
-- carries the sports of that complex's switched-on courts, read from
-- active_courts so a soft-deleted court is absent too.
--
-- The city is not matched here: a hub matches a city case- and accent-insensitively,
-- and this database has no unaccent extension to do that in SQL. Callers group
-- the rows by city themselves. The order is by city then name so the grouping,
-- and the spelling a hub takes from its first row, is stable.
SELECT
    cx.id,
    cx.name,
    cx.slug,
    cx.address,
    cx.city,
    cx.is_active,
    cx.updated_at,
    ARRAY(
        SELECT DISTINCT ac.sport::text
        FROM active_courts ac
        WHERE ac.complex_id = cx.id
          AND ac.is_active
        ORDER BY ac.sport::text
    )::text[] AS sports
FROM active_complexes cx
WHERE cx.is_active
ORDER BY cx.city, cx.name;
