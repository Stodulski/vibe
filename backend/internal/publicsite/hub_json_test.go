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
	LogoURL       *string        `json:"logo_url"`
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

// A complex's logo is its logo URL when it has one and null when it has none, as its
// cover is: the hub card reads the logo from the same document.
func TestCityHubJSONCarriesEachComplexLogoOrNull(t *testing.T) {
	now := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	withLogo := hubRow("Club Norte", "club-norte", "Av. Rivadavia 100", "Banfield", []string{"padel"}, now, true)
	withLogo.LogoURL = strPtr("https://cdn.example.test/norte-logo.png")
	bare := hubRow("Canchas Sur", "canchas-sur", "Calle 9 200", "Banfield", []string{"tennis"}, now, true)
	bare.CoverURL = strPtr("https://cdn.example.test/sur.jpg")
	store := &stubStore{hubs: []complexstore.HubComplex{withLogo, bare}}

	w := serveHubJSON(newHubHandler(store), "banfield")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}

	var body cityHubBody
	decodeHubBody(t, w, &body)
	if len(body.Complexes) != 2 {
		t.Fatalf("complexes = %+v; want two", body.Complexes)
	}
	if got := body.Complexes[0].LogoURL; got == nil || *got != "https://cdn.example.test/norte-logo.png" {
		t.Errorf("club-norte logo_url = %v; want its logo", got)
	}
	if got := body.Complexes[1].LogoURL; got != nil {
		t.Errorf("canchas-sur logo_url = %q; want null, the complex has none", *got)
	}
	if !strings.Contains(w.Body.String(), `"logo_url":null`) {
		t.Errorf("a complex without a logo must encode logo_url as null:\n%s", w.Body.String())
	}
}

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

// A city no complex was ever recorded in is not a hub, so its page is a 404.
func TestCityHubJSONIsNotFoundForACityThatNeverHadAComplex(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{
		hubs:   []complexstore.HubComplex{hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true)},
		cities: []complexstore.HubCity{{City: "Banfield", Province: "Buenos Aires"}},
	}

	w := serveHubJSON(newHubHandler(store), "ushuaia")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", w.Code)
	}
}

// A city whose complexes are all switched off or soft-deleted keeps its page as an
// empty hub: its name and province, no complexes, no courts, and the live hubs to
// link to. Its lists are written as [] and never null.
func TestCityHubJSONAnswersAnEmptyHubForACityWhoseComplexesAreAllGone(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	norte := hubRow("Club Norte", "club-norte", "a", "Banfield", []string{"padel"}, now, true)
	norte.Province = "Buenos Aires"
	store := &stubStore{
		hubs: []complexstore.HubComplex{norte},
		// The stored spelling is all upper case: the hub names it in display form.
		cities: []complexstore.HubCity{
			{City: "Banfield", Province: "Buenos Aires"},
			{City: "QUILMES", Province: "Buenos Aires"},
		},
	}

	w := serveHubJSON(newHubHandler(store), "quilmes")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 for a city that had a complex", w.Code)
	}
	if got := w.Header().Get("Cache-Control"); got != hubCacheControl {
		t.Errorf("Cache-Control = %q; want %q", got, hubCacheControl)
	}
	body := w.Body.String()
	for _, want := range []string{`"complexes":[]`, `"sports":[]`} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %s, an empty list written as []:\n%s", want, body)
		}
	}
	if strings.Contains(body, "null") {
		t.Errorf("an empty hub encodes a null list:\n%s", body)
	}

	var doc cityHubBody
	decodeHubBody(t, w, &doc)
	if doc.Hub.Slug != "quilmes" || doc.Hub.Name != "Quilmes" || doc.Hub.Province != "Buenos Aires" {
		t.Errorf("hub = %+v; want quilmes named Quilmes in Buenos Aires", doc.Hub)
	}
	if doc.Hub.ComplexCount != 0 || doc.Hub.CourtCount != 0 {
		t.Errorf("hub counts = %d complexes, %d courts; want 0 and 0", doc.Hub.ComplexCount, doc.Hub.CourtCount)
	}
	if len(doc.Complexes) != 0 {
		t.Errorf("complexes = %+v; want none", doc.Complexes)
	}
	if len(doc.OtherHubs) != 1 || doc.OtherHubs[0].Slug != "banfield" || doc.OtherHubs[0].ComplexCount != 1 {
		t.Errorf("other_hubs = %+v; want the live banfield hub alone", doc.OtherHubs)
	}
}

// The city lookup is read only when no live hub matches, so a live hub keeps
// answering even when the lookup of cities that had a complex would fail.
func TestCityHubJSONServesALiveHubWithoutReadingCityHistory(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{
		hubs:      []complexstore.HubComplex{hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true)},
		citiesErr: errors.New("db down"),
	}

	w := serveHubJSON(newHubHandler(store), "banfield")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 for a live hub", w.Code)
	}
}

// A failed city lookup cannot tell a city that never had a complex (404) from one
// that did (an empty hub), so it is answered as unavailable, never as a 404.
func TestCityHubJSONCityLookupFailureAnswersUnavailableWithRetryAfter(t *testing.T) {
	store := &stubStore{citiesErr: errors.New("db down")}

	w := serveHubJSON(newHubHandler(store), "quilmes")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want 503, not a 404 for a city that may have had a complex", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("a 503 without Retry-After")
	}
}

// The hub index lists only the cities with a live complex: an empty hub is reached
// by its own page, never listed beside the live ones.
func TestHubListJSONLeavesOutACityWhoseComplexesAreAllGone(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{
		hubs:   []complexstore.HubComplex{hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true)},
		cities: []complexstore.HubCity{{City: "Banfield"}, {City: "Quilmes"}},
	}

	w := serveHubList(newHubHandler(store))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
	var body hubListBody
	decodeHubBody(t, w, &body)
	if len(body.Hubs) != 1 || body.Hubs[0].Slug != "banfield" {
		t.Errorf("hubs = %+v; want banfield alone", body.Hubs)
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
