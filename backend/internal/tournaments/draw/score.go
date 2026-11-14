package draw

import (
	"errors"
	"fmt"
)

// Set is one set of a padel match as the owner loads it: the games each side
// won. A Super set is the match tiebreak that replaces the deciding set, played
// to ten points with a two point lead, where A and B are points.
type Set struct {
	A, B  int
	Super bool
}

// Outcome is a decided match: who won (1 for the first pair, 2 for the second)
// and the sets and games each side took, the figures a group table adds up. A
// match tiebreak counts as one set and one game for its winner. A match whose
// score was not loaded has NoScore set and carries only its winner.
type Outcome struct {
	Winner       int
	SetsA, SetsB int
	GamesA       int
	GamesB       int
	// NoScore marks a result of only who won: it counts as a win in a group
	// table and adds no sets or games.
	NoScore bool
}

// WinnerOnly is the outcome of a match whose owner loaded only who won (1 for
// the first pair, 2 for the second).
func WinnerOnly(winner int) Outcome { return Outcome{Winner: winner, NoScore: true} }

// ErrInvalidScore is returned for a score that is not a finished match of the
// category's format.
var ErrInvalidScore = errors.New("invalid score")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidScore, fmt.Sprintf(format, args...))
}

// Format is how a category plays its matches: the sets a side needs to win, the
// games that make a set and whether a deciding set (the one played when each
// side has won all but one of the sets it needs) is a match tiebreak.
type Format struct {
	// SetsToWin is 1 (a single set) or 2 (best of three).
	SetsToWin int
	// GamesPerSet is the games a set is played to, 6 in standard padel.
	GamesPerSet int
	// SuperTiebreak makes the deciding set a match tiebreak. It only applies
	// when SetsToWin is 2: a single set has no deciding set apart from itself.
	SuperTiebreak bool
}

// DefaultFormat is best of three sets of six games with a match tiebreak as the
// third, what a category plays unless its owner says otherwise.
var DefaultFormat = Format{SetsToWin: 2, GamesPerSet: 6, SuperTiebreak: true}

// maxSets is the most sets a match can take.
func (f Format) maxSets() int { return 2*f.SetsToWin - 1 }

// superDecider reports whether the last possible set is a match tiebreak.
func (f Format) superDecider() bool { return f.SuperTiebreak && f.SetsToWin == 2 }

// Walkover is the outcome when a side forfeits: loser is 1 or 2, and the other
// side wins every set it needs 6-0 (games_per_set to nothing).
func (f Format) Walkover(loser int) Outcome {
	games := f.SetsToWin * f.GamesPerSet
	if loser == 1 {
		return Outcome{Winner: 2, SetsB: f.SetsToWin, GamesB: games}
	}
	return Outcome{Winner: 1, SetsA: f.SetsToWin, GamesA: games}
}

// regularSet reports whether a games score is a finished set of g games: g to
// g-2 or less, g+1 to g-1, or g+1 to g (the tiebreak), either way round.
func regularSet(a, b, g int) bool {
	hi, lo := max(a, b), min(a, b)
	switch {
	case lo < 0:
		return false
	case hi == g:
		return lo <= g-2
	case hi == g+1:
		return lo == g-1 || lo == g
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

// checkSet refuses a set that is not a finished one of this format. i is its
// position, zero based.
func (f Format) checkSet(i int, s Set) error {
	isDecider := i == f.maxSets()-1
	switch {
	case s.Super && !(f.superDecider() && isDecider):
		return invalid("a match tiebreak is not played in this category's format")
	case !s.Super && f.superDecider() && isDecider:
		return invalid("set %d is a match tiebreak, played to 10 points with a two point lead", i+1)
	case s.Super && !superTiebreak(s.A, s.B):
		return invalid("a match tiebreak is played to 10 points with a two point lead")
	case !s.Super && !regularSet(s.A, s.B, f.GamesPerSet):
		return invalid("set %d (%d-%d) is not a finished set of %d games", i+1, s.A, s.B, f.GamesPerSet)
	}
	return nil
}

// add counts a checked set for the side that won it. A match tiebreak is worth
// one set and one game to its winner.
func (o *Outcome) add(s Set) {
	a, b := s.A, s.B
	if s.Super {
		a, b = 0, 0
		if s.A > s.B {
			a = 1
		} else {
			b = 1
		}
	}
	o.GamesA, o.GamesB = o.GamesA+a, o.GamesB+b
	if s.A > s.B {
		o.SetsA++
	} else {
		o.SetsB++
	}
}

// ParseResult validates the sets of a match of this format and returns who won
// and the figures it is worth. The match ends the moment a side has the sets it
// needs, so a set played after that is refused; a deciding set that is a match
// tiebreak is written as such.
func (f Format) ParseResult(sets []Set) (Outcome, error) {
	if len(sets) < f.SetsToWin || len(sets) > f.maxSets() {
		return Outcome{}, invalid("a match has %s, got %d", f.setsRange(), len(sets))
	}
	var out Outcome
	for i, s := range sets {
		if out.SetsA == f.SetsToWin || out.SetsB == f.SetsToWin {
			return Outcome{}, invalid("set %d was played after the match was decided", i+1)
		}
		if err := f.checkSet(i, s); err != nil {
			return Outcome{}, err
		}
		out.add(s)
	}
	switch {
	case out.SetsA == f.SetsToWin:
		out.Winner = 1
	case out.SetsB == f.SetsToWin:
		out.Winner = 2
	default:
		return Outcome{}, invalid("the match is not decided: neither side has %d sets", f.SetsToWin)
	}
	return out, nil
}

func (f Format) setsRange() string {
	if f.SetsToWin == f.maxSets() {
		return "one set"
	}
	return fmt.Sprintf("%d or %d sets", f.SetsToWin, f.maxSets())
}
