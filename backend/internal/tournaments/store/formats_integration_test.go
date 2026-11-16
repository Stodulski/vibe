//go:build integration

package store_test

import (
	"testing"

	"github.com/stodulski/vibe-server/internal/tournaments/draw"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

func (f *fixture) formatted(t *testing.T, format string, sets, games int, super bool) (*tournamentstore.Tournament, *tournamentstore.Category) {
	t.Helper()
	return f.tournamentOf(t, tournamentstore.NewCategory{
		Name: "Short", Format: format, Capacity: 8, Price: 1_000_000, GroupSize: 3, QualifiersPerGroup: 2,
		SetsToWin: sets, GamesPerSet: games, SuperTiebreakDecider: super, MatchDurationMinutes: 60,
	})
}

func (f *fixture) setResult(t *testing.T, id tournamentstore.ResultInput) error {
	t.Helper()
	_, err := f.Stores.Tournaments.SetResult(f.ctx, id)
	return err
}

// S6b: the category's format is stored and read back, whatever the defaults are.
func TestACategoryKeepsItsMatchFormat(t *testing.T) {
	f := newFixture(t)
	_, cat := f.formatted(t, tournamentstore.FormatElimination, 1, 4, false)
	if cat.SetsToWin != 1 || cat.GamesPerSet != 4 || cat.SuperTiebreakDecider || cat.MatchDurationMinutes != 60 {
		t.Fatalf("category = %+v", cat)
	}
	want := draw.Format{SetsToWin: 1, GamesPerSet: 4}
	if got := cat.MatchFormat(); got != want {
		t.Errorf("MatchFormat = %+v, want %+v", got, want)
	}
}

// S6b: a category that plays one set of six games takes one finished set.
func TestABestOfOneCategoryTakesASingleSet(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.formatted(t, tournamentstore.FormatElimination, 1, 6, true)
	f.pairs(t, cat, 4)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	first := matchesOf(f.detail(t, tr), tournamentstore.StageBracket, 1)

	err := f.setResult(t, tournamentstore.ResultInput{MatchID: first[0].ID, Sets: []draw.Set{{A: 6, B: 3}, {A: 6, B: 4}}})
	requireErr(t, err, draw.ErrInvalidScore)
	err = f.setResult(t, tournamentstore.ResultInput{MatchID: first[0].ID, Sets: []draw.Set{{A: 6, B: 5}}})
	requireErr(t, err, draw.ErrInvalidScore)
	err = f.setResult(t, tournamentstore.ResultInput{MatchID: first[0].ID, Sets: []draw.Set{{A: 10, B: 7, Super: true}}})
	requireErr(t, err, draw.ErrInvalidScore)
	if err := f.setResult(t, tournamentstore.ResultInput{MatchID: first[0].ID, Sets: []draw.Set{{A: 7, B: 6}}}); err != nil {
		t.Fatalf("7-6: %v", err)
	}
	got := matchByID(f.detail(t, tr), first[0].ID)
	if got.Status != tournamentstore.MatchCompleted || *got.WinnerID != *first[0].Pair1ID {
		t.Errorf("match = %+v", got)
	}
}

// S6b: best of three without a super tiebreak plays a regular third set, in
// games of four when the category says so.
func TestShortSetsAndARegularDecidingSet(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.formatted(t, tournamentstore.FormatElimination, 2, 4, false)
	f.pairs(t, cat, 4)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	first := matchesOf(f.detail(t, tr), tournamentstore.StageBracket, 1)

	for _, sets := range map[string][]draw.Set{
		"six games":      {{A: 6, B: 4}, {A: 6, B: 3}},
		"super disabled": {{A: 4, B: 2}, {A: 2, B: 4}, {A: 10, B: 7, Super: true}},
		"four three":     {{A: 4, B: 3}, {A: 4, B: 1}},
	} {
		requireErr(t, f.setResult(t, tournamentstore.ResultInput{MatchID: first[0].ID, Sets: sets}), draw.ErrInvalidScore)
	}
	err := f.setResult(t, tournamentstore.ResultInput{MatchID: first[0].ID, Sets: []draw.Set{{A: 4, B: 2}, {A: 3, B: 5}, {A: 5, B: 4}}})
	if err != nil {
		t.Fatalf("4-2 3-5 5-4: %v", err)
	}
}

// S6b: a walkover in a group table is worth the category's sets of games-0.
func TestAGroupWalkoverCountsTheCategoryFormat(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.formatted(t, tournamentstore.FormatGroups, 1, 4, false)
	f.pairs(t, cat, 3)
	f.generate(t, cat, pairIDs(f.detail(t, tr)))
	d := f.detail(t, tr)
	m := matchesOf(d, tournamentstore.StageGroup, 0)[0]
	if err := f.setResult(t, tournamentstore.ResultInput{MatchID: m.ID, WalkoverLoser: 2}); err != nil {
		t.Fatalf("walkover: %v", err)
	}
	var found bool
	for _, st := range tournamentstore.GroupTables(f.detail(t, tr), cat.ID) {
		for _, row := range st.Rows {
			if row.Pair == *m.Pair1ID {
				found = true
				if row.Won != 1 || row.SetsFor != 1 || row.GamesFor != 4 || row.GamesAgainst != 0 {
					t.Errorf("row = %+v, want one set of 4-0", row)
				}
			}
		}
	}
	if !found {
		t.Fatal("the winner has no row in the table")
	}
}
