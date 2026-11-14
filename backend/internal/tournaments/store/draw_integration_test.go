//go:build integration

package store_test

import (
	"errors"

	"github.com/google/uuid"

	"testing"

	"github.com/stodulski/vibe-server/internal/tournaments/draw"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// S3a/S4: a single elimination draw is sized to the next power of two and the
// byes go to the top seeds, who are already in the second round.
func TestEliminationDrawSizesTheBracketAndGivesByesToTheTopSeeds(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 16, 3, 2)
	f.pairs(t, cat, 5)
	d := f.detail(t, tr)
	ids := pairIDs(d)
	f.generate(t, cat, []uuid.UUID{ids[4], ids[3], ids[2]}) // the last three signed up are the top seeds

	d = f.detail(t, tr)
	if got := len(d.Matches); got != 7 {
		t.Fatalf("%d matches, want 4+2+1", got)
	}
	byes := 0
	for _, m := range matchesOf(d, tournamentstore.StageBracket, 1) {
		if m.Bye {
			byes++
			if m.Status != tournamentstore.MatchCompleted || m.WinnerID == nil {
				t.Errorf("a bye must have advanced its pair: %+v", m)
			}
		}
	}
	if byes != 3 {
		t.Fatalf("%d byes, want 3 (8 slots, 5 pairs)", byes)
	}
	// Seeds 1, 2 and 3 sit in the second round already.
	second := matchesOf(d, tournamentstore.StageBracket, 2)
	filled := 0
	for _, m := range second {
		for _, p := range []*uuid.UUID{m.Pair1ID, m.Pair2ID} {
			if p != nil {
				filled++
			}
		}
	}
	if filled != 3 {
		t.Fatalf("%d pairs in the second round, want the 3 bye winners", filled)
	}
	for _, p := range d.Pairs {
		if p.Seed == nil {
			t.Errorf("pair %v was not seeded", p.ID)
		}
	}
	if d.Categories[0].DrawGeneratedAt == nil {
		t.Error("the draw must be recorded on the category")
	}
}

// S4: regenerating re-draws; it needs two paid pairs; held pairs are not drawn.
func TestDrawNeedsPaidPairs(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatElimination, 8, 3, 2)
	if err := f.Stores.Tournaments.GenerateDraw(f.ctx, tournamentstore.DrawInput{CategoryID: cat.ID}); !errors.Is(err, tournamentstore.ErrNotEnoughPairs) {
		t.Fatalf("an empty category: %v", err)
	}
	if _, err := f.signUp(cat, 1, "203.0.113.7"); err != nil { // held, never paid
		t.Fatal(err)
	}
	f.pairs(t, cat, 0)
	reg, err := f.signUp(cat, 2, "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.pay(reg, "mp-1"); err != nil {
		t.Fatal(err)
	}
	if err := f.Stores.Tournaments.GenerateDraw(f.ctx, tournamentstore.DrawInput{CategoryID: cat.ID}); !errors.Is(err, tournamentstore.ErrNotEnoughPairs) {
		t.Fatalf("one paid pair is not a draw: %v", err)
	}
	_ = tr
}

// S3b: groups follow the category's settings, snake seeded, with a round robin
// each, and the knockout is sized from the qualifiers.
func TestGroupsDrawFollowsTheCategorySettings(t *testing.T) {
	f := newFixture(t)
	tr, cat := f.tournament(t, tournamentstore.FormatGroups, 16, 3, 2)
	f.pairs(t, cat, 7)
	f.generate(t, cat, nil)

	d := f.detail(t, tr)
	if len(d.Groups) != 2 {
		t.Fatalf("%d groups, want 2 for 7 pairs with groups of 3 as the target", len(d.Groups))
	}
	sizes := map[string]int{}
	for _, p := range d.Pairs {
		if p.GroupID == nil {
			t.Fatalf("pair %v is in no group", p.ID)
		}
		for _, g := range d.Groups {
			if g.ID == *p.GroupID {
				sizes[g.Name]++
			}
		}
	}
	if sizes["A"] != 4 || sizes["B"] != 3 {
		t.Fatalf("group sizes %v, want A=4 B=3", sizes)
	}
	if got := len(matchesOf(d, tournamentstore.StageGroup, 0)); got != 6+3 {
		t.Fatalf("%d group matches, want 6+3", got)
	}
	// 2 groups x 2 qualifiers = 4: a semifinal round and a final, no byes.
	if got := len(matchesOf(d, tournamentstore.StageBracket, 0)); got != 3 {
		t.Fatalf("%d bracket matches, want 3", got)
	}
	for _, m := range matchesOf(d, tournamentstore.StageBracket, 1) {
		if m.Pair1ID != nil || m.Pair2ID != nil || m.Source1 == nil || m.Source2 == nil {
			t.Errorf("a first knockout match waits for its groups: %+v", m)
		}
	}
}

func TestGroupSettingsThatDoNotFitAreRefused(t *testing.T) {
	f := newFixture(t)
	_, cat := f.tournament(t, tournamentstore.FormatGroups, 16, 4, 3)
	f.pairs(t, cat, 4) // one group of 4, 3 qualify: fine; but 2 pairs do not fill a group above its qualifiers
	if err := f.Stores.Tournaments.GenerateDraw(f.ctx, tournamentstore.DrawInput{CategoryID: cat.ID}); err != nil {
		t.Fatalf("4 pairs, 1 group, 3 qualify: %v", err)
	}
	_, cat2 := f.tournament(t, tournamentstore.FormatGroups, 16, 3, 2)
	f.pairs(t, cat2, 2)
	err := f.Stores.Tournaments.GenerateDraw(f.ctx, tournamentstore.DrawInput{CategoryID: cat2.ID})
	if !errors.Is(err, draw.ErrGroupCount) {
		t.Fatalf("2 pairs cannot form a group of 3 with 2 qualifying: %v", err)
	}
}
