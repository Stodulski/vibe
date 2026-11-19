package complexes

import (
	"errors"
	"testing"
)

func TestServiceListActiveComplexesForHubsPassesAStoreFailureThrough(t *testing.T) {
	f := newFixture(t)
	f.store.getErr = errors.New("db down")

	if _, err := f.service.ListActiveComplexesForHubs(t.Context()); err == nil {
		t.Error("a store failure was swallowed; the hub would publish as empty instead of answering unavailable")
	}
}

func TestServiceListComplexCitiesForHubsPassesAStoreFailureThrough(t *testing.T) {
	f := newFixture(t)
	f.store.getErr = errors.New("db down")

	if _, err := f.service.ListComplexCitiesForHubs(t.Context()); err == nil {
		t.Error("a store failure was swallowed; a city with no live complex would answer 404 instead of unavailable")
	}
}
