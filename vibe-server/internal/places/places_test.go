package places

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// capturedRequest is everything a test needs to assert on what this package
// actually sent upstream: the method, path, headers Google requires, and the
// decoded JSON body (POST) or raw query (GET).
type capturedRequest struct {
	Method string
	Path   string
	Header http.Header
	Query  url.Values
	Body   map[string]any // decoded JSON body, nil for a bodyless request
}

// newTestHandler returns a Handler pointed at a stub upstream, along with a
// pointer to the last request the stub received, so tests can assert on what
// was actually sent to Google.
func newTestHandler(t *testing.T, respond func(w http.ResponseWriter, captured capturedRequest)) (*Handler, *capturedRequest) {
	t.Helper()
	h, captured, _ := newLoggingTestHandler(t, respond)
	return h, captured
}

// testAPIKey is deliberately distinctive: the leak tests search log lines and
// response bodies for it, and a value like "key" would match by accident.
const testAPIKey = "AIza-SECRET-PLACES-KEY-0123456789"

// newLoggingTestHandler is newTestHandler with the responder's log kept, for
// the tests that assert on what this package writes out about a failure.
func newLoggingTestHandler(t *testing.T, respond func(w http.ResponseWriter, captured capturedRequest)) (*Handler, *capturedRequest, *bytes.Buffer) {
	t.Helper()

	captured := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Method = r.Method
		captured.Path = r.URL.Path
		captured.Header = r.Header.Clone()
		captured.Query = r.URL.Query()

		captured.Body = nil
		if r.Body != nil {
			var body map[string]any
			// A GET (Details) request has no body; a decode failure there is
			// expected and ignored.
			if json.NewDecoder(r.Body).Decode(&body) == nil {
				captured.Body = body
			}
		}

		respond(w, *captured)
	}))
	t.Cleanup(srv.Close)

	logs := &bytes.Buffer{}
	h := NewHandler(Config{
		APIKey:  testAPIKey,
		Referer: "https://api.example.test/",
		BaseURL: srv.URL,
	}, httpx.NewResponder(slog.New(slog.NewTextHandler(logs, nil))))

	return h, captured, logs
}

// newDeadUpstreamHandler points the proxy at an address nothing is listening
// on, which is how a timeout, a DNS failure or a refused connection reaches
// this package: as a *url.Error whose message is the whole request URL.
func newDeadUpstreamHandler(t *testing.T) (*Handler, *bytes.Buffer) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	dead := srv.URL
	srv.Close()

	logs := &bytes.Buffer{}
	h := NewHandler(Config{
		APIKey:  testAPIKey,
		Referer: "https://api.example.test/",
		BaseURL: dead,
	}, httpx.NewResponder(slog.New(slog.NewTextHandler(logs, nil))))

	return h, logs
}

