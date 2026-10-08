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
