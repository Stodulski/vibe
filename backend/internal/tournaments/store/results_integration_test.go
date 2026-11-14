//go:build integration

package store_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/tournaments/draw"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// winBy loads a straight-sets win for the given pair of the match.
func (f *fixture) winBy(t *testing.T, m *tournamentstore.Match, pair uuid.UUID) *tournamentstore.Match {
	t.Helper()
	side := 1
	if *m.Pair2ID == pair {
		side = 2
	}
	return f.win(t, m, side)
}

func matchByID(d *tournamentstore.Detail, id uuid.UUID) *tournamentstore.Match {
	for _, m := range d.Matches {
		if m.ID == id {
			return m
		}
	}
	return nil
}

// S6: the winner advances by itself to the next match.
func TestTheWinnerAdvancesToTheNextMatch(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	f.pairs(t, cat, 4)
	ids := pairIDs(f.detail(t, tr))
	f.generate(t, cat, ids)

	d := f.detail(t, tr)
	first := matchesOf(d, tournamentstore.StageBracket, 1)
	final := matchesOf(d, tournamentstore.StageBracket, 2)[0]
	if final.Pair1ID != nil || final.Pair2ID != nil {
		t.Fatal("the final starts empty")
	}
	// Seeds 1 and 4 meet, and 2 and 3.
	won := f.winBy(t, first[0], *first[0].Pair2ID)
	if won.Status != tournamentstore.MatchCompleted || *won.WinnerID != *first[0].Pair2ID {
		t.Fatalf("result not recorded: %+v", won)
	}
	d = f.detail(t, tr)
	if got := matchByID(d, final.ID); got.Pair1ID == nil || *got.Pair1ID != *first[0].Pair2ID || got.Pair2ID != nil {
		t.Fatalf("the winner must take slot 1 of the final: %+v", got)
	}
	f.winBy(t, first[1], *first[1].Pair1ID)
	d = f.detail(t, tr)
	if got := matchByID(d, final.ID); got.Pair2ID == nil || *got.Pair2ID != *first[1].Pair1ID {
		t.Fatalf("the other winner must take slot 2: %+v", got)
	}
}

func TestInvalidScoresAndNotReadyMatchesAreRefused(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	f.pairs(t, cat, 4)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	d := f.detail(t, tr)
	first := matchesOf(d, tournamentstore.StageBracket, 1)[0]
	final := matchesOf(d, tournamentstore.StageBracket, 2)[0]

	_, err := f.Stores.Tournaments.SetResult(f.ctx, tournamentstore.ResultInput{MatchID: first.ID, Sets: []draw.Set{{A: 6, B: 5}, {A: 6, B: 3}}})
	requireErr(t, err, draw.ErrInvalidScore)
	_, err = f.Stores.Tournaments.SetResult(f.ctx, tournamentstore.ResultInput{MatchID: final.ID, Sets: []draw.Set{{A: 6, B: 4}, {A: 6, B: 3}}})
	requireErr(t, err, tournamentstore.ErrMatchNotReady)
	// A super tiebreak as the third set is a result.
	_, err = f.Stores.Tournaments.SetResult(f.ctx, tournamentstore.ResultInput{
		MatchID: first.ID, Sets: []draw.Set{{A: 6, B: 4}, {A: 3, B: 6}, {A: 10, B: 8, Super: true}},
	})
	if err != nil {
		t.Fatalf("super tiebreak: %v", err)
	}
}

// S6: W.O. is possible and advances the other pair.
func TestWalkoverAdvancesTheOtherPair(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	f.pairs(t, cat, 4)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	d := f.detail(t, tr)
	first := matchesOf(d, tournamentstore.StageBracket, 1)[0]

	got, err := f.Stores.Tournaments.SetResult(f.ctx, tournamentstore.ResultInput{MatchID: first.ID, WalkoverLoser: 1})
	if err != nil || got.WalkoverLoser != 1 || *got.WinnerID != *first.Pair2ID {
		t.Fatalf("walkover = %+v, %v", got, err)
	}
	_, err = f.Stores.Tournaments.SetResult(f.ctx, tournamentstore.ResultInput{MatchID: first.ID, WalkoverLoser: 3})
	requireErr(t, err, draw.ErrInvalidScore)
}

