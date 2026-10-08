package publicsite

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
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
		`<p>Av. Santa Fe 1234, CABA, Buenos Aires</p>`,
		`Teléfono: +541100000000`,
		`<li>Lunes: 08:00 a 23:00</li>`,
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
