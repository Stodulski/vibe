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
	"unicode/utf8"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// hubCacheControl is how long a crawler or a proxy may reuse a city hub document.
const hubCacheControl = "public, max-age=300, stale-while-revalidate=60"

// hubRetryAfterSeconds is the Retry-After on a hub answered 503. A failed read is
// transient, so a crawler is asked to come back rather than to index the fault.
const hubRetryAfterSeconds = "60"

// hubOtherLimit caps the other_hubs a city hub document lists.
const hubOtherLimit = 12

// minIndexableHubComplexes is the fewest complexes a hub needs to be listed in the
// sitemap: a one-complex hub funnels to a single page, which Google treats as a doorway.
const minIndexableHubComplexes = 2

// cityHub is one city's listing: the slug its URL carries, the name its first
// complex stores in display form, the province most of its complexes name, its live
// complexes, and the latest update among them.
type cityHub struct {
	Slug      string
	City      string
	Province  string
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

// hubConnectors are the words a Spanish place name keeps lowercase, except as its
// first word.
var hubConnectors = map[string]bool{
	"de": true, "del": true, "la": true, "las": true, "los": true, "el": true, "y": true,
}

// displayName is the form a city or province is shown in. Spaces are trimmed and
// collapsed. A name in one case, all lower or all upper, gets Spanish title case; a
// name typed in mixed case is kept as typed, as its owner may intend it.
func displayName(name string) string {
	joined := strings.Join(strings.Fields(name), " ")
	lower := strings.ToLower(joined)
	if joined != lower && joined != strings.ToUpper(joined) {
		return joined
	}

	words := strings.Fields(lower)
	for i, word := range words {
		if i > 0 && hubConnectors[word] {
			continue
		}
		words[i] = titleHyphenated(word)
	}
	return strings.Join(words, " ")
}

// titleHyphenated upper-cases the first letter of each hyphen-separated part of a
// lowercase word, so "vicente-lópez" becomes "Vicente-López".
func titleHyphenated(word string) string {
	parts := strings.Split(word, "-")
	for i, part := range parts {
		if r, size := utf8.DecodeRuneInString(part); size > 0 {
			parts[i] = string(unicode.ToUpper(r)) + part[size:]
		}
	}
	return strings.Join(parts, "-")
}

// cityHubsFrom groups the switched-on complexes into one hub per city slug, sorted
// by slug. A switched-off complex is left out entirely, including its update
// time. A hub takes the display name its first complex stores.
func cityHubsFrom(rows []complexstore.HubComplex) []cityHub {
	bySlug := make(map[string]*cityHub)
	for _, row := range rows {
		if !row.IsActive {
			continue
		}
		slug := complexstore.CitySlug(row.City)
		if slug == "" {
			continue
		}

		hub, ok := bySlug[slug]
		if !ok {
			hub = &cityHub{Slug: slug, City: displayName(row.City)}
			bySlug[slug] = hub
		}
		hub.Complexes = append(hub.Complexes, row)
		if row.UpdatedAt.After(hub.LastMod) {
			hub.LastMod = row.UpdatedAt
		}
	}

	hubs := make([]cityHub, 0, len(bySlug))
	for _, hub := range bySlug {
		hub.Province = hubProvince(hub.Complexes)
		hubs = append(hubs, *hub)
	}
	sort.Slice(hubs, func(i, j int) bool { return hubs[i].Slug < hubs[j].Slug })
	return hubs
}

// hubProvince is the province most of a hub's complexes name, ties going to the name
// first in alphabetical order. Complexes that name no province do not count, so a
// hub none of whose complexes names one has "".
func hubProvince(rows []complexstore.HubComplex) string {
	counts := make(map[string]int)
	for _, row := range rows {
		if province := displayName(row.Province); province != "" {
			counts[province]++
		}
	}

	best := ""
	for province, n := range counts {
		if best == "" || n > counts[best] || (n == counts[best] && province < best) {
			best = province
		}
	}
	return best
}

// hubIndexLess orders the hub index by province, then name. A hub with no province
// comes last, and a slug settles a tie in name.
func hubIndexLess(a, b cityHub) bool {
	if (a.Province == "") != (b.Province == "") {
		return a.Province != ""
	}
	if a.Province != b.Province {
		return a.Province < b.Province
	}
	if a.City != b.City {
		return a.City < b.City
	}
	return a.Slug < b.Slug
}

// courtSports is one sport per switched-on court across the hub's complexes.
func (h cityHub) courtSports() []string {
	var sports []string
	for _, c := range h.Complexes {
		sports = append(sports, c.CourtSports...)
	}
	return sports
}

// hubSportLabel is the Spanish label of a sport a court carries. A sport with no
// label is shown as stored.
func hubSportLabel(sport string) string {
	if label, ok := hubSportLabels[sport]; ok {
		return label
	}
	return sport
}

// hubSportLabelList returns the distinct label of each sport in courtSports, in the
// order they first appear. The result is never nil, so it encodes as an empty list
// rather than null.
func hubSportLabelList(courtSports []string) []string {
	seen := make(map[string]bool)
	labels := make([]string, 0, len(courtSports))
	for _, sport := range courtSports {
		label := hubSportLabel(sport)
		if !seen[label] {
			seen[label] = true
			labels = append(labels, label)
		}
	}
	return labels
}

// hubSportCount is how many switched-on courts of one sport a hub or a complex has.
type hubSportCount struct {
	Sport      string
	CourtCount int
}

// courtCountsBySport counts the courts of each sport, most-courted first and then by
// label, so the sport a hub is best known for leads its list.
func courtCountsBySport(courtSports []string) []hubSportCount {
	counts := make(map[string]int)
	for _, sport := range courtSports {
		counts[hubSportLabel(sport)]++
	}

	bySport := make([]hubSportCount, 0, len(counts))
	for label, n := range counts {
		bySport = append(bySport, hubSportCount{Sport: label, CourtCount: n})
	}
	sort.Slice(bySport, func(i, j int) bool {
		if bySport[i].CourtCount != bySport[j].CourtCount {
			return bySport[i].CourtCount > bySport[j].CourtCount
		}
		return bySport[i].Sport < bySport[j].Sport
	})
	return bySport
}

// writeHubSitemapEntries appends one sitemap entry per city hub large enough to be
// indexed. A hub's lastmod is the latest update among its complexes, so a crawler
// refetches it when a complex in that city changes.
func writeHubSitemapEntries(b *strings.Builder, hubs []cityHub, baseURL string) {
	for _, hub := range hubs {
		if len(hub.Complexes) < minIndexableHubComplexes {
			continue
		}
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

// findCityHub returns the hub a city names, matched case- and accent-insensitively,
// and every live hub, for the other hubs it links to. A city with a switched-on
// complex is answered from the live hubs. A city whose complexes are all switched
// off or deleted is answered as an empty hub (see emptyCityHub), so its page outlives
// its last complex. A city no complex was ever recorded in is data.ErrRecordNotFound.
func (s *Service) findCityHub(ctx context.Context, city string) (cityHub, []cityHub, error) {
	slug := complexstore.CitySlug(city)
	hubs, err := s.cityHubs(ctx)
	if err != nil {
		return cityHub{}, nil, err
	}
	for _, hub := range hubs {
		if hub.Slug == slug {
			return hub, hubs, nil
		}
	}
	empty, err := s.emptyCityHub(ctx, slug)
	if err != nil {
		return cityHub{}, nil, err
	}
	return empty, hubs, nil
}

// emptyCityHub is the hub of a city that had a complex but has no switched-on one now:
// the name its stored city carries in display form, the first province named among
// its stored complexes, and no complexes. It is read only when no live hub matched.
// A city no complex was ever recorded in is data.ErrRecordNotFound, and a failed read
// is ErrComplexUnavailable, never a 404: the read cannot tell a city that never had
// a complex from one whose complexes are all gone.
func (s *Service) emptyCityHub(ctx context.Context, slug string) (cityHub, error) {
	if slug == "" {
		return cityHub{}, data.ErrRecordNotFound
	}
	rows, err := s.store.ListComplexCitiesForHubs(ctx)
	if err != nil {
		return cityHub{}, fmt.Errorf("%w: %w", ErrComplexUnavailable, err)
	}

	var hub cityHub
	found := false
	for _, row := range rows {
		if complexstore.CitySlug(row.City) != slug {
			continue
		}
		if !found {
			hub = cityHub{Slug: slug, City: displayName(row.City)}
			found = true
		}
		if hub.Province == "" {
			hub.Province = displayName(row.Province)
		}
	}
	if !found {
		return cityHub{}, data.ErrRecordNotFound
	}
	return hub, nil
}

// hubError answers a city hub read that failed: 404 for a city no complex was ever
// recorded in, 503 with Retry-After for a failed read, and 500 for anything else.
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
