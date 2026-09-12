package bookings

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/slots"
)

// The fixture's court is sold in 90-minute slots and its complex opens at
// 09:00, so its grid runs 09:00, 10:30, 12:00, 13:30, 15:00, 16:30, 18:00,
// 19:30, 21:00. onGrid is the position every test here books; offGrid is
// forty-five minutes into it — inside the opening hours, priced by the same
// band, and not a position the storefront ever drew.
const (
	onGrid  = "18:00"
	offGrid = "18:45"
	// onGridEnd is where a booking starting at onGrid finishes.
	onGridEnd = "19:30"
)

// staffFixture wires the owner-dashboard path: a court, an open week, and a
// client, so the only thing left to refuse a booking is the rule under test.
func staffFixture(t *testing.T) (f *fixture, complexID, courtID uuid.UUID) {
	t.Helper()

	f = newFixture(t)
	complexID, courtID = uuid.New(), uuid.New()
	openAllWeek(f, courtID, "09:00", "23:00")
	f.courts.court = &courtstore.Court{
		ID: courtID, ComplexID: complexID, Name: "Court 1",
		IsActive: true,
	}
	f.clients.client = &clientstore.Client{ID: uuid.New(), FirstName: "Ana", Phone: "+541100000000"}
	return f, complexID, courtID
}

func staffCreate(t *testing.T, f *fixture, complexID, courtID uuid.UUID, startTime string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String()}, staffBookBody(courtID, startTime)))
	return w
}

func publicBookAt(t *testing.T, f *fixture, complexID, courtID uuid.UUID, startTime string) *httptest.ResponseRecorder {
	t.Helper()

	date, _ := bookableDate()
	body := fmt.Sprintf(`{"complex_id":%q,"court_id":%q,"date":%q,"start_time":%q,"duration_minutes":90,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000",`+
		`"client_email":"ana@example.com"}`, complexID, courtID, date.Format("2006-01-02"), startTime)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", body))
	return w
}

// ─── Finding 88, at the handlers that consume ValidFormat ───────────────────

// Both write paths copy input.StartTime into the booking verbatim and derive
// everything else from slots.ToMinutes, so a string ValidFormat accepts but is
// not HH:MM is stored as one time and shown to the client as another.
//
// This used to be one test asserting the two paths validate identically, which
// stopped describing reality once the owner path dropped the grid/schedule
// check the public path still runs (see grid.go). What survives the split is
// narrower but still real on both paths: ValidFormat itself is unconditional,
// so a malformed string is refused as bad input regardless of whether the
// minute it would misread as happens to be one the path in question would
// otherwise accept. The values below reinterpret to 18:00 or 09:00 — both real
// positions on the fixture's grid and inside the priced band — precisely so
// that nothing downstream, on either path, could be the thing that refuses
// them instead of ValidFormat.
func TestThePublicPathRefusesAStartTimeItWouldReinterpret(t *testing.T) {
	notTimes := []struct {
		in        string
		readAsMin int
	}{
		{"18:0x", 1080}, // 18:00
		{"+9:00", 540},  // 09:00
	}

	for _, tt := range notTimes {
		// The premise: the string is not HH:MM, and the minute it would be
		// read as is a position on this fixture's grid. Without both halves
		// the assertions below could pass for the wrong reason.
		if slots.ToMinutes(tt.in) != tt.readAsMin {
			t.Fatalf("ToMinutes(%q) = %d, want %d", tt.in, slots.ToMinutes(tt.in), tt.readAsMin)
		}
		if (tt.readAsMin-slots.ToMinutes("09:00"))%90 != 0 {
			t.Fatalf("%q reinterprets to %s, which is not on the fixture's grid — pick another",
				tt.in, slots.FromMinutes(tt.readAsMin))
		}

		t.Run(tt.in, func(t *testing.T) {
			f, complexID, courtID := staffFixture(t)
			f.complexes.complex = publicComplex(complexID)

			w := publicBookAt(t, f, complexID, courtID, tt.in)

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("%q is not HH:MM and must be refused as input; got %d (%s)", tt.in, w.Code, w.Body.String())
			}
			if len(f.store.inserted) != 0 {
				t.Errorf("a booking was stored for %q, at %s", tt.in, f.store.inserted[0].StartTime)
			}
			if f.checkout.created != 0 {
				t.Error("a client was sent to MercadoPago for a time that is not a time")
			}
		})
	}
}

