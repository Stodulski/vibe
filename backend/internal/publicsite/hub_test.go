package publicsite

import (
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
)

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
