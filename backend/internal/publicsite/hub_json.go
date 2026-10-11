package publicsite

import (
	"net/http"
	"sort"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// hubLinkJSON is one city hub as another document names it: the slug its URL uses,
// its display name, its province, and how many switched-on complexes it lists.
type hubLinkJSON struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Province     string `json:"province"`
	ComplexCount int    `json:"complex_count"`
}

// hubSportJSON is how many switched-on courts of one sport a hub or complex has.
type hubSportJSON struct {
	Sport      string `json:"sport"`
	CourtCount int    `json:"court_count"`
}

// cityHubJSON is the hub a city's document opens with: its link fields, its court
// count, and its courts by sport.
type cityHubJSON struct {
	hubLinkJSON
	CourtCount int            `json:"court_count"`
	Sports     []hubSportJSON `json:"sports"`
}

// hubSummaryJSON is one city hub in the hub index: its link fields and the Spanish
// label of each sport its courts carry.
type hubSummaryJSON struct {
	hubLinkJSON
	Sports []string `json:"sports"`
}

// hubComplexJSON is one switched-on complex in a city hub, with the fields the hub
// shows for it.
type hubComplexJSON struct {
	Slug          string         `json:"slug"`
	Name          string         `json:"name"`
	Address       string         `json:"address"`
	City          string         `json:"city"`
	Province      string         `json:"province"`
	CoverURL      *string        `json:"cover_url"`
	CourtCount    int            `json:"court_count"`
	Sports        []string       `json:"sports"`
	CourtsBySport []hubSportJSON `json:"courts_by_sport"`
}

func hubLinkOf(hub cityHub) hubLinkJSON {
	return hubLinkJSON{Slug: hub.Slug, Name: hub.City, Province: hub.Province, ComplexCount: len(hub.Complexes)}
}

// hubSportsOf encodes a sport court count list as a non-nil list, so an empty one
// encodes as [] rather than null.
func hubSportsOf(counts []hubSportCount) []hubSportJSON {
	sports := make([]hubSportJSON, 0, len(counts))
	for _, c := range counts {
		sports = append(sports, hubSportJSON(c))
	}
	return sports
}

func cityHubOf(hub cityHub) cityHubJSON {
	courtSports := hub.courtSports()
	return cityHubJSON{
		hubLinkJSON: hubLinkOf(hub),
		CourtCount:  len(courtSports),
		Sports:      hubSportsOf(courtCountsBySport(courtSports)),
	}
}

func hubSummaryOf(hub cityHub) hubSummaryJSON {
	counts := courtCountsBySport(hub.courtSports())
	labels := make([]string, 0, len(counts))
	for _, c := range counts {
		labels = append(labels, c.Sport)
	}
	return hubSummaryJSON{hubLinkJSON: hubLinkOf(hub), Sports: labels}
}

func hubComplexesOf(hub cityHub) []hubComplexJSON {
	complexes := make([]hubComplexJSON, 0, len(hub.Complexes))
	for _, c := range hub.Complexes {
		complexes = append(complexes, hubComplexJSON{
			Slug:          c.Slug,
			Name:          c.Name,
			Address:       c.Address,
			City:          displayName(c.City),
			Province:      displayName(c.Province),
			CoverURL:      c.CoverURL,
			CourtCount:    len(c.CourtSports),
			Sports:        hubSportLabelList(c.CourtSports),
			CourtsBySport: hubSportsOf(courtCountsBySport(c.CourtSports)),
		})
	}
	return complexes
}

// otherHubsOf lists every hub but hub: the ones in its province first, then the
// ones with the most complexes, then by name, capped at hubOtherLimit.
func otherHubsOf(hub cityHub, all []cityHub) []hubLinkJSON {
	others := make([]cityHub, 0, len(all))
	for _, other := range all {
		if other.Slug != hub.Slug {
			others = append(others, other)
		}
	}

	sameProvince := func(h cityHub) bool { return hub.Province != "" && h.Province == hub.Province }
	sort.SliceStable(others, func(i, j int) bool {
		a, b := others[i], others[j]
		if sameProvince(a) != sameProvince(b) {
			return sameProvince(a)
		}
		if len(a.Complexes) != len(b.Complexes) {
			return len(a.Complexes) > len(b.Complexes)
		}
		if a.City != b.City {
			return a.City < b.City
		}
		return a.Slug < b.Slug
	})

	if len(others) > hubOtherLimit {
		others = others[:hubOtherLimit]
	}
	links := make([]hubLinkJSON, 0, len(others))
	for _, other := range others {
		links = append(links, hubLinkOf(other))
	}
	return links
}

// HubsJSON handles GET /api/v1/public/hubs: every city hub that has a switched-on
// complex, by province then name, with its province, how many complexes each lists,
// and the sports its courts carry.
func (h *Handler) HubsJSON(w http.ResponseWriter, r *http.Request) {
	hubs, err := h.svc.cityHubs(r.Context())
	if err != nil {
		h.hubError(w, r, err)
		return
	}
	sort.Slice(hubs, func(i, j int) bool { return hubIndexLess(hubs[i], hubs[j]) })
	summaries := make([]hubSummaryJSON, 0, len(hubs))
	for _, hub := range hubs {
		summaries = append(summaries, hubSummaryOf(hub))
	}
	h.writeHubJSON(w, r, httpx.Envelope{"hubs": summaries})
}

// CityHubJSON serves one city's hub as JSON: the same switched-on complexes, in the
// same order, that the HTML hub at the same city lists, and the other hubs to link.
func (h *Handler) CityHubJSON(w http.ResponseWriter, r *http.Request) {
	hub, all, err := h.svc.findCityHub(r.Context(), httpx.ReadStringParam(r, "city"))
	if err != nil {
		h.hubError(w, r, err)
		return
	}
	h.writeHubJSON(w, r, httpx.Envelope{
		"hub":        cityHubOf(hub),
		"complexes":  hubComplexesOf(hub),
		"other_hubs": otherHubsOf(hub, all),
	})
}

// writeHubJSON writes a hub document under the city hubs' public cache header, so
// a crawler and a proxy reuse every hub document the same way.
func (h *Handler) writeHubJSON(w http.ResponseWriter, r *http.Request, body httpx.Envelope) {
	w.Header().Set("Cache-Control", hubCacheControl)
	h.respond.JSON(w, r, http.StatusOK, body)
}
