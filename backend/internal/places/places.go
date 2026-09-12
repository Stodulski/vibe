// Package places proxies the Google Places API (New) so the browser never
// sees the API key. It is a pure pass-through: no store, no domain rules, no
// state — except for the shape translation described below.
//
// The outward response shapes (the "predictions" list and the flattened
// address the browser receives) are the legacy Places API's shapes, kept
// byte-for-byte so the client never had to change. Google's legacy Places API
// was marked legacy on 2025-03-01 and is no longer enabled on new Cloud
// projects, so upstream now speaks Places API (New) — a POST with a JSON body
// for Autocomplete, a GET with header-based auth and a field mask for
// Details, and a distinct response shape for both. Every upstream call is
// translated back into the legacy shape before it reaches the browser.
package places

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// minAutocompleteLength is the shortest input worth sending upstream. Below it
// Google returns noise and the request is billable anyway.
const minAutocompleteLength = 3

// requestTimeout bounds a single upstream call.
const requestTimeout = 10 * time.Second

// defaultBaseURL is Google's Places API (New) endpoint root. Tests point this
// at a local server instead.
const defaultBaseURL = "https://places.googleapis.com/v1"

// defaultReferer is sent on every upstream call. The Google API key is
// restricted by HTTP referrer, so this has to match the registered origin —
// that key restriction is enforced on the request's Referer header
// regardless of which Places API generation is called.
const defaultReferer = "https://api.vibe.com.ar/"

// regionCode is the Places API (New) region/language pairing this proxy
// always requests: Spanish results restricted to Argentina, matching the
// legacy call's "language=es" + "components=country:ar".
const (
	languageCode = "es"
	regionCode   = "AR"
)

// detailsFieldMask is the Places API (New) field mask for Details. It asks
// for exactly what flatten and the response envelope need — nothing more,
// since the field mask also controls SKU billing.
const detailsFieldMask = "id,formattedAddress,location,addressComponents"

// Config is what this module needs from the application configuration.
type Config struct {
	// APIKey authenticates against Google Places, sent as the
	// X-Goog-Api-Key header.
	APIKey string
	// Referer overrides the origin sent upstream. Empty means the registered
	// production origin.
	Referer string
	// BaseURL overrides the Google endpoint root. Empty means the real one.
	BaseURL string
}

// Handler serves the Places proxy routes.
type Handler struct {
	cfg     Config
	respond *httpx.Responder
	client  *http.Client
}

// NewHandler returns a Handler for the given configuration.
func NewHandler(cfg Config, respond *httpx.Responder) *Handler {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Referer == "" {
		cfg.Referer = defaultReferer
	}
	return &Handler{
		cfg:     cfg,
		respond: respond,
		client:  &http.Client{Timeout: requestTimeout},
	}
}

// send executes req against the upstream, decoding the response into dst on
// success or into an *upstreamError on failure.
//
// Unlike the legacy API — which reported quota exhaustion inside a 200 body —
// Places API (New) reports every failure through the HTTP status, with a
// `{"error": {...}}` envelope alongside it. That makes this a single decode
// instead of the legacy handler's two.
func (h *Handler) send(req *http.Request, dst any) error {
	req.Header.Set("X-Goog-Api-Key", h.cfg.APIKey)
	req.Header.Set("Referer", h.cfg.Referer)

	resp, err := h.client.Do(req) //nolint:gosec // G704: req's host is always h.cfg.BaseURL, a fixed operator-configured value (Google's Places API root in production); the only request-derived value ever reaching a URL built by this package is a url.PathEscape'd place_id confined to one path segment in Details (see below), which cannot redirect the call to a different host.
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBounded(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseUpstreamError(resp.StatusCode, body)
	}

	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("places upstream: response does not match the expected shape: %w", err)
	}

	return nil
}

// autocompleteRequest is the Places API (New) Autocomplete request body.
// addressPrimaryTypes narrows autocomplete to street-level places. The legacy
// API took one collection, "address"; the new one rejects that word and wants
// concrete Table B types, at most five, so these four cover a street with or
// without a number and a building or unit inside it.
var addressPrimaryTypes = []string{"street_address", "route", "premise", "subpremise"}

type autocompleteRequest struct {
	Input                string   `json:"input"`
	LanguageCode         string   `json:"languageCode"`
	RegionCode           string   `json:"regionCode"`
	IncludedRegionCodes  []string `json:"includedRegionCodes"`
	IncludedPrimaryTypes []string `json:"includedPrimaryTypes,omitempty"`
	SessionToken         string   `json:"sessionToken,omitempty"`
}

// autocompleteResponse is the Places API (New) Autocomplete response shape,
// narrowed to the fields legacyPrediction needs.
type autocompleteResponse struct {
	Suggestions []struct {
		PlacePrediction struct {
			PlaceID string `json:"placeId"`
			Text    struct {
				Text string `json:"text"`
			} `json:"text"`
			StructuredFormat struct {
				MainText struct {
					Text string `json:"text"`
				} `json:"mainText"`
				SecondaryText struct {
					Text string `json:"text"`
				} `json:"secondaryText"`
			} `json:"structuredFormat"`
		} `json:"placePrediction"`
	} `json:"suggestions"`
}

// legacyPrediction is one prediction shaped exactly like the legacy Places
// Autocomplete API returned it. The client (frontend's addressApi.ts)
// validates against these field names, so this shape is the compatibility
// contract and must not change.
type legacyPrediction struct {
	PlaceID              string                     `json:"place_id"`
	Description          string                     `json:"description"`
	StructuredFormatting legacyStructuredFormatting `json:"structured_formatting"`
}

