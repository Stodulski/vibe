-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- TOURNAMENT AGENDA — category traits, declared players, open-play modality
-- ============================================================================
-- Owner decisions 2026-11-16, S1-S3, S10-S14.
--
-- A tournament category is built from a modality (Caballeros, Damas or Mixto) and
-- either a level (1ra to 9na, stored 1 to 9) or a Suma (the two players' levels
-- added, 2 to 18). The service composes its name from those traits, so the
-- existing UNIQUE (tournament_id, name) refuses a repeated combination. A category
-- may carry prizes and its own start day and time.
--
-- An open-play event may carry a modality and a level range (level_min is the
-- strongest, lowest number). Its participants declare gender and level.
--
-- Everything is additive and nullable: rows that exist today keep NULL here and
-- no row is rewritten. The service requires these values for the rows it creates;
-- the CHECKs pass on NULL.
--
-- Adding a CHECK without NOT VALID scans the table under the ACCESS EXCLUSIVE lock
-- that ALTER TABLE holds. The tournament tables are new. open_play_events and
-- open_play_registrations are not (028), so if their scan is too long in
-- production, split the CHECKs into NOT VALID here and VALIDATE CONSTRAINT in a
-- later file, as 031 and 032 do.

-- Categories: modality, level or suma, prizes, and when the category starts.
ALTER TABLE tournament_categories
    ADD COLUMN modality  TEXT,
    ADD COLUMN level     INTEGER,
    ADD COLUMN suma      INTEGER,
    ADD COLUMN prizes    TEXT,
    ADD COLUMN starts_on DATE,
    ADD COLUMN starts_at TIME,
    ADD CONSTRAINT tournament_categories_modality_valid
        CHECK (modality IN ('men', 'women', 'mixed')),
    ADD CONSTRAINT tournament_categories_level_check
        CHECK (level BETWEEN 1 AND 9),
    ADD CONSTRAINT tournament_categories_suma_check
        CHECK (suma BETWEEN 2 AND 18),
    ADD CONSTRAINT tournament_categories_level_or_suma
        CHECK (NOT (level IS NOT NULL AND suma IS NOT NULL)),
    ADD CONSTRAINT tournament_categories_prizes_check
        CHECK (prizes IS NULL OR char_length(prizes) <= 200);

-- The payer of a pair is the registration's person, so the payer's declared gender
-- and level sit on open_play_registrations below. The partner has no registration
-- of their own, so the partner's sit on the pair.
ALTER TABLE tournament_pairs
    ADD COLUMN partner_gender TEXT,
    ADD COLUMN partner_level  INTEGER,
    ADD CONSTRAINT tournament_pairs_partner_gender_valid
        CHECK (partner_gender IN ('men', 'women')),
    ADD CONSTRAINT tournament_pairs_partner_level_check
        CHECK (partner_level BETWEEN 1 AND 9);

-- The person who signs up, declared at sign-up: gender and level. Every open-play
-- sign-up writes it, tournament pairs included; a legacy sign-up leaves it NULL.
ALTER TABLE open_play_registrations
    ADD COLUMN player_gender TEXT,
    ADD COLUMN player_level  INTEGER,
    ADD CONSTRAINT open_play_registrations_player_gender_valid
        CHECK (player_gender IN ('men', 'women')),
    ADD CONSTRAINT open_play_registrations_player_level_check
        CHECK (player_level BETWEEN 1 AND 9);

-- Open-play events: a modality and a level range, both or neither. Tournament
-- category events leave all three NULL: their rules live on the category.
ALTER TABLE open_play_events
    ADD COLUMN modality  TEXT,
    ADD COLUMN level_min INTEGER,
    ADD COLUMN level_max INTEGER,
    ADD CONSTRAINT open_play_events_modality_valid
        CHECK (modality IN ('men', 'women', 'mixed')),
    ADD CONSTRAINT open_play_events_level_range_check
        CHECK (level_min BETWEEN 1 AND 9 AND level_max BETWEEN 1 AND 9 AND level_min <= level_max),
    ADD CONSTRAINT open_play_events_level_both_or_neither
        CHECK ((level_min IS NULL) = (level_max IS NULL));

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- Drops exactly what the Up added. Category names stay: they are the composed
-- names the uniqueness is enforced on.
ALTER TABLE open_play_events
    DROP CONSTRAINT open_play_events_level_both_or_neither,
    DROP CONSTRAINT open_play_events_level_range_check,
    DROP CONSTRAINT open_play_events_modality_valid,
    DROP COLUMN level_max,
    DROP COLUMN level_min,
    DROP COLUMN modality;

ALTER TABLE open_play_registrations
    DROP CONSTRAINT open_play_registrations_player_level_check,
    DROP CONSTRAINT open_play_registrations_player_gender_valid,
    DROP COLUMN player_level,
    DROP COLUMN player_gender;

ALTER TABLE tournament_pairs
    DROP CONSTRAINT tournament_pairs_partner_level_check,
    DROP CONSTRAINT tournament_pairs_partner_gender_valid,
    DROP COLUMN partner_level,
    DROP COLUMN partner_gender;

ALTER TABLE tournament_categories
    DROP CONSTRAINT tournament_categories_prizes_check,
    DROP CONSTRAINT tournament_categories_level_or_suma,
    DROP CONSTRAINT tournament_categories_suma_check,
    DROP CONSTRAINT tournament_categories_level_check,
    DROP CONSTRAINT tournament_categories_modality_valid,
    DROP COLUMN starts_at,
    DROP COLUMN starts_on,
    DROP COLUMN prizes,
    DROP COLUMN suma,
    DROP COLUMN level,
    DROP COLUMN modality;
