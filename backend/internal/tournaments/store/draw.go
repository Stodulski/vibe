package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
	"github.com/stodulski/vibe-server/internal/tournaments/draw"
)

// DrawInput is an owner generating a category's draw.
type DrawInput struct {
	CategoryID uuid.UUID
	// SeedOrder lists pair ids, best seed first. Pairs it does not name follow
	// in sign-up order, or shuffled when Shuffle is set.
	SeedOrder []uuid.UUID
	// Shuffle reorders the pairs the owner did not seed; nil keeps them in order.
	Shuffle func([]uuid.UUID)
}

// GenerateDraw makes the category's draw from its paid pairs: a single
// elimination bracket, or groups with a round robin each and the knockout the
// qualifiers play. It replaces a draw that already exists, but only while no
// result was loaded; generating it closes the category's sign-up for good.
//
// Group sizes follow the category's settings: as few groups of at most
// group_size as hold the pairs, sizes differing by at most one
// (draw.SplitGroups); a split the settings do not allow answers an error
// wrapping draw.ErrGroupCount.
func (m *Store) GenerateDraw(ctx context.Context, in DrawInput) error {
	return m.inCategory(ctx, in.CategoryID, func(e *engine, cat db.TournamentCategory) error {
		results, err := e.q.CountCategoryResults(e.ctx, cat.ID)
		if err != nil {
			return fmt.Errorf("tournaments: count results: %w", err)
		}
		if results > 0 {
			return ErrHasResults
		}
		pairs, err := e.q.ListDrawablePairs(e.ctx, cat.ID)
		if err != nil {
			return fmt.Errorf("tournaments: list pairs: %w", err)
		}
		ordered, err := orderPairs(pairs, in.SeedOrder, in.Shuffle)
		if err != nil {
			return err
		}
		if err := e.clearDraw(cat.ID); err != nil {
			return err
		}
		if cat.Format == FormatGroups {
			err = e.drawGroups(cat, ordered)
		} else {
			err = e.drawElimination(cat, ordered)
		}
		if err != nil {
			return err
		}
		return e.q.SetCategoryDraw(e.ctx, cat.ID)
	})
}

// orderPairs puts the drawable pairs in seed order: the owner's list first.
func orderPairs(pairs []db.TournamentPair, seedOrder []uuid.UUID, shuffle func([]uuid.UUID)) ([]db.TournamentPair, error) {
	byID := make(map[uuid.UUID]db.TournamentPair, len(pairs))
	for _, p := range pairs {
		byID[data.PgToUUID(p.ID)] = p
	}
	out := make([]db.TournamentPair, 0, len(pairs))
	for _, id := range seedOrder {
		p, ok := byID[id]
		if !ok {
			return nil, ErrUnknownPair
		}
		delete(byID, id)
		out = append(out, p)
	}
	var rest []uuid.UUID
	for _, p := range pairs {
		if _, left := byID[data.PgToUUID(p.ID)]; left {
			rest = append(rest, data.PgToUUID(p.ID))
		}
	}
	if shuffle != nil {
		shuffle(rest)
	}
	for _, id := range rest {
		out = append(out, byID[id])
	}
	return out, nil
}

func (e *engine) clearDraw(categoryID pgtype.UUID) error {
	if err := e.q.DeleteTournamentMatches(e.ctx, categoryID); err != nil {
		return fmt.Errorf("tournaments: clear matches: %w", err)
	}
	if err := e.q.ClearPairPlacements(e.ctx, categoryID); err != nil {
		return fmt.Errorf("tournaments: clear placements: %w", err)
	}
	if err := e.q.DeleteTournamentGroups(e.ctx, categoryID); err != nil {
		return fmt.Errorf("tournaments: clear groups: %w", err)
	}
	return nil
}

// entry is what fills a bracket slot: a known pair, or the Rank-th of a group.
type entry struct {
	pair pgtype.UUID
	src  *Slot
}

// firstRoundMatch is one first-round bracket match; bye means b never fills.
type firstRoundMatch struct {
	a, b entry
	bye  bool
}

func (e *engine) drawElimination(cat db.TournamentCategory, ordered []db.TournamentPair) error {
	if len(ordered) < 2 {
		return ErrNotEnoughPairs
	}
	for i, p := range ordered {
		if err := e.q.SetPairPlacement(e.ctx, db.SetPairPlacementParams{ID: p.ID, Seed: data.Int4ToPg(i + 1)}); err != nil {
			return fmt.Errorf("tournaments: seed pair: %w", err)
		}
	}
	first := draw.FirstRound(len(ordered))
	specs := make([]firstRoundMatch, len(first))
	for i, p := range first {
		specs[i] = firstRoundMatch{a: entry{pair: ordered[p.SeedA-1].ID}, bye: p.SeedB == 0}
		if p.SeedB != 0 {
			specs[i].b = entry{pair: ordered[p.SeedB-1].ID}
		}
	}
	return e.buildBracket(cat, specs)
}

