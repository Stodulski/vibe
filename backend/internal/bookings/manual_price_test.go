package bookings

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// The complex's own staff must be able to book any time of day manually,
// whether or not the complex is open then — that is the whole point of this
// file. staffFixture opens 09:00-23:00 and prices exactly that span
// (openAllWeek), so 03:00 is deliberately outside both: nothing but an
// explicit price, submitted by staff, can make it bookable.

// An explicit price lets an owner book an hour outside the complex's opening
// hours and outside every price rule.
func TestTheOwnersDashboardBooksAnOffHoursTimeWithAnExplicitPrice(t *testing.T) {
	f, complexID, courtID := staffFixture(t)

	const manualPrice = 400_000
	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String()}, staffBookBodyWithPrice(courtID, "03:00", 90, manualPrice)))

	if w.Code != http.StatusCreated {
		t.Fatalf("an explicit price must let an off-hours booking through; want 201, got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 1 {
		t.Fatalf("want the booking stored; got %d", len(f.store.inserted))
	}
	if f.store.inserted[0].StartTime != "03:00" {
		t.Errorf("want the booking stored at 03:00; got %s", f.store.inserted[0].StartTime)
	}
	if f.store.inserted[0].Price != manualPrice {
		t.Errorf("the booking was priced %d, want the submitted %d", f.store.inserted[0].Price, manualPrice)
	}
}

// Without a price, the same off-hours booking is refused with a field-level
// error on price — the signal the frontend uses to show the manual price
// input — rather than a generic failure.
func TestTheOwnersDashboardRefusesAnOffHoursTimeWithoutAPrice(t *testing.T) {
	f, complexID, courtID := staffFixture(t)

	w := staffCreate(t, f, complexID, courtID, "03:00")

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("an off-hours booking with no price rule and no override must be refused; want 422, got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"price"`) {
		t.Errorf("the refusal must name the price field; got %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), httpx.CodePriceRequired) {
		t.Errorf("the refusal must carry %s; got %s", httpx.CodePriceRequired, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("a booking was stored without a price for an unpriced slot")
	}
}

// A booking that runs into the following day is an ordinary sale. It used to be
// refused because bookings carried a single date and two ordered times of day;
// it now carries a range (the generated span) and the check that forbade the
// shape is gone, so the owner's dashboard books 23:30 to 01:30 the way it
// books any other pair of hours.
func TestTheOwnersDashboardBooksAcrossMidnight(t *testing.T) {
	f, complexID, courtID := staffFixture(t)

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String()}, staffBookBodyWithPrice(courtID, "23:30", 120, 400_000)))

	if w.Code != http.StatusCreated {
		t.Fatalf("23:30 to 01:30 is an ordinary two-hour booking; want 201, got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 1 {
		t.Fatalf("the booking must be recorded; got %d", len(f.store.inserted))
	}

	// The booking is stored as a start and a duration, and nothing else. There
	// is no second clock reading to disagree with the span: bookings.end_time
	// dropped the column that carried one, precisely because 01:30 is smaller
	// than 23:30 and says nothing about which day it is on.
	stored := f.store.inserted[0]
	if stored.StartTime != "23:30" || stored.DurationMinutes != 120 {
		t.Errorf("stored 23:30 for 120 minutes; got %s for %d", stored.StartTime, stored.DurationMinutes)
	}
}

// 10:07 is not on the package's 30-minute grid step, so it cannot be
// represented by anything downstream — no slot lock, no reconciliation
// against the storefront — even though the owner's dashboard does not care
// whether the complex is open at that hour.
func TestTheOwnersDashboardRefusesAnOffBoundaryTimeEvenOffHours(t *testing.T) {
	f, complexID, courtID := staffFixture(t)

	w := staffCreate(t, f, complexID, courtID, "10:07")

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("10:07 is not on the 30-minute step; want 422, got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "start_time") {
		t.Errorf("the refusal must name start_time; got %s", w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("a booking was stored off the 30-minute step")
	}
}

// Dropping the opening-hours check does not weaken slotTaken: an off-hours
// booking that overlaps a live one is still refused, exactly as an in-hours
// one would be. The overlap check itself lives in internal/data/slot_guard.go
// and is duration- and hours-agnostic; this pins that the owner write path
// still surfaces its refusal rather than skipping it for off-hours requests.
func TestAnOffHoursOwnerBookingIsStillRefusedWhenItOverlapsAnExistingBooking(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	f.store.insertErr = bookingstore.ErrSlotUnavailable

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String()}, staffBookBodyWithPrice(courtID, "03:00", 90, 400_000)))

	if w.Code != http.StatusConflict {
		t.Fatalf("an off-hours booking overlapping a live one must still be refused; want 409, got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("a booking was stored over an overlapping one")
	}
}

// The public path is unchanged: 03:00 is still outside the complex's opening
// hours, and the storefront still refuses it — regardless of what the owner's
// dashboard now accepts.
func TestThePublicPathStillRefusesAnOffHoursStartTime(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	f.complexes.complex = publicComplex(complexID)

	w := publicBookAt(t, f, complexID, courtID, "03:00")

	if w.Code != http.StatusConflict {
		t.Errorf("03:00 is outside the complex's opening hours; want 409, got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("a public booking was stored outside the complex's opening hours")
	}
	if f.checkout.created != 0 {
		t.Error("a client was sent to MercadoPago for a time the complex is closed")
	}
}

// The public path's request body has no price field at all, and
// httpx.ReadJSON runs with DisallowUnknownFields — so a client-submitted
// price is not silently ignored, it is rejected outright as a 400 before any
// booking logic runs. A customer can never name their own price.
func TestThePublicPathRejectsARequestBodyCarryingAPrice(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	f.complexes.complex = publicComplex(complexID)
	date, _ := bookableDate()

	body := `{"complex_id":"` + complexID.String() + `","court_id":"` + courtID.String() +
		`","date":"` + date.Format("2006-01-02") + `","start_time":"18:00","duration_minutes":90,` +
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000",` +
		`"client_email":"ana@example.com","price":1}`

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", body))

	if w.Code != http.StatusBadRequest {
		t.Errorf("a price field in the public request body must be rejected; want 400, got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("a booking was stored from a request carrying a client-submitted price")
	}
}
