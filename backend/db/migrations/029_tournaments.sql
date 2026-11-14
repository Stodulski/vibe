-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- TOURNAMENTS — stage 2 of events, on top of the open-play base (028)
-- ============================================================================
-- Owner decisions 2026-11-14: the owner runs a tournament in categories; pairs
-- sign up and pay online per category, the owner draws groups and/or a knockout
-- bracket, schedules matches on courts and loads results.
--
-- Money is not reinvented. Every category owns one row of open_play_events (its
-- tournament_id is set, it holds no courts) and a pair's sign-up is one
-- open_play_registrations row, held, paid, confirmed, cancelled, refunded and
-- swept by the code that already does it for canchas libres. What a pair adds
-- (the partner, the seed, the group) lives in tournament_pairs, one row per
-- registration. The event's capacity is the category's capacity in pairs, its
-- price the entry fee per pair, and the payer is the registration's person.
--
-- Tenant-scoped like every table in 001_init.sql's ACCESS section.
CREATE TABLE tournaments (
    id            UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id    UUID NOT NULL,
    name          TEXT NOT NULL,
    sport         sport_type NOT NULL DEFAULT 'padel',
    start_date    DATE NOT NULL,
    end_date      DATE NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active',
    cancel_reason TEXT,
    cancelled_at  TIMESTAMPTZ,
    cancelled_by  UUID,
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tournaments_id_complex_id_key UNIQUE (id, complex_id),
    CONSTRAINT tournaments_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    CONSTRAINT tournaments_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id),
    CONSTRAINT tournaments_cancelled_by_fkey FOREIGN KEY (cancelled_by) REFERENCES users (id),
    CONSTRAINT tournaments_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    CONSTRAINT tournaments_dates_check CHECK (end_date >= start_date AND end_date <= start_date + 60),
    CONSTRAINT tournaments_status_valid CHECK (status IN ('active', 'cancelled')),
    CONSTRAINT tournaments_cancel_columns CHECK ((status = 'cancelled') = (cancelled_at IS NOT NULL))
);

CREATE TRIGGER set_updated_at BEFORE UPDATE ON tournaments
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
CREATE INDEX idx_tournaments_complex_dates ON tournaments (complex_id, end_date);

ALTER TABLE tournaments ENABLE ROW LEVEL SECURITY;
ALTER TABLE tournaments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tournaments
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON tournaments
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- A category's sign-up container is an open_play_events row that belongs to a
-- tournament. It holds no courts, so it blocks nothing; the open-play lists and
-- routes leave it out and the tournament routes own it.
ALTER TABLE open_play_events ADD COLUMN tournament_id UUID;
ALTER TABLE open_play_events ADD CONSTRAINT open_play_events_tournament_fkey
    FOREIGN KEY (tournament_id, complex_id) REFERENCES tournaments (id, complex_id) NOT VALID;
ALTER TABLE open_play_events VALIDATE CONSTRAINT open_play_events_tournament_fkey;
CREATE INDEX idx_open_play_events_tournament ON open_play_events (tournament_id) WHERE tournament_id IS NOT NULL;

