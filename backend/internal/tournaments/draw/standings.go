package draw

import (
	"sort"

	"github.com/google/uuid"
)

// Result is a played match of a group: the two pairs and how it ended, Winner 1
// meaning A.
type Result struct {
	A, B    uuid.UUID
	Outcome Outcome
}

// Row is one pair's line of a group table.
type Row struct {
	Pair                   uuid.UUID
	Played, Won, Lost      int
	SetsFor, SetsAgainst   int
	GamesFor, GamesAgainst int
	Rank                   int
	seedIndex              int
}

// SetDiff is sets won minus sets lost.
func (r Row) SetDiff() int { return r.SetsFor - r.SetsAgainst }

// GameDiff is games won minus games lost.
func (r Row) GameDiff() int { return r.GamesFor - r.GamesAgainst }

// Standings ranks the pairs of a group (given in seed order) from the matches
// played so far: matches won, then set difference, then game difference (both
// only from the matches that have a score), then
// the head to head among the pairs still level (matches won against each
// other), and finally the seed order so the table is always the same.
func Standings(pairs []uuid.UUID, results []Result) []Row {
	rows := make([]Row, len(pairs))
	index := map[uuid.UUID]int{}
	for i, p := range pairs {
		rows[i] = Row{Pair: p, seedIndex: i}
		index[p] = i
	}
	h2h := map[[2]uuid.UUID]bool{} // winner, loser
	for _, res := range results {
		a, okA := index[res.A]
		b, okB := index[res.B]
		if !okA || !okB {
			continue
		}
		o := res.Outcome
		rows[a].add(o, o.SetsA, o.SetsB, o.GamesA, o.GamesB, o.Winner == 1)
		rows[b].add(o, o.SetsB, o.SetsA, o.GamesB, o.GamesA, o.Winner == 2)
		if o.Winner == 1 {
			h2h[[2]uuid.UUID{res.A, res.B}] = true
		} else {
			h2h[[2]uuid.UUID{res.B, res.A}] = true
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return lessByTotals(rows[i], rows[j]) })
	rows = breakTies(rows, h2h)
	for i := range rows {
		rows[i].Rank = i + 1
	}
	return rows
}

// add counts a played match for one side. The sets and games count only when
// the match has a score.
func (r *Row) add(o Outcome, setsFor, setsAgainst, gamesFor, gamesAgainst int, won bool) {
	r.Played++
	if won {
		r.Won++
	} else {
		r.Lost++
	}
	if o.NoScore {
		return
	}
	r.SetsFor += setsFor
	r.SetsAgainst += setsAgainst
	r.GamesFor += gamesFor
	r.GamesAgainst += gamesAgainst
}

// lessByTotals orders by wins, set difference and game difference, best first.
func lessByTotals(a, b Row) bool {
	if a.Won != b.Won {
		return a.Won > b.Won
	}
	if a.SetDiff() != b.SetDiff() {
		return a.SetDiff() > b.SetDiff()
	}
	return a.GameDiff() > b.GameDiff()
}

func sameTotals(a, b Row) bool { return !lessByTotals(a, b) && !lessByTotals(b, a) }

// breakTies reorders every run of pairs level on the totals by the matches won
// among themselves, then by seed.
func breakTies(rows []Row, h2h map[[2]uuid.UUID]bool) []Row {
	for start := 0; start < len(rows); {
		end := start + 1
		for end < len(rows) && sameTotals(rows[start], rows[end]) {
			end++
		}
		if end-start > 1 {
			run := rows[start:end]
			wins := map[uuid.UUID]int{}
			for _, a := range run {
				for _, b := range run {
					if h2h[[2]uuid.UUID{a.Pair, b.Pair}] {
						wins[a.Pair]++
					}
				}
			}
			sort.SliceStable(run, func(i, j int) bool {
				if wins[run[i].Pair] != wins[run[j].Pair] {
					return wins[run[i].Pair] > wins[run[j].Pair]
				}
				return run[i].seedIndex < run[j].seedIndex
			})
		}
		start = end
	}
	return rows
}
