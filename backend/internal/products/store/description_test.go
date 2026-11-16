package store

import "testing"

// The cash register shows this description as stored: Spanish, product and
// quantity first, the owner's note after a period, never an em dash.
func TestRestockCashDescription(t *testing.T) {
	note := "Proveedor nuevo"
	empty := ""
	if got := restockCashDescription("Grip", 3, nil); got != "Reposición: Grip x3" {
		t.Errorf("got %q", got)
	}
	if got := restockCashDescription("Grip", 3, &empty); got != "Reposición: Grip x3" {
		t.Errorf("empty note: got %q", got)
	}
	if got := restockCashDescription("Grip", 3, &note); got != "Reposición: Grip x3. Proveedor nuevo" {
		t.Errorf("with note: got %q", got)
	}
}
