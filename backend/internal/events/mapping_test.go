package events

import (
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
)

// The public event lists each court with what a player compares (roof, surface,
// lighting), in the same order as court_names, so a ticket can describe every court.
func TestPublicEventCarriesEachCourtWithItsFacts(t *testing.T) {
	grass := "synthetic_grass"
	first, second := uuid.New(), uuid.New()
	e := &eventstore.Event{ID: uuid.New(), Courts: []eventstore.Court{
		{ID: first, Name: "Cancha 1", CourtType: "indoor", Surface: &grass, HasLighting: true},
		{ID: second, Name: "Cancha 2", CourtType: "outdoor"},
	}}
	got := toGenPublicEvent(e, &Page{Complex: complexstore.NewComplexForTest(uuid.New(), nil, nil)})

	if len(got.CourtNames) != 2 || got.CourtNames[0] != "Cancha 1" || got.CourtNames[1] != "Cancha 2" {
		t.Fatalf("court_names changed: %v", got.CourtNames)
	}
	if len(got.Courts) != 2 {
		t.Fatalf("courts = %+v", got.Courts)
	}
	a, b := got.Courts[0], got.Courts[1]
	if a.Id != first || a.Name != "Cancha 1" || string(a.CourtType) != "indoor" || a.Surface == nil || string(*a.Surface) != grass || !a.HasLighting {
		t.Fatalf("first court = %+v", a)
	}
	if b.Id != second || string(b.CourtType) != "outdoor" || b.Surface != nil || b.HasLighting {
		t.Fatalf("second court = %+v", b)
	}
}
