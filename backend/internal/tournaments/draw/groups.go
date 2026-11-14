package draw

import (
	"errors"
	"fmt"
	"sort"
)

// ErrGroupCount is returned when n pairs cannot be split into groups the
// category's settings allow.
var ErrGroupCount = errors.New("the pairs cannot be split into groups with those settings")

// GroupCount is how many groups n pairs need when a group holds at most
// groupSize: the fewest that fit.
func GroupCount(n, groupSize int) int {
	if groupSize < 1 {
		return 0
	}
	return (n + groupSize - 1) / groupSize
}

// SplitGroups splits n pairs into the fewest groups of at most groupSize,
// sizes differing by at most one (the bigger groups first, so the best seeds
// land in them). Every group must be larger than qualifiers, so someone is
// left out of each, and the knockout needs at least two qualifiers in all.
func SplitGroups(n, groupSize, qualifiers int) ([]int, error) {
	groups := GroupCount(n, groupSize)
	if groups < 1 || qualifiers < 1 || qualifiers >= groupSize {
		return nil, fmt.Errorf("%w: %d pairs, groups of %d, %d qualify", ErrGroupCount, n, groupSize, qualifiers)
	}
	base, extra := n/groups, n%groups
	if base <= qualifiers {
		return nil, fmt.Errorf("%w: %d pairs, groups of %d, %d qualify", ErrGroupCount, n, groupSize, qualifiers)
	}
	if groups*qualifiers < 2 {
		return nil, fmt.Errorf("%w: fewer than two qualifiers", ErrGroupCount)
	}
	sizes := make([]int, groups)
	for i := range sizes {
		sizes[i] = base
		if i < extra {
			sizes[i]++
		}
	}
	return sizes, nil
}

// SnakeAssign deals seeds 0..n-1 (best first) into groups of the given sizes
// the way a snake goes: left to right, then right to left, skipping the groups
// that are full. It returns, per group, its seeds best first.
func SnakeAssign(sizes []int) [][]int {
	groups := make([][]int, len(sizes))
	total := 0
	for _, s := range sizes {
		total += s
	}
	seed := 0
	for row := 0; seed < total; row++ {
		for i := range sizes {
			g := i
			if row%2 == 1 {
				g = len(sizes) - 1 - i
			}
			if len(groups[g]) < sizes[g] && seed < total {
				groups[g] = append(groups[g], seed)
				seed++
			}
		}
	}
	return groups
}

// RRMatch is one round-robin match between two members of a group, by index.
type RRMatch struct {
	Round int
	A, B  int
}

// RoundRobin lists every pairing of k members once, grouped in rounds by the
// circle method: in a round nobody plays twice, and with an odd k one member
// rests each round.
func RoundRobin(k int) []RRMatch {
	if k < 2 {
		return nil
	}
	players := make([]int, 0, k+1)
	for i := 0; i < k; i++ {
		players = append(players, i)
	}
	if k%2 == 1 {
		players = append(players, -1)
	}
	n := len(players)
	var out []RRMatch
	for round := 1; round < n; round++ {
		for i := 0; i < n/2; i++ {
			a, b := players[i], players[n-1-i]
			if a < 0 || b < 0 {
				continue
			}
			out = append(out, RRMatch{Round: round, A: a, B: b})
		}
		// Rotate everyone but the first.
		last := players[n-1]
		copy(players[2:], players[1:n-1])
		players[1] = last
	}
	return out
}

// Qualifier is a place in the knockout earned in a group: the group's index and
// the rank in its table (1 is the winner).
type Qualifier struct {
	Group, Rank int
}

// Qualifiers lists who advances from the given number of groups, advance per
// group, best seed first: every group winner (in group order), then every
// runner-up, and so on.
func Qualifiers(groups, advance int) []Qualifier {
	out := make([]Qualifier, 0, groups*advance)
	for rank := 1; rank <= advance; rank++ {
		for g := 0; g < groups; g++ {
			out = append(out, Qualifier{Group: g, Rank: rank})
		}
	}
	return out
}

// KnockoutPairing is one first-round knockout match between group qualifiers.
// HasB is false when A has a bye.
type KnockoutPairing struct {
	Position int
	A, B     Qualifier
	HasB     bool
}

// KnockoutFirstRound lays out the first round of the bracket the group winners
// and runners-up play: the standard seeding with the qualifiers as the seeds,
// then pairings of two members of the same group are swapped away where another
// qualifier of the same rank can take the place, so groupmates do not meet
// again in the first round whenever that is possible.
func KnockoutFirstRound(groups, advance int) []KnockoutPairing {
	quals := Qualifiers(groups, advance)
	first := FirstRound(len(quals))
	out := make([]KnockoutPairing, len(first))
	for i, p := range first {
		out[i] = KnockoutPairing{Position: p.Position, A: quals[p.SeedA-1]}
		if p.SeedB != 0 {
			out[i].B, out[i].HasB = quals[p.SeedB-1], true
		}
	}
	separateGroupmates(out)
	return out
}

// separateGroupmates swaps the B side of each same-group pairing with the same
// rank elsewhere when that clears this pairing without creating another.
func separateGroupmates(pairings []KnockoutPairing) {
	for i := range pairings {
		p := &pairings[i]
		if !p.HasB || p.A.Group != p.B.Group {
			continue
		}
		for _, j := range candidateOrder(len(pairings), i) {
			if swapB(&pairings[i], &pairings[j]) {
				break
			}
		}
	}
}

// candidateOrder is every index but self, nearest first, so a swap moves the
// qualifier as little as it can.
func candidateOrder(n, self int) []int {
	out := make([]int, 0, n-1)
	for j := 0; j < n; j++ {
		if j != self {
			out = append(out, j)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return abs(out[a]-self) < abs(out[b]-self) })
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// swapB tries to exchange the B qualifier of p with the same-rank qualifier on
// either side of q; it keeps the swap only when no groupmates meet in p or q.
func swapB(p, q *KnockoutPairing) bool {
	b := p.B
	if q.HasB && q.B.Rank == b.Rank && q.B.Group != p.A.Group && q.A.Group != b.Group {
		p.B, q.B = q.B, p.B
		return true
	}
	if q.A.Rank == b.Rank && q.A.Group != p.A.Group && (!q.HasB || q.B.Group != b.Group) {
		p.B, q.A = q.A, p.B
		return true
	}
	return false
}