// legacyStructuredFormatting is the legacy "structured_formatting" object.
type legacyStructuredFormatting struct {
	MainText      string `json:"main_text"`
	SecondaryText string `json:"secondary_text"`
}

// toLegacy translates a Places API (New) suggestion list into the legacy
// prediction shape the browser expects.
func (r autocompleteResponse) toLegacy() []legacyPrediction {
	predictions := make([]legacyPrediction, 0, len(r.Suggestions))
	for _, s := range r.Suggestions {
		p := s.PlacePrediction
		predictions = append(predictions, legacyPrediction{
			PlaceID:     p.PlaceID,
			Description: p.Text.Text,
			StructuredFormatting: legacyStructuredFormatting{
				MainText:      p.StructuredFormat.MainText.Text,
				SecondaryText: p.StructuredFormat.SecondaryText.Text,
			},
		})
	}
	return predictions
}

// Autocomplete handles GET /api/v1/places/autocomplete.
//
// Inputs shorter than three characters return an empty list without calling
// Google, since every upstream call is billable.
func (h *Handler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	input := r.URL.Query().Get("input")
	if len(input) < minAutocompleteLength {
		h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"predictions": []any{}})
		return
	}

	reqBody := autocompleteRequest{
		Input:                input,
		LanguageCode:         languageCode,
		RegionCode:           regionCode,
		IncludedRegionCodes:  []string{"ar"},
		IncludedPrimaryTypes: addressPrimaryTypes,
	}
	if token := r.URL.Query().Get("session_token"); token != "" {
		reqBody.SessionToken = token
	}

	payload, err := json.Marshal(reqBody) //nolint:gosec // G117: "SessionToken" matches the secret-name pattern, but it is Google's Autocomplete/Details session-correlation token (billing grouping), not a credential — safe to marshal, forward, and log.
	if err != nil {
		h.failUpstream(w, r, err)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, h.cfg.BaseURL+"/places:autocomplete", bytes.NewReader(payload))
	if err != nil {
		h.failUpstream(w, r, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	var result autocompleteResponse
	if err := h.send(req, &result); err != nil {
		h.failUpstream(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"predictions": result.toLegacy()})
}

// placeDetails is the Places API (New) Details response shape, narrowed to
// the fields the field mask requests.
type placeDetails struct {
	FormattedAddress string `json:"formattedAddress"`
	Location         struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"location"`
	AddressComponents []struct {
		LongText  string   `json:"longText"`
		ShortText string   `json:"shortText"`
		Types     []string `json:"types"`
	} `json:"addressComponents"`
}

// address is a Google place flattened into the fields the complex form needs.
type address struct {
	Street   string
	City     string
	Province string
}

// flatten reduces Google's address-component list to a street, city and
// province. Components arrive as an unordered list of typed fragments, so each
// field is picked by type rather than by position. Places API (New) uses the
// same component type taxonomy as the legacy API (street_number, route,
// locality, administrative_area_level_1/2), only the component field names
// changed (longText/shortText instead of long_name/short_name).
func (d placeDetails) flatten() address {
	var streetNumber, route, city, province string

	for _, c := range d.AddressComponents {
		for _, t := range c.Types {
			switch t {
			case "street_number":
				streetNumber = c.LongText
			case "route":
				route = c.LongText
			case "locality":
				city = c.LongText
			case "administrative_area_level_2":
				// Fallback for addresses outside a named locality.
				if city == "" {
					city = c.LongText
				}
			case "administrative_area_level_1":
				province = c.LongText
			}
		}
	}

	street := route
	if streetNumber != "" {
		street += " " + streetNumber
	}

	return address{Street: street, City: city, Province: province}
}

// Details handles GET /api/v1/places/details, returning one place flattened
// into the fields the complex form binds to.
func (h *Handler) Details(w http.ResponseWriter, r *http.Request) {
	placeID := r.URL.Query().Get("place_id")
	if placeID == "" {
		h.respond.BadRequest(w, r, errors.New("place_id is required"))
		return
	}

	params := url.Values{
		"languageCode": {languageCode},
		"regionCode":   {regionCode},
	}
	if token := r.URL.Query().Get("session_token"); token != "" {
		params.Set("sessionToken", token)
	}

	reqURL := h.cfg.BaseURL + "/places/" + url.PathEscape(placeID) + "?" + params.Encode()
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, reqURL, nil) //nolint:gosec // G704: the host and path prefix are h.cfg.BaseURL + "/places/", both fixed; only the final path segment is the caller-supplied place_id, and url.PathEscape confines it to that one segment — it cannot introduce a new host, scheme, or path traversal.
	if err != nil {
		h.failUpstream(w, r, err)
		return
	}
	req.Header.Set("X-Goog-FieldMask", detailsFieldMask)

	var details placeDetails
	if err := h.send(req, &details); err != nil {
		h.failUpstream(w, r, err)
		return
	}

	addr := details.flatten()
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"address":           addr.Street,
		"city":              addr.City,
		"province":          addr.Province,
		"formatted_address": details.FormattedAddress,
		"latitude":          fmt.Sprintf("%f", details.Location.Latitude),
		"longitude":         fmt.Sprintf("%f", details.Location.Longitude),
	})
}
