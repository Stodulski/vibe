package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
	"github.com/stodulski/vibe-server/internal/tournaments/draw"
)

// engine applies the rules that move a bracket along, inside one transaction
// that already holds the category's row lock.
type engine struct {
	ctx context.Context
	q   *db.Queries
}

// inCategory runs fn in a transaction holding the category's row lock, after
// checking the tournament and the category are both live. Every write that
// changes a draw, a schedule or a result goes through it, so two of them never
// interleave on one bracket.
func (m *Store) inCategory(ctx context.Context, categoryID uuid.UUID, fn func(e *engine, cat db.TournamentCategory) error) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	return m.DB.WithTx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		return m.lockedCategory(ctx, q, data.UUIDToPg(categoryID), fn)
	})
}

func (m *Store) lockedCategory(ctx context.Context, q *db.Queries, categoryID pgtype.UUID, fn func(e *engine, cat db.TournamentCategory) error) error {
	cat, err := q.LockTournamentCategory(ctx, categoryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return fmt.Errorf("tournaments: lock category: %w", err)
	}
	if err := data.AssertTenant(ctx, data.PgToUUID(cat.ComplexID)); err != nil {
		return err
	}
	t, err := q.GetTournament(ctx, db.GetTournamentParams{ID: cat.TournamentID})
	if err != nil {
		return fmt.Errorf("tournaments: read tournament: %w", err)
	}
	ev, err := q.GetOpenPlayEvent(ctx, db.GetOpenPlayEventParams{ID: cat.EventID})
	if err != nil {
		return fmt.Errorf("tournaments: read category event: %w", err)
	}
	if t.Status != StatusActive || ev.Status != "active" {
		return ErrCancelled
	}
	return fn(&engine{ctx: ctx, q: q}, cat)
}

// settle completes a match that needs no play: a bye, or a match against a pair
// that withdrew (a walkover for the other side). A match that is already
// completed, or still waiting for a pair, is left as it is.
func (e *engine) settle(id pgtype.UUID) error {
	m, err := e.q.GetTournamentMatch(e.ctx, id)
	if err != nil {
		return fmt.Errorf("tournaments: read match: %w", err)
	}
	if m.Status == MatchCompleted {
		return nil
	}
	switch {
	case m.Bye && m.Pair1ID.Valid:
		return e.complete(m, m.Pair1ID, pgtype.Text{}, 0)
	case m.Bye || !m.Pair1ID.Valid || !m.Pair2ID.Valid:
		return nil
	}
	p1, err := e.q.GetTournamentPair(e.ctx, m.Pair1ID)
	if err != nil {
		return fmt.Errorf("tournaments: read pair: %w", err)
	}
	p2, err := e.q.GetTournamentPair(e.ctx, m.Pair2ID)
	if err != nil {
		return fmt.Errorf("tournaments: read pair: %w", err)
	}
	switch {
	case p1.Status == PairWithdrawn:
		return e.complete(m, m.Pair2ID, pgtype.Text{}, 1)
	case p2.Status == PairWithdrawn:
		return e.complete(m, m.Pair1ID, pgtype.Text{}, 2)
	}
	return nil
}

// complete records a match's result and carries it on: the winner moves to the
// next bracket match, or a finished group fills the bracket.
func (e *engine) complete(m db.TournamentMatch, winner pgtype.UUID, sets pgtype.Text, walkoverLoser int) error {
	loser := pgtype.Int4{}
	if walkoverLoser > 0 {
		loser = data.Int4ToPg(walkoverLoser)
	}
	if err := e.q.SetMatchResult(e.ctx, db.SetMatchResultParams{
		ID: m.ID, WinnerPairID: winner, Sets: sets, WalkoverLoser: loser,
	}); err != nil {
		return fmt.Errorf("tournaments: set result: %w", err)
	}
	return e.carryOn(m, winner)
}

func (e *engine) carryOn(m db.TournamentMatch, winner pgtype.UUID) error {
	if m.Stage == StageGroup {
		return e.fillIfGroupDone(m.CategoryID, m.GroupID)
	}
	if !m.NextMatchID.Valid {
		return nil
	}
	if err := e.q.SetMatchSlot(e.ctx, db.SetMatchSlotParams{
		ID: m.NextMatchID, Slot: m.NextSlot.Int32, PairID: winner,
	}); err != nil {
		return fmt.Errorf("tournaments: advance winner: %w", err)
	}
	return e.settle(m.NextMatchID)
}