// The owner's dashboard keeps refusing a malformed start time too — ValidFormat
// runs before the grid ever did and still runs unconditionally — but the
// reason this is worth pinning has flipped from the public test above: it is
// no longer that nothing else on the path *could* refuse it (the old premise,
// now false — the owner path would happily accept a real off-grid or
// off-hours minute), it is that a malformed string is bad input regardless of
// what it would otherwise be read as.
func TestTheOwnersDashboardRefusesAMalformedStartTimeEvenThoughItAcceptsAnyRealHour(t *testing.T) {
	notTimes := []string{"18:0x", "+9:00"}

	for _, in := range notTimes {
		t.Run(in, func(t *testing.T) {
			f, complexID, courtID := staffFixture(t)

			w := staffCreate(t, f, complexID, courtID, in)

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("%q is not HH:MM and must be refused as input; got %d (%s)", in, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "start_time") {
				t.Errorf("the refusal must name start_time; got %s", w.Body.String())
			}
			if len(f.store.inserted) != 0 {
				t.Errorf("a booking was stored for %q, at %s", in, f.store.inserted[0].StartTime)
			}
		})
	}
}

// publicComplex is the active, MercadoPago-connected complex the public path
// needs before it will get as far as the rules under test.
func publicComplex(complexID uuid.UUID) *complexstore.Complex {
	token := "seller-token"
	c := complexstore.NewComplexForTest(complexID, &token, nil)
	c.Name = "Vibe"
	c.Slug = "vibe"
	c.IsActive = true
	c.DepositPercentage = 30
	c.CancellationHours = 24
	return c
}

// ─── Finding 41: off-grid starts ────────────────────────────────────────────

