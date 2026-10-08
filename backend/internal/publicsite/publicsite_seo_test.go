package publicsite

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/complexes"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
)

// The cover photo is the complex's own picture, so it comes before the logo
// both in the social preview and in the structured data.
func TestPrerenderPrefersTheCoverPhotoForSocialPreview(t *testing.T) {
	logo, cover := "https://cdn.example/logo.png", "https://cdn.example/cover.jpg"
	store := &stubStore{
		complex: &complexstore.Complex{
			ID: uuid.New(), Name: "Vibe Palermo", Phone: "+541100000000",
			Address: "Av. Santa Fe 1234", City: "CABA", Province: "Buenos Aires",
			CountryCode: "AR", LogoURL: &logo, CoverURL: &cover, IsActive: true,
		},
	}

	h := NewHandler(NewService(store, frontendServing(t, baseTemplate)), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "vibe-palermo"))
	body := w.Body.String()

	if !strings.Contains(body, `content="https://cdn.example/cover.jpg"`) {
		t.Errorf("og:image should be the cover photo; the page does not carry it")
	}
	if strings.Contains(body, `content="https://cdn.example/logo.png"`) {
		t.Errorf("the logo should not be the social image when a cover photo exists")
	}
	if !strings.Contains(body, `"image":"https://cdn.example/cover.jpg"`) {
		t.Errorf("JSON-LD image should be the cover photo")
	}
}

func TestStructuredDataImageFallsBackFromCoverToLogo(t *testing.T) {
	logo, cover := "https://cdn.example/logo.png", "https://cdn.example/cover.jpg"
	cases := []struct {
		name     string
		complex  *complexstore.Complex
		wantFile string
	}{
		{"cover wins", &complexstore.Complex{ID: uuid.New(), Name: "Vibe", LogoURL: &logo, CoverURL: &cover}, cover},
		{"logo when there is no cover", &complexstore.Complex{ID: uuid.New(), Name: "Vibe", LogoURL: &logo}, logo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(structuredData(tc.complex, nil, "https://vibe.example/x")), &parsed); err != nil {
				t.Fatalf("structured data is not valid JSON: %v", err)
			}
			if parsed["image"] != tc.wantFile {
				t.Errorf("image = %v; want %s", parsed["image"], tc.wantFile)
			}
		})
	}
}

// Amenities are published under their Spanish labels, the same ones the public
// page shows; unknown keys are skipped instead of being published raw.
func TestStructuredDataNamesAmenitiesInSpanish(t *testing.T) {
	complex := &complexstore.Complex{ID: uuid.New(), Name: "Vibe", Amenities: []string{"parking", "bar", "not_a_known_key"}}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(structuredData(complex, nil, "https://vibe.example/x")), &parsed); err != nil {
		t.Fatalf("structured data is not valid JSON: %v", err)
	}

	features, ok := parsed["amenityFeature"].([]any)
	if !ok || len(features) != 2 {
		t.Fatalf("want two amenity features; got %v", parsed["amenityFeature"])
	}
	want := []string{"Estacionamiento", "Bar"}
	for i, raw := range features {
		feature := raw.(map[string]any)
		if feature["@type"] != "LocationFeatureSpecification" || feature["value"] != true {
			t.Errorf("feature %d has the wrong shape: %v", i, feature)
		}
		if feature["name"] != want[i] {
			t.Errorf("feature %d name = %v; want %s", i, feature["name"], want[i])
		}
	}
}

func TestStructuredDataOmitsAmenitiesWhenThereAreNone(t *testing.T) {
	complex := &complexstore.Complex{ID: uuid.New(), Name: "Vibe", Amenities: []string{"not_a_known_key"}}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(structuredData(complex, nil, "https://vibe.example/x")), &parsed); err != nil {
		t.Fatalf("structured data is not valid JSON: %v", err)
	}
	if _, present := parsed["amenityFeature"]; present {
		t.Errorf("amenityFeature should be omitted when no amenity is known; got %v", parsed["amenityFeature"])
	}
}