// groupStandings ranks a group from its completed matches.
func (e *engine) groupStandings(groupID pgtype.UUID, matches []db.TournamentMatch) ([]draw.Row, bool, error) {
	pairs, err := e.q.ListPairsOfGroup(e.ctx, groupID)
	if err != nil {
		return nil, false, fmt.Errorf("tournaments: read group pairs: %w", err)
	}
	ids := make([]uuid.UUID, len(pairs))
	for i, p := range pairs {
		ids[i] = data.PgToUUID(p.ID)
	}
	var results []draw.Result
	complete := true
	for _, m := range matches {
		if m.Stage != StageGroup || m.GroupID != groupID {
			continue
		}
		if m.Status != MatchCompleted {
			complete = false
			continue
		}
		mm := matchFromDB(m)
		out, ok := mm.Outcome()
		if !ok {
			complete = false
			continue
		}
		results = append(results, draw.Result{A: *mm.Pair1ID, B: *mm.Pair2ID, Outcome: out})
	}
	return draw.Standings(ids, results), complete, nil
}

// fillIfGroupDone puts the qualifiers of a finished group into the bracket
// slots that wait for them. It runs again when a result of the group is
// corrected, so the slots follow the table.
func (e *engine) fillIfGroupDone(categoryID, groupID pgtype.UUID) error {
	matches, err := e.q.ListCategoryMatches(e.ctx, categoryID)
	if err != nil {
		return fmt.Errorf("tournaments: read matches: %w", err)
	}
	rows, done, err := e.groupStandings(groupID, matches)
	if err != nil || !done {
		return err
	}
	cat, err := e.q.GetTournamentCategory(e.ctx, db.GetTournamentCategoryParams{ID: categoryID})
	if err != nil {
		return fmt.Errorf("tournaments: read category: %w", err)
	}
	for _, m := range matches {
		if m.Stage != StageBracket {
			continue
		}
		for slot, src := range [2]struct {
			group pgtype.UUID
			rank  pgtype.Int4
		}{{m.Slot1GroupID, m.Slot1Rank}, {m.Slot2GroupID, m.Slot2Rank}} {
			if src.group != groupID || int(src.rank.Int32) > int(cat.QualifiersPerGroup) || int(src.rank.Int32) > len(rows) {
				continue
			}
			if err := e.place(m, slot+1, data.UUIDToPg(rows[src.rank.Int32-1].Pair)); err != nil {
				return err
			}
		}
	}
	return nil
}

// place puts a qualifier in a bracket slot and settles the match: a bye
// completes (again, when the slot changed under a correction) and a match
// against a withdrawn pair becomes a walkover.
func (e *engine) place(m db.TournamentMatch, slot int, pair pgtype.UUID) error {
	if err := e.q.SetMatchSlot(e.ctx, db.SetMatchSlotParams{ID: m.ID, Slot: int32(slot), PairID: pair}); err != nil { //nolint:gosec // G115: 1 or 2
		return fmt.Errorf("tournaments: fill slot: %w", err)
	}
	fresh, err := e.q.GetTournamentMatch(e.ctx, m.ID)
	if err != nil {
		return fmt.Errorf("tournaments: read match: %w", err)
	}
	if fresh.Bye && fresh.Status == MatchCompleted {
		return e.complete(fresh, fresh.Pair1ID, pgtype.Text{}, 0)
	}
	return e.settle(m.ID)
}

// feedsLocked reports whether a bracket match fed by the group already has a
// result downstream, which makes the group's table final: a real match that was
// played, or a bye whose next match was.
func (e *engine) feedsLocked(categoryID, groupID pgtype.UUID) (bool, error) {
	matches, err := e.q.ListCategoryMatches(e.ctx, categoryID)
	if err != nil {
		return false, fmt.Errorf("tournaments: read matches: %w", err)
	}
	for _, m := range matches {
		if m.Stage != StageBracket || (m.Slot1GroupID != groupID && m.Slot2GroupID != groupID) {
			continue
		}
		target := m
		if m.Bye {
			if !m.NextMatchID.Valid {
				continue
			}
			if target, err = e.q.GetTournamentMatch(e.ctx, m.NextMatchID); err != nil {
				return false, fmt.Errorf("tournaments: read match: %w", err)
			}
		}
		if target.Status == MatchCompleted {
			return true, nil
		}
	}
	return false, nil
}
