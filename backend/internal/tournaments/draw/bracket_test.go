package draw_test

import (
	"slices"
	"testing"

	"github.com/stodulski/vibe-server/internal/tournaments/draw"
)

func TestBracketSizeIsTheNextPowerOfTwo(t *testing.T) {
	cases := map[int]int{2: 2, 3: 4, 4: 4, 5: 8, 8: 8, 9: 16, 16: 16, 17: 32, 32: 32, 33: 64}
	for n, want := range cases {
		if got := draw.BracketSize(n); got != want {
			t.Errorf("BracketSize(%d) = %d, want %d", n, got, want)
		}
	}
}

func TestRounds(t *testing.T) {
	for size, want := range map[int]int{2: 1, 4: 2, 8: 3, 16: 4, 64: 6} {
		if got := draw.Rounds(size); got != want {
			t.Errorf("Rounds(%d) = %d, want %d", size, got, want)
		}
	}
}

func TestSeedOrderIsTheStandardPlacement(t *testing.T) {
	if got, want := draw.SeedOrder(8), []int{1, 8, 4, 5, 2, 7, 3, 6}; !slices.Equal(got, want) {
		t.Errorf("SeedOrder(8) = %v, want %v", got, want)
	}
	if got, want := draw.SeedOrder(16), []int{1, 16, 8, 9, 4, 13, 5, 12, 2, 15, 7, 10, 3, 14, 6, 11}; !slices.Equal(got, want) {
		t.Errorf("SeedOrder(16) = %v, want %v", got, want)
	}
}

// FirstRound must hold for every field from 2 to 33 pairs.
func TestFirstRoundForEveryFieldSize(t *testing.T) {
	for n := 2; n <= 33; n++ {
		size := draw.BracketSize(n)
		first := draw.FirstRound(n)
		if len(first) != size/2 {
			t.Fatalf("n=%d: %d pairings, want %d", n, len(first), size/2)
		}
		seen := map[int]bool{}
		byes := 0
		for i, p := range first {
			if p.Position != i {
				t.Errorf("n=%d: pairing %d has position %d", n, i, p.Position)
			}
			if p.SeedA == 0 {
				t.Fatalf("n=%d: pairing %d has no first seed", n, i)
			}
			for _, s := range []int{p.SeedA, p.SeedB} {
				if s == 0 {
					continue
				}
				if s > n {
					t.Errorf("n=%d: seed %d does not exist", n, s)
				}
				if seen[s] {
					t.Errorf("n=%d: seed %d placed twice", n, s)
				}
				seen[s] = true
			}
			if p.SeedB == 0 {
				byes++
			}
		}
		if len(seen) != n {
			t.Errorf("n=%d: %d seeds placed", n, len(seen))
		}
		if byes != size-n {
			t.Errorf("n=%d: %d byes, want %d", n, byes, size-n)
		}
		// The byes go to the top seeds.
		for _, p := range first {
			if p.SeedB == 0 && p.SeedA > size-n {
				t.Errorf("n=%d: seed %d has a bye but only the top %d should", n, p.SeedA, size-n)
			}
		}
		// Seeds 1 and 2 can only meet in the final: opposite halves of the draw.
		if size >= 4 {
			half := len(first) / 2
			if !hasSeed(first[:half], 1) || !hasSeed(first[half:], 2) {
				t.Errorf("n=%d: seeds 1 and 2 are not in opposite halves: %+v", n, first)
			}
		}
	}
}

func TestNext(t *testing.T) {
	cases := []struct{ round, pos, wantRound, wantPos, wantSlot int }{
		{1, 0, 2, 0, 1}, {1, 1, 2, 0, 2}, {1, 2, 2, 1, 1}, {1, 3, 2, 1, 2}, {2, 1, 3, 0, 2},
	}
	for _, c := range cases {
		r, p, s := draw.Next(c.round, c.pos)
		if r != c.wantRound || p != c.wantPos || s != c.wantSlot {
			t.Errorf("Next(%d,%d) = %d,%d,%d want %d,%d,%d", c.round, c.pos, r, p, s, c.wantRound, c.wantPos, c.wantSlot)
		}
	}
}

func hasSeed(ps []draw.Pairing, seed int) bool {
	for _, p := range ps {
		if p.SeedA == seed || p.SeedB == seed {
			return true
		}
	}
	return false
}
