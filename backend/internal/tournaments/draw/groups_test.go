package draw_test

import (
	"slices"
	"testing"

	"github.com/stodulski/vibe-server/internal/tournaments/draw"
)

func TestSplitGroups(t *testing.T) {
	good := []struct {
		n, groups int
		want      []int
	}{
		{3, 1, []int{3}}, {4, 1, []int{4}}, {6, 2, []int{3, 3}}, {7, 2, []int{4, 3}},
		{8, 2, []int{4, 4}}, {10, 3, []int{4, 3, 3}}, {12, 3, []int{4, 4, 4}}, {11, 3, []int{4, 4, 3}},
	}
	for _, c := range good {
		got, err := draw.SplitGroups(c.n, c.groups)
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("SplitGroups(%d,%d) = %v, %v; want %v", c.n, c.groups, got, err, c.want)
		}
	}
	for _, c := range [][2]int{{5, 1}, {8, 3}, {2, 1}, {9, 2}, {4, 0}, {13, 3}} {
		if got, err := draw.SplitGroups(c[0], c[1]); err == nil {
			t.Errorf("SplitGroups(%d,%d) = %v, want an error", c[0], c[1], got)
		}
	}
}

func TestValidGroupCounts(t *testing.T) {
	cases := map[int][]int{3: {1}, 6: {2}, 10: {3}, 12: {3, 4}, 5: nil, 2: nil, 24: {6, 7, 8}}
	for n, want := range cases {
		if got := draw.ValidGroupCounts(n); !slices.Equal(got, want) {
			t.Errorf("ValidGroupCounts(%d) = %v, want %v", n, got, want)
		}
	}
}

func TestSnakeAssign(t *testing.T) {
	got := draw.SnakeAssign([]int{4, 3, 3})
	want := [][]int{{0, 5, 6, 9}, {1, 4, 7}, {2, 3, 8}}
	for i := range want {
		if !slices.Equal(got[i], want[i]) {
			t.Fatalf("SnakeAssign = %v, want %v", got, want)
		}
	}
	// Every seed lands in exactly one group.
	got = draw.SnakeAssign([]int{4, 4, 4, 3, 3})
	seen := map[int]bool{}
	for _, g := range got {
		for _, s := range g {
			if seen[s] {
				t.Fatalf("seed %d twice", s)
			}
			seen[s] = true
		}
	}
	if len(seen) != 18 {
		t.Fatalf("%d seeds placed, want 18", len(seen))
	}
	// The top seeds sit in different groups.
	for g := range got[:5] {
		if got[g][0] != g {
			t.Errorf("group %d starts with seed %d, want %d", g, got[g][0], g)
		}
	}
}

func TestRoundRobinPlaysEveryPairOnce(t *testing.T) {
	for _, k := range []int{2, 3, 4, 5, 6} {
		matches := draw.RoundRobin(k)
		if want := k * (k - 1) / 2; len(matches) != want {
			t.Fatalf("k=%d: %d matches, want %d", k, len(matches), want)
		}
		met := map[[2]int]bool{}
		busy := map[[2]int]bool{} // (round, player)
		for _, m := range matches {
			lo, hi := min(m.A, m.B), max(m.A, m.B)
			if lo == hi || lo < 0 || hi >= k {
				t.Fatalf("k=%d: bad match %+v", k, m)
			}
			if met[[2]int{lo, hi}] {
				t.Errorf("k=%d: %d-%d plays twice", k, lo, hi)
			}
			met[[2]int{lo, hi}] = true
			for _, p := range []int{m.A, m.B} {
				if busy[[2]int{m.Round, p}] {
					t.Errorf("k=%d: player %d plays twice in round %d", k, p, m.Round)
				}
				busy[[2]int{m.Round, p}] = true
			}
		}
	}
}

func TestKnockoutKeepsGroupmatesApart(t *testing.T) {
	for groups := 2; groups <= 8; groups++ {
		for advance := 1; advance <= 3; advance++ {
			if groups*advance < 2 || advance > groups {
				continue
			}
			for _, p := range draw.KnockoutFirstRound(groups, advance) {
				if p.HasB && p.A.Group == p.B.Group {
					t.Errorf("groups=%d advance=%d: %+v is a same-group rematch", groups, advance, p)
				}
			}
		}
	}
}

func TestKnockoutFirstRoundShape(t *testing.T) {
	// Two groups, top two: A1-B2 and B1-A2.
	got := draw.KnockoutFirstRound(2, 2)
	want := []draw.KnockoutPairing{
		{Position: 0, A: draw.Qualifier{Group: 0, Rank: 1}, B: draw.Qualifier{Group: 1, Rank: 2}, HasB: true},
		{Position: 1, A: draw.Qualifier{Group: 1, Rank: 1}, B: draw.Qualifier{Group: 0, Rank: 2}, HasB: true},
	}
	// Positions are the standard order: seed 1 first, seed 2 last.
	if len(got) != 2 || got[0].A != want[0].A || got[0].B != want[0].B || got[1].A != want[1].A || got[1].B != want[1].B {
		t.Errorf("KnockoutFirstRound(2,2) = %+v", got)
	}
	// Three groups, top two: six qualifiers, size 8, the two group winners seeded first get byes.
	byes := 0
	for _, p := range draw.KnockoutFirstRound(3, 2) {
		if !p.HasB {
			byes++
		}
	}
	if byes != 2 {
		t.Errorf("3 groups x 2: %d byes, want 2", byes)
	}
}