func (e *engine) drawGroups(cat db.TournamentCategory, ordered []db.TournamentPair) error {
	sizes, err := draw.SplitGroups(len(ordered), int(cat.GroupSize), int(cat.QualifiersPerGroup))
	if err != nil {
		return err
	}
	groupIDs := make([]pgtype.UUID, len(sizes))
	for i, seeds := range draw.SnakeAssign(sizes) {
		g, err := e.q.InsertTournamentGroup(e.ctx, db.InsertTournamentGroupParams{
			CategoryID: cat.ID, ComplexID: cat.ComplexID, Name: groupName(i), Position: int32(i + 1), //nolint:gosec // G115: a handful of groups
		})
		if err != nil {
			return fmt.Errorf("tournaments: insert group: %w", err)
		}
		groupIDs[i] = g.ID
		members := make([]pgtype.UUID, len(seeds))
		for j, seed := range seeds {
			members[j] = ordered[seed].ID
			if err := e.q.SetPairPlacement(e.ctx, db.SetPairPlacementParams{
				ID: ordered[seed].ID, Seed: data.Int4ToPg(seed + 1), GroupID: g.ID,
			}); err != nil {
				return fmt.Errorf("tournaments: place pair: %w", err)
			}
		}
		for n, rr := range draw.RoundRobin(len(members)) {
			if _, err := e.q.InsertTournamentMatch(e.ctx, db.InsertTournamentMatchParams{
				CategoryID: cat.ID, ComplexID: cat.ComplexID, Stage: StageGroup, GroupID: g.ID,
				Round: int32(rr.Round), Position: int32(n), //nolint:gosec // G115: tiny
				Pair1ID: members[rr.A], Pair2ID: members[rr.B],
			}); err != nil {
				return fmt.Errorf("tournaments: insert group match: %w", err)
			}
		}
	}
	first := draw.KnockoutFirstRound(len(sizes), int(cat.QualifiersPerGroup))
	specs := make([]firstRoundMatch, len(first))
	for i, p := range first {
		specs[i] = firstRoundMatch{
			a: entry{src: &Slot{GroupID: data.PgToUUID(groupIDs[p.A.Group]), Rank: p.A.Rank}}, bye: !p.HasB,
		}
		if p.HasB {
			specs[i].b = entry{src: &Slot{GroupID: data.PgToUUID(groupIDs[p.B.Group]), Rank: p.B.Rank}}
		}
	}
	return e.buildBracket(cat, specs)
}

// groupName is "A", "B", ... and "G27" once the letters run out.
func groupName(i int) string {
	if i < 26 {
		return string(rune('A' + i))
	}
	return fmt.Sprintf("G%d", i+1)
}

