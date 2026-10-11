package publicsite

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
)

// The JSON documents below spell their field names out, so the wire contract is
// pinned here rather than read back from the encoder it would be checking.
type hubSportBody struct {
	Sport      string `json:"sport"`
	CourtCount int    `json:"court_count"`
}

type hubLinkBody struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Province     string `json:"province"`
	ComplexCount int    `json:"complex_count"`
}

type cityHubSummaryBody struct {
	Slug         string         `json:"slug"`
	Name         string         `json:"name"`
	Province     string         `json:"province"`
	ComplexCount int            `json:"complex_count"`
	CourtCount   int            `json:"court_count"`
	Sports       []hubSportBody `json:"sports"`
}

type hubComplexBody struct {
	Slug          string         `json:"slug"`
	Name          string         `json:"name"`
	Address       string         `json:"address"`
	City          string         `json:"city"`
	Province      string         `json:"province"`
	CoverURL      *string        `json:"cover_url"`
	CourtCount    int            `json:"court_count"`
	Sports        []string       `json:"sports"`
	CourtsBySport []hubSportBody `json:"courts_by_sport"`
}

type cityHubBody struct {
	Hub       cityHubSummaryBody `json:"hub"`
	Complexes []hubComplexBody   `json:"complexes"`
	OtherHubs []hubLinkBody      `json:"other_hubs"`
}

type hubIndexItemBody struct {
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Province     string   `json:"province"`
	ComplexCount int      `json:"complex_count"`
	Sports       []string `json:"sports"`
}

type hubListBody struct {
	Hubs []hubIndexItemBody `json:"hubs"`
}

func newHubHandler(store Store) *Handler {
	return NewHandler(NewService(store, "https://vibe.com.ar"), testResponder())
}

// serveHubJSON calls the city hub JSON handler the way the router does.
func serveHubJSON(h *Handler, city string) *httptest.ResponseRecorder {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/public/hubs/x/data", nil)
	r.SetPathValue("city", city)
	w := httptest.NewRecorder()
	h.CityHubJSON(w, r)
	return w
}

// serveHubList calls the hub list JSON handler the way the router does.
func serveHubList(h *Handler) *httptest.ResponseRecorder {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/public/hubs", nil)
	w := httptest.NewRecorder()
	h.HubsJSON(w, r)
	return w
}

func decodeHubBody(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("body is not the expected JSON: %v\n%s", err, w.Body.String())
	}
}

func strPtr(s string) *string { return &s }

func TestCityHubJSONIsServedWithItsCacheHeader(t *testing.T) {
	jan := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	norte := hubRow("Club Norte", "club-norte", "Av. Rivadavia 100", "Banfield", []string{"padel"}, jan, true)
	norte.Province = "Buenos Aires"
	sur := hubRow("Canchas Sur", "canchas-sur", "Calle 9 200", "banfield", []string{"tennis", "squash"}, jan, true)
	sur.Province = "buenos aires"
	sur.CoverURL = strPtr("https://cdn.example.test/sur.jpg")
	rosario := hubRow("Club Rosa", "club-rosa", "Calle 5 500", "ROSARIO", nil, jan, true)
	rosario.Province = "Santa Fe"
	store := &stubStore{hubs: []complexstore.HubComplex{norte, sur, rosario}}

	w := serveHubJSON(newHubHandler(store), "banfield")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q; want application/json", got)
	}
	if got := w.Header().Get("Cache-Control"); got != hubCacheControl {
		t.Errorf("Cache-Control = %q; want %q", got, hubCacheControl)
	}

	var body cityHubBody
	decodeHubBody(t, w, &body)
	want := cityHubBody{
		Hub: cityHubSummaryBody{
			Slug: "banfield", Name: "Banfield", Province: "Buenos Aires", ComplexCount: 2, CourtCount: 3,
			Sports: []hubSportBody{{Sport: "Pádel", CourtCount: 1}, {Sport: "Tenis", CourtCount: 1}, {Sport: "squash", CourtCount: 1}},
		},
		Complexes: []hubComplexBody{
			{
				Slug: "club-norte", Name: "Club Norte", Address: "Av. Rivadavia 100", City: "Banfield",
				Province: "Buenos Aires", CoverURL: nil, CourtCount: 1, Sports: []string{"Pádel"},
				CourtsBySport: []hubSportBody{{Sport: "Pádel", CourtCount: 1}},
			},
			{
				Slug: "canchas-sur", Name: "Canchas Sur", Address: "Calle 9 200", City: "Banfield",
				Province: "Buenos Aires", CoverURL: strPtr("https://cdn.example.test/sur.jpg"), CourtCount: 2,
				Sports:        []string{"Tenis", "squash"},
				CourtsBySport: []hubSportBody{{Sport: "Tenis", CourtCount: 1}, {Sport: "squash", CourtCount: 1}},
			},
		},
		OtherHubs: []hubLinkBody{{Slug: "rosario", Name: "Rosario", Province: "Santa Fe", ComplexCount: 1}},
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("hub body = %+v; want %+v", body, want)
	}
}

