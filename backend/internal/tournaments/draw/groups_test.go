package draw_test

import (
	"slices"
	"testing"

	"github.com/stodulski/vibe-server/internal/tournaments/draw"
)

func TestSplitGroups(t *testing.T) {
	cases := []struct {
		n, size, qualifiers int
		want                []int
	}{
		{3, 3, 2, []int{3}}, {6, 3, 2, []int{3, 3}}, {7, 4, 2, []int{7}}, {10, 4, 2, []int{5, 5}},
		{10, 3, 1, []int{4, 3, 3}}, {12, 4, 2, []int{4, 4, 4}}, {11, 4, 2, []int{6, 5}},
		{8, 5, 2, []int{8}}, {5, 5, 2, []int{5}}, {9, 3, 2, []int{3, 3, 3}},
		// group_size is the target: leftovers join existing groups.
		{7, 3, 2, []int{4, 3}}, {8, 3, 2, []int{4, 4}}, {5, 3, 2, []int{5}}, {4, 3, 2, []int{4}},
	}
	for _, c := range cases {
		got, err := draw.SplitGroups(c.n, c.size, c.qualifiers)
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("SplitGroups(%d,%d,%d) = %v, %v; want %v", c.n, c.size, c.qualifiers, got, err, c.want)
		}
	}
	// Bad settings, a group no larger than its qualifiers, a single qualifier.
	for _, c := range [][3]int{{4, 3, 3}, {4, 3, 0}, {2, 3, 2}, {3, 2, 2}, {1, 4, 1}, {0, 3, 2}} {
		if got, err := draw.SplitGroups(c[0], c[1], c[2]); err == nil {
			t.Errorf("SplitGroups(%d,%d,%d) = %v, want an error", c[0], c[1], c[2], got)
		}
	}
}

func TestGroupCount(t *testing.T) {
	for _, c := range [][3]int{{3, 3, 1}, {2, 3, 1}, {4, 3, 1}, {7, 3, 2}, {8, 3, 2}, {10, 4, 2}, {12, 4, 3}, {13, 4, 3}} {
		if got := draw.GroupCount(c[0], c[1]); got != c[2] {
			t.Errorf("GroupCount(%d,%d) = %d, want %d", c[0], c[1], got, c[2])
		}
	}
}

// Across 2..33 pairs and the usual settings: either the split is valid (sizes
// add up, differ by at most one, every group larger than the qualifiers, at
// least two qualifiers) or it is refused only when even the best split would
// leave a group too small.
func TestSplitGroupsRange(t *testing.T) {
	for _, cfg := range [][2]int{{3, 1}, {3, 2}, {4, 1}, {4, 2}, {4, 3}, {5, 2}, {6, 2}} {
		size, q := cfg[0], cfg[1]
		for n := 2; n <= 33; n++ {
			got, err := draw.SplitGroups(n, size, q)
			groups := max(1, n/size)
			if err != nil {
				if n/groups > q && groups*q >= 2 {
					t.Errorf("SplitGroups(%d,%d,%d) refused although %d groups work: %v", n, size, q, groups, err)
				}
				continue
			}
			sum := 0
			for _, s := range got {
				sum += s
				if s <= q || s < got[len(got)-1] || s > got[len(got)-1]+1 {
					t.Errorf("SplitGroups(%d,%d,%d) = %v: bad size %d", n, size, q, got, s)
				}
			}
			if sum != n || len(got) != groups || len(got)*q < 2 {
				t.Errorf("SplitGroups(%d,%d,%d) = %v: sum %d, groups %d", n, size, q, got, sum, len(got))
			}
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
