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

// ResultInput is an owner loading (or correcting) the result of a match, in
// exactly one of three shapes: the sets, only the winning side (Winner 1 or 2,
// no score), or the side that did not show up (WalkoverLoser 1 or 2).
type ResultInput struct {
	MatchID       uuid.UUID
	Sets          []draw.Set
	Winner        int
	WalkoverLoser int
}

// SetResult records the result of a match and carries it on: the winner goes to
// the next bracket match, the group table updates, and a finished group fills
// its qualifiers into the bracket. The sets are validated against the category's format (draw.Format);
// an invalid score answers an error wrapping draw.ErrInvalidScore.
//
// Loading a result again corrects it, allowed while the next match has no
// result: for a bracket match the one it feeds, for a group match the bracket
// matches its qualifiers already play. Otherwise ErrCorrectionLocked.
func (m *Store) SetResult(ctx context.Context, in ResultInput) (*Match, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	var out db.TournamentMatch
	err := m.DB.WithTx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		match, err := q.GetTournamentMatch(ctx, data.UUIDToPg(in.MatchID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("tournaments: read match: %w", err)
		}
		return m.lockedCategory(ctx, q, match.CategoryID, func(e *engine, cat db.TournamentCategory) error {
			// Read again under the lock: another result may have landed meanwhile.
			if match, err = q.GetTournamentMatch(ctx, match.ID); err != nil {
				return fmt.Errorf("tournaments: read match: %w", err)
			}
			if err := e.applyResult(match, in, categoryFormat(cat)); err != nil {
				return err
			}
			out, err = q.GetTournamentMatch(ctx, match.ID)
			return err
		})
	})
	if err != nil {
		return nil, err
	}
	return matchFromDB(out), nil
}

func (e *engine) applyResult(match db.TournamentMatch, in ResultInput, format draw.Format) error {
	if match.Bye || !match.Pair1ID.Valid || !match.Pair2ID.Valid {
		return ErrMatchNotReady
	}
	outcome, sets, loser, err := parseInput(in, format)
	if err != nil {
		return err
	}
	if match.Status == MatchCompleted {
		if err := e.checkCorrectable(match); err != nil {
			return err
		}
	}
	winner := match.Pair1ID
	if outcome.Winner == 2 {
		winner = match.Pair2ID
	}
	return e.complete(match, winner, sets, loser)
}

func parseInput(in ResultInput, format draw.Format) (draw.Outcome, pgtype.Text, int, error) {
	shapes := 0
	for _, given := range []bool{len(in.Sets) > 0, in.Winner != 0, in.WalkoverLoser != 0} {
		if given {
			shapes++
		}
	}
	if shapes != 1 {
		return draw.Outcome{}, pgtype.Text{}, 0, fmt.Errorf("%w: send exactly one of the sets, the winner or the walkover side", draw.ErrInvalidScore)
	}
	switch {
	case in.WalkoverLoser != 0:
		if in.WalkoverLoser != 1 && in.WalkoverLoser != 2 {
			return draw.Outcome{}, pgtype.Text{}, 0, fmt.Errorf("%w: the walkover side is 1 or 2", draw.ErrInvalidScore)
		}
		return format.Walkover(in.WalkoverLoser), pgtype.Text{}, in.WalkoverLoser, nil
	case in.Winner != 0:
		if in.Winner != 1 && in.Winner != 2 {
			return draw.Outcome{}, pgtype.Text{}, 0, fmt.Errorf("%w: the winner is side 1 or 2", draw.ErrInvalidScore)
		}
		return draw.WinnerOnly(in.Winner), pgtype.Text{}, 0, nil
	}
	outcome, err := format.ParseResult(in.Sets)
	if err != nil {
		return draw.Outcome{}, pgtype.Text{}, 0, err
	}
	return outcome, encodeSets(in.Sets), 0, nil
}

