package courts

import "testing"

func TestSurfacesBySportMatchesTheSpec(t *testing.T) {
	want := map[string][]string{
		"soccer":     {"synthetic_grass", "natural_grass", "concrete", "parquet", "synthetic_floor", "sand", "rubber"},
		"padel":      {"synthetic_grass", "concrete"},
		"tennis":     {"synthetic_grass", "natural_grass", "clay", "concrete", "synthetic_floor"},
		"hockey":     {"synthetic_grass"},
		"basketball": {"concrete", "parquet", "synthetic_floor"},
		"volleyball": {"parquet", "synthetic_floor", "sand"},
		"pickleball": {"concrete", "synthetic_floor"},
	}
	if len(surfacesBySport) != len(want) {
		t.Fatalf("want %d sports; got %d", len(want), len(surfacesBySport))
	}
	for sport, surfaces := range want {
		for _, s := range allSurfaces {
			expect := false
			for _, w := range surfaces {
				expect = expect || w == s
			}
			if got := surfaceAllowedForSport(sport, s); got != expect {
				t.Errorf("%s/%s: want allowed=%v; got %v", sport, s, expect, got)
			}
		}
	}
	if surfaceAllowedForSport("cricket", "concrete") {
		t.Error("an unknown sport must allow nothing")
	}
}

func TestEverySportHasAtLeastOneSurfaceAndOnlyKnownOnes(t *testing.T) {
	for sport, surfaces := range surfacesBySport {
		if len(surfaces) == 0 {
			t.Errorf("%s has no surfaces", sport)
		}
		for _, s := range surfaces {
			if !knownSurface(s) {
				t.Errorf("%s lists unknown surface %q", sport, s)
			}
		}
	}
}

func TestCheckSurface(t *testing.T) {
	tests := []struct {
		sport, surface, want string
	}{
		{"padel", "", ""},
		{"padel", "concrete", ""},
		{"padel", "clay", surfaceSportMismatchMsg},
		{"padel", "ice", surfaceUnknownMessage},
	}
	for _, tt := range tests {
		if got := checkSurface(tt.sport, tt.surface); got != tt.want {
			t.Errorf("checkSurface(%q,%q) = %q; want %q", tt.sport, tt.surface, got, tt.want)
		}
	}
}