// Short inputs must not reach Google: every upstream call is billable.
func TestAutocompleteSkipsUpstreamForShortInput(t *testing.T) {
	called := false
	h, _ := newTestHandler(t, func(w http.ResponseWriter, _ capturedRequest) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	for _, input := range []string{"", "a", "ab"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?input="+input, nil)

		h.Autocomplete(w, r)

		if called {
			t.Fatalf("input %q reached the upstream API", input)
		}
		if w.Code != http.StatusOK {
			t.Errorf("want 200; got %d", w.Code)
		}

		var body struct {
			Predictions []any `json:"predictions"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("body is not valid JSON: %v", err)
		}
		if len(body.Predictions) != 0 {
			t.Errorf("want an empty prediction list; got %v", body.Predictions)
		}
	}
}

func TestAutocompleteForwardsPredictions(t *testing.T) {
	h, captured := newTestHandler(t, func(w http.ResponseWriter, _ capturedRequest) {
		_, _ = w.Write([]byte(`{"suggestions":[{"placePrediction":{
			"placeId":"place-1",
			"text":{"text":"Av. Santa Fe 1234"},
			"structuredFormat":{
				"mainText":{"text":"Av. Santa Fe 1234"},
				"secondaryText":{"text":"CABA, Argentina"}
			}
		}}]}`))
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?input=santa+fe&session_token=tok-1", nil)

	h.Autocomplete(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	var body struct {
		Predictions []map[string]any `json:"predictions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if len(body.Predictions) != 1 {
		t.Fatalf("want one prediction; got %v", body.Predictions)
	}
	p := body.Predictions[0]
	if p["place_id"] != "place-1" {
		t.Errorf("want place_id forwarded; got %v", p["place_id"])
	}
	if p["description"] != "Av. Santa Fe 1234" {
		t.Errorf("want the suggestion's text as description; got %v", p["description"])
	}
	sf, _ := p["structured_formatting"].(map[string]any)
	if sf["main_text"] != "Av. Santa Fe 1234" || sf["secondary_text"] != "CABA, Argentina" {
		t.Errorf("structured_formatting was not translated; got %v", sf)
	}

	// The upstream call itself: POST with a JSON body, key and content type
	// as headers (never as a query parameter — that was the legacy shape).
	if captured.Method != http.MethodPost {
		t.Errorf("want POST to Autocomplete (New); got %s", captured.Method)
	}
	if captured.Path != "/places:autocomplete" {
		t.Errorf("want the places:autocomplete path; got %q", captured.Path)
	}
	if got := captured.Header.Get("X-Goog-Api-Key"); got != testAPIKey {
		t.Errorf("want the configured key as X-Goog-Api-Key; got %q", got)
	}
	if got := captured.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("want application/json; got %q", got)
	}
	if captured.Query.Get("key") != "" {
		t.Errorf("the API key must never appear in the query string; got %q", captured.Query.Get("key"))
	}
	if got := captured.Body["input"]; got != "santa fe" {
		t.Errorf("want the input forwarded in the JSON body; got %v", got)
	}
	if got := captured.Body["sessionToken"]; got != "tok-1" {
		t.Errorf("the session token must be forwarded; got %v", got)
	}
	if got := captured.Body["regionCode"]; got != "AR" {
		t.Errorf("want results restricted to Argentina; got %v", got)
	}
	regions, _ := captured.Body["includedRegionCodes"].([]any)
	if len(regions) != 1 || regions[0] != "ar" {
		t.Errorf("want includedRegionCodes [ar]; got %v", captured.Body["includedRegionCodes"])
	}
	// The legacy "address" collection is rejected by the new API with
	// INVALID_ARGUMENT, which is how this filter reached production broken once.
	types, _ := captured.Body["includedPrimaryTypes"].([]any)
	want := []any{"street_address", "route", "premise", "subpremise"}
	if len(types) != len(want) {
		t.Fatalf("want includedPrimaryTypes %v; got %v", want, captured.Body["includedPrimaryTypes"])
	}
	for i := range want {
		if types[i] != want[i] {
			t.Errorf("includedPrimaryTypes[%d]: want %v; got %v", i, want[i], types[i])
		}
	}
}

func TestDetailsRequiresPlaceID(t *testing.T) {
	called := false
	h, _ := newTestHandler(t, func(http.ResponseWriter, capturedRequest) { called = true })

	w := httptest.NewRecorder()
	h.Details(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400; got %d", w.Code)
	}
	if called {
		t.Error("a request without place_id must not reach the upstream API")
	}
}

func TestDetailsFlattensAddressComponents(t *testing.T) {
	h, captured := newTestHandler(t, func(w http.ResponseWriter, _ capturedRequest) {
		_, _ = w.Write([]byte(`{
			"formattedAddress":"Av. Santa Fe 1234, CABA, Argentina",
			"addressComponents":[
				{"longText":"1234","shortText":"1234","types":["street_number"]},
				{"longText":"Avenida Santa Fe","shortText":"Av. Santa Fe","types":["route"]},
				{"longText":"Buenos Aires","shortText":"Buenos Aires","types":["locality"]},
				{"longText":"Ciudad Autónoma de Buenos Aires","shortText":"CABA","types":["administrative_area_level_1"]}
			],
			"location":{"latitude":-34.595,"longitude":-58.396}
		}`))
	})

	w := httptest.NewRecorder()
	h.Details(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?place_id=abc&session_token=tok-1", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}

	// The street number follows the route, matching how Argentine addresses
	// are written, not the order Google returns the components in.
	if body["address"] != "Avenida Santa Fe 1234" {
		t.Errorf(`want "Avenida Santa Fe 1234"; got %q`, body["address"])
	}
	if body["city"] != "Buenos Aires" {
		t.Errorf(`want city "Buenos Aires"; got %q`, body["city"])
	}
	if body["province"] != "Ciudad Autónoma de Buenos Aires" {
		t.Errorf("province was not extracted; got %q", body["province"])
	}
	if body["formatted_address"] != "Av. Santa Fe 1234, CABA, Argentina" {
		t.Errorf("formatted_address was not forwarded; got %q", body["formatted_address"])
	}
	if body["latitude"] != "-34.595000" || body["longitude"] != "-58.396000" {
		t.Errorf("coordinates were not formatted; got %s / %s", body["latitude"], body["longitude"])
	}

	// The upstream call itself: GET, key and field mask as headers, place_id
	// as a path segment, session token as a query parameter.
	if captured.Method != http.MethodGet {
		t.Errorf("want GET for Details (New); got %s", captured.Method)
	}
	if captured.Path != "/places/abc" {
		t.Errorf("want place_id as a path segment; got %q", captured.Path)
	}
	if got := captured.Header.Get("X-Goog-Api-Key"); got != testAPIKey {
		t.Errorf("want the configured key as X-Goog-Api-Key; got %q", got)
	}
	if got := captured.Header.Get("X-Goog-FieldMask"); got != detailsFieldMask {
		t.Errorf("want the field mask %q; got %q", detailsFieldMask, got)
	}
	if got := captured.Query.Get("sessionToken"); got != "tok-1" {
		t.Errorf("the session token must be forwarded; got %q", got)
	}
	if got := captured.Query.Get("regionCode"); got != "AR" {
		t.Errorf("want results restricted to Argentina; got %q", got)
	}
	if captured.Query.Get("key") != "" {
		t.Errorf("the API key must never appear in the query string; got %q", captured.Query.Get("key"))
	}
}

// Addresses outside a named locality carry no "locality" component, so the
// city falls back to the second-level administrative area.
func TestDetailsFallsBackToAdministrativeAreaForCity(t *testing.T) {
	h, _ := newTestHandler(t, func(w http.ResponseWriter, _ capturedRequest) {
		_, _ = w.Write([]byte(`{"addressComponents":[
			{"longText":"Ruta 8 km 60","types":["route"]},
			{"longText":"Pilar","types":["administrative_area_level_2"]}
		]}`))
	})

	w := httptest.NewRecorder()
	h.Details(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?place_id=abc", nil))

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if body["city"] != "Pilar" {
		t.Errorf(`want the fallback city "Pilar"; got %q`, body["city"])
	}
}

// A route with no street number must not gain a trailing space.
func TestDetailsHandlesMissingStreetNumber(t *testing.T) {
	h, _ := newTestHandler(t, func(w http.ResponseWriter, _ capturedRequest) {
		_, _ = w.Write([]byte(`{"addressComponents":[
			{"longText":"Avenida Santa Fe","types":["route"]}
		]}`))
	})

	w := httptest.NewRecorder()
	h.Details(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?place_id=abc", nil))

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if body["address"] != "Avenida Santa Fe" {
		t.Errorf(`want "Avenida Santa Fe" with no trailing space; got %q`, body["address"])
	}
}

// An unreadable upstream body (200 but not JSON) is the dependency failing,
// not this service, so it is a 502. A 500 would send the caller looking for a
// bug here.
func TestUnreadableUpstreamBodyIsABadGateway(t *testing.T) {
	h, _ := newTestHandler(t, func(w http.ResponseWriter, _ capturedRequest) {
		_, _ = w.Write([]byte(`not json`))
	})

	w := httptest.NewRecorder()
	h.Details(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?place_id=abc", nil))

	if w.Code != http.StatusBadGateway {
		t.Errorf("want 502 when the upstream response is unreadable; got %d", w.Code)
	}
}

// --- Upstream status ---------------------------------------------------------
//
// Places API (New) reports every failure through the HTTP status, with a
// {"error": {"code", "message", "status"}} envelope alongside it — unlike the
// legacy API, which could also report failure inside a 200 body. This proxy
// still has to translate that status into one the browser can act on.

func TestAutocompleteHonoursTheUpstreamStatus(t *testing.T) {
	tests := []struct {
		name         string
		upstreamCode int
		upstreamBody string
		want         int
	}{
		{"answered", http.StatusOK, `{"suggestions":[]}`, http.StatusOK},
		{"http rate limit", http.StatusTooManyRequests, `{"error":{"code":429,"message":"rate limited","status":"RESOURCE_EXHAUSTED"}}`, http.StatusTooManyRequests},
		{"quota exhausted", http.StatusTooManyRequests, `{"error":{"code":429,"message":"quota","status":"RESOURCE_EXHAUSTED"}}`, http.StatusTooManyRequests},
		{"upstream server error", http.StatusInternalServerError, `{"error":{"code":500,"message":"boom","status":"INTERNAL"}}`, http.StatusBadGateway},
		{"upstream gateway error", http.StatusBadGateway, `boom`, http.StatusBadGateway},
		{"key rejected: API not enabled", http.StatusForbidden, `{"error":{"code":403,"message":"Places API (New) has not been used","status":"PERMISSION_DENIED"}}`, http.StatusBadGateway},
		{"upstream rejects the request", http.StatusBadRequest, `{"error":{"code":400,"message":"bad input","status":"INVALID_ARGUMENT"}}`, http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newTestHandler(t, func(w http.ResponseWriter, _ capturedRequest) {
				w.WriteHeader(tt.upstreamCode)
				_, _ = w.Write([]byte(tt.upstreamBody))
			})

			w := httptest.NewRecorder()
			h.Autocomplete(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?input=santa+fe", nil))

			if w.Code != tt.want {
				t.Errorf("upstream %d %s: want %d for the client; got %d (%s)",
					tt.upstreamCode, tt.upstreamBody, tt.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestDetailsHonoursTheUpstreamStatus(t *testing.T) {
	tests := []struct {
		name         string
		upstreamCode int
		upstreamBody string
		want         int
	}{
		{"answered", http.StatusOK, `{"formattedAddress":"x"}`, http.StatusOK},
		{"http rate limit", http.StatusTooManyRequests, `{"error":{"code":429,"message":"rate limited","status":"RESOURCE_EXHAUSTED"}}`, http.StatusTooManyRequests},
		// The place_id is the caller's input, so this one really is about them.
		{"unknown place id", http.StatusNotFound, `{"error":{"code":404,"message":"not found","status":"NOT_FOUND"}}`, http.StatusNotFound},
		{"upstream server error", http.StatusServiceUnavailable, `{"error":{"code":503,"message":"down","status":"UNAVAILABLE"}}`, http.StatusBadGateway},
		{"key rejected: API not enabled", http.StatusForbidden, `{"error":{"code":403,"message":"Places API (New) has not been used","status":"PERMISSION_DENIED"}}`, http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newTestHandler(t, func(w http.ResponseWriter, _ capturedRequest) {
				w.WriteHeader(tt.upstreamCode)
				_, _ = w.Write([]byte(tt.upstreamBody))
			})

			w := httptest.NewRecorder()
			h.Details(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?place_id=abc", nil))

			if w.Code != tt.want {
				t.Errorf("upstream %d %s: want %d for the client; got %d (%s)",
					tt.upstreamCode, tt.upstreamBody, tt.want, w.Code, w.Body.String())
			}
		})
	}
}

// PERMISSION_DENIED is the failure this migration exists to make loud: the
// key's Cloud project not having Places API (New) enabled must reach the log
// distinguishably from a plain outage, even though the browser gets the same
// 502 either way.
func TestPermissionDeniedIsLoggedByName(t *testing.T) {
	h, _, logs := newLoggingTestHandler(t, func(w http.ResponseWriter, _ capturedRequest) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":403,"message":"Places API (New) has not been used in project 123 before or it is disabled","status":"PERMISSION_DENIED"}}`))
	})

	w := httptest.NewRecorder()
	h.Autocomplete(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?input=santa+fe", nil))

	if w.Code != http.StatusBadGateway {
		t.Errorf("want 502; got %d", w.Code)
	}
	if !strings.Contains(logs.String(), "PERMISSION_DENIED") {
		t.Errorf("PERMISSION_DENIED must be distinguishable in the log:\n%s", logs.String())
	}
}

// A quota failure (RESOURCE_EXHAUSTED) must reach the browser as 429, not as
// this service's generic 502 — the caller needs to know backing off is the
// right response.
func TestResourceExhaustedIs429(t *testing.T) {
	h, _ := newTestHandler(t, func(w http.ResponseWriter, _ capturedRequest) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":429,"message":"Quota exceeded","status":"RESOURCE_EXHAUSTED"}}`))
	})

	w := httptest.NewRecorder()
	h.Autocomplete(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?input=santa+fe", nil))

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("want 429; got %d (%s)", w.Code, w.Body.String())
	}
}

// Details NOT_FOUND must reach the browser as 404 — the caller supplied a
// place_id that does not resolve.
func TestDetailsNotFoundIs404(t *testing.T) {
	h, _ := newTestHandler(t, func(w http.ResponseWriter, _ capturedRequest) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"message":"place not found","status":"NOT_FOUND"}}`))
	})

	w := httptest.NewRecorder()
	h.Details(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?place_id=nonexistent", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
}

// --- API key confinement -----------------------------------------------------
//
// Places API (New) takes the key only as the X-Goog-Api-Key header, never in
// the URL — unlike the legacy endpoints, whose query-string key meant a
// transport failure's *url.Error carried it in the message. That's the
// property these tests now protect: the key must never leak into the URL, the
// log, or the response body, however this package fails.

func TestTransportFailureDoesNotLogTheAPIKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*Handler, http.ResponseWriter, *http.Request)
		path string
	}{
		{"autocomplete", (*Handler).Autocomplete, "/?input=santa+fe"},
		{"details", (*Handler).Details, "/?place_id=abc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, logs := newDeadUpstreamHandler(t)

			w := httptest.NewRecorder()
			tc.call(h, w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, nil))

			if w.Code != http.StatusBadGateway {
				t.Errorf("want 502 when the upstream is unreachable; got %d", w.Code)
			}
			if logged := logs.String(); strings.Contains(logged, testAPIKey) {
				t.Errorf("the API key reached the log:\n%s", logged)
			}
			if strings.Contains(w.Body.String(), testAPIKey) {
				t.Errorf("the API key reached the response body: %s", w.Body.String())
			}
			// The failure still has to be recorded, even with nothing left to
			// redact now that the key travels as a header rather than in the
			// URL.
			if logs.String() == "" {
				t.Error("the upstream failure was not logged at all")
			}
		})
	}
}

// A misconfigured base URL fails inside http.NewRequest, whose parse error
// carries the whole URL — still not the key, since the key is a header.
func TestAMalformedBaseURLDoesNotLogTheAPIKey(t *testing.T) {
	logs := &bytes.Buffer{}
	h := NewHandler(Config{
		APIKey:  testAPIKey,
		BaseURL: "http://\x7f-control-characters-are-not-a-url",
	}, httpx.NewResponder(slog.New(slog.NewTextHandler(logs, nil))))

	w := httptest.NewRecorder()
	h.Details(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?place_id=abc", nil))

	if w.Code != http.StatusBadGateway {
		t.Errorf("want 502; got %d", w.Code)
	}
	if logged := logs.String(); strings.Contains(logged, testAPIKey) {
		t.Errorf("the API key reached the log:\n%s", logged)
	}
}

// The upstream's own explanation is worth logging, but it must not become the
// caller's error message: it is Google's text about the platform's account.
func TestUpstreamDetailStaysOutOfTheClientResponse(t *testing.T) {
	h, _, logs := newLoggingTestHandler(t, func(w http.ResponseWriter, _ capturedRequest) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":403,"message":"the provided API key is expired","status":"PERMISSION_DENIED"}}`))
	})

	w := httptest.NewRecorder()
	h.Details(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?place_id=abc", nil))

	if strings.Contains(w.Body.String(), "expired") {
		t.Errorf("the upstream's explanation reached the client: %s", w.Body.String())
	}
	if !strings.Contains(logs.String(), "PERMISSION_DENIED") {
		t.Errorf("the upstream's status was not logged:\n%s", logs.String())
	}
}

// A hostile or broken upstream must not be able to make this process read an
// unbounded body.
func TestUpstreamBodyIsBounded(t *testing.T) {
	h, _ := newTestHandler(t, func(w http.ResponseWriter, _ capturedRequest) {
		// Valid JSON far larger than the cap, so an unbounded read would
		// succeed and a bounded one will not.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"formattedAddress":"`))
		_, _ = w.Write(bytes.Repeat([]byte("a"), maxResponseBytes+1024))
		_, _ = w.Write([]byte(`"}`))
	})

	w := httptest.NewRecorder()
	h.Details(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?place_id=abc", nil))

	if w.Code != http.StatusBadGateway {
		t.Errorf("want 502 for an oversized upstream body; got %d", w.Code)
	}
}
