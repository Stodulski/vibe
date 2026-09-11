package complexes

import "testing"

// The suggestion is the only new logic behind the availability endpoint, and
// the one thing a wrong answer makes worse than no answer: a slug offered as
// free and then refused by the INSERT is a form that lied to fill itself in.
func TestSuggestSlug(t *testing.T) {
	taken := func(slugs ...string) map[string]bool {
		m := make(map[string]bool, len(slugs))
		for _, s := range slugs {
			m[s] = true
		}
		return m
	}

	tests := []struct {
		name  string
		base  string
		taken map[string]bool
		want  string
	}{
		{
			// Starts at 2, not 1: the first duplicate is the SECOND club to
			// want the name, and "-1" implies a "-0" that never exists.
			name:  "first duplicate gets -2",
			base:  "club-norte",
			taken: taken("club-norte"),
			want:  "club-norte-2",
		},
		{
			name:  "skips variants already gone",
			base:  "club-norte",
			taken: taken("club-norte", "club-norte-2", "club-norte-3"),
			want:  "club-norte-4",
		},
		{
			// A gap left by a deleted club is reused: the map is what the
			// database actually holds, not a counter.
			name:  "fills a gap rather than counting",
			base:  "club-norte",
			taken: taken("club-norte", "club-norte-3", "club-norte-4"),
			want:  "club-norte-2",
		},
		{
			// Fifty clubs with one name is not a case worth serving. The empty
			// string tells the client to say nothing rather than offer a
			// suggestion it invented.
			name: "gives up past the bound",
			base: "club",
			taken: func() map[string]bool {
				m := map[string]bool{"club": true}
				for n := 2; n <= 50; n++ {
					m[suffixed("club", n)] = true
				}
				return m
			}(),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := suggestSlug(tt.base, tt.taken); got != tt.want {
				t.Errorf("suggestSlug(%q) = %q, want %q", tt.base, got, tt.want)
			}
		})
	}
}

func suffixed(base string, n int) string {
	return base + "-" + itoa(n)
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