// The body carries the facts the page shows to a visitor, so a crawler that
// does not run JavaScript reads the same name, address, hours and services a
// person sees after the app loads.
func TestPrerenderWritesTheVisibleFactsIntoTheRootElement(t *testing.T) {
	rootTemplate := strings.Replace(baseTemplate, "<body></body>", `<body><div id="root"></div></body>`, 1)
	store := &stubStore{
		complex: &complexstore.Complex{
			ID: uuid.New(), Name: "Vibe Palermo", Phone: "+541100000000",
			Address: "Av. Santa Fe 1234", City: "CABA", Province: "Buenos Aires",
			CountryCode: "AR", Amenities: []string{"parking"}, IsActive: true,
		},
		schedules: []*complexstore.Schedule{
			{Day: "monday", OpenTime: "08:00", CloseTime: "23:00"},
			{Day: "sunday", IsClosed: true},
		},
	}

	h := NewHandler(NewService(store, frontendServing(t, rootTemplate)), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "vibe-palermo"))
	body := w.Body.String()

	for _, want := range []string{
		`<div id="root"><main>`,
		`<h1>Vibe Palermo</h1>`,
		`<p>Av. Santa Fe 1234, CABA</p>`,
		`<p>+541100000000</p>`,
		`<li>Lunes: 08:00 - 23:00</li>`,
		`<li>Domingo: Cerrado</li>`,
		`<li>Estacionamiento</li>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("prerendered body is missing %q", want)
		}
	}
}

// The body is HTML built from owner-supplied text, so it is escaped like the
// rest of the substituted metadata.
func TestPrerenderEscapesOwnerTextInTheBody(t *testing.T) {
	rootTemplate := strings.Replace(baseTemplate, "<body></body>", `<body><div id="root"></div></body>`, 1)
	store := &stubStore{
		complex: &complexstore.Complex{ID: uuid.New(), Name: "Vibe <i>x</i>", Address: "Calle <b>1</b>", IsActive: true},
	}

	h := NewHandler(NewService(store, frontendServing(t, rootTemplate)), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "x"))
	body := w.Body.String()

	if strings.Contains(body, "<i>x</i>") || strings.Contains(body, "<b>1</b>") {
		t.Errorf("owner text was injected unescaped into the body")
	}
	// The title carries the escaped name too, so the assertions target the body's
	// own elements to prove the body escapes on its own.
	if !strings.Contains(body, "<h1>Vibe &lt;i&gt;x&lt;/i&gt;</h1>") {
		t.Errorf("the name should appear escaped inside the body's heading")
	}
	if !strings.Contains(body, "<p>Calle &lt;b&gt;1&lt;/b&gt;</p>") {
		t.Errorf("the address should appear escaped inside the body")
	}
}

// The title names the city so a search for the venue's area finds it, and the
// description says how to book and what the venue offers, in the copy the
// public page itself uses.
func TestPageTitleNamesTheCity(t *testing.T) {
	cases := []struct{ name, city, want string }{
		{"Grid", "Banfield", "Grid en Banfield - Reservá tu cancha"},
		{"Grid", "", "Grid - Reservá tu cancha"},
	}
	for _, tc := range cases {
		if got := pageTitle(tc.name, tc.city); got != tc.want {
			t.Errorf("pageTitle(%q, %q) = %q; want %q", tc.name, tc.city, got, tc.want)
		}
	}
}

func TestPageDescriptionSaysOnlineOnlyWhenPaymentsAreEnabled(t *testing.T) {
	cases := []struct {
		name     string
		online   bool
		services []string
		want     string
	}{
		{"online without services", true, nil, "Reservá tu cancha en Grid, Banfield online. Horarios y ubicación."},
		{"phone only without services", false, nil, "Reservá tu cancha en Grid, Banfield por WhatsApp. Horarios y ubicación."},
		{"online lists the services", true, []string{"Estacionamiento", "Bar", "Wi-Fi"}, "Reservá tu cancha en Grid, Banfield online. Estacionamiento, Bar, Wi-Fi."},
		{"at most three services", true, []string{"Estacionamiento", "Bar", "Wi-Fi", "Duchas"}, "Reservá tu cancha en Grid, Banfield online. Estacionamiento, Bar, Wi-Fi."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pageDescription("Grid", "Banfield", tc.online, nil, tc.services); got != tc.want {
				t.Errorf("pageDescription = %q; want %q", got, tc.want)
			}
		})
	}
}

// The sports lead the sentence in lowercase Spanish, joined the way Spanish
// joins a list, and the services follow as before.
func TestPageDescriptionNamesTheSports(t *testing.T) {
	cases := []struct {
		name     string
		sports   []string
		services []string
		want     string
	}{
		{"one sport", []string{"Pádel"}, nil, "Reservá canchas de pádel en Grid, Banfield online. Horarios y ubicación."},
		{"two sports", []string{"Pádel", "Tenis"}, nil, "Reservá canchas de pádel y tenis en Grid, Banfield online. Horarios y ubicación."},
		{"three sports with services", []string{"Pádel", "Fútbol", "Básquet"}, []string{"Bar"}, "Reservá canchas de pádel, fútbol y básquet en Grid, Banfield online. Bar."},
		{"at most three sports", []string{"Pádel", "Tenis", "Fútbol", "Hockey"}, nil, "Reservá canchas de pádel, tenis y fútbol en Grid, Banfield online. Horarios y ubicación."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pageDescription("Grid", "Banfield", true, tc.sports, tc.services); got != tc.want {
				t.Errorf("pageDescription = %q; want %q", got, tc.want)
			}
		})
	}
}

func TestPageDescriptionWithoutCity(t *testing.T) {
	want := "Reservá tu cancha en Grid online. Horarios y ubicación."
	if got := pageDescription("Grid", "", true, nil, nil); got != want {
		t.Errorf("pageDescription = %q; want %q", got, want)
	}
}

// court builds one court of a stub venue; the tests below vary only what they
// assert about.
func court(name, sport, courtType string, active bool) complexes.CourtWithPrices {
	return complexes.CourtWithPrices{Court: &courtstore.Court{
		Name: name, Sport: sport, CourtType: courtType, IsActive: active,
	}}
}

// The body lists the complex's active courts with their sport, court type and
// the owner's own description, in the Spanish the public page uses.
func TestPrerenderListsTheActiveCourts(t *testing.T) {
	rootTemplate := strings.Replace(baseTemplate, "<body></body>", `<body><div id="root"></div></body>`, 1)
	cement := "Piso de cemento pulido"
	store := &stubStore{
		complex: &complexstore.Complex{ID: uuid.New(), Name: "Vibe Palermo", City: "CABA", IsActive: true},
		courts: []complexes.CourtWithPrices{
			{Court: &courtstore.Court{Name: "Cancha 1", Sport: "padel", CourtType: "indoor", IsActive: true, Description: &cement}},
			court("Cancha 2", "tennis", "outdoor", true),
		},
	}

	h := NewHandler(NewService(store, frontendServing(t, rootTemplate)), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "vibe-palermo"))
	body := w.Body.String()

	for _, want := range []string{
		`<h2>Canchas</h2><ul>`,
		`<li>Cancha 1: Pádel, Techada<p>Piso de cemento pulido</p></li>`,
		`<li>Cancha 2: Tenis, Descubierta</li>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("prerendered body is missing %q", want)
		}
	}
}