// S6: a result can be corrected while the next match has none.
func TestACorrectionIsAllowedWhileTheNextMatchHasNoResult(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	f.pairs(t, cat, 4)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	d := f.detail(t, tr)
	first := matchesOf(d, tournamentstore.StageBracket, 1)
	final := matchesOf(d, tournamentstore.StageBracket, 2)[0]

	f.winBy(t, first[0], *first[0].Pair1ID)
	f.winBy(t, first[0], *first[0].Pair2ID) // corrected: the other pair won
	d = f.detail(t, tr)
	if got := matchByID(d, final.ID); *got.Pair1ID != *first[0].Pair2ID {
		t.Fatalf("the final must follow the correction: %+v", got)
	}

	f.winBy(t, first[1], *first[1].Pair1ID)
	final = matchByID(f.detail(t, tr), final.ID)
	f.winBy(t, final, *final.Pair1ID) // the first semifinal's winner takes the final
	_, err := f.Stores.Tournaments.SetResult(f.ctx, tournamentstore.ResultInput{
		MatchID: first[0].ID, Sets: []draw.Set{{A: 6, B: 0}, {A: 6, B: 0}},
	})
	requireErr(t, err, tournamentstore.ErrCorrectionLocked)
}

// S4: regenerating and moving are allowed only while no result is loaded.
func TestDrawChangesAreRefusedOnceAResultExists(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	f.pairs(t, cat, 4)
	ids := pairIDs(f.detail(t, tr))
	f.generate(t, cat, ids)
	f.generate(t, cat, []uuid.UUID{ids[3], ids[2]}) // regenerating before any result is fine

	d := f.detail(t, tr)
	f.win(t, matchesOf(d, tournamentstore.StageBracket, 1)[0], 1)
	err := f.Stores.Tournaments.GenerateDraw(f.ctx, tournamentstore.DrawInput{CategoryID: cat.ID})
	requireErr(t, err, tournamentstore.ErrHasResults)
	requireErr(t, f.Stores.Tournaments.MovePair(f.ctx, cat.ID, ids[0], ids[1]), tournamentstore.ErrHasResults)
}

// S4: pairs can be moved by hand before the tournament starts.
func TestMovingPairsSwapsTheirPlaces(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	f.pairs(t, cat, 4)
	ids := pairIDs(f.detail(t, tr))
	f.generate(t, cat, ids)
	before := matchesOf(f.detail(t, tr), tournamentstore.StageBracket, 1)

	if err := f.Stores.Tournaments.MovePair(f.ctx, cat.ID, ids[0], ids[1]); err != nil {
		t.Fatalf("MovePair: %v", err)
	}
	after := matchesOf(f.detail(t, tr), tournamentstore.StageBracket, 1)
	if *after[0].Pair1ID != ids[1] || *before[0].Pair1ID != ids[0] {
		t.Fatalf("seed 1 and seed 2 must trade places: before %v after %v", *before[0].Pair1ID, *after[0].Pair1ID)
	}
	requireErr(t, f.Stores.Tournaments.MovePair(f.ctx, cat.ID, ids[0], ids[0]), tournamentstore.ErrInvalidMove)
	requireErr(t, f.Stores.Tournaments.MovePair(f.ctx, cat.ID, ids[0], uuid.New()), tournamentstore.ErrUnknownPair)
}

// A move also carries a bye: the pair that now holds it is in the second round.
func TestMovingAPairIntoAByeCarriesItToTheSecondRound(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	f.pairs(t, cat, 3) // size 4, one bye for seed 1
	ids := pairIDs(f.detail(t, tr))
	f.generate(t, cat, ids)
	if err := f.Stores.Tournaments.MovePair(f.ctx, cat.ID, ids[0], ids[2]); err != nil {
		t.Fatal(err)
	}
	d := f.detail(t, tr)
	final := matchesOf(d, tournamentstore.StageBracket, 2)[0]
	if final.Pair1ID == nil || *final.Pair1ID != ids[2] {
		t.Fatalf("the pair that took the bye must be in the final: %+v", final)
	}
}

