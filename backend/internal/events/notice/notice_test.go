package notice

import (
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
)

// S8: the event confirmation gives the address with its city, as every other
// email does, and the complex's contact for the client's questions.
func TestRegisteredCarriesTheAddressWithItsCityAndTheContact(t *testing.T) {
	email := "ana@example.com"
	owner := uuid.New()
	n := Registered(&eventstore.Registration{ID: uuid.New(), FirstName: "Ana", Email: &email},
		&eventstore.Event{ID: uuid.New(), Name: "Canchas abiertas", Date: time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)},
		&complexstore.Complex{Name: "Vibe", Slug: "vibe", Address: "Av. Santa Fe 1200", City: "Rosario",
			Phone: "+5493411234567", OwnerID: owner}, "https://vibe.test")

	if n.Address != "Av. Santa Fe 1200, Rosario" {
		t.Errorf("Address = %q, want the street with its city", n.Address)
	}
	if n.ComplexPhone != "+5493411234567" || n.OwnerID != owner.String() {
		t.Errorf("contact = %q / %q", n.ComplexPhone, n.OwnerID)
	}
}