// The owner's court name and description are HTML-escaped like the rest of the
// body; the escaped form is what proves it, since the raw tags must not appear.
func TestPrerenderEscapesOwnerTextInTheCourtList(t *testing.T) {
	rootTemplate := strings.Replace(baseTemplate, "<body></body>", `<body><div id="root"></div></body>`, 1)
	desc := "<b>cemento</b>"
	store := &stubStore{
		complex: &complexstore.Complex{ID: uuid.New(), Name: "Vibe", IsActive: true},
		courts: []complexes.CourtWithPrices{
			{Court: &courtstore.Court{Name: "Cancha <i>1</i>", Sport: "padel", CourtType: "indoor", IsActive: true, Description: &desc}},
		},
	}

	h := NewHandler(NewService(store, frontendServing(t, rootTemplate)), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "vibe"))
	body := w.Body.String()

	if strings.Contains(body, "<i>1</i>") || strings.Contains(body, "<b>cemento</b>") {
		t.Errorf("owner court text was injected unescaped into the body")
	}
	if !strings.Contains(body, "<li>Cancha &lt;i&gt;1&lt;/i&gt;: Pádel, Techada<p>&lt;b&gt;cemento&lt;/b&gt;</p></li>") {
		t.Errorf("the court should appear escaped inside the body's court list")
	}
}