// S3b/S6: group standings fill the bracket when the group finishes, and a
// correction of the table is allowed until the bracket has played.
func TestAFinishedGroupFillsItsQualifiersIntoTheBracket(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatGroups, 8, 3, 2)
	f.pairs(t, cat, 3)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	d := f.detail(t, tr)
	seed := map[int]uuid.UUID{}
	for _, p := range d.Pairs {
		seed[*p.Seed] = p.ID
	}
	final := matchesOf(d, tournamentstore.StageBracket, 1)[0]
	if final.Pair1ID != nil {
		t.Fatal("the bracket waits for the group")
	}
	// Seed 1 beats everybody, seed 2 beats seed 3.
	for _, m := range matchesOf(d, tournamentstore.StageGroup, 0) {
		winner := seed[1]
		if *m.Pair1ID != seed[1] && *m.Pair2ID != seed[1] {
			winner = seed[2]
		}
		f.winBy(t, m, winner)
		got := matchByID(f.detail(t, tr), final.ID)
		if m != matchesOf(d, tournamentstore.StageGroup, 0)[2] && got.Pair1ID != nil {
			t.Fatal("the bracket must wait until every group match is played")
		}
	}
	got := matchByID(f.detail(t, tr), final.ID)
	if got.Pair1ID == nil || *got.Pair1ID != seed[1] || got.Pair2ID == nil || *got.Pair2ID != seed[2] {
		t.Fatalf("the final must be the group's winner against its runner-up: %+v", got)
	}

	// Correcting a group result re-ranks the table and the bracket follows.
	gm := matchesOf(d, tournamentstore.StageGroup, 0)
	for _, m := range gm {
		if (*m.Pair1ID == seed[2] && *m.Pair2ID == seed[3]) || (*m.Pair1ID == seed[3] && *m.Pair2ID == seed[2]) {
			f.winBy(t, m, seed[3])
		}
	}
	got = matchByID(f.detail(t, tr), final.ID)
	if *got.Pair2ID != seed[3] {
		t.Fatalf("after the correction seed 3 is the runner-up: %+v", got)
	}

	// Once the final has a result the table is final.
	f.winBy(t, got, seed[1])
	_, err := f.Stores.Tournaments.SetResult(f.ctx, tournamentstore.ResultInput{MatchID: gm[0].ID, Sets: []draw.Set{{A: 6, B: 0}, {A: 6, B: 0}}})
	requireErr(t, err, tournamentstore.ErrCorrectionLocked)
}

