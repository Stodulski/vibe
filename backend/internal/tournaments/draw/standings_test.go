package draw_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/tournaments/draw"
)

func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.New()
	}
	return out
}

// result builds a played match from the pairs' sets, A against B.
func result(a, b uuid.UUID, sets ...draw.Set) draw.Result {
	out, err := draw.ParseResult(sets)
	if err != nil {
		panic(err)
	}
	return draw.Result{A: a, B: b, Outcome: out}
}

func order(rows []draw.Row) []uuid.UUID {
	out := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		out[i] = r.Pair
	}
	return out
}

func wantOrder(t *testing.T, rows []draw.Row, want ...uuid.UUID) {
	t.Helper()
	got := order(rows)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d: got pair %v want %v (rows %+v)", i, got[i], want[i], rows)
		}
		if rows[i].Rank != i+1 {
			t.Fatalf("position %d has rank %d", i, rows[i].Rank)
		}
	}
}

func TestStandingsByWins(t *testing.T) {
	p := ids(3)
	rows := draw.Standings(p, []draw.Result{
		result(p[0], p[1], draw.Set{A: 6, B: 4}, draw.Set{A: 6, B: 4}),
		result(p[0], p[2], draw.Set{A: 6, B: 4}, draw.Set{A: 6, B: 4}),
		result(p[1], p[2], draw.Set{A: 6, B: 4}, draw.Set{A: 6, B: 4}),
	})
	wantOrder(t, rows, p[0], p[1], p[2])
	if rows[0].Won != 2 || rows[0].Played != 2 || rows[2].Lost != 2 {
		t.Fatalf("rows: %+v", rows)
	}
}

func TestStandingsTiebreakSetDifference(t *testing.T) {
	// p0 and p1 each beat p2 once; p1 does it in straight sets, p0 needs a tiebreak,
	// so p1 is ahead although p0 is the better seed.
	p := ids(3)
	rows := draw.Standings(p, []draw.Result{
		result(p[0], p[2], draw.Set{A: 6, B: 4}, draw.Set{A: 4, B: 6}, draw.Set{A: 10, B: 8, Super: true}),
		result(p[1], p[2], draw.Set{A: 6, B: 4}, draw.Set{A: 6, B: 4}),
	})
	wantOrder(t, rows, p[1], p[0], p[2])
}

func TestStandingsTiebreakGameDifference(t *testing.T) {
	// Same wins, same sets: the bigger margin in games goes first.
	p := ids(3)
	rows := draw.Standings(p, []draw.Result{
		result(p[0], p[2], draw.Set{A: 6, B: 4}, draw.Set{A: 6, B: 4}),
		result(p[1], p[2], draw.Set{A: 6, B: 0}, draw.Set{A: 6, B: 0}),
	})
	wantOrder(t, rows, p[1], p[0], p[2])
}

func TestStandingsTiebreakHeadToHead(t *testing.T) {
	// B (seed 1) and A (seed 2) end level on wins, sets and games; A beat B, so A goes first.
	b, a, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	rows := draw.Standings([]uuid.UUID{b, a, c, d}, []draw.Result{
		result(a, b, draw.Set{A: 6, B: 4}, draw.Set{A: 6, B: 4}),
		result(b, d, draw.Set{A: 6, B: 4}, draw.Set{A: 6, B: 4}),
		result(c, a, draw.Set{A: 6, B: 4}, draw.Set{A: 6, B: 4}),
	})
	wantOrder(t, rows, c, a, b, d)
}

func TestStandingsASeedDecidesWhatNothingElseCan(t *testing.T) {
	// A three way cycle with identical scores: the seed order decides.
	p := ids(3)
	rows := draw.Standings(p, []draw.Result{
		result(p[0], p[1], draw.Set{A: 6, B: 4}, draw.Set{A: 6, B: 4}),
		result(p[1], p[2], draw.Set{A: 6, B: 4}, draw.Set{A: 6, B: 4}),
		result(p[2], p[0], draw.Set{A: 6, B: 4}, draw.Set{A: 6, B: 4}),
	})
	wantOrder(t, rows, p[0], p[1], p[2])
}

func TestStandingsWalkoverCountsAsAWin(t *testing.T) {
	p := ids(2)
	rows := draw.Standings(p, []draw.Result{{A: p[0], B: p[1], Outcome: draw.Walkover(1)}})
	if rows[0].Pair != p[1] || rows[0].Won != 1 || rows[0].SetsFor != 2 || rows[0].GamesFor != 12 {
		t.Errorf("rows: %+v", rows)
	}
}

func TestStandingsListsPairsWithNoMatchesYet(t *testing.T) {
	p := ids(3)
	rows := draw.Standings(p, nil)
	if len(rows) != 3 || rows[0].Pair != p[0] || rows[2].Pair != p[2] {
		t.Errorf("rows: %+v", rows)
	}
}
