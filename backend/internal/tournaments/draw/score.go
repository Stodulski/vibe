package draw

import (
	"errors"
	"fmt"
)

// Set is one set of a padel match as the owner loads it: the games each side
// won. A Super set is the match tiebreak that replaces the third set, played to
// ten points with a two point lead, where A and B are points.
type Set struct {
	A, B  int
	Super bool
}

// Outcome is a decided match: who won (1 for the first pair, 2 for the second)
// and the sets and games each side took, the figures a group table adds up. A
// match tiebreak counts as one set and one game for its winner.
type Outcome struct {
	Winner       int
	SetsA, SetsB int
	GamesA       int
	GamesB       int
}

// ErrInvalidScore is returned for a score that is not a finished best-of-three
// padel match.
var ErrInvalidScore = errors.New("invalid score")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidScore, fmt.Sprintf(format, args...))
}

// walkoverGames is what a walkover is worth to the side that did not forfeit:
// two sets won 6-0, which is what a group table counts.
const walkoverGames = 6

// Walkover is the outcome when a side forfeits: loser is 1 or 2, and the other
// side wins 6-0 6-0.
func Walkover(loser int) Outcome {
	if loser == 1 {
		return Outcome{Winner: 2, SetsB: 2, GamesB: 2 * walkoverGames}
	}
	return Outcome{Winner: 1, SetsA: 2, GamesA: 2 * walkoverGames}
}

// regularSet reports whether a games score is a finished set: 6-0 to 6-4, 7-5
// or 7-6 (tiebreak), either way round.
func regularSet(a, b int) bool {
	hi, lo := max(a, b), min(a, b)
	switch {
	case lo < 0:
		return false
	case hi == 6:
		return lo <= 4
	case hi == 7:
		return lo == 5 || lo == 6
	}
	return false
}

// superTiebreak reports whether a points score is a finished match tiebreak:
// first to ten with a two point lead.
func superTiebreak(a, b int) bool {
	hi, lo := max(a, b), min(a, b)
	if lo < 0 || hi < 10 {
		return false
	}
	if hi == 10 {
		return lo <= 8
	}
	return hi-lo == 2
}

// ParseResult validates the sets of a best-of-three match and returns who won
// and the figures it is worth. The match ends the moment a side has two sets, so
// a set played after that is refused; a third set that is a match tiebreak is
// written as such.
func ParseResult(sets []Set) (Outcome, error) {
	if len(sets) < 2 || len(sets) > 3 {
		return Outcome{}, invalid("a match has two or three sets, got %d", len(sets))
	}
	var out Outcome
	for i, s := range sets {
		if out.SetsA == 2 || out.SetsB == 2 {
			return Outcome{}, invalid("set %d was played after the match was decided", i+1)
		}
		switch {
		case s.Super && i != 2:
			return Outcome{}, invalid("a match tiebreak can only be the third set")
		case s.Super && !superTiebreak(s.A, s.B):
			return Outcome{}, invalid("a match tiebreak is played to 10 points with a two point lead")
		case !s.Super && !regularSet(s.A, s.B):
			return Outcome{}, invalid("set %d (%d-%d) is not a finished set", i+1, s.A, s.B)
		}
		games := func(v int) int {
			if s.Super {
				return 0
			}
			return v
		}
		out.GamesA, out.GamesB = out.GamesA+games(s.A), out.GamesB+games(s.B)
		if s.A > s.B {
			out.SetsA++
			if s.Super {
				out.GamesA++
			}
		} else {
			out.SetsB++
			if s.Super {
				out.GamesB++
			}
		}
	}
	switch {
	case out.SetsA == 2:
		out.Winner = 1
	case out.SetsB == 2:
		out.Winner = 2
	default:
		return Outcome{}, invalid("the match is not decided: neither side has two sets")
	}
	return out, nil
}
