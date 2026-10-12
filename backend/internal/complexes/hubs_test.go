package complexes

import (
	"errors"
	"testing"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
)

// The city hubs and the sitemap read active complexes through the service, the
// same way the sitemap reads its slugs, so publicsite never holds the store.
func TestServiceListActiveComplexesForHubsReturnsTheStoresRows(t *testing.T) {
	f := newFixture(t)
	f.store.hubs = []complexstore.HubComplex{{Name: "Club Norte", Slug: "club-norte", City: "Banfield", IsActive: true}}

	got, err := f.service.ListActiveComplexesForHubs(t.Context())
	if err != nil {
		t.Fatalf("ListActiveComplexesForHubs: %v", err)
	}
	if len(got) != 1 || got[0].Slug != "club-norte" {
		t.Errorf("got %+v; want the one complex the store returned", got)
	}
}

func TestServiceListActiveComplexesForHubsPassesAStoreFailureThrough(t *testing.T) {
	f := newFixture(t)
	f.store.getErr = errors.New("db down")

	if _, err := f.service.ListActiveComplexesForHubs(t.Context()); err == nil {
		t.Error("a store failure was swallowed; the hub would publish as empty instead of answering unavailable")
	}
}

// A city that once had a complex is found through every complex, switched off or
// soft-deleted included, so the service passes the store's city list through.
func TestServiceListComplexCitiesForHubsReturnsTheStoresCities(t *testing.T) {
	f := newFixture(t)
	f.store.cities = []complexstore.HubCity{{City: "Quilmes", Province: "Buenos Aires"}}

	got, err := f.service.ListComplexCitiesForHubs(t.Context())
	if err != nil {
		t.Fatalf("ListComplexCitiesForHubs: %v", err)
	}
	if len(got) != 1 || got[0].City != "Quilmes" {
		t.Errorf("got %+v; want the one city the store returned", got)
	}
}

func TestServiceListComplexCitiesForHubsPassesAStoreFailureThrough(t *testing.T) {
	f := newFixture(t)
	f.store.getErr = errors.New("db down")

	if _, err := f.service.ListComplexCitiesForHubs(t.Context()); err == nil {
		t.Error("a store failure was swallowed; a city with no live complex would answer 404 instead of unavailable")
	}
}