// buildBracket writes every round of a single-elimination bracket whose first
// round is specs, links each match to the one its winner plays next, and lets
// the byes whose pair is already known advance.
func (e *engine) buildBracket(cat db.TournamentCategory, specs []firstRoundMatch) error {
	rounds := draw.Rounds(len(specs) * 2)
	ids := make([][]pgtype.UUID, rounds)
	for r := 1; r <= rounds; r++ {
		count := len(specs) >> (r - 1)
		ids[r-1] = make([]pgtype.UUID, count)
		for pos := 0; pos < count; pos++ {
			p := db.InsertTournamentMatchParams{
				CategoryID: cat.ID, ComplexID: cat.ComplexID, Stage: StageBracket,
				Round: int32(r), Position: int32(pos), //nolint:gosec // G115: tiny
			}
			if r == 1 {
				applySpec(&p, specs[pos])
			}
			row, err := e.q.InsertTournamentMatch(e.ctx, p)
			if err != nil {
				return fmt.Errorf("tournaments: insert bracket match: %w", err)
			}
			ids[r-1][pos] = row.ID
		}
	}
	for r := 1; r < rounds; r++ {
		for pos, id := range ids[r-1] {
			_, nextPos, slot := draw.Next(r, pos)
			if err := e.q.SetMatchNext(e.ctx, db.SetMatchNextParams{
				ID: id, NextMatchID: ids[r][nextPos], NextSlot: data.Int4ToPg(slot),
			}); err != nil {
				return fmt.Errorf("tournaments: link bracket: %w", err)
			}
		}
	}
	for i, s := range specs {
		if s.bye && s.a.pair.Valid {
			if err := e.settle(ids[0][i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func applySpec(p *db.InsertTournamentMatchParams, s firstRoundMatch) {
	p.Bye = s.bye
	p.Pair1ID, p.Pair2ID = s.a.pair, s.b.pair
	if s.a.src != nil {
		p.Slot1GroupID, p.Slot1Rank = data.UUIDToPg(s.a.src.GroupID), data.Int4ToPg(s.a.src.Rank)
	}
	if s.b.src != nil {
		p.Slot2GroupID, p.Slot2Rank = data.UUIDToPg(s.b.src.GroupID), data.Int4ToPg(s.b.src.Rank)
	}
}

// MovePair swaps two pairs of a drawn category: their places in the bracket's
// first round, or their groups. It is allowed only while no result was loaded.
func (m *Store) MovePair(ctx context.Context, categoryID, pairA, pairB uuid.UUID) error {
	return m.inCategory(ctx, categoryID, func(e *engine, cat db.TournamentCategory) error {
		if !cat.DrawGeneratedAt.Valid {
			return ErrNoDraw
		}
		results, err := e.q.CountCategoryResults(e.ctx, cat.ID)
		if err != nil {
			return fmt.Errorf("tournaments: count results: %w", err)
		}
		if results > 0 {
			return ErrHasResults
		}
		a, err := e.drawnPair(cat, pairA)
		if err != nil {
			return err
		}
		b, err := e.drawnPair(cat, pairB)
		if err != nil {
			return err
		}
		if a.ID == b.ID {
			return ErrInvalidMove
		}
		if cat.Format == FormatGroups && a.GroupID == b.GroupID {
			return ErrInvalidMove
		}
		if err := e.q.SetPairPlacement(e.ctx, db.SetPairPlacementParams{ID: a.ID, Seed: b.Seed, GroupID: b.GroupID}); err != nil {
			return fmt.Errorf("tournaments: move pair: %w", err)
		}
		if err := e.q.SetPairPlacement(e.ctx, db.SetPairPlacementParams{ID: b.ID, Seed: a.Seed, GroupID: a.GroupID}); err != nil {
			return fmt.Errorf("tournaments: move pair: %w", err)
		}
		return e.swapInMatches(cat, a.ID, b.ID)
	})
}

func (e *engine) drawnPair(cat db.TournamentCategory, id uuid.UUID) (db.TournamentPair, error) {
	pairs, err := e.q.ListCategoryPairs(e.ctx, cat.ID)
	if err != nil {
		return db.TournamentPair{}, fmt.Errorf("tournaments: list pairs: %w", err)
	}
	for _, p := range pairs {
		if data.PgToUUID(p.ID) == id && p.Seed.Valid && p.Status == PairActive {
			return p, nil
		}
	}
	return db.TournamentPair{}, ErrUnknownPair
}

// swapInMatches exchanges two pairs in the matches that name them: the group
// matches of their groups, or the bracket's first round, where a bye is then
// completed again for whoever now holds it.
func (e *engine) swapInMatches(cat db.TournamentCategory, a, b pgtype.UUID) error {
	matches, err := e.q.ListCategoryMatches(e.ctx, cat.ID)
	if err != nil {
		return fmt.Errorf("tournaments: read matches: %w", err)
	}
	swap := func(id pgtype.UUID) pgtype.UUID {
		switch id {
		case a:
			return b
		case b:
			return a
		}
		return id
	}
	for _, m := range matches {
		isRoundOne := m.Stage == StageBracket && m.Round == 1
		if cat.Format == FormatGroups && m.Stage == StageBracket {
			continue
		}
		if m.Stage == StageBracket && !isRoundOne {
			continue
		}
		p1, p2 := swap(m.Pair1ID), swap(m.Pair2ID)
		if p1 == m.Pair1ID && p2 == m.Pair2ID {
			continue
		}
		if err := e.q.SetMatchPairs(e.ctx, db.SetMatchPairsParams{ID: m.ID, Pair1ID: p1, Pair2ID: p2}); err != nil {
			return fmt.Errorf("tournaments: swap pairs: %w", err)
		}
	}
	return e.reapplyByes(cat.ID)
}

// reapplyByes completes every first-round bye again with the pair that holds it
// now, carrying it to the next round over whoever was there.
func (e *engine) reapplyByes(categoryID pgtype.UUID) error {
	matches, err := e.q.ListCategoryMatches(e.ctx, categoryID)
	if err != nil {
		return fmt.Errorf("tournaments: read matches: %w", err)
	}
	for _, m := range matches {
		if m.Bye && m.Pair1ID.Valid {
			if err := e.complete(m, m.Pair1ID, pgtype.Text{}, 0); err != nil {
				return err
			}
		}
	}
	return nil
}
