package store

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
)

// HubComplex is one live, switched-on complex as a city hub lists it: the public
// fields a listing shows, and the sport of each of its switched-on courts.
type HubComplex struct {
	ID       uuid.UUID
	Name     string
	Slug     string
	Address  string
	City     string
	Province string
	CoverURL *string
	IsActive bool
	// CourtSports has one entry per switched-on court, sorted by sport, so a
	// sport's court count is how many times it appears.
	CourtSports []string
	UpdatedAt   time.Time
}

// accentFold maps the accented letters a city name can carry onto their base
// letter. It runs after lowercasing, so only lowercase forms are listed.
var accentFold = strings.NewReplacer(
	"á", "a", "à", "a", "ä", "a", "â", "a",
	"é", "e", "è", "e", "ë", "e", "ê", "e",
	"í", "i", "ì", "i", "ï", "i", "î", "i",
	"ó", "o", "ò", "o", "ö", "o", "ô", "o",
	"ú", "u", "ù", "u", "ü", "u", "û", "u",
	"ñ", "n", "ç", "c",
)

// CitySlug is the URL form of a city: lowercase, accents removed, a run of spaces
// or hyphens joined into one hyphen, and every other character dropped. A city
// hub is keyed by it, and so is the link from a complex page back to its hub, so
// "Vicente López", "vicente-lopez" and "VICENTE LÓPEZ" name the same hub.
func CitySlug(city string) string {
	folded := accentFold.Replace(strings.ToLower(strings.TrimSpace(city)))

	var b strings.Builder
	pendingDash := false
	for _, r := range folded {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if pendingDash && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingDash = false
			b.WriteRune(r)
		case r == '-' || unicode.IsSpace(r):
			pendingDash = true
		}
	}
	return b.String()
}

// ListActiveComplexesForHubs returns every live, switched-on complex, ordered by
// city then name. Callers group the rows by city; see db/queries/hubs.sql for
// why the city match is not done in SQL.
func (m *Store) ListActiveComplexesForHubs(ctx context.Context) ([]HubComplex, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListActiveComplexesForHubs(ctx)
	if err != nil {
		return nil, err
	}

	complexes := make([]HubComplex, 0, len(rows))
	for _, row := range rows {
		complexes = append(complexes, HubComplex{
			ID:          data.PgToUUID(row.ID),
			Name:        row.Name,
			Slug:        row.Slug,
			Address:     row.Address,
			City:        row.City,
			Province:    row.Province,
			CoverURL:    data.PgToTextPtr(row.CoverUrl),
			IsActive:    row.IsActive,
			CourtSports: row.CourtSports,
			UpdatedAt:   data.PgToTime(row.UpdatedAt),
		})
	}
	return complexes, nil
}
