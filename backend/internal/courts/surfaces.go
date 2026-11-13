package courts

import "slices"

// surfacesBySport is which playing surfaces each sport may be given. It is the
// one place the pairing lives; the eight surface values themselves are also
// enforced by the courts_surface_known CHECK (db/migrations/026).
var surfacesBySport = map[string][]string{
	"soccer":     {"synthetic_grass", "natural_grass", "concrete", "parquet", "synthetic_floor", "sand", "rubber"},
	"padel":      {"synthetic_grass", "concrete"},
	"tennis":     {"synthetic_grass", "natural_grass", "clay", "concrete", "synthetic_floor"},
	"hockey":     {"synthetic_grass"},
	"basketball": {"concrete", "parquet", "synthetic_floor"},
	"volleyball": {"parquet", "synthetic_floor", "sand"},
	"pickleball": {"concrete", "synthetic_floor"},
}

// allSurfaces is the closed set of surface values, in display order.
var allSurfaces = []string{
	"synthetic_grass", "natural_grass", "clay", "concrete",
	"parquet", "synthetic_floor", "sand", "rubber",
}

// knownSurface reports whether s is one of the eight surface values.
func knownSurface(s string) bool { return slices.Contains(allSurfaces, s) }

// surfaceAllowedForSport reports whether a court of this sport may have this
// surface. An unknown sport allows nothing.
func surfaceAllowedForSport(sport, surface string) bool {
	return slices.Contains(surfacesBySport[sport], surface)
}

const (
	surfaceUnknownMessage   = "must be one of: synthetic_grass, natural_grass, clay, concrete, parquet, synthetic_floor, sand, rubber"
	surfaceSportMismatchMsg = "is not available for this sport"
)

// checkSurface returns the field message for a surface that cannot be stored on
// a court of the given sport, or "" when it can. An empty surface is no surface
// and always passes.
func checkSurface(sport, surface string) string {
	switch {
	case surface == "":
		return ""
	case !knownSurface(surface):
		return surfaceUnknownMessage
	case !surfaceAllowedForSport(sport, surface):
		return surfaceSportMismatchMsg
	}
	return ""
}