-- ============================================================================
-- TOURNAMENT CATEGORIES — "6ta", "7ma", "Mixto"
-- ============================================================================
-- format 'elimination' is a single-elimination bracket; 'groups' is round robin
-- groups then a knockout of the best qualifiers_per_group of each. The owner
-- sets group_size (the most pairs per group, suggested 3) and
-- qualifiers_per_group (suggested 2) per category; the draw derives the group
-- count from the pairs signed up, sizes differing by at most one.
-- draw_generated_at closes sign-up: it is set when the draw is first generated.
--
-- The match format is the owner's per category: sets_to_win (1 = a single set,
-- 2 = best of three), games_per_set (6 is standard padel, 4 is the short set),
-- super_tiebreak_decider (the deciding set is a match tiebreak to 10; only
-- applies when sets_to_win = 2) and match_duration_minutes, the court time a
-- match is given when it is scheduled without one. The defaults are best of
-- three, 6 games, a match tiebreak and 90 minutes.
CREATE TABLE tournament_categories (
    id                UUID PRIMARY KEY DEFAULT uuidv7(),
    tournament_id     UUID NOT NULL,
    complex_id        UUID NOT NULL,
    event_id          UUID NOT NULL,
    name              TEXT NOT NULL,
    format            TEXT NOT NULL,
    group_size           INTEGER NOT NULL DEFAULT 3,
    qualifiers_per_group INTEGER NOT NULL DEFAULT 2,
    sets_to_win            INTEGER NOT NULL DEFAULT 2,
    games_per_set          INTEGER NOT NULL DEFAULT 6,
    super_tiebreak_decider BOOLEAN NOT NULL DEFAULT TRUE,
    match_duration_minutes INTEGER NOT NULL DEFAULT 90,
    draw_generated_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tournament_categories_id_complex_id_key UNIQUE (id, complex_id),
    CONSTRAINT tournament_categories_event_key UNIQUE (event_id),
    CONSTRAINT tournament_categories_name_key UNIQUE (tournament_id, name),
    CONSTRAINT tournament_categories_tournament_fkey
        FOREIGN KEY (tournament_id, complex_id) REFERENCES tournaments (id, complex_id) ON DELETE CASCADE,
    CONSTRAINT tournament_categories_event_fkey
        FOREIGN KEY (event_id, complex_id) REFERENCES open_play_events (id, complex_id),
    CONSTRAINT tournament_categories_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 60),
    CONSTRAINT tournament_categories_format_valid CHECK (format IN ('elimination', 'groups')),
    CONSTRAINT tournament_categories_group_size_check CHECK (group_size BETWEEN 2 AND 12),
    CONSTRAINT tournament_categories_qualifiers_check CHECK (qualifiers_per_group >= 1 AND qualifiers_per_group < group_size),
    CONSTRAINT tournament_categories_sets_to_win_check CHECK (sets_to_win IN (1, 2)),
    CONSTRAINT tournament_categories_games_per_set_check CHECK (games_per_set BETWEEN 4 AND 9),
    CONSTRAINT tournament_categories_match_duration_check CHECK (match_duration_minutes BETWEEN 30 AND 480)
);

CREATE TRIGGER set_updated_at BEFORE UPDATE ON tournament_categories
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

ALTER TABLE tournament_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE tournament_categories FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tournament_categories
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON tournament_categories
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- ============================================================================
-- TOURNAMENT GROUPS
-- ============================================================================
CREATE TABLE tournament_groups (
    id          UUID PRIMARY KEY DEFAULT uuidv7(),
    category_id UUID NOT NULL,
    complex_id  UUID NOT NULL,
    name        TEXT NOT NULL,
    position    INTEGER NOT NULL,
    CONSTRAINT tournament_groups_id_complex_id_key UNIQUE (id, complex_id),
    CONSTRAINT tournament_groups_position_key UNIQUE (category_id, position),
    CONSTRAINT tournament_groups_category_fkey
        FOREIGN KEY (category_id, complex_id) REFERENCES tournament_categories (id, complex_id) ON DELETE CASCADE
);

ALTER TABLE tournament_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE tournament_groups FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tournament_groups
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON tournament_groups
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- ============================================================================
-- TOURNAMENT PAIRS — one per sign-up registration
-- ============================================================================
-- The payer is the registration's person; the partner is snapshotted here.
-- Phone is required for both, email is optional. seed is the place in the
-- owner's seed order at the last draw; group_id the group a groups-format draw
-- put the pair in. 'withdrawn' is a pair that left after the draw (its matches
-- are walkovers).
CREATE TABLE tournament_pairs (
    id                 UUID PRIMARY KEY DEFAULT uuidv7(),
    category_id        UUID NOT NULL,
    complex_id         UUID NOT NULL,
    registration_id    UUID NOT NULL,
    partner_first_name TEXT NOT NULL,
    partner_last_name  TEXT NOT NULL,
    partner_phone      TEXT NOT NULL,
    partner_email      TEXT,
    seed               INTEGER,
    group_id           UUID,
    status             TEXT NOT NULL DEFAULT 'active',
    withdrawn_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tournament_pairs_id_complex_id_key UNIQUE (id, complex_id),
    CONSTRAINT tournament_pairs_registration_key UNIQUE (registration_id),
    CONSTRAINT tournament_pairs_category_fkey
        FOREIGN KEY (category_id, complex_id) REFERENCES tournament_categories (id, complex_id) ON DELETE CASCADE,
    CONSTRAINT tournament_pairs_registration_fkey
        FOREIGN KEY (registration_id, complex_id) REFERENCES open_play_registrations (id, complex_id),
    CONSTRAINT tournament_pairs_group_fkey
        FOREIGN KEY (group_id, complex_id) REFERENCES tournament_groups (id, complex_id),
    CONSTRAINT tournament_pairs_status_valid CHECK (status IN ('active', 'withdrawn')),
    CONSTRAINT tournament_pairs_withdrawn_columns CHECK ((status = 'withdrawn') = (withdrawn_at IS NOT NULL))
);

