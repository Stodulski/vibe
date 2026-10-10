package publicsite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
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

// hubCacheControl is how long a crawler or a proxy may reuse a hub page. It matches
// the prerendered complex pages.
const hubCacheControl = "public, max-age=300, stale-while-revalidate=60"

// hubRetryAfterSeconds is the Retry-After on a hub answered 503. A failed read is
// transient, so a crawler is asked to come back rather than to index the fault.
const hubRetryAfterSeconds = "60"

// hubDescriptionRunes is the most characters a hub's meta description runs.
const hubDescriptionRunes = 150

// hubVisibleNames is how many complex names a hub's description names before it
// counts the rest.
const hubVisibleNames = 3

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

// hubDescription is the hub's meta description. It names the city and the
// complexes in it, and runs at most hubDescriptionRunes characters.
func hubDescription(hub cityHub) string {
	names := make([]string, 0, len(hub.Complexes))
	for _, c := range hub.Complexes {
		names = append(names, c.Name)
	}

	var list string
	switch n := len(names); {
	case n > hubVisibleNames:
		list = strings.Join(names[:hubVisibleNames], ", ") + fmt.Sprintf(" y %d más", n-hubVisibleNames)
	case n > 1:
		list = strings.Join(names[:n-1], ", ") + " y " + names[n-1]
	case n == 1:
		list = names[0]
	}

	desc := "Reservá tu cancha en " + hub.City
	if list != "" {
		desc += ": " + list
	}
	return truncateRunes(desc+".", hubDescriptionRunes)
}

// truncateRunes cuts s to at most limit characters, ending a cut with an ellipsis.
func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit-3]) + "..."
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

// sportLabelsOf returns the sport labels comma-separated, as the HTML listing shows
// them.
func sportLabelsOf(sports []string) string {
	return strings.Join(hubSportLabelList(sports), ", ")
}

// hubListItem and hubItemList are the schema.org ItemList the hub carries.
type hubListItem struct {
	Type     string `json:"@type"`
	Position int    `json:"position"`
	URL      string `json:"url"`
}

type hubItemList struct {
	Context  string        `json:"@context"`
	Type     string        `json:"@type"`
	Name     string        `json:"name"`
	Elements []hubListItem `json:"itemListElement"`
}

// hubJSONLD encodes the hub's ItemList of complex URLs. encoding/json escapes <, >
// and & as it writes, so no owner-supplied value can close the script element.
func hubJSONLD(hub cityHub, base string) string {
	list := hubItemList{
		Context: "https://schema.org",
		Type:    "ItemList",
		Name:    "Canchas en " + hub.City,
	}
	for i, c := range hub.Complexes {
		list.Elements = append(list.Elements, hubListItem{
			Type:     "ListItem",
			Position: i + 1,
			URL:      complexURL(base, c.Slug),
		})
	}
	data, err := json.Marshal(list)
	if err != nil {
		// Unreachable: the value is plain strings and ints. An empty object keeps
		// the script element valid JSON if it ever is reached.
		return "{}"
	}
	return string(data)
}

// renderCityHub returns the hub page for one city. Every owner-supplied value is
// HTML-escaped where it is written.
func renderCityHub(hub cityHub, frontendURL string) string {
	base := strings.TrimRight(frontendURL, "/")
	hubURL := base + "/canchas/" + hub.Slug
	title := "Canchas en " + hub.City + " - Reservá tu cancha | Vibe"

	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"es-AR\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
	fmt.Fprintf(&b, "<meta name=\"description\" content=\"%s\">\n", html.EscapeString(hubDescription(hub)))
	fmt.Fprintf(&b, "<link rel=\"canonical\" href=\"%s\">\n", html.EscapeString(hubURL))
	fmt.Fprintf(&b, "<script type=\"application/ld+json\">%s</script>\n", hubJSONLD(hub, base))
	b.WriteString("</head>\n<body>\n<main>\n")
	fmt.Fprintf(&b, "<h1>Canchas en %s</h1>\n", html.EscapeString(hub.City))
	b.WriteString("<ul>\n")
	for _, c := range hub.Complexes {
		fmt.Fprintf(&b, "<li>\n<a href=\"%s\">%s</a>\n", html.EscapeString(complexURL(base, c.Slug)), html.EscapeString(c.Name))
		fmt.Fprintf(&b, "<span>%s</span>\n", html.EscapeString(c.Address))
		if sports := sportLabelsOf(c.Sports); sports != "" {
			fmt.Fprintf(&b, "<span>%s</span>\n", html.EscapeString(sports))
		}
		b.WriteString("</li>\n")
	}
	b.WriteString("</ul>\n</main>\n</body>\n</html>\n")
	return b.String()
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

// CityHub returns the hub page for the city a path names.
func (s *Service) CityHub(ctx context.Context, city string) (string, error) {
	hub, err := s.findCityHub(ctx, city)
	if err != nil {
		return "", err
	}
	return renderCityHub(hub, s.frontendURL), nil
}

// CityHub handles GET /api/v1/public/hubs/{city}: the HTML listing of one city's
// switched-on complexes, for search engines and people alike.
func (h *Handler) CityHub(w http.ResponseWriter, r *http.Request) {
	page, err := h.svc.CityHub(r.Context(), httpx.ReadStringParam(r, "city"))
	if err != nil {
		h.hubError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", hubCacheControl)
	w.WriteHeader(http.StatusOK)
	// The response is committed; a write failure can no longer be reported.
	_, _ = w.Write([]byte(page))
}

// hubError answers a hub read that failed, for the HTML and the JSON hub alike: 404
// for a city with no switched-on complex, 503 with Retry-After for a failed read,
// and 500 for anything else.
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
