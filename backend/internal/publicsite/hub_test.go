package publicsite

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
)

const hubFrontend = "https://app.vibe.com.ar"

func hubRow(name, slug, address, city string, sports []string, updated time.Time, active bool) complexstore.HubComplex {
	return complexstore.HubComplex{
		ID:        uuid.New(),
		Name:      name,
		Slug:      slug,
		Address:   address,
		City:      city,
		IsActive:  active,
		Sports:    sports,
		UpdatedAt: updated,
	}
}

func TestCitySlugFoldsCaseAccentsAndSpaces(t *testing.T) {
	tests := map[string]string{
		"Banfield":                "banfield",
		"Vicente López":           "vicente-lopez",
		"  San   Isidro ":         "san-isidro",
		"Ñuñez":                   "nunez",
		"San Martín de los Andes": "san-martin-de-los-andes",
		"Vicente-López":           "vicente-lopez",
		"Banfield <b>":            "banfield-b",
		"":                        "",
	}
	for city, want := range tests {
		if got := citySlug(city); got != want {
			t.Errorf("citySlug(%q) = %q; want %q", city, got, want)
		}
	}
}

func TestCityHubsGroupsActiveComplexesByCitySlug(t *testing.T) {
	t1 := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)

	rows := []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "Av. Rivadavia 100", "Banfield", []string{"padel"}, t1, true),
		hubRow("Canchas Sur", "canchas-sur", "Calle 2 200", "banfield", []string{"soccer"}, t2, true),
		// Switched off: not listed, and its later update must not move the hub's lastmod.
		hubRow("Club Off", "club-off", "Calle 3 300", "Banfield", []string{"tennis"}, t3, false),
		hubRow("Club Quilmes", "club-quilmes", "Calle 4 400", "Quilmes", nil, t1, true),
	}

	hubs := cityHubsFrom(rows)
	if len(hubs) != 2 {
		t.Fatalf("got %d hubs; want 2 (banfield and quilmes)", len(hubs))
	}
	if hubs[0].Slug != "banfield" || hubs[1].Slug != "quilmes" {
		t.Fatalf("hub slugs = %q, %q; want banfield, quilmes", hubs[0].Slug, hubs[1].Slug)
	}
	banfield := hubs[0]
	if banfield.City != "Banfield" {
		t.Errorf("banfield hub city = %q; want the spelling of its first complex, Banfield", banfield.City)
	}
	if len(banfield.Complexes) != 2 {
		t.Errorf("banfield hub lists %d complexes; want the 2 active ones", len(banfield.Complexes))
	}
	if !banfield.LastMod.Equal(t2) {
		t.Errorf("banfield lastmod = %v; want %v, the latest active complex", banfield.LastMod, t2)
	}
}

func TestHubDescriptionNamesTheComplexesWithinTheLimit(t *testing.T) {
	now := time.Now()
	one := cityHub{Slug: "banfield", City: "Banfield", Complexes: []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true),
	}}
	if got, want := hubDescription(one), "Reservá tu cancha en Banfield: Club Norte."; got != want {
		t.Errorf("one complex: description = %q; want %q", got, want)
	}

	two := cityHub{Slug: "banfield", City: "Banfield", Complexes: []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true),
		hubRow("Canchas Sur", "canchas-sur", "b", "Banfield", nil, now, true),
	}}
	if got, want := hubDescription(two), "Reservá tu cancha en Banfield: Club Norte y Canchas Sur."; got != want {
		t.Errorf("two complexes: description = %q; want %q", got, want)
	}

	four := cityHub{Slug: "banfield", City: "Banfield", Complexes: []complexstore.HubComplex{
		hubRow("Club A", "a", "a", "Banfield", nil, now, true),
		hubRow("Club B", "b", "a", "Banfield", nil, now, true),
		hubRow("Club C", "c", "a", "Banfield", nil, now, true),
		hubRow("Club D", "d", "a", "Banfield", nil, now, true),
	}}
	if got, want := hubDescription(four), "Reservá tu cancha en Banfield: Club A, Club B, Club C y 1 más."; got != want {
		t.Errorf("four complexes: description = %q; want %q", got, want)
	}

	var long []complexstore.HubComplex
	for range 12 {
		long = append(long, hubRow(strings.Repeat("Complejo Deportivo ", 4), "x", "a", "Banfield", nil, now, true))
	}
	got := hubDescription(cityHub{Slug: "banfield", City: "Banfield", Complexes: long})
	if n := utf8.RuneCountInString(got); n > 150 {
		t.Errorf("description runs %d characters; want at most 150", n)
	}
	if !strings.HasPrefix(got, "Reservá tu cancha en Banfield: ") {
		t.Errorf("description %q does not name the city first", got)
	}
}

