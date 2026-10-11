package publicsite

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
)

// hubRow is a switched-on or switched-off complex whose courts carry the given
// sports, one entry per active court.
func hubRow(name, slug, address, city string, sports []string, updated time.Time, active bool) complexstore.HubComplex {
	return complexstore.HubComplex{
		ID:          uuid.New(),
		Name:        name,
		Slug:        slug,
		Address:     address,
		City:        city,
		IsActive:    active,
		CourtSports: sports,
		UpdatedAt:   updated,
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
		if got := complexstore.CitySlug(city); got != want {
			t.Errorf("CitySlug(%q) = %q; want %q", city, got, want)
		}
	}
}

func TestDisplayNameNormalizesOnlyAnAllOneCaseName(t *testing.T) {
	tests := map[string]string{
		"caleta olivia":           "Caleta Olivia",
		"VILLA DEL PARQUE":        "Villa del Parque",
		"buenos aires":            "Buenos Aires",
		"  san   isidro  ":        "San Isidro",
		"ÑUÑEZ":                   "Ñuñez",
		"DE LA CRUZ":              "De la Cruz",
		"la plata":                "La Plata",
		"VICENTE-LÓPEZ":           "Vicente-López",
		"San Martín de los Andes": "San Martín de los Andes",
		"Vicente López":           "Vicente López",
		"McDonald":                "McDonald",
		"  Caleta   Olivia ":      "Caleta Olivia",
		"":                        "",
	}
	for input, want := range tests {
		if got := displayName(input); got != want {
			t.Errorf("displayName(%q) = %q; want %q", input, got, want)
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

func TestCityHubsNamesTheHubInDisplayForm(t *testing.T) {
	now := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	rows := []complexstore.HubComplex{
		hubRow("Club Norte", "club-norte", "a", "VILLA DEL PARQUE", nil, now, true),
	}

	hubs := cityHubsFrom(rows)
	if len(hubs) != 1 || hubs[0].City != "Villa del Parque" {
		t.Fatalf("hubs = %+v; want one hub named Villa del Parque", hubs)
	}
}

func TestHubProvinceIsTheMostCommonNamedProvince(t *testing.T) {
	now := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	withProvince := func(province string) complexstore.HubComplex {
		row := hubRow("Club", "club-"+uuid.NewString(), "a", "Caleta Olivia", nil, now, true)
		row.Province = province
		return row
	}
	tests := []struct {
		name string
		rows []complexstore.HubComplex
		want string
	}{
		{
			name: "the most common province wins, spelled as the display form",
			rows: []complexstore.HubComplex{withProvince("santa cruz"), withProvince("Santa Cruz"), withProvince("Chubut")},
			want: "Santa Cruz",
		},
		{
			name: "a tie goes to the name first in alphabetical order",
			rows: []complexstore.HubComplex{withProvince("Santa Cruz"), withProvince("Chubut")},
			want: "Chubut",
		},
		{
			name: "an empty province does not count",
			rows: []complexstore.HubComplex{withProvince(""), withProvince(""), withProvince("Chubut")},
			want: "Chubut",
		},
		{
			name: "no complex names a province",
			rows: []complexstore.HubComplex{withProvince(""), withProvince("  ")},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hubs := cityHubsFrom(tt.rows)
			if len(hubs) != 1 {
				t.Fatalf("got %d hubs; want 1", len(hubs))
			}
			if hubs[0].Province != tt.want {
				t.Errorf("province = %q; want %q", hubs[0].Province, tt.want)
			}
		})
	}
}

func TestCourtCountsBySportCountsEachCourtAndSortsByCountThenLabel(t *testing.T) {
	got := courtCountsBySport([]string{"padel", "soccer", "padel", "tennis", "soccer", "padel", "squash"})
	want := []hubSportCount{
		{Sport: "Pádel", CourtCount: 3},
		{Sport: "Fútbol", CourtCount: 2},
		// Equal counts fall back to the label: Tenis before squash.
		{Sport: "Tenis", CourtCount: 1},
		{Sport: "squash", CourtCount: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("courtCountsBySport = %+v; want %+v", got, want)
	}
}

func TestOtherHubsListsTheSameProvinceFirstThenTheBiggestCappedAtTwelve(t *testing.T) {
	now := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	var rows []complexstore.HubComplex
	add := func(city, province string, complexes int) {
		for i := range complexes {
			row := hubRow(fmt.Sprintf("%s %d", city, i), fmt.Sprintf("%s-%d", complexstore.CitySlug(city), i), "a", city, nil, now, true)
			row.Province = province
			rows = append(rows, row)
		}
	}
	add("Caleta Olivia", "Santa Cruz", 1)
	add("Río Gallegos", "Santa Cruz", 1)
	add("Pico Truncado", "Santa Cruz", 3)
	add("Rosario", "Santa Fe", 5)
	add("Córdoba", "Córdoba", 2)
	for i := range 12 {
		add(fmt.Sprintf("Ciudad %02d", i), "", 1)
	}

	hubs := cityHubsFrom(rows)
	var current cityHub
	for _, hub := range hubs {
		if hub.Slug == "caleta-olivia" {
			current = hub
		}
	}
	others := otherHubsOf(current, hubs)

	if len(others) != hubOtherLimit {
		t.Fatalf("got %d other hubs; want the cap of %d", len(others), hubOtherLimit)
	}
	wantFirst := []string{"Pico Truncado", "Río Gallegos"}
	for i, name := range wantFirst {
		if others[i].Name != name {
			t.Errorf("other_hubs[%d] = %q; want %q (same province, then most complexes, then name)", i, others[i].Name, name)
		}
	}
	if others[2].Name == "Caleta Olivia" {
		t.Error("the hub itself is listed among its other hubs")
	}
	if others[2].Name != "Rosario" {
		t.Errorf("other_hubs[2] = %q; want Rosario, the biggest hub outside the province", others[2].Name)
	}
}
