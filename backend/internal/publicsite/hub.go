package publicsite

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// hubCacheControl is how long a crawler or a proxy may reuse a city hub document.
const hubCacheControl = "public, max-age=300, stale-while-revalidate=60"

// hubRetryAfterSeconds is the Retry-After on a hub answered 503. A failed read is
// transient, so a crawler is asked to come back rather than to index the fault.
const hubRetryAfterSeconds = "60"

// cityHub is one city's listing: the slug its URL carries, the spelling its first
// complex stores, its live complexes, and the latest update among them.
type cityHub struct {
	Slug      string
	City      string
	Complexes []complexstore.HubComplex
	LastMod   time.Time
}

// hubSportLabels names the sports a court can carry, in the Spanish the public
// pages use. It mirrors the frontend's es_AR sport labels.
var hubSportLabels = map[string]string{
	"padel":      "Pádel",
	"tennis":     "Tenis",
	"soccer":     "Fútbol",
	"basketball": "Básquet",
	"volleyball": "Vóley",
	"hockey":     "Hockey",
	"pickleball": "Pickleball",
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

// citySlug is the URL form of a city: lowercase, accents removed, a run of spaces
// or hyphens joined into one hyphen, and every other character dropped. A hub is
// looked up by it, so "Vicente López", "vicente-lopez" and "VICENTE LÓPEZ" name
// the same hub.
func citySlug(city string) string {
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

// cityHubsFrom groups the switched-on complexes into one hub per city slug, sorted
// by slug. A switched-off complex is left out entirely, including its update
// time. A hub takes the spelling its first complex stores.
func cityHubsFrom(rows []complexstore.HubComplex) []cityHub {
	bySlug := make(map[string]*cityHub)
	for _, row := range rows {
		if !row.IsActive {
			continue
		}
		slug := citySlug(row.City)
		if slug == "" {
			continue
		}

		hub, ok := bySlug[slug]
		if !ok {
			hub = &cityHub{Slug: slug, City: row.City}
			bySlug[slug] = hub
		}
		hub.Complexes = append(hub.Complexes, row)
		if row.UpdatedAt.After(hub.LastMod) {
			hub.LastMod = row.UpdatedAt
		}
	}

	hubs := make([]cityHub, 0, len(bySlug))
	for _, hub := range bySlug {
		hubs = append(hubs, *hub)
	}
	sort.Slice(hubs, func(i, j int) bool { return hubs[i].Slug < hubs[j].Slug })
	return hubs
}

// hubSportLabelList returns the Spanish label for each of a complex's sports. A sport
// with no label is shown as stored. The result is never nil, so it encodes as an
// empty list rather than null.
func hubSportLabelList(sports []string) []string {
	labels := make([]string, 0, len(sports))
	for _, sport := range sports {
		if label, ok := hubSportLabels[sport]; ok {
			labels = append(labels, label)
		} else {
			labels = append(labels, sport)
		}
	}
	return labels
}

// writeHubSitemapEntries appends one sitemap entry per city hub. A hub's lastmod
// is the latest update among its complexes, so a crawler refetches it when a
// complex in that city changes.
func writeHubSitemapEntries(b *strings.Builder, hubs []cityHub, baseURL string) {
	for _, hub := range hubs {
		b.WriteString("  <url>\n")
		fmt.Fprintf(b, "    <loc>%s/canchas/%s</loc>\n", baseURL, hub.Slug)
		fmt.Fprintf(b, "    <lastmod>%s</lastmod>\n", hub.LastMod.Format("2006-01-02"))
		b.WriteString("    <changefreq>daily</changefreq>\n")
		b.WriteString("    <priority>0.7</priority>\n")
		b.WriteString("  </url>\n")
	}
}

// cityHubs reads the switched-on complexes and groups them by city. A failed read
// is ErrComplexUnavailable: the city is real, only this read failed.
func (s *Service) cityHubs(ctx context.Context) ([]cityHub, error) {
	rows, err := s.store.ListActiveComplexesForHubs(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrComplexUnavailable, err)
	}
	return cityHubsFrom(rows), nil
}

// findCityHub returns the hub a city names, matched case- and accent-insensitively.
// A city with no switched-on complex is data.ErrRecordNotFound.
func (s *Service) findCityHub(ctx context.Context, city string) (cityHub, error) {
	slug := citySlug(city)
	hubs, err := s.cityHubs(ctx)
	if err != nil {
		return cityHub{}, err
	}
	for _, hub := range hubs {
		if hub.Slug == slug {
			return hub, nil
		}
	}
	return cityHub{}, data.ErrRecordNotFound
}

// hubError answers a city hub read that failed: 404 for a city with no switched-on
// complex, 503 with Retry-After for a failed read, and 500 for anything else.
func (h *Handler) hubError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, data.ErrRecordNotFound):
		h.respond.NotFound(w, r)
	case errors.Is(err, ErrComplexUnavailable):
		// A crawler retries a transient 5xx. A 200 here would index a fault in the
		// city's place, so the read failure is answered as unavailable instead.
		h.respond.LogError(r, err)
		w.Header().Set("Retry-After", hubRetryAfterSeconds)
		h.respond.Refuse(w, r, httpx.Unavailable(nil))
	default:
		h.respond.ServerError(w, r, err)
	}
}