func TestCityHubPageIsASemanticListOfItsComplexes(t *testing.T) {
	now := time.Now()
	hub := cityHub{Slug: "banfield", City: "Banfield", Complexes: []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "Av. Rivadavia 100", "Banfield", []string{"padel", "tennis"}, now, true),
		hubRow("Canchas Sur", "canchas-sur", "Calle 2 200", "Banfield", []string{"soccer"}, now, true),
	}}

	page := renderCityHub(hub, hubFrontend)

	for _, want := range []string{
		"<title>Canchas en Banfield - Reservá tu cancha | Vibe</title>",
		`<link rel="canonical" href="https://app.vibe.com.ar/canchas/banfield">`,
		"<h1>Canchas en Banfield</h1>",
		`<a href="https://app.vibe.com.ar/c/club-norte">Club Norte</a>`,
		"Av. Rivadavia 100",
		"Pádel, Tenis",
		`<a href="https://app.vibe.com.ar/c/canchas-sur">Canchas Sur</a>`,
		"Calle 2 200",
		"Fútbol",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page is missing %q", want)
		}
	}
}

func TestCityHubPageEscapesOwnerText(t *testing.T) {
	now := time.Now()
	hub := cityHub{Slug: "banfield-b", City: "Banfield <b>", Complexes: []complexstore.HubComplex{
		hubRow(`Club <script>alert(1)</script>`, "club-x", `Av. "Mitre" & 1`, "Banfield <b>", []string{"padel"}, now, true),
	}}

	page := renderCityHub(hub, hubFrontend)

	if strings.Contains(page, "<script>alert(1)") || strings.Contains(page, "<b>") {
		t.Errorf("owner text reached the page unescaped")
	}
	for _, want := range []string{
		"<title>Canchas en Banfield &lt;b&gt; - Reservá tu cancha | Vibe</title>",
		"Club &lt;script&gt;alert(1)&lt;/script&gt;",
		"Av. &#34;Mitre&#34; &amp; 1",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page is missing the escaped form %q", want)
		}
	}
}

func TestCityHubPageCarriesAnItemListOfTheComplexURLs(t *testing.T) {
	now := time.Now()
	hub := cityHub{Slug: "banfield", City: "Banfield", Complexes: []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "a", "Banfield", nil, now, true),
		hubRow("Canchas Sur", "canchas-sur", "b", "Banfield", nil, now, true),
	}}

	page := renderCityHub(hub, hubFrontend)

	const open = `<script type="application/ld+json">`
	start := strings.Index(page, open)
	if start < 0 {
		t.Fatal("page has no JSON-LD block")
	}
	body := page[start+len(open):]
	end := strings.Index(body, "</script>")
	if end < 0 {
		t.Fatal("JSON-LD block is not closed")
	}

	var ld struct {
		Type     string `json:"@type"`
		Elements []struct {
			Type     string `json:"@type"`
			Position int    `json:"position"`
			URL      string `json:"url"`
		} `json:"itemListElement"`
	}
	if err := json.Unmarshal([]byte(body[:end]), &ld); err != nil {
		t.Fatalf("JSON-LD does not parse: %v", err)
	}
	if ld.Type != "ItemList" {
		t.Errorf("JSON-LD @type = %q; want ItemList", ld.Type)
	}
	want := []string{"https://app.vibe.com.ar/c/club-norte", "https://app.vibe.com.ar/c/canchas-sur"}
	if len(ld.Elements) != len(want) {
		t.Fatalf("ItemList has %d items; want %d", len(ld.Elements), len(want))
	}
	for i, el := range ld.Elements {
		if el.Type != "ListItem" || el.Position != i+1 || el.URL != want[i] {
			t.Errorf("item %d = %+v; want ListItem at position %d for %s", i, el, i+1, want[i])
		}
	}
}