// Only active courts are listed, and no price is published: the page says what
// the venue offers, and prices belong to the booking flow.
func TestPrerenderOmitsInactiveCourtsAndPrices(t *testing.T) {
	rootTemplate := strings.Replace(baseTemplate, "<body></body>", `<body><div id="root"></div></body>`, 1)
	active := court("Cancha 1", "padel", "indoor", true)
	active.Prices = []*courtstore.CourtPrice{{Price: 987654, DayType: "monday"}}
	store := &stubStore{
		complex: &complexstore.Complex{ID: uuid.New(), Name: "Vibe", IsActive: true},
		courts:  []complexes.CourtWithPrices{active, court("Cancha 9", "hockey", "outdoor", false)},
	}

	h := NewHandler(NewService(store, frontendServing(t, rootTemplate)), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "vibe"))
	body := w.Body.String()

	if !strings.Contains(body, "<li>Cancha 1: Pádel, Techada</li>") {
		t.Errorf("the active court is missing from the body")
	}
	if strings.Contains(body, "Cancha 9") || strings.Contains(body, "Hockey") || strings.Contains(body, "hockey") {
		t.Errorf("an inactive court must not appear in the page")
	}
	if strings.Contains(body, "987654") {
		t.Errorf("a court price was published in the prerendered page")
	}
}

// The description names the distinct sports of the active courts, in the order
// the courts are listed and at most three of them, so a search for a sport the
// venue plays finds it. A sport that only an inactive court plays is not named.
func TestPrerenderNamesTheActiveSportsInTheDescription(t *testing.T) {
	store := &stubStore{
		complex: &complexstore.Complex{ID: uuid.New(), Name: "Vibe Palermo", City: "CABA", IsActive: true},
		courts: []complexes.CourtWithPrices{
			court("Cancha 1", "padel", "indoor", true),
			court("Cancha 2", "padel", "indoor", true),
			court("Cancha 3", "tennis", "indoor", false),
			court("Cancha 4", "soccer", "outdoor", true),
			court("Cancha 5", "hockey", "outdoor", true),
			court("Cancha 6", "basketball", "indoor", true),
		},
	}

	h := NewHandler(NewService(store, frontendServing(t, baseTemplate)), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "vibe-palermo"))
	body := w.Body.String()

	want := `content="Reservá canchas de pádel, fútbol y hockey en Vibe Palermo, CABA por WhatsApp. Horarios y ubicación."`
	if !strings.Contains(body, want) {
		t.Errorf("the description should name the three first distinct active sports; want %s", want)
	}
	if strings.Contains(body, "Tenis") || strings.Contains(body, "Básquet") {
		t.Errorf("a sport outside the first three, or one played only by an inactive court, is named")
	}
}

// A court read that fails costs the court list and the sports in the description,
// and nothing else: the page is still served with the complex's own title.
func TestACourtFailureStillRendersTheComplexsOwnPage(t *testing.T) {
	rootTemplate := strings.Replace(baseTemplate, "<body></body>", `<body><div id="root"></div></body>`, 1)
	store := &stubStore{
		complex:   &complexstore.Complex{ID: uuid.New(), Name: "Vibe Palermo", City: "CABA", IsActive: true},
		publicErr: errors.New("db down"),
	}

	h := NewHandler(NewService(store, frontendServing(t, rootTemplate)), testResponder())
	w := httptest.NewRecorder()
	h.Prerender(w, slugRequest(t, "vibe-palermo"))
	body := w.Body.String()

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 for a degraded page", w.Code)
	}
	if !strings.Contains(body, "<title>Vibe Palermo en CABA - Reservá tu cancha | Vibe</title>") {
		t.Errorf("the page should still carry the complex's own title")
	}
	if strings.Contains(body, "Canchas") {
		t.Errorf("no court list should be written when the courts could not be read")
	}
}
