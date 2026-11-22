package bookings

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// L19 (6): a complex that takes no deposit charges the whole price at the
// checkout, and the checkout says so; one that takes a deposit still asks
// for a seña.
func TestTheCheckoutSaysWhetherItChargesADepositOrTheWholePrice(t *testing.T) {
	for name, tc := range map[string]struct {
		percentage int
		full       bool
	}{"no deposit": {0, true}, "a deposit": {30, false}} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			complexID, courtID := preparePublicBooking(f)
			f.complexes.complex.DepositPercentage = tc.percentage

			w := httptest.NewRecorder()
			f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

			if w.Code != http.StatusCreated {
				t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
			}
			if f.checkout.lastInput.FullPayment != tc.full {
				t.Errorf("FullPayment = %v, want %v (deposit %d of price %d)", f.checkout.lastInput.FullPayment, tc.full,
					f.store.inserted[0].DepositAmount, f.store.inserted[0].Price)
			}
		})
	}
}