// checkCorrectable refuses a correction once the result has been built upon.
func (e *engine) checkCorrectable(match db.TournamentMatch) error {
	if match.Stage == StageGroup {
		locked, err := e.feedsLocked(match.CategoryID, match.GroupID)
		if err != nil {
			return err
		}
		if locked {
			return ErrCorrectionLocked
		}
		return nil
	}
	if !match.NextMatchID.Valid {
		return nil
	}
	next, err := e.q.GetTournamentMatch(e.ctx, match.NextMatchID)
	if err != nil {
		return fmt.Errorf("tournaments: read next match: %w", err)
	}
	if next.Status == MatchCompleted {
		return ErrCorrectionLocked
	}
	return nil
}

// WithdrawPair marks the pair of a registration as withdrawn after the draw: its
// matches that were not played become walkovers for the other side, the
// opponent advancing, and a match it has not met its opponent in yet becomes one
// when the opponent arrives. A registration that is not a drawn pair is a no-op.
func (m *Store) WithdrawPair(ctx context.Context, registrationID uuid.UUID) error {
	pair, err := m.PairOf(ctx, registrationID)
	if err != nil {
		if errors.Is(err, ErrNotPair) {
			return nil
		}
		return err
	}
	err = m.inCategory(ctx, pair.CategoryID, func(e *engine, cat db.TournamentCategory) error {
		if !cat.DrawGeneratedAt.Valid {
			return nil
		}
		id := data.UUIDToPg(pair.ID)
		if _, err := e.q.WithdrawTournamentPair(e.ctx, id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("tournaments: withdraw pair: %w", err)
		}
		return e.settleMatchesOf(cat.ID, id)
	})
	if errors.Is(err, ErrCancelled) {
		return nil
	}
	return err
}

// settleMatchesOf turns the unplayed matches of a withdrawn pair into walkovers.
func (e *engine) settleMatchesOf(categoryID, pair pgtype.UUID) error {
	matches, err := e.q.ListCategoryMatches(e.ctx, categoryID)
	if err != nil {
		return fmt.Errorf("tournaments: read matches: %w", err)
	}
	for _, m := range matches {
		if m.Status == MatchPending && (m.Pair1ID == pair || m.Pair2ID == pair) {
			if err := e.settle(m.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// Standings is a group with its table as of the matches played so far.
type Standings struct {
	Group *Group
	Rows  []draw.Row
}

// GroupTables computes the tables of every group of a category's detail. Pure
// read-side helper over what Detail returned.
func GroupTables(d *Detail, categoryID uuid.UUID) []Standings {
	format := draw.DefaultFormat
	for _, c := range d.Categories {
		if c.ID == categoryID {
			format = c.MatchFormat()
		}
	}
	var out []Standings
	for _, g := range d.Groups {
		if g.CategoryID != categoryID {
			continue
		}
		var pairs []*PairRow
		for _, p := range d.Pairs {
			if p.GroupID != nil && *p.GroupID == g.ID {
				pairs = append(pairs, p)
			}
		}
		sortBySeed(pairs)
		ids := make([]uuid.UUID, len(pairs))
		for i, p := range pairs {
			ids[i] = p.ID
		}
		var results []draw.Result
		for _, m := range d.Matches {
			if m.GroupID == nil || *m.GroupID != g.ID {
				continue
			}
			if o, ok := m.Outcome(format); ok {
				results = append(results, draw.Result{A: *m.Pair1ID, B: *m.Pair2ID, Outcome: o})
			}
		}
		out = append(out, Standings{Group: g, Rows: draw.Standings(ids, results)})
	}
	return out
}

func sortBySeed(pairs []*PairRow) {
	for i := 1; i < len(pairs); i++ {
		for j := i; j > 0 && seedOf(pairs[j]) < seedOf(pairs[j-1]); j-- {
			pairs[j], pairs[j-1] = pairs[j-1], pairs[j]
		}
	}
}

func seedOf(p *PairRow) int {
	if p.Seed == nil {
		return 1 << 30
	}
	return *p.Seed
}
