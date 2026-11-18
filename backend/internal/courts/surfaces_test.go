package courts

import "testing"

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