CREATE TRIGGER set_updated_at BEFORE UPDATE ON tournament_pairs
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
CREATE INDEX idx_tournament_pairs_category ON tournament_pairs (category_id);
CREATE INDEX idx_tournament_pairs_group ON tournament_pairs (group_id) WHERE group_id IS NOT NULL;
-- The partner's phone is looked up when someone else signs up with it.
CREATE INDEX idx_tournament_pairs_partner_phone ON tournament_pairs (category_id, partner_phone);

ALTER TABLE tournament_pairs ENABLE ROW LEVEL SECURITY;
ALTER TABLE tournament_pairs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tournament_pairs
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON tournament_pairs
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- ============================================================================
-- TOURNAMENT MATCHES — group round robin and the knockout bracket
-- ============================================================================
-- A bracket match (stage 'bracket') is at (round, position); round 1 is the
-- first, the highest round the final. Its winner goes to next_match_id in slot
-- next_slot. In a groups-format draw the first knockout round starts with
-- placeholders: slotN_group_id/slotN_rank name "the Nth of that group", and the
-- pair id is filled when the group finishes. A bye match has one slot that will
-- never fill; its present pair is the winner as soon as it is known.
--
-- winner_pair_id and sets record the result; walkover_loser (1 or 2) marks a
-- walkover, sets then being null. The schedule is all-or-nothing.
CREATE TABLE tournament_matches (
    id               UUID PRIMARY KEY DEFAULT uuidv7(),
    category_id      UUID NOT NULL,
    complex_id       UUID NOT NULL,
    stage            TEXT NOT NULL,
    group_id         UUID,
    round            INTEGER NOT NULL,
    position         INTEGER NOT NULL,
    pair1_id         UUID,
    pair2_id         UUID,
    slot1_group_id   UUID,
    slot1_rank       INTEGER,
    slot2_group_id   UUID,
    slot2_rank       INTEGER,
    bye              BOOLEAN NOT NULL DEFAULT FALSE,
    next_match_id    UUID,
    next_slot        INTEGER,
    status           TEXT NOT NULL DEFAULT 'pending',
    winner_pair_id   UUID,
    sets             JSONB,
    walkover_loser   INTEGER,
    completed_at     TIMESTAMPTZ,
    court_id         UUID,
    scheduled_on     DATE,
    start_time       TIME,
    duration_minutes INTEGER,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tournament_matches_id_complex_id_key UNIQUE (id, complex_id),
    CONSTRAINT tournament_matches_category_fkey
        FOREIGN KEY (category_id, complex_id) REFERENCES tournament_categories (id, complex_id) ON DELETE CASCADE,
    CONSTRAINT tournament_matches_group_fkey
        FOREIGN KEY (group_id, complex_id) REFERENCES tournament_groups (id, complex_id) ON DELETE CASCADE,
    CONSTRAINT tournament_matches_pair1_fkey
        FOREIGN KEY (pair1_id, complex_id) REFERENCES tournament_pairs (id, complex_id),
    CONSTRAINT tournament_matches_pair2_fkey
        FOREIGN KEY (pair2_id, complex_id) REFERENCES tournament_pairs (id, complex_id),
    CONSTRAINT tournament_matches_winner_fkey
        FOREIGN KEY (winner_pair_id, complex_id) REFERENCES tournament_pairs (id, complex_id),
    CONSTRAINT tournament_matches_court_fkey
        FOREIGN KEY (court_id, complex_id) REFERENCES courts (id, complex_id),
    CONSTRAINT tournament_matches_next_fkey FOREIGN KEY (next_match_id) REFERENCES tournament_matches (id) ON DELETE SET NULL,
    CONSTRAINT tournament_matches_stage_valid CHECK (stage IN ('group', 'bracket')),
    CONSTRAINT tournament_matches_group_stage CHECK ((stage = 'group') = (group_id IS NOT NULL)),
    CONSTRAINT tournament_matches_status_valid CHECK (status IN ('pending', 'completed')),
    CONSTRAINT tournament_matches_next_slot_valid CHECK (next_slot IS NULL OR next_slot IN (1, 2)),
    CONSTRAINT tournament_matches_walkover_valid CHECK (walkover_loser IS NULL OR walkover_loser IN (1, 2)),
    CONSTRAINT tournament_matches_schedule_all_or_none CHECK (
        (court_id IS NULL AND scheduled_on IS NULL AND start_time IS NULL AND duration_minutes IS NULL)
        OR (court_id IS NOT NULL AND scheduled_on IS NOT NULL AND start_time IS NOT NULL AND duration_minutes BETWEEN 30 AND 480)
    )
);