func TestCityHubJSONIsNotFoundForACityWithNoActiveComplex(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		city string
		rows []complexstore.HubComplex
	}{
		{
			name: "unknown city",
			city: "ushuaia",
			rows: []complexstore.HubComplex{hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true)},
		},
		{
			name: "city whose only complex is switched off",
			city: "quilmes",
			rows: []complexstore.HubComplex{hubRow("Club Off", "club-off", "a", "Quilmes", nil, now, false)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := serveHubJSON(newHubHandler(&stubStore{hubs: tt.rows}), tt.city)
			if w.Code != http.StatusNotFound {
				t.Errorf("status = %d; want 404", w.Code)
			}
		})
	}
}

func TestCityHubJSONLeavesOutSwitchedOffComplexes(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{hubs: []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true),
		hubRow("Club Off", "club-off", "b", "Banfield", nil, now, false),
	}}

	w := serveHubJSON(newHubHandler(store), "banfield")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}

	var body cityHubBody
	decodeHubBody(t, w, &body)
	if len(body.Complexes) != 1 || body.Complexes[0].Slug != "club-norte" {
		t.Errorf("complexes = %+v; want only club-norte", body.Complexes)
	}
	if body.Hub.ComplexCount != 1 {
		t.Errorf("complex_count = %d; want 1", body.Hub.ComplexCount)
	}
}

// The store answers one sport per switched-on court, so two padel courts are two
// courts of one sport and one complex's court count is every court it returns.
func TestCityHubJSONCountsEachCourtOnceUnderItsSport(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{hubs: []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "a", "Banfield", []string{"padel", "padel", "soccer"}, now, true),
	}}

	w := serveHubJSON(newHubHandler(store), "banfield")

	var body cityHubBody
	decodeHubBody(t, w, &body)
	if body.Hub.CourtCount != 3 || body.Complexes[0].CourtCount != 3 {
		t.Errorf("court_count = hub %d, complex %d; want 3 and 3", body.Hub.CourtCount, body.Complexes[0].CourtCount)
	}
	if want := []hubSportBody{{Sport: "Pádel", CourtCount: 2}, {Sport: "Fútbol", CourtCount: 1}}; !reflect.DeepEqual(body.Hub.Sports, want) {
		t.Errorf("hub sports = %+v; want %+v", body.Hub.Sports, want)
	}
	if want := []string{"Pádel", "Fútbol"}; !reflect.DeepEqual(body.Complexes[0].Sports, want) {
		t.Errorf("complex sports = %v; want %v, each sport once", body.Complexes[0].Sports, want)
	}
}

