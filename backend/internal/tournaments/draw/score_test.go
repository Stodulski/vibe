package draw_test

import (
	"testing"

	"github.com/stodulski/vibe-server/internal/tournaments/draw"
)

func TestParseResultValid(t *testing.T) {
	cases := []struct {
		name string
		sets []draw.Set
		want draw.Outcome
	}{
		{"straight sets", []draw.Set{{A: 6, B: 4}, {A: 6, B: 3}}, draw.Outcome{Winner: 1, SetsA: 2, GamesA: 12, GamesB: 7}},
		{"b wins in two", []draw.Set{{A: 2, B: 6}, {A: 6, B: 7}}, draw.Outcome{Winner: 2, SetsB: 2, GamesA: 8, GamesB: 13}},
		{"super tiebreak", []draw.Set{{A: 6, B: 4}, {A: 4, B: 6}, {A: 10, B: 7, Super: true}}, draw.Outcome{Winner: 1, SetsA: 2, SetsB: 1, GamesA: 11, GamesB: 10}},
		{"long super tiebreak", []draw.Set{{A: 6, B: 4}, {A: 4, B: 6}, {A: 11, B: 13, Super: true}}, draw.Outcome{Winner: 2, SetsA: 1, SetsB: 2, GamesA: 10, GamesB: 11}},
		{"six love", []draw.Set{{A: 6, B: 0}, {A: 6, B: 0}}, draw.Outcome{Winner: 1, SetsA: 2, GamesA: 12}},
	}
	for _, c := range cases {
		got, err := draw.DefaultFormat.ParseResult(c.sets)
		if err != nil || got != c.want {
			t.Errorf("%s: got %+v, %v; want %+v", c.name, got, err, c.want)
		}
	}
}

func TestParseResultInvalid(t *testing.T) {
	cases := map[string][]draw.Set{
		"no sets":               {},
		"one set":               {{A: 6, B: 4}},
		"four sets":             {{A: 6, B: 4}, {A: 4, B: 6}, {A: 6, B: 4}, {A: 6, B: 4}},
		"set after the match":   {{A: 6, B: 4}, {A: 6, B: 3}, {A: 6, B: 0}},
		"unfinished set":        {{A: 6, B: 5}, {A: 6, B: 3}},
		"six five is not final": {{A: 5, B: 6}, {A: 3, B: 6}},
		"seven three":           {{A: 7, B: 3}, {A: 6, B: 3}},
		"eight six":             {{A: 8, B: 6}, {A: 6, B: 3}},
		"seven seven":           {{A: 7, B: 7}, {A: 6, B: 3}},
		"negative":              {{A: -1, B: 6}, {A: 6, B: 3}},
		"split sets no third":   {{A: 6, B: 4}, {A: 4, B: 6}},
		"super in first set":    {{A: 10, B: 7, Super: true}, {A: 6, B: 3}},
		"super after a sweep":   {{A: 6, B: 4}, {A: 6, B: 3}, {A: 10, B: 7, Super: true}},
		"super too short":       {{A: 6, B: 4}, {A: 4, B: 6}, {A: 9, B: 7, Super: true}},
		"super by one":          {{A: 6, B: 4}, {A: 4, B: 6}, {A: 11, B: 10, Super: true}},
		"super too wide":        {{A: 6, B: 4}, {A: 4, B: 6}, {A: 12, B: 7, Super: true}},
		"regular decider":       {{A: 6, B: 4}, {A: 3, B: 6}, {A: 7, B: 5}},
	}
	for name, sets := range cases {
		if got, err := draw.DefaultFormat.ParseResult(sets); err == nil {
			t.Errorf("%s: accepted, got %+v", name, got)
		}
	}
}

func TestWalkover(t *testing.T) {
	got := draw.DefaultFormat.Walkover(2)
	if got.Winner != 1 || got.SetsA != 2 || got.SetsB != 0 || got.GamesA != 12 || got.GamesB != 0 {
		t.Errorf("Walkover(2) = %+v", got)
	}
	if got := draw.DefaultFormat.Walkover(1); got.Winner != 2 || got.SetsB != 2 || got.GamesB != 12 {
		t.Errorf("Walkover(1) = %+v", got)
	}
}

// bestOfThree is the format of a category that plays a regular deciding set.
var bestOfThree = draw.Format{SetsToWin: 2, GamesPerSet: 6}

// S6b: the format is the category's, so a best of three can end on a regular set.
func TestBestOfThreeWithoutSuperTiebreak(t *testing.T) {
	got, err := bestOfThree.ParseResult([]draw.Set{{A: 6, B: 4}, {A: 3, B: 6}, {A: 7, B: 5}})
	want := draw.Outcome{Winner: 1, SetsA: 2, SetsB: 1, GamesA: 16, GamesB: 15}
	if err != nil || got != want {
		t.Errorf("regular decider: got %+v, %v; want %+v", got, err, want)
	}
	got, err = bestOfThree.ParseResult([]draw.Set{{A: 7, B: 6}, {A: 6, B: 7}, {A: 7, B: 6}})
	if err != nil || got.Winner != 1 || got.GamesA != 20 || got.GamesB != 19 {
		t.Errorf("seven six: got %+v, %v", got, err)
	}
	invalid := map[string][]draw.Set{
		"super not enabled":   {{A: 6, B: 4}, {A: 4, B: 6}, {A: 10, B: 7, Super: true}},
		"one set":             {{A: 6, B: 4}},
		"set after decided":   {{A: 6, B: 4}, {A: 6, B: 3}, {A: 6, B: 0}},
		"split without third": {{A: 6, B: 4}, {A: 4, B: 6}},
	}
	for name, sets := range invalid {
		if got, err := bestOfThree.ParseResult(sets); err == nil {
			t.Errorf("%s: accepted, got %+v", name, got)
		}
	}
}

