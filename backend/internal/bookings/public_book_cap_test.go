package bookings

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
)

// POST /book without a hold token used to insert under no per-address cap at
// all, so a script that never took holds could keep a venue's slots in unpaid
// checkouts without limit. It now inserts under the same cap a hold does, and
// an address over it is refused before any checkout is created.
func TestAStorefrontBookingWithoutAHoldIsRefusedOverTheAddressCap(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	f.store.publicCapErr = bookingstore.ErrHoldLimit

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("an address over the cap must be answered 429; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.publicCaps) != 1 || f.store.publicCaps[0] != maxActiveHoldsPerIP {
		t.Errorf("the booking must be inserted under the hold cap; got caps %v", f.store.publicCaps)
	}
	if len(f.store.inserted) != 0 {
		t.Errorf("a refused booking must not be inserted; got %d", len(f.store.inserted))
	}
	if f.checkout.created != 0 {
		t.Errorf("no checkout may be created for a refused booking; got %d", f.checkout.created)
	}
	if len(f.locks.releasedWith) != 1 {
		t.Errorf("the checkout's slot lock must be released; got %d releases", len(f.locks.releasedWith))
	}
}

// S2, finding 1: POST /book without a hold token is refused by the address's
// spent budget like a hold is, with the same kind the storefront tells apart
// from somebody else holding the slot, and no checkout created.
func TestAStorefrontBookingWithoutAHoldIsRefusedByTheCooldown(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	until := time.Now().Add(6 * time.Minute)
	f.store.publicCapErr = &bookingstore.HoldCooldownError{Until: until}

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if problemKind(body) != string(KindHoldCooldown) {
		t.Errorf("want kind %q; got %q", KindHoldCooldown, problemKind(body))
	}
	if detail, _ := body["detail"].(string); !strings.Contains(detail, clock(until)) {
		t.Errorf("want the detail to name %s; got %q", clock(until), detail)
	}
	if f.checkout.created != 0 {
		t.Errorf("no checkout may be created for a refused booking; got %d", f.checkout.created)
	}
}
