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
		{"three sets", []draw.Set{{A: 6, B: 4}, {A: 3, B: 6}, {A: 7, B: 5}}, draw.Outcome{Winner: 1, SetsA: 2, SetsB: 1, GamesA: 16, GamesB: 15}},
		{"seven six", []draw.Set{{A: 7, B: 6}, {A: 6, B: 7}, {A: 7, B: 6}}, draw.Outcome{Winner: 1, SetsA: 2, SetsB: 1, GamesA: 20, GamesB: 19}},
		{"super tiebreak", []draw.Set{{A: 6, B: 4}, {A: 4, B: 6}, {A: 10, B: 7, Super: true}}, draw.Outcome{Winner: 1, SetsA: 2, SetsB: 1, GamesA: 11, GamesB: 10}},
		{"long super tiebreak", []draw.Set{{A: 6, B: 4}, {A: 4, B: 6}, {A: 11, B: 13, Super: true}}, draw.Outcome{Winner: 2, SetsA: 1, SetsB: 2, GamesA: 10, GamesB: 11}},
		{"six love", []draw.Set{{A: 6, B: 0}, {A: 6, B: 0}}, draw.Outcome{Winner: 1, SetsA: 2, GamesA: 12}},
	}
	for _, c := range cases {
		got, err := draw.ParseResult(c.sets)
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
	}
	for name, sets := range cases {
		if got, err := draw.ParseResult(sets); err == nil {
			t.Errorf("%s: accepted, got %+v", name, got)
		}
	}
}

func TestWalkover(t *testing.T) {
	got := draw.Walkover(2)
	if got.Winner != 1 || got.SetsA != 2 || got.SetsB != 0 || got.GamesA != 12 || got.GamesB != 0 {
		t.Errorf("Walkover(2) = %+v", got)
	}
	if got := draw.Walkover(1); got.Winner != 2 || got.SetsB != 2 || got.GamesB != 12 {
		t.Errorf("Walkover(1) = %+v", got)
	}
}