// S6b: a category that plays one set takes exactly one finished set.
func TestBestOfOneSet(t *testing.T) {
	one := draw.Format{SetsToWin: 1, GamesPerSet: 6, SuperTiebreak: true}
	got, err := one.ParseResult([]draw.Set{{A: 6, B: 3}})
	if err != nil || got != (draw.Outcome{Winner: 1, SetsA: 1, GamesA: 6, GamesB: 3}) {
		t.Errorf("6-3: got %+v, %v", got, err)
	}
	if got, err = one.ParseResult([]draw.Set{{A: 6, B: 7}}); err != nil || got.Winner != 2 || got.GamesB != 7 {
		t.Errorf("6-7: got %+v, %v", got, err)
	}
	invalid := map[string][]draw.Set{
		"no sets":        {},
		"two sets":       {{A: 6, B: 3}, {A: 6, B: 3}},
		"unfinished":     {{A: 6, B: 5}},
		"super tiebreak": {{A: 10, B: 7, Super: true}},
	}
	for name, sets := range invalid {
		if got, err := one.ParseResult(sets); err == nil {
			t.Errorf("%s: accepted, got %+v", name, got)
		}
	}
}

// S6b: a set is won at games_per_set with two games of lead, at games_per_set+1
// against games_per_set-1, or by a tiebreak at games_per_set+1 to games_per_set.
func TestGamesPerSetDecideWhatAFinishedSetIs(t *testing.T) {
	short := draw.Format{SetsToWin: 2, GamesPerSet: 4, SuperTiebreak: true}
	valid := [][]draw.Set{
		{{A: 4, B: 0}, {A: 4, B: 2}}, {{A: 5, B: 3}, {A: 3, B: 5}, {A: 10, B: 8, Super: true}},
		{{A: 5, B: 4}, {A: 4, B: 5}, {A: 10, B: 2, Super: true}},
	}
	for _, sets := range valid {
		if got, err := short.ParseResult(sets); err != nil {
			t.Errorf("%v: refused: %v (%+v)", sets, err, got)
		}
	}
	invalid := map[string][]draw.Set{
		"four three": {{A: 4, B: 3}, {A: 4, B: 0}},
		"six four":   {{A: 6, B: 4}, {A: 4, B: 0}},
		"five two":   {{A: 5, B: 2}, {A: 4, B: 0}},
		"six five":   {{A: 6, B: 5}, {A: 4, B: 0}},
		"seven five": {{A: 7, B: 5}, {A: 4, B: 0}},
	}
	for name, sets := range invalid {
		if got, err := short.ParseResult(sets); err == nil {
			t.Errorf("%s: accepted, got %+v", name, got)
		}
	}
	// the same scores follow a longer set when the category plays nine games
	nine := draw.Format{SetsToWin: 1, GamesPerSet: 9}
	if _, err := nine.ParseResult([]draw.Set{{A: 9, B: 7}}); err != nil {
		t.Errorf("9-7: %v", err)
	}
	if _, err := nine.ParseResult([]draw.Set{{A: 10, B: 9}}); err != nil {
		t.Errorf("10-9: %v", err)
	}
	if _, err := nine.ParseResult([]draw.Set{{A: 6, B: 4}}); err == nil {
		t.Error("6-4 accepted in a nine game set")
	}
}

// S6b: a walkover is worth sets_to_win sets of games_per_set-0.
func TestWalkoverFollowsTheFormat(t *testing.T) {
	cases := []struct {
		f    draw.Format
		want draw.Outcome
	}{
		{draw.Format{SetsToWin: 1, GamesPerSet: 6}, draw.Outcome{Winner: 1, SetsA: 1, GamesA: 6}},
		{draw.Format{SetsToWin: 2, GamesPerSet: 4, SuperTiebreak: true}, draw.Outcome{Winner: 1, SetsA: 2, GamesA: 8}},
		{draw.Format{SetsToWin: 2, GamesPerSet: 9}, draw.Outcome{Winner: 1, SetsA: 2, GamesA: 18}},
	}
	for _, c := range cases {
		if got := c.f.Walkover(2); got != c.want {
			t.Errorf("%+v: Walkover(2) = %+v, want %+v", c.f, got, c.want)
		}
	}
	if got := (draw.Format{SetsToWin: 1, GamesPerSet: 6}).Walkover(1); got.Winner != 2 || got.SetsB != 1 || got.GamesB != 6 {
		t.Errorf("Walkover(1) = %+v", got)
	}
}