CREATE TRIGGER set_updated_at BEFORE UPDATE ON tournament_matches
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
CREATE INDEX idx_tournament_matches_category ON tournament_matches (category_id, stage, round, position);
CREATE INDEX idx_tournament_matches_court_day ON tournament_matches (court_id, scheduled_on) WHERE court_id IS NOT NULL;

ALTER TABLE tournament_matches ENABLE ROW LEVEL SECURITY;
ALTER TABLE tournament_matches FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tournament_matches
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON tournament_matches
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- ============================================================================
-- court_event_occupancy — every hour an event or a scheduled match holds
-- ============================================================================
-- Every place that asked open_play_event_occupancy "is this court free then?"
-- asks this instead: it is that function plus the scheduled tournament matches,
-- so a match blocks its court for its duration wherever an event does. kind is
-- 'event' or 'match'; label is what the owner's calendar shows (the event's
-- name, or "tournament · category" for a match). A bye never plays, and a
-- cancelled tournament or category releases its matches.
--
-- open_play_event_occupancy stays as it was (a binary mid-deploy may still call
-- it); nothing in the code reads it any more.
-- +goose StatementBegin
CREATE FUNCTION court_event_occupancy(p_complex_id uuid, p_court_ids uuid[], p_from date, p_to date)
RETURNS TABLE (event_id uuid, complex_id uuid, court_id uuid, occurs_on date, span tstzrange, kind text, label text, parent_id uuid)
    LANGUAGE sql
    STABLE
    PARALLEL SAFE
    SET search_path = pg_catalog, public
AS $$
    SELECT o.event_id, o.complex_id, o.court_id, o.occurs_on, o.span, 'event'::text, e.name, NULL::uuid
    FROM open_play_event_occupancy(p_complex_id, p_court_ids, p_from, p_to) o
    JOIN open_play_events e ON e.id = o.event_id
    UNION ALL
    SELECT m.id, m.complex_id, m.court_id, m.scheduled_on,
           tstzrange(
               booking_starts_at(m.scheduled_on, m.start_time),
               booking_starts_at(m.scheduled_on, m.start_time) + make_interval(mins => m.duration_minutes),
               '[)'),
           'match'::text, t.name || ' · ' || c.name, t.id
    FROM tournament_matches m
    JOIN tournament_categories c ON c.id = m.category_id
    JOIN tournaments t ON t.id = c.tournament_id
    JOIN open_play_events ce ON ce.id = c.event_id
    WHERE m.court_id IS NOT NULL
      AND NOT m.bye
      AND t.status = 'active'
      AND ce.status = 'active'
      AND m.scheduled_on BETWEEN p_from AND p_to
      AND (p_complex_id IS NULL OR m.complex_id = p_complex_id)
      AND (p_court_ids IS NULL OR m.court_id = ANY (p_court_ids))
$$;
-- +goose StatementEnd

COMMENT ON FUNCTION court_event_occupancy(uuid, uuid[], date, date) IS
    'The hours active open-play events and scheduled tournament matches hold on their courts: each holds them like a booking.';

-- +goose Down

SET LOCAL lock_timeout = '3s';

DROP FUNCTION court_event_occupancy(uuid, uuid[], date, date);
DROP TABLE tournament_matches;
DROP TABLE tournament_pairs;
DROP TABLE tournament_groups;
DROP TABLE tournament_categories;
-- Registrations and payments of category events cannot outlive their tournament.
-- Row-level security applies to the owner too, so the deletes run under the bypass.
SET LOCAL app.bypass_tenant = 'on';
DELETE FROM failed_refunds WHERE payment_id IN (
    SELECT p.id FROM payments p
    JOIN open_play_registrations r ON r.id = p.registration_id
    JOIN open_play_events e ON e.id = r.event_id WHERE e.tournament_id IS NOT NULL);
DELETE FROM payments WHERE registration_id IN (
    SELECT r.id FROM open_play_registrations r JOIN open_play_events e ON e.id = r.event_id WHERE e.tournament_id IS NOT NULL);
DELETE FROM open_play_registrations WHERE event_id IN (SELECT id FROM open_play_events WHERE tournament_id IS NOT NULL);
DELETE FROM open_play_events WHERE tournament_id IS NOT NULL;
ALTER TABLE open_play_events DROP COLUMN tournament_id;
DROP TABLE tournaments;