// 18:45 is inside the opening hours and inside the price band, so every other
// check on both paths passes it. On the owner's dashboard it is no longer the
// court's own start times that refuse it — that check belongs to the public
// path alone now — but it is still not a position slots.OnGridStep's
// 30-minute step can express: nothing can hold a slot lock keyed on a start
// time no other request will ever ask for.
func TestTheOwnersDashboardRefusesAnOffBoundaryStartTime(t *testing.T) {
	f, complexID, courtID := staffFixture(t)

	// The premise, asserted rather than assumed: this start is off the
	// 30-minute step, and it is priced — so the price check cannot be what
	// refuses it.
	if slots.OnGridStep(offGrid) {
		t.Fatalf("%s must be off the 30-minute step for this test to mean anything", offGrid)
	}
	if !slots.OnGridStep(onGrid) {
		t.Fatalf("%s must be on the 30-minute step", onGrid)
	}

	w := staffCreate(t, f, complexID, courtID, offGrid)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("%s does not fall on the 30-minute step; want 422, got %d (%s)", offGrid, w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "start_time") {
		t.Errorf("the refusal must name start_time; got %s", w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Errorf("a booking was stored at %s, off the 30-minute step", f.store.inserted[0].StartTime)
	}

	// And the same handler still accepts a position on the step, so this is a
	// boundary check and not a blanket refusal.
	f2, complexID2, courtID2 := staffFixture(t)
	if w := staffCreate(t, f2, complexID2, courtID2, onGrid); w.Code != http.StatusCreated {
		t.Errorf("%s is on the 30-minute step and must still be bookable; got %d (%s)", onGrid, w.Code, w.Body.String())
	}
}

func TestThePublicPageRefusesAnOffGridStartTime(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	f.complexes.complex = publicComplex(complexID)

	w := publicBookAt(t, f, complexID, courtID, offGrid)

	if w.Code != http.StatusConflict {
		t.Errorf("%s is not one of the court's start times; want 409, got %d (%s)", offGrid, w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Errorf("a booking was stored at %s, across two grid positions", f.store.inserted[0].StartTime)
	}
	if len(f.locks.acquired) != 0 {
		t.Errorf("a slot lock was taken at a start time no other request can name: %v", f.locks.acquired)
	}
	if f.checkout.created != 0 {
		t.Error("a client was sent to MercadoPago for a slot that is not for sale")
	}
}

// The owner's dashboard deliberately does not ask whether the venue is open
// at all: the complex's own staff may book any time of day manually, whether
// or not the complex is open then. The public page still asks — see
// TestThePublicPageRefusesADayTheComplexIsShut — and the two paths write to
// the same table.
func TestTheOwnersDashboardAllowsADayTheComplexIsShut(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	date, _ := bookableDate()
	for _, s := range f.complexes.schedules {
		if s.Day == slots.DayName(date.Weekday()) {
			s.IsClosed = true
		}
	}

	w := staffCreate(t, f, complexID, courtID, onGrid)

	if w.Code != http.StatusCreated {
		t.Errorf("want 201 for staff booking on a day the venue is shut; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 1 {
		t.Error("no booking was stored on a day the complex is closed, though staff may book one")
	}
}

// The public page keeps asking whether the venue is open, unchanged.
func TestThePublicPageRefusesADayTheComplexIsShut(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	f.complexes.complex = publicComplex(complexID)
	date, _ := bookableDate()
	for _, s := range f.complexes.schedules {
		if s.Day == slots.DayName(date.Weekday()) {
			s.IsClosed = true
		}
	}

	w := publicBookAt(t, f, complexID, courtID, onGrid)

	if w.Code != http.StatusConflict {
		t.Errorf("want 409 for a public booking on a day the venue is shut; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("a public booking was stored on a day the complex is closed")
	}
}

// ─── Finding 39: blocked slots ──────────────────────────────────────────────

// The blocked-slot write refuses to block over a live booking. Nothing refused
// a booking over a live block, on any of the three writes that can put a
// confirmed booking on a court.
func TestTheOwnersDashboardRefusesHoursTheOwnerBlocked(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	date, _ := bookableDate()
	f.courts.block(courtID, date, onGrid, onGridEnd)

	w := staffCreate(t, f, complexID, courtID, onGrid)

	if w.Code != http.StatusConflict {
		t.Errorf("want 409 for hours taken off sale; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("a booking was stored over a blocked slot")
	}
	// The read must have been made for this court on this date. A check that
	// asked about another court, or another day, would answer "not blocked"
	// for every request and pass every test that only counts insertions.
	if len(f.courts.blockedQueries) == 0 {
		t.Fatal("the handler never asked whether the slot was blocked")
	}
	q := f.courts.blockedQueries[0]
	if q.courtID != courtID || q.date != date.Format("2006-01-02") {
		t.Errorf("blocked slots were read for court %s on %s; want %s on %s",
			q.courtID, q.date, courtID, date.Format("2006-01-02"))
	}
}

func TestThePublicPageRefusesHoursTheOwnerBlocked(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	f.complexes.complex = publicComplex(complexID)
	date, _ := bookableDate()
	f.courts.block(courtID, date, onGrid, onGridEnd)

	w := publicBookAt(t, f, complexID, courtID, onGrid)

	if w.Code != http.StatusConflict {
		t.Errorf("want 409 for hours taken off sale; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("a booking was stored over a blocked slot")
	}
	if len(f.locks.acquired) != 0 {
		t.Errorf("slots were held for a booking that cannot be made: %v", f.locks.acquired)
	}
	if f.checkout.created != 0 {
		t.Error("a client was charged for hours the owner had withdrawn")
	}
}

// Confirming a pending booking is what makes it hold its slot, and it is the
// moment an owner's block placed during checkout has to be seen. The
// transaction behind it re-checks the bookings table under the court-day
// advisory lock and does not look at blocked_slots at all.
func TestConfirmingAPaymentRefusesHoursTheOwnerBlocked(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	date, _ := bookableDate()

	// The instants matter as much as the clock strings: confirming reads the
	// booking's own span to ask whether those hours are still on sale, the same
	// way the database stores it. A row read from the database always carries
	// them, so a fixture without them is not a booking the code will ever meet.
	booking := &bookingstore.Booking{
		ID: uuid.New(), ComplexID: complexID, CourtID: courtID, ClientID: uuid.New(),
		Date: date, StartTime: onGrid,
		StartsAt: slots.At(date, onGrid), EndsAt: slots.At(date, onGridEnd),
		Price: 500_000, Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid,
		RefundStatus: bookingstore.RefundStatusNone,
	}
	f.store.booking = booking
	f.courts.block(courtID, date, onGrid, onGridEnd)

	w := httptest.NewRecorder()
	f.handler.ConfirmPayment(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String(), "bookingID": booking.ID.String()},
		`{"method":"cash","amount":500000}`))

	if w.Code != http.StatusConflict {
		t.Errorf("want 409 confirming onto hours taken off sale; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.payments.confirmed) != 0 {
		t.Error("the booking was confirmed onto a blocked slot, and a payment recorded against it")
	}
	if booking.Status == "confirmed" {
		t.Error("a refused confirmation must not leave the booking marked confirmed")
	}
}

// A block that ends where the booking starts does not touch it. Without this
// the cheapest way to pass the tests above is to refuse whenever the court has
// any block on that date at all.
func TestABookingMayStartTheMinuteABlockEnds(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	date, _ := bookableDate()
	f.courts.block(courtID, date, "16:30", onGrid)

	w := staffCreate(t, f, complexID, courtID, onGrid)

	if w.Code != http.StatusCreated {
		t.Errorf("a block ending at %s does not cover a booking starting at %s; got %d (%s)",
			onGrid, onGrid, w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 1 {
		t.Errorf("want the booking stored; got %d", len(f.store.inserted))
	}
}

// If the blocked-slot read fails there is no answer to "are these hours on
// sale", and the honest response is a server error rather than proceeding as
// though nothing were blocked.
func TestAFailedBlockedSlotReadRefusesTheBooking(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	f.courts.blockedErr = errors.New("database unavailable")

	w := staffCreate(t, f, complexID, courtID, onGrid)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500 when the blocked-slot read fails; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("a booking was stored without knowing whether the hours were on sale")
	}
}

// ─── Finding 95: one grid interval ──────────────────────────────────────────

// The midnight boundary, the schedule containment and the after-midnight
// closing were each computed from a 1440 written out at the call site — three
// literals across two handlers. They now come from slots.Grid, and this asserts
// the property those literals existed to hold, over every permitted slot
// length, rather than the absence of a number from a file.
func TestTheGridIsTheOnlyThingThatKnowsHowLongADayIs(t *testing.T) {
	for _, duration := range slots.PermittedDurations() {
		grid := slots.NewGrid("09:00", "00:00")

		// The property, stated as the relationship rather than as a number:
		// every position the grid offers is a position it will sell.
		for _, s := range grid.Slots(duration) {
			if err := grid.Validate(s.Start, duration); err != nil {
				t.Errorf("%d-minute grid offers %s but refuses to sell it: %v", duration, s.Start, err)
			}
		}

		// And the boundary is closing time, not midnight. This used to assert
		// ErrCrossesMidnight here: a venue trading to 00:00 could not sell its
		// last slot, because a booking was a date and two ordered times of day
		// and 24:00 was not expressible. It carries a range now
		// (the generated span), so the hours run to closing and the refusal past
		// it names the schedule.
		if err := grid.Validate(slots.FromMinutes(1440-duration), duration); err != nil {
			t.Errorf("%d-minute grid must sell right up to a midnight close; got %v", duration, err)
		}
		if err := grid.Validate(slots.FromMinutes(1440-duration+30), duration); !errors.Is(err, slots.ErrOutsideSchedule) {
			t.Errorf("%d-minute grid: past closing must be ErrOutsideSchedule; got %v", duration, err)
		}
	}
}

// A venue that trades past midnight sells the hours it trades. This is the
// whole point of the range migration, expressed at the grid: 18:00-02:00 means
// eight hours on sale, not six with the last two withheld because a TIME column
// could not say "tomorrow".
func TestAnOvernightScheduleSellsItsHoursPastMidnight(t *testing.T) {
	grid := slots.NewGrid("18:00", "02:00")

	got := grid.Slots(60)
	if len(got) == 0 {
		t.Fatal("an 18:00-02:00 venue must offer something")
	}

	last := got[len(got)-1]
	if last.Start != "01:00" || last.End != "02:00" {
		t.Errorf("the last 60-minute slot of an 18:00-02:00 day is 01:00-02:00; got %s-%s", last.Start, last.End)
	}

	// The slot that spans the boundary is offered and sellable, and its end
	// reads as a clock time on the following day rather than as 24:00.
	if err := grid.Validate("23:30", 60); err != nil {
		t.Errorf("23:30-00:30 is inside 18:00-02:00 and must sell; got %v", err)
	}
	if err := grid.Validate("01:30", 60); !errors.Is(err, slots.ErrOutsideSchedule) {
		t.Errorf("01:30-02:30 runs past closing; want ErrOutsideSchedule, got %v", err)
	}
}

// A sanity check on the fixture the rest of this file leans on: the public
// write path and the storefront derive the same grid from the same open/close
// hours, so a position one accepts for a given duration is a position the
// other offers for that same duration. This used to also describe the owner's
// dashboard, back when both write paths validated against slots.Grid; now that
// the owner path books off it entirely, that half of the old assertion is
// false by design (see the companion test below) and only the public path's
// half survives here.
func TestThePublicPathAcceptsExactlyWhatTheStorefrontOffers(t *testing.T) {
	grid := slots.NewGrid("09:00", "23:00")
	const duration = 90
	offered := map[string]bool{}
	for _, s := range grid.Slots(duration) {
		offered[s.Start] = true
	}

	for m := 0; m < 1440; m++ {
		start := slots.FromMinutes(m)
		accepted := grid.Validate(start, duration) == nil
		if accepted != offered[start] {
			t.Fatalf("%s: the public write path %s it, the storefront %s it",
				start, verb(accepted, "accepts", "refuses"), verb(offered[start], "offers", "omits"))
		}
	}
}

// The owner's dashboard now accepts positions the storefront never offers —
// any real HH:MM on the 30-minute step, whether or not it is inside the
// complex's opening hours — which is the opposite of the property the test
// above pins for the public path. This asserts that inversion directly: every
// minute the storefront's grid omits for this duration, because it falls
// outside the fixture's 09:00-23:00 hours, is a start the owner's dashboard
// still accepts (findPrice aside — 09:00-23:00 is also the fixture's priced
// span, so an omitted minute here is one only the schedule, not the price,
// would have refused).
func TestTheOwnersDashboardAcceptsStartTimesTheStorefrontOmits(t *testing.T) {
	grid := slots.NewGrid("09:00", "23:00")
	const duration = 90
	offered := map[string]bool{}
	for _, s := range grid.Slots(duration) {
		offered[s.Start] = true
	}

	// 03:00 is outside 09:00-23:00, on the 30-minute step, and not a position
	// this fixture's grid ever offers for a 90-minute booking.
	const omittedByStorefront = "03:00"
	if offered[omittedByStorefront] {
		t.Fatalf("%s must be a position the storefront omits for this test to mean anything", omittedByStorefront)
	}
	if !slots.OnGridStep(omittedByStorefront) {
		t.Fatalf("%s must be on the 30-minute step", omittedByStorefront)
	}

	f, complexID, courtID := staffFixture(t)
	w := httptest.NewRecorder()
	body := staffBookBodyWithPrice(courtID, omittedByStorefront, 90, 100_000)
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String()}, body))

	if w.Code != http.StatusCreated {
		t.Errorf("the storefront omits %s, but the owner's dashboard must still accept it; got %d (%s)",
			omittedByStorefront, w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 1 {
		t.Errorf("want the booking stored; got %d", len(f.store.inserted))
	}
}

func verb(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}

// staffBookBody's date is a week out, so nothing here is affected by the
// "already passed" guard; this pins that down rather than leaving it implied.
func TestTheFixtureBooksAFutureDate(t *testing.T) {
	date, _ := bookableDate()
	if !date.After(time.Now()) {
		t.Fatalf("bookableDate returned %s, which is not in the future", date)
	}
}

// A booking that runs past midnight has to see the next day's blocks too, and
// slotIsBlocked read only the date the booking was filed under.
//
// The refusal was never in danger — InsertSafe's transactional guard compares
// spans and has no date filter — but it arrives as the generic "slot
// unavailable" 409, which tells a client the hours were sold when in fact the
// owner closed the court. This is the pre-check's whole job: to say which of
// the two happened while the request still knows.
//
// It calls slotIsBlocked directly rather than posting a booking, because the
// handlers' schedule check refuses hours past the venue's closing time before
// the blocked-slot read is ever reached, and the rule under test is the read.
func TestABookingCrossingMidnightSeesTheNextDaysBlock(t *testing.T) {
	f, _, courtID := staffFixture(t)
	date, _ := bookableDate()
	f.courts.block(courtID, date.AddDate(0, 0, 1), "00:00", "01:00")

	startAt := slots.At(date, "23:00")
	endAt := startAt.Add(2 * time.Hour)

	blocked, err := f.service.slotIsBlocked(context.Background(), courtID, date, startAt, endAt)
	if err != nil {
		t.Fatalf("slotIsBlocked: %v", err)
	}
	if !blocked {
		t.Fatalf("23:00 + 2h runs into a 00:00-01:00 block on the following day and must read as blocked; "+
			"the read asked only about %s", date.Format("2006-01-02"))
	}

	next := date.AddDate(0, 0, 1).Format("2006-01-02")
	var asked bool
	for _, q := range f.courts.blockedQueries {
		if q.courtID == courtID && q.date == next {
			asked = true
		}
	}
	if !asked {
		t.Errorf("the following day's blocks were never read; queries = %v", f.courts.blockedQueries)
	}
}

// The control: the same block, and a booking that ends before midnight. The
// second day must not be read at all, or every ordinary booking pays for a
// query it has no use for — and a block at 00:00 would start refusing hours it
// does not touch.
func TestABookingThatEndsBeforeMidnightIgnoresTheNextDay(t *testing.T) {
	f, _, courtID := staffFixture(t)
	date, _ := bookableDate()
	f.courts.block(courtID, date.AddDate(0, 0, 1), "00:00", "01:00")

	startAt := slots.At(date, "22:00")
	endAt := startAt.Add(2 * time.Hour)

	blocked, err := f.service.slotIsBlocked(context.Background(), courtID, date, startAt, endAt)
	if err != nil {
		t.Fatalf("slotIsBlocked: %v", err)
	}
	if blocked {
		t.Error("22:00 + 2h ends exactly at midnight and touches no part of a 00:00-01:00 block")
	}
	if len(f.courts.blockedQueries) != 1 {
		t.Errorf("one day's blocks are enough for a booking that stays inside its date; got %d reads",
			len(f.courts.blockedQueries))
	}
}
