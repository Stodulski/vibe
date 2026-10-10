package publicsite

import (
	"net/http"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// hubSummaryJSON is one city hub as the JSON API carries it: the slug its URL uses,
// the spelling its first complex stores, and how many switched-on complexes it
// lists.
type hubSummaryJSON struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	ComplexCount int    `json:"complex_count"`
}

// hubComplexJSON is one switched-on complex in a city hub, with the fields the HTML
// hub shows for it.
type hubComplexJSON struct {
	Slug    string   `json:"slug"`
	Name    string   `json:"name"`
	Address string   `json:"address"`
	City    string   `json:"city"`
	Sports  []string `json:"sports"`
}

func hubSummaryOf(hub cityHub) hubSummaryJSON {
	return hubSummaryJSON{Slug: hub.Slug, Name: hub.City, ComplexCount: len(hub.Complexes)}
}

func hubComplexesOf(hub cityHub) []hubComplexJSON {
	complexes := make([]hubComplexJSON, 0, len(hub.Complexes))
	for _, c := range hub.Complexes {
		complexes = append(complexes, hubComplexJSON{
			Slug:    c.Slug,
			Name:    c.Name,
			Address: c.Address,
			City:    c.City,
			Sports:  hubSportLabelList(c.Sports),
		})
	}
	return complexes
}

// HubsJSON handles GET /api/v1/public/hubs: every city hub that has a switched-on
// complex, sorted by slug, with how many complexes each lists.
func (h *Handler) HubsJSON(w http.ResponseWriter, r *http.Request) {
	hubs, err := h.svc.cityHubs(r.Context())
	if err != nil {
		h.hubError(w, r, err)
		return
	}
	summaries := make([]hubSummaryJSON, 0, len(hubs))
	for _, hub := range hubs {
		summaries = append(summaries, hubSummaryOf(hub))
	}
	h.writeHubJSON(w, r, httpx.Envelope{"hubs": summaries})
}

// CityHubJSON serves one city's hub as JSON: the same switched-on complexes, in the
// same order, that the HTML hub at the same city lists.
func (h *Handler) CityHubJSON(w http.ResponseWriter, r *http.Request) {
	hub, err := h.svc.findCityHub(r.Context(), httpx.ReadStringParam(r, "city"))
	if err != nil {
		h.hubError(w, r, err)
		return
	}
	h.writeHubJSON(w, r, httpx.Envelope{
		"hub":       hubSummaryOf(hub),
		"complexes": hubComplexesOf(hub),
	})
}

// writeHubJSON writes a hub document under the hub's public cache header, the one
// the HTML hub carries, so a crawler and a proxy reuse both the same way.
func (h *Handler) writeHubJSON(w http.ResponseWriter, r *http.Request, body httpx.Envelope) {
	w.Header().Set("Cache-Control", hubCacheControl)
	h.respond.JSON(w, r, http.StatusOK, body)
}
