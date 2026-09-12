package bookings

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// The hours around midnight, which this file has now guarded in both
// directions.
//
// The original defect: the guard compared > 1440, which let exactly 1440
// through. A 23:00 booking on a 60-minute court lands on that boundary and
// slots.FromMinutes wraps its end to "00:00" — a row whose end_time is before
// its start_time, invisible to the overlap check in internal/data/slotguard,
// so the court could be sold a second time for the same hours. Both handlers
// carried the same off-by-one, and both then refused every such booking.
//
// The refusal was the right answer while a booking was a date and two ordered
// times of day. It no longer is: a booking carries a tstzrange
// (the generated span), and what stops the second sale is
// bookings_no_overlapping_span — a database constraint that holds for every
// write path, including the ones nobody has written yet, rather than a check
// two handlers remembered to make. So these hours are sold, and the tests below
// assert that they are.
//
// A 60-minute booking is the case that reaches the boundary from a one-slot
// request; a 90-minute booking would need to start at 23:30 for the same
// shape.

// midnightCourt wires a fixture whose complex stays open until midnight, so
// nothing but the midnight guard itself can reject a 23:00 booking.
func midnightCourt(f *fixture) (complexID, courtID uuid.UUID) {
	complexID, courtID = preparePublicBooking(f)

	for _, s := range f.complexes.schedules {
		s.CloseTime = "00:00"
	}
	// Rebuilt rather than mutated. A band carries its span in minutes, derived
	// from these two times when the row is read (the span_min generated column), so
	// assigning TimeTo on an existing struct changes the label and leaves the
	// span describing the old hours — a fixture that reads correctly and prices
	// something else.
	rebuilt := make([]*courtstore.CourtPrice, 0, len(f.courts.prices))
	for _, p := range f.courts.prices {
		rebuilt = append(rebuilt, courtstore.NewCourtPriceForTest(p.CourtID, p.DayType, p.TimeFrom, "00:00", p.Price))
	}
	f.courts.prices = rebuilt
	f.courts.court = &courtstore.Court{
		ID: courtID, ComplexID: complexID, Name: "Court 1",
		IsActive: true,
	}
	return complexID, courtID
}

// The staff-facing handler. There is no payment step here, so a booking it
// accepts is confirmed immediately — the double sale is complete the moment the
// row lands.
func TestCreateAcceptsABookingThatEndsAtMidnight(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := midnightCourt(f)

	date := time.Now().In(timezone.Argentina).AddDate(0, 0, 7).Format("2006-01-02")
	body := fmt.Sprintf(`{"court_id":%q,"date":%q,"start_time":"23:00","duration_minutes":60,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000"}`,
		courtID, date)

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID, nil, body))

	if w.Code != http.StatusCreated {
		t.Fatalf("23:00-00:00 is an hour the venue is open for and must sell; want 201, got %d (%s)",
			w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 1 {
		t.Fatalf("the booking must be recorded; got %d", len(f.store.inserted))
	}

	// The row is a start and a duration. There is no second clock reading to
	// contradict it: 23:00 + 60 used to be stored as 00:00, which reads as
	// earlier than the start, and that column is gone.
	stored := f.store.inserted[0]
	if stored.StartTime != "23:00" || stored.DurationMinutes != 60 {
		t.Errorf("stored 23:00 for 60 minutes; got %s for %d", stored.StartTime, stored.DurationMinutes)
	}
}

// The public handler carried the same off-by-one, and its bookings are the ones
// a stranger pays for. The complex is open until midnight and the price row
// covers the slot, so nothing may stand between this booking and a checkout.
func TestPublicBookAcceptsABookingThatEndsAtMidnight(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := midnightCourt(f)

	date := time.Now().In(timezone.Argentina).AddDate(0, 0, 7).Format("2006-01-02")
	body := fmt.Sprintf(`{"complex_id":%q,"court_id":%q,"date":%q,"start_time":"23:00","duration_minutes":60,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000",`+
		`"client_email":"ana@example.com"}`, complexID, courtID, date)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", body))

	if w.Code != http.StatusCreated {
		t.Fatalf("the venue is open until midnight and the hour is priced; want 201, got %d (%s)",
			w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 1 {
		t.Fatalf("the booking must be recorded; got %d", len(f.store.inserted))
	}
	if f.checkout.created != 1 {
		t.Errorf("a booking a stranger is about to pay for needs its checkout; got %d", f.checkout.created)
	}
}

// A block on the last hour of the day has to stop a booking that runs into the
// next one, and for a long time it did not.
//
// slotIsBlocked compared clock strings, and the candidate's end came from
// slots.Add, which wraps: 23:00 plus sixty minutes is "00:00". So the check
// asked whether 23:00–00:00 overlaps 23:00–23:59 and answered no, because
// "00:00" is not after "23:00" — the booking read as ending twenty-three hours
// before it began.
//
// Nothing else would have caught it. The bookings_no_overlapping_span exclusion constraint is
// single-table and never sees blocked_slots, so this handler is the only thing
// between an owner's maintenance window and a public sale.
func TestPublicBookRefusesAnOvernightBookingOverABlockedHour(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := midnightCourt(f)

	date := time.Now().In(timezone.Argentina).AddDate(0, 0, 7).Format("2006-01-02")
	f.courts.blocked = map[string][]*courtstore.BlockedSlot{
		date: {{ID: uuid.New(), CourtID: courtID, StartTime: "23:00", EndTime: "23:59"}},
	}

	body := fmt.Sprintf(`{"complex_id":%q,"court_id":%q,"date":%q,"start_time":"23:00","duration_minutes":60,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000",`+
		`"client_email":"ana@example.com"}`, complexID, courtID, date)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", body))

	if w.Code != http.StatusConflict {
		t.Fatalf("the owner blocked 23:00-23:59; a 23:00 booking must be refused. want 409, got %d (%s)",
			w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Errorf("nothing may be written for a blocked hour; got %d inserts", len(f.store.inserted))
	}
}

// The mirror of the case above: the same overnight booking, on a court with
// nothing blocked, still goes through. Without this, the fix could have been
// "refuse everything at 23:00" and the test above would still pass.
func TestPublicBookAcceptsAnOvernightBookingWhenTheBlockIsElsewhere(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := midnightCourt(f)

	date := time.Now().In(timezone.Argentina).AddDate(0, 0, 7).Format("2006-01-02")
	f.courts.blocked = map[string][]*courtstore.BlockedSlot{
		date: {{ID: uuid.New(), CourtID: courtID, StartTime: "10:00", EndTime: "11:00"}},
	}

	body := fmt.Sprintf(`{"complex_id":%q,"court_id":%q,"date":%q,"start_time":"23:00","duration_minutes":60,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000",`+
		`"client_email":"ana@example.com"}`, complexID, courtID, date)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", body))

	if w.Code != http.StatusCreated {
		t.Fatalf("the morning block does not touch 23:00; want 201, got %d (%s)", w.Code, w.Body.String())
	}
}