// The lookup ignores case and accents, as the HTML hub does, and the hub's slug
// is the one canonical spelling in every spelling's response.
func TestCityHubJSONLookupIgnoresCaseAndAccents(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{hubs: []complexstore.HubComplex{
		hubRow("Club Lopez", "club-lopez", "a", "Vicente López", nil, now, true),
	}}
	h := newHubHandler(store)

	for _, city := range []string{"vicente-lopez", "Vicente-López", "VICENTE-LÓPEZ"} {
		t.Run(city, func(t *testing.T) {
			w := serveHubJSON(h, city)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d; want 200", w.Code)
			}
			var body cityHubBody
			decodeHubBody(t, w, &body)
			if body.Hub.Slug != "vicente-lopez" {
				t.Errorf("hub.slug = %q; want vicente-lopez", body.Hub.Slug)
			}
		})
	}
}

func TestCityHubJSONStoreFailureAnswersUnavailableWithRetryAfter(t *testing.T) {
	store := &stubStore{hubsErr: errors.New("db down")}

	w := serveHubJSON(newHubHandler(store), "banfield")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want 503 so a client retries instead of caching a fault", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("a 503 without Retry-After")
	}
	if got := w.Header().Get("Cache-Control"); got == hubCacheControl {
		t.Errorf("a 503 carries the hub's public Cache-Control %q", got)
	}
}

// The index lists every hub by province, then name, with an empty province last,
// and each hub's sports as labels, the most-courted first.
func TestHubListJSONListsEachCityWithItsActiveComplexCount(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	withProvince := func(row complexstore.HubComplex, province string) complexstore.HubComplex {
		row.Province = province
		return row
	}
	store := &stubStore{hubs: []complexstore.HubComplex{
		withProvince(hubRow("Club Norte", "club-norte", "a", "Banfield", []string{"padel"}, now, true), "Buenos Aires"),
		withProvince(hubRow("Club Off", "club-off", "b", "Quilmes", []string{"padel"}, now, false), "Buenos Aires"),
		withProvince(hubRow("Club Lopez", "club-lopez", "c", "Vicente López", []string{"tennis"}, now, true), "Buenos Aires"),
		hubRow("Canchas Sur", "canchas-sur", "d", "banfield", []string{"soccer", "soccer"}, now, true),
		withProvince(hubRow("Club Caleta", "club-caleta", "e", "Caleta Olivia", []string{"soccer"}, now, true), "Santa Cruz"),
		hubRow("Club Gesell", "club-gesell", "f", "villa gesell", nil, now, true),
	}}

	w := serveHubList(newHubHandler(store))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q; want application/json", got)
	}
	if got := w.Header().Get("Cache-Control"); got != hubCacheControl {
		t.Errorf("Cache-Control = %q; want %q", got, hubCacheControl)
	}

	var body hubListBody
	decodeHubBody(t, w, &body)
	want := hubListBody{Hubs: []hubIndexItemBody{
		{Slug: "banfield", Name: "Banfield", Province: "Buenos Aires", ComplexCount: 2, Sports: []string{"Fútbol", "Pádel"}},
		{Slug: "vicente-lopez", Name: "Vicente López", Province: "Buenos Aires", ComplexCount: 1, Sports: []string{"Tenis"}},
		{Slug: "caleta-olivia", Name: "Caleta Olivia", Province: "Santa Cruz", ComplexCount: 1, Sports: []string{"Fútbol"}},
		{Slug: "villa-gesell", Name: "Villa Gesell", Province: "", ComplexCount: 1, Sports: []string{}},
	}}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("hub list = %+v; want %+v", body, want)
	}
}

func TestHubListJSONIsAnEmptyListWithNoActiveComplex(t *testing.T) {
	w := serveHubList(newHubHandler(&stubStore{}))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
	if got := strings.TrimSpace(w.Body.String()); got != `{"hubs":[]}` {
		t.Errorf("body = %s; want an empty list, not null", got)
	}
}

func TestHubListJSONStoreFailureAnswersUnavailableWithRetryAfter(t *testing.T) {
	store := &stubStore{hubsErr: errors.New("db down")}

	w := serveHubList(newHubHandler(store))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want 503 so a client retries instead of caching a fault", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("a 503 without Retry-After")
	}
	if got := w.Header().Get("Cache-Control"); got == hubCacheControl {
		t.Errorf("a 503 carries the hub's public Cache-Control %q", got)
	}
}
