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
type hubSummaryBody struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	ComplexCount int    `json:"complex_count"`
}

type hubComplexBody struct {
	Slug    string   `json:"slug"`
	Name    string   `json:"name"`
	Address string   `json:"address"`
	City    string   `json:"city"`
	Sports  []string `json:"sports"`
}

type cityHubBody struct {
	Hub       hubSummaryBody   `json:"hub"`
	Complexes []hubComplexBody `json:"complexes"`
}

type hubListBody struct {
	Hubs []hubSummaryBody `json:"hubs"`
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

func TestCityHubJSONIsServedWithItsCacheHeader(t *testing.T) {
	jan := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	store := &stubStore{hubs: []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "Av. Rivadavia 100", "Banfield", []string{"padel"}, jan, true),
		hubRow("Canchas Sur", "canchas-sur", "Calle 9 200", "banfield", []string{"tennis", "squash"}, jan, true),
	}}

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
		Hub: hubSummaryBody{Slug: "banfield", Name: "Banfield", ComplexCount: 2},
		Complexes: []hubComplexBody{
			{Slug: "club-norte", Name: "Club Norte", Address: "Av. Rivadavia 100", City: "Banfield", Sports: []string{"Pádel"}},
			{Slug: "canchas-sur", Name: "Canchas Sur", Address: "Calle 9 200", City: "banfield", Sports: []string{"Tenis", "squash"}},
		},
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

func TestHubListJSONListsEachCityWithItsActiveComplexCount(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	store := &stubStore{hubs: []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true),
		hubRow("Club Off", "club-off", "b", "Quilmes", nil, now, false),
		hubRow("Club Lopez", "club-lopez", "c", "Vicente López", nil, now, true),
		hubRow("Canchas Sur", "canchas-sur", "d", "banfield", nil, now, true),
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
	want := hubListBody{Hubs: []hubSummaryBody{
		{Slug: "banfield", Name: "Banfield", ComplexCount: 2},
		{Slug: "vicente-lopez", Name: "Vicente López", ComplexCount: 1},
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