// S8: after the draw a pair that leaves forfeits: its matches are walkovers.
func TestAWithdrawnPairForfeitsItsMatches(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	regs := f.pairs(t, cat, 4)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	d := f.detail(t, tr)
	first := matchesOf(d, tournamentstore.StageBracket, 1)
	pair, err := f.Stores.Tournaments.PairOf(f.ctx, regs[0].ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.Stores.Tournaments.WithdrawPair(f.ctx, regs[0].ID); err != nil {
		t.Fatal(err)
	}
	d = f.detail(t, tr)
	m := matchByID(d, first[0].ID)
	if m.Status != tournamentstore.MatchCompleted || m.WalkoverLoser == 0 || *m.WinnerID == pair.ID {
		t.Fatalf("the withdrawn pair's match must be a walkover for the opponent: %+v", m)
	}
	// The opponent is in the final, waiting for the other semifinal.
	final := matchesOf(d, tournamentstore.StageBracket, 2)[0]
	if final.Pair1ID == nil || *final.Pair1ID == pair.ID {
		t.Fatalf("the opponent advances: %+v", final)
	}
	// A pair of another registration that is not a tournament pair is a no-op.
	if err := f.Stores.Tournaments.WithdrawPair(f.ctx, uuid.New()); err != nil {
		t.Fatalf("not a pair: %v", err)
	}
}

// A withdrawn pair that is waiting for its opponent loses when the opponent arrives.
func TestAMatchAgainstAWithdrawnPairIsAWalkoverWhenTheOpponentArrives(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	regs := f.pairs(t, cat, 3) // seed 1 has a bye and waits in the final
	ids := pairIDs(f.detail(t, tr))
	f.generate(t, cat, ids)
	if err := f.Stores.Tournaments.WithdrawPair(f.ctx, regs[0].ID); err != nil {
		t.Fatal(err)
	}
	d := f.detail(t, tr)
	semi := matchesOf(d, tournamentstore.StageBracket, 1)
	for _, m := range semi {
		if !m.Bye {
			f.winBy(t, m, *m.Pair1ID)
		}
	}
	final := matchesOf(f.detail(t, tr), tournamentstore.StageBracket, 2)[0]
	if final.Status != tournamentstore.MatchCompleted || final.WalkoverLoser == 0 {
		t.Fatalf("the final must be a walkover against the withdrawn seed 1: %+v", final)
	}
}

// S6d: the owner can load only who won, and the winner advances like with a
// score. The match keeps no sets.
func TestAWinnerOnlyResultAdvancesTheWinner(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	f.pairs(t, cat, 4)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	d := f.detail(t, tr)
	first := matchesOf(d, tournamentstore.StageBracket, 1)
	final := matchesOf(d, tournamentstore.StageBracket, 2)[0]

	got, err := f.Stores.Tournaments.SetResult(f.ctx, tournamentstore.ResultInput{MatchID: first[0].ID, Winner: 2})
	if err != nil {
		t.Fatalf("winner only: %v", err)
	}
	if got.Status != tournamentstore.MatchCompleted || *got.WinnerID != *first[0].Pair2ID || len(got.Sets) != 0 || got.WalkoverLoser != 0 {
		t.Fatalf("result = %+v", got)
	}
	if next := matchByID(f.detail(t, tr), final.ID); next.Pair1ID == nil || *next.Pair1ID != *first[0].Pair2ID {
		t.Fatalf("the winner must reach the final: %+v", next)
	}
	// A winner-only result is corrected like any other, by a score too.
	if _, err := f.Stores.Tournaments.SetResult(f.ctx, tournamentstore.ResultInput{
		MatchID: first[0].ID, Sets: []draw.Set{{A: 6, B: 0}, {A: 6, B: 0}},
	}); err != nil {
		t.Fatalf("correct with a score: %v", err)
	}
	if next := matchByID(f.detail(t, tr), final.ID); *next.Pair1ID != *first[0].Pair1ID {
		t.Fatalf("the final must follow the correction: %+v", next)
	}
}

// S6d: exactly one of the three shapes, and the winner is a side.
func TestAResultTakesExactlyOneShape(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	f.pairs(t, cat, 4)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	first := matchesOf(f.detail(t, tr), tournamentstore.StageBracket, 1)[0]
	score := []draw.Set{{A: 6, B: 0}, {A: 6, B: 0}}

	for name, in := range map[string]tournamentstore.ResultInput{
		"winner and sets":     {Winner: 1, Sets: score},
		"winner and walkover": {Winner: 1, WalkoverLoser: 2},
		"sets and walkover":   {Sets: score, WalkoverLoser: 2},
		"nothing":             {},
		"winner is a side":    {Winner: 3},
	} {
		in.MatchID = first.ID
		_, err := f.Stores.Tournaments.SetResult(f.ctx, in)
		if !errors.Is(err, draw.ErrInvalidScore) {
			t.Errorf("%s: err = %v, want an invalid score", name, err)
		}
	}
	if got := matchByID(f.detail(t, tr), first.ID); got.Status != tournamentstore.MatchPending {
		t.Errorf("a refused result must not be recorded: %+v", got)
	}
}

// S6d: a group with scored and unscored matches ranks by wins, and the figures
// come only from the scored ones. The group still finishes and fills the bracket.
func TestAGroupWithUnscoredMatchesRanksByWinsAndFeedsTheBracket(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.formatted(t, tournamentstore.FormatGroups, 2, 6, true)
	f.pairs(t, cat, 3)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	d := f.detail(t, tr)
	var group []*tournamentstore.Match
	for _, m := range d.Matches {
		if m.Stage == tournamentstore.StageGroup {
			group = append(group, m)
		}
	}
	if len(group) != 3 {
		t.Fatalf("three pairs play three matches, got %d", len(group))
	}
	// Side 1 wins every match: the first two without a score, the last with one.
	for _, m := range group[:2] {
		if _, err := f.Stores.Tournaments.SetResult(f.ctx, tournamentstore.ResultInput{MatchID: m.ID, Winner: 1}); err != nil {
			t.Fatalf("winner only: %v", err)
		}
	}
	f.win(t, group[2], 1)

	tables := tournamentstore.GroupTables(f.detail(t, tr), cat.ID)
	if len(tables) != 1 {
		t.Fatalf("tables = %+v", tables)
	}
	rows := tables[0].Rows
	if rows[0].Won < rows[1].Won || rows[1].Won < rows[2].Won {
		t.Errorf("rows must be ordered by wins: %+v", rows)
	}
	var sets int
	for _, row := range rows {
		sets += row.SetsFor
	}
	if sets != 2 {
		t.Errorf("only the scored match adds sets (2 for its winner), got %d: %+v", sets, rows)
	}
	for _, m := range f.detail(t, tr).Matches {
		if m.Stage == tournamentstore.StageBracket && m.Source1 != nil && m.Pair1ID == nil {
			t.Errorf("the finished group must fill its qualifiers into the bracket: %+v", m)
		}
	}
}
