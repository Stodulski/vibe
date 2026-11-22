-- name: InsertTournament :one
INSERT INTO tournaments (complex_id, name, sport, start_date, end_date, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetTournament :one
-- The tenant predicate is the explicit half of the isolation the row-level
-- security policies enforce; see GetBookingByID for why it is optional.
SELECT * FROM tournaments
WHERE id = $1
  AND (sqlc.narg('complex_id')::uuid IS NULL
       OR complex_id = sqlc.narg('complex_id')::uuid);

-- name: LockTournament :one
-- Cancelling a tournament and creating or changing its categories take this
-- lock first, so they cannot interleave.
SELECT * FROM tournaments WHERE id = $1 FOR UPDATE;

-- name: ListTournaments :many
-- The tournaments of one complex that overlap the window, soonest first.
-- include_cancelled is what the owner's list sets and the public one does not.
SELECT * FROM tournaments
WHERE complex_id = sqlc.arg(complex_id)
  AND end_date >= sqlc.arg(date_from)::date
  AND start_date <= sqlc.arg(date_to)::date
  AND (sqlc.arg(include_cancelled)::bool OR status = 'active')
ORDER BY start_date, id
LIMIT sqlc.arg(page_limit)::int;

-- name: CancelTournament :one
UPDATE tournaments
SET status = 'cancelled', cancelled_at = NOW(), cancelled_by = $2, cancel_reason = $3
WHERE id = $1 AND status = 'active'
RETURNING *;

-- name: InsertTournamentCategory :one
INSERT INTO tournament_categories (
    tournament_id, complex_id, event_id, name, format, group_size, qualifiers_per_group,
    sets_to_win, games_per_set, super_tiebreak_decider, match_duration_minutes, court_fee,
    modality, level, suma, prizes, starts_on, starts_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
RETURNING *;

-- name: GetTournamentCategory :one
SELECT * FROM tournament_categories
WHERE id = $1
  AND (sqlc.narg('complex_id')::uuid IS NULL
       OR complex_id = sqlc.narg('complex_id')::uuid);

-- name: GetTournamentCategoryByEvent :one
SELECT * FROM tournament_categories WHERE event_id = $1;

-- name: LockTournamentCategory :one
-- Every change to a category's draw, matches or results takes this lock first,
-- so two of them can never interleave on the same bracket.
SELECT * FROM tournament_categories WHERE id = $1 FOR UPDATE;

-- name: ListTournamentCategories :many
-- The categories of a tournament with what their sign-up container says: the
-- capacity in pairs, the entry price per pair, whether it was cancelled, and how
-- many pairs are paid and held (the count capacity is checked against,
-- CountOpenPlaySpotsTaken: held includes a lapsed hold kept for its open
-- payment check).
SELECT c.*,
       e.capacity AS capacity,
       e.price AS price,
       e.status AS event_status,
       t.start_date AS tournament_start_date,
       (SELECT count(*) FROM open_play_registrations r
         WHERE r.event_id = e.id AND r.status = 'confirmed')::int AS confirmed_count,
       (SELECT count(*) FROM open_play_registrations r
         WHERE r.event_id = e.id
           AND ((r.status = 'held' AND r.held_until > NOW() - INTERVAL '15 minutes')
             OR (r.status = 'expired' AND r.payment_check_notify AND r.payment_check_due_at IS NOT NULL
                 AND r.held_until > NOW() - INTERVAL '15 minutes')))::int AS held_count
FROM tournament_categories c
JOIN open_play_events e ON e.id = c.event_id
JOIN tournaments t ON t.id = c.tournament_id
WHERE c.tournament_id = $1
ORDER BY c.created_at, c.id;

-- name: SetCategoryDraw :exec
-- Records that the draw exists, which closes sign-up for good.
UPDATE tournament_categories
SET draw_generated_at = COALESCE(draw_generated_at, NOW())
WHERE id = $1;

-- name: InsertTournamentPair :one
INSERT INTO tournament_pairs (
    category_id, complex_id, registration_id, partner_first_name, partner_last_name, partner_phone, partner_email,
    partner_gender, partner_level
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetTournamentPair :one
SELECT * FROM tournament_pairs WHERE id = $1;

-- name: GetTournamentPairByRegistration :one
SELECT * FROM tournament_pairs WHERE registration_id = $1;

-- name: ListTournamentPairs :many
-- Every pair of a tournament with its registration (the payer, the state of the
-- spot) and the payment that carries the money, as the owner's pair list reads
-- it, oldest sign-up first. Registrations that never held a spot (expired
-- holds, a cancelled one that never carried money) are left out.
SELECT pr.*,
       r.first_name, r.last_name, r.phone, r.email, r.player_gender, r.player_level,
       r.status AS registration_status, r.price AS registration_price, r.service_fee AS registration_service_fee,
       r.held_until, r.confirmed_at, r.cancelled_at AS registration_cancelled_at, r.cancelled_by, r.created_at AS signed_up_at,
       p.id AS payment_id,
       COALESCE(p.status::text, '')::text AS payment_status,
       COALESCE(p.amount, 0)::int AS payment_amount,
       COALESCE(p.service_fee, 0)::int AS payment_service_fee,
       COALESCE(p.refund_amount, 0)::int AS payment_refund_amount
FROM tournament_pairs pr
JOIN tournament_categories c ON c.id = pr.category_id
JOIN open_play_registrations r ON r.id = pr.registration_id
LEFT JOIN LATERAL (
    SELECT * FROM payments pp
    WHERE pp.registration_id = r.id
    ORDER BY (pp.mp_payment_id IS NOT NULL) DESC, pp.created_at DESC
    LIMIT 1
) p ON TRUE
WHERE c.tournament_id = $1
  AND (r.status IN ('held', 'confirmed') OR (r.status = 'cancelled' AND p.id IS NOT NULL))
ORDER BY r.created_at, pr.id;

-- name: ListDrawablePairs :many
-- The pairs a draw is made of: paid sign-ups that have not withdrawn, in the
-- owner's seed order where there is one (unseeded last), then by sign-up.
SELECT pr.* FROM tournament_pairs pr
JOIN open_play_registrations r ON r.id = pr.registration_id
WHERE pr.category_id = $1 AND pr.status = 'active' AND r.status = 'confirmed'
ORDER BY pr.seed NULLS LAST, r.created_at, pr.id;

-- name: SetPairPlacement :exec
UPDATE tournament_pairs SET seed = $2, group_id = $3 WHERE id = $1;

-- name: ClearPairPlacements :exec
UPDATE tournament_pairs SET seed = NULL, group_id = NULL WHERE category_id = $1;

-- name: WithdrawTournamentPair :one
UPDATE tournament_pairs
SET status = 'withdrawn', withdrawn_at = NOW()
WHERE id = $1 AND status = 'active'
RETURNING *;

-- name: InsertTournamentGroup :one
INSERT INTO tournament_groups (category_id, complex_id, name, position)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListTournamentGroups :many
SELECT * FROM tournament_groups WHERE category_id = $1 ORDER BY position;

-- name: DeleteTournamentGroups :exec
DELETE FROM tournament_groups WHERE category_id = $1;

-- name: InsertTournamentMatch :one
INSERT INTO tournament_matches (
    category_id, complex_id, stage, group_id, round, position,
    pair1_id, pair2_id, slot1_group_id, slot1_rank, slot2_group_id, slot2_rank, bye
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: SetMatchNext :exec
UPDATE tournament_matches SET next_match_id = $2, next_slot = $3 WHERE id = $1;

-- name: DeleteTournamentMatches :exec
DELETE FROM tournament_matches WHERE category_id = $1;

-- name: GetTournamentMatch :one
SELECT * FROM tournament_matches WHERE id = $1;

-- name: ListCategoryMatches :many
SELECT * FROM tournament_matches
WHERE category_id = $1
ORDER BY stage DESC, round, position, id;

-- name: ListTournamentMatches :many
-- Every match of every category of a tournament, bracket after group stage.
SELECT m.* FROM tournament_matches m
JOIN tournament_categories c ON c.id = m.category_id
WHERE c.tournament_id = $1
ORDER BY m.category_id, m.stage DESC, m.round, m.position, m.id;

-- name: ListTournamentGroupsByTournament :many
SELECT g.* FROM tournament_groups g
JOIN tournament_categories c ON c.id = g.category_id
WHERE c.tournament_id = $1
ORDER BY g.category_id, g.position;

-- name: SetMatchSlot :exec
-- Puts a pair in one of a match's two slots (1 or 2), or empties it.
UPDATE tournament_matches
SET pair1_id = CASE WHEN sqlc.arg(slot)::int = 1 THEN sqlc.narg('pair_id')::uuid ELSE pair1_id END,
    pair2_id = CASE WHEN sqlc.arg(slot)::int = 2 THEN sqlc.narg('pair_id')::uuid ELSE pair2_id END
WHERE id = sqlc.arg(id);

-- name: SetMatchPairs :exec
UPDATE tournament_matches SET pair1_id = $2, pair2_id = $3 WHERE id = $1;

-- name: SetMatchResult :exec
-- sets travels as text with an explicit cast: the pool's exec mode would send a
-- []byte as bytea, which Postgres refuses to read as json (see audit.Insert).
UPDATE tournament_matches
SET status = 'completed', winner_pair_id = $2, sets = sqlc.narg('sets')::text::jsonb, walkover_loser = $3, completed_at = NOW()
WHERE id = $1;

-- name: ScheduleMatch :exec
UPDATE tournament_matches
SET court_id = $2, scheduled_on = $3, start_time = $4, duration_minutes = $5, reminder_sent_at = NULL
WHERE id = $1;

-- name: UnscheduleMatch :exec
UPDATE tournament_matches
SET court_id = NULL, scheduled_on = NULL, start_time = NULL, duration_minutes = NULL, reminder_sent_at = NULL
WHERE id = $1;

-- name: CountCategoryResults :one
-- Matches of a category that were played (a bye is not a result).
SELECT count(*)::int FROM tournament_matches
WHERE category_id = $1 AND status = 'completed' AND NOT bye;

-- name: PairPlayersTaken :one
-- Whether any of the phones is a player of a live pair of the category: the
-- payer of a paid or still-held sign-up, or its partner. A pair whose hold ran
-- out does not count here even while its spot is kept for its payment check
-- (CountOpenPlaySpotsTaken): its players may sign up again at once, and a late
-- payment on the old pair is then refused (PairOfRegistrationPlayersTaken),
-- like PhoneHoldsAnotherLiveSpot for events.
SELECT EXISTS (
  SELECT 1 FROM tournament_pairs tp
  JOIN open_play_registrations r ON r.id = tp.registration_id
  WHERE tp.category_id = sqlc.arg(category_id)
    AND tp.status = 'active'
    AND (r.status = 'confirmed' OR (r.status = 'held' AND r.held_until > NOW()))
    AND (r.phone = ANY(sqlc.arg(phones)::text[]) OR tp.partner_phone = ANY(sqlc.arg(phones)::text[]))
) AS taken;

-- name: PairOfRegistrationPlayersTaken :one
-- Whether either player of a registration's pair is a player of another live
-- pair of the category: the payer of a paid or still-held sign-up, or its
-- partner. A late payment on a pair whose hold ran out is refused when one is:
-- its players signed up again in another pair meanwhile.
SELECT EXISTS (
  SELECT 1
  FROM tournament_pairs me
  JOIN open_play_registrations mr ON mr.id = me.registration_id
  JOIN tournament_pairs tp ON tp.category_id = me.category_id AND tp.id <> me.id AND tp.status = 'active'
  JOIN open_play_registrations r ON r.id = tp.registration_id
  WHERE me.registration_id = sqlc.arg(registration_id)
    AND (r.status = 'confirmed' OR (r.status = 'held' AND r.held_until > NOW()))
    AND (r.phone IN (mr.phone, me.partner_phone) OR tp.partner_phone IN (mr.phone, me.partner_phone))
) AS taken;

-- name: ListPairsOfGroup :many
-- The pairs of a group in seed order, withdrawn ones included: a group table
-- lists every pair that was drawn into it.
SELECT * FROM tournament_pairs WHERE group_id = $1 ORDER BY seed, id;

-- name: ListCategoryPairs :many
SELECT * FROM tournament_pairs WHERE category_id = $1 ORDER BY seed NULLS LAST, id;

-- name: SetCategoryCourtFee :exec
-- The fee a pair pays per match, in centavos (0 is none). A charge already
-- recorded keeps its own amount.
UPDATE tournament_categories SET court_fee = $2 WHERE id = $1;

-- name: ListTournamentMatchFees :many
-- The court fees that stand in a tournament: a charge whose income movement has
-- not been voided. A voided one reads as not charged.
SELECT f.*
FROM tournament_match_fees f
JOIN tournament_matches m ON m.id = f.match_id
JOIN tournament_categories c ON c.id = m.category_id
WHERE c.tournament_id = $1
  AND NOT EXISTS (SELECT 1 FROM cash_movements v WHERE v.voids_movement_id = f.cash_movement_id)
ORDER BY f.match_id, f.side;

-- name: GetMatchFeeForUpdate :one
-- The row of a (match, side) with whether its charge was voided. Locked: the
-- caller holds the category lock, this keeps the row put while it is rewritten.
SELECT f.*,
       EXISTS (SELECT 1 FROM cash_movements v WHERE v.voids_movement_id = f.cash_movement_id) AS voided
FROM tournament_match_fees f
WHERE f.match_id = $1 AND f.side = $2
FOR UPDATE OF f;

-- name: UpsertMatchFee :one
-- Records a charge; after a void it rewrites the row of the voided one.
INSERT INTO tournament_match_fees (complex_id, match_id, side, pair_id, amount, method, cash_movement_id, charged_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (match_id, side) DO UPDATE
SET pair_id = EXCLUDED.pair_id, amount = EXCLUDED.amount, method = EXCLUDED.method,
    cash_movement_id = EXCLUDED.cash_movement_id, charged_by = EXCLUDED.charged_by, charged_at = NOW()
RETURNING *;

-- name: CountCategoryLiveFees :one
-- The charges that stand in a category; a draw change is refused while there are any.
SELECT count(*)::int FROM tournament_match_fees f
JOIN tournament_matches m ON m.id = f.match_id
WHERE m.category_id = $1
  AND NOT EXISTS (SELECT 1 FROM cash_movements v WHERE v.voids_movement_id = f.cash_movement_id);

-- name: ListTournamentMatchesForReminder :many
-- The scheduled matches with both pairs known and playing, of a live category,
-- that start within the next two hours and were not reminded yet: the window
-- the booking reminder uses, over the same booking_starts_at.
SELECT m.id AS match_id, tc.tournament_id, m.complex_id
FROM tournament_matches m
JOIN tournament_categories tc ON tc.id = m.category_id
JOIN open_play_events e ON e.id = tc.event_id
JOIN tournaments t ON t.id = tc.tournament_id
JOIN tournament_pairs p1 ON p1.id = m.pair1_id
JOIN tournament_pairs p2 ON p2.id = m.pair2_id
WHERE m.status = 'pending'
  AND NOT m.bye
  AND m.court_id IS NOT NULL
  AND m.reminder_sent_at IS NULL
  AND p1.status = 'active' AND p2.status = 'active'
  AND e.status = 'active' AND t.status = 'active'
  AND booking_starts_at(m.scheduled_on, m.start_time) > sqlc.arg(now)::timestamptz
  AND booking_starts_at(m.scheduled_on, m.start_time) <= sqlc.arg(now)::timestamptz + INTERVAL '2 hours'
ORDER BY booking_starts_at(m.scheduled_on, m.start_time), m.id;

-- name: MarkTournamentMatchReminded :exec
UPDATE tournament_matches
SET reminder_sent_at = NOW()
WHERE id = $1;
