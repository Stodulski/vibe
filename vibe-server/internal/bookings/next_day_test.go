package bookings

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// A booking that runs past midnight is the case every one of these tests is
// about, and until bookings.end_time was dropped the product had no way to say so.
//
// bookings.end_time stored what the clock would read — internal/slots.Add is
// modular arithmetic — so a 23:00 booking of two hours shipped "01:00" on
// every payload and into every confirmation message, with nothing saying which
// 01:00. A client read an end two hours before its own start. The replacement
// is the span's two instants on the wire and one rendering of them in copy.

// overnightBooking is a confirmed 23:00 booking of two hours, carrying the two
// instants a row read out of the database always carries.
//
// Date is anchored the way pgx hands a `date` column back — midnight UTC —
// because that is the value the confirmation path actually meets, and it is
// what made the block check on that path answer about the wrong hours.
func overnightBooking(complexID uuid.UUID) *data.Booking {
	b := futureBooking(complexID)
	day := timezone.Day(b.Date)

	b.Date = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	b.StartTime = "23:00"
	b.DurationMinutes = 120
	b.StartsAt = slots.At(day, "23:00")
	b.EndsAt = b.StartsAt.Add(2 * time.Hour)
	return b
}

// The public success page renders from PublicStatus alone when the browser has
// no cached copy of the booking. For a booking that crosses midnight the end
// is the one field that cannot be expressed as a time of day, so the payload
// carries it as an instant — and the instant has to land on the following
// local date, not on the date the booking is filed under.
func TestPublicStatusCarriesAnEndOnTheNextLocalDay(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := overnightBooking(complexID)
	f.linkResolver.booking = booking
	f.complexes.complex = &data.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.courts.court = &data.Court{ID: booking.CourtID, Name: "Cancha 1", Sport: "padel", CourtType: "indoor"}

	w := httptest.NewRecorder()
	f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token=overnight", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body, ok := decode(t, w)["booking"].(map[string]any)
	if !ok {
		t.Fatalf("the response must carry a booking object; got %s", w.Body.String())
	}

	// The two fields that stay: the calendar day the booking is filed under and
	// the time of day it starts. Neither of them can answer the question below.
	wantDate := booking.StartsAt.Format("2006-01-02")
	if body["date"] != wantDate {
		t.Errorf("date: want %q; got %v", wantDate, body["date"])
	}
	if body["start_time"] != "23:00" {
		t.Errorf("start_time: want %q; got %v", "23:00", body["start_time"])
	}

	// There is no end_time key any more, and its absence is part of the
	// contract: a client that kept reading it would silently render nothing
	// rather than fail, which is how a dropped field ships unnoticed.
	if _, present := body["end_time"]; present {
		t.Errorf("end_time must not be on the payload: it is the lossy clock reading the schema dropped; got %v",
			body["end_time"])
	}

	endsAt, err := time.Parse(time.RFC3339, str(t, body, "ends_at"))
	if err != nil {
		t.Fatalf("ends_at must be an RFC3339 instant; got %v (%v)", body["ends_at"], err)
	}
	startsAt, err := time.Parse(time.RFC3339, str(t, body, "starts_at"))
	if err != nil {
		t.Fatalf("starts_at must be an RFC3339 instant; got %v (%v)", body["starts_at"], err)
	}

	if !startsAt.Equal(booking.StartsAt) || !endsAt.Equal(booking.EndsAt) {
		t.Errorf("the payload's instants must be the booking's span; got %s/%s want %s/%s",
			startsAt, endsAt, booking.StartsAt, booking.EndsAt)
	}
	if got := endsAt.In(timezone.Argentina).Format("2006-01-02"); got != wantDate1(wantDate) {
		t.Errorf("ends_at falls on %s; a 23:00 booking of two hours ends on %s, the day after the one it is "+
			"filed under. A time of day cannot carry that, which is the whole reason this field exists.",
			got, wantDate1(wantDate))
	}
	// The offset is the venue's, not the process's: a client reading "01:00"
	// out of this string must be reading the clock at the court.
	if _, offset := endsAt.In(timezone.Argentina).Zone(); offset != argentinaOffsetSeconds(booking.EndsAt) {
		t.Errorf("ends_at must be readable on the venue's clock; offset = %d", offset)
	}
}

// The confirmation the client actually receives is the other half. It is one
// line of copy in an email and a WhatsApp template, and it used to be built as
// `StartTime + " - " + EndTime` — which for this booking said "23:00 - 01:00".
func TestTheConfirmationCopyMarksAnEndOnTheNextDay(t *testing.T) {
	f, complexID, courtID := staffFixture(t)

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String()},
		staffBookBodyWithPrice(courtID, "23:00", 120, 400_000)))

	if w.Code != http.StatusCreated {
		t.Fatalf("23:00 for two hours is an ordinary booking; want 201, got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.notify.confirmed) != 1 {
		t.Fatalf("the client must be told about the booking; got %d notifications", len(f.notify.confirmed))
	}

	hours := f.notify.confirmed[0].StartTime
	if !strings.Contains(hours, "23:00") || !strings.Contains(hours, "01:00") {
		t.Fatalf("the confirmation must name both ends of the booking; got %q", hours)
	}
	if !strings.Contains(hours, "día sig.") {
		t.Errorf("the confirmation says %q, which reads as ending two hours before it started. "+
			"An end on the following day has to say so.", hours)
	}
}

// The confirmation for a booking inside one day must NOT carry the marker.
// Without this control the test above passes for a helper that appends the
// words to every booking, which would be worse than the bug.
func TestAnOrdinaryConfirmationCarriesNoNextDayMarker(t *testing.T) {
	f, complexID, courtID := staffFixture(t)

	w := staffCreate(t, f, complexID, courtID, onGrid)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.notify.confirmed) != 1 {
		t.Fatalf("the client must be told about the booking; got %d notifications", len(f.notify.confirmed))
	}

	hours := f.notify.confirmed[0].StartTime
	if hours != "18:00 a 19:30" {
		t.Errorf("an 18:00 booking of ninety minutes reads %q; want %q", hours, "18:00 a 19:30")
	}
}

// Confirming a payment is the second write that puts a booking on a court, and
// it is the one path that reaches slotIsBlocked holding a row read out of the
// database rather than values parsed from the request.
//
// That row's Date is a `date` column, which pgx decodes as midnight UTC, while
// its StartsAt and EndsAt are real instants off the span. The block side was
// built from Date and therefore landed three hours early: a block filed
// 00:00-01:00 became 21:00-22:00 the previous evening, and a booking running
// 23:00 to 01:00 was answered "not blocked". Nothing downstream catches it —
// InsertAndConfirmBooking has no blocked-slot check at all — so the owner's
// maintenance block and a confirmed booking could hold the same hours.
func TestConfirmingAPaymentRefusesANextDayBlock(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	booking := overnightBooking(complexID)
	booking.ComplexID = complexID
	booking.CourtID = courtID
	booking.Status = "pending"
	booking.CollectionStatus = data.CollectionStatusUnpaid
	f.store.booking = booking

	// The block is filed on the day after the booking's own date, at the hours
	// the booking runs into.
	f.courts.block(courtID, booking.StartsAt.AddDate(0, 0, 1), "00:00", "01:00")

	w := httptest.NewRecorder()
	f.handler.ConfirmPayment(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String(), "bookingID": booking.ID.String()},
		`{"amount":400000,"method":"cash"}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("a booking running 23:00 to 01:00 covers a 00:00-01:00 block on the following day and must "+
			"not be confirmed onto it; want 409, got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), blockedSlotMessage) {
		t.Errorf("the refusal must say the court is closed, not that the hours were sold; got %s", w.Body.String())
	}
}

// The control for the day above: the same booking, the same block hours, filed
// on the booking's own date instead of the next one. Those are hours the
// booking does not touch — it starts at 23:00 — so it must be confirmed.
// Without this, a check that read every nearby day would pass the test above.
func TestConfirmingAPaymentIgnoresABlockTheBookingDoesNotReach(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	booking := overnightBooking(complexID)
	booking.ComplexID = complexID
	booking.CourtID = courtID
	booking.Status = "pending"
	booking.CollectionStatus = data.CollectionStatusUnpaid
	f.store.booking = booking

	f.courts.block(courtID, booking.StartsAt, "00:00", "01:00")

	w := httptest.NewRecorder()
	f.handler.ConfirmPayment(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String(), "bookingID": booking.ID.String()},
		`{"amount":400000,"method":"cash"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("a 00:00-01:00 block on the booking's own date is fourteen hours before it starts; "+
			"want 200, got %d (%s)", w.Code, w.Body.String())
	}
}

// slotIsBlocked, asked directly with the shapes the confirmation path holds:
// a Date decoded as midnight UTC and instants off the span. It is the same
// rule the handler test above exercises, one layer down, so a failure names
// the comparison rather than the response code.
func TestSlotIsBlockedPutsBothSidesOnTheVenuesClock(t *testing.T) {
	f, _, courtID := staffFixture(t)
	day, _ := bookableDate()
	local := timezone.Day(day)
	utcDate := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)

	f.courts.block(courtID, local.AddDate(0, 0, 1), "00:00", "01:00")

	startAt := slots.At(local, "23:00")
	blocked, err := f.handler.slotIsBlocked(context.Background(), courtID, utcDate, startAt, startAt.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("slotIsBlocked: %v", err)
	}
	if !blocked {
		t.Fatal("the block's instants were built from a date anchored in UTC while the candidate's came off " +
			"the span in Argentina: three hours apart, so a block the booking covers reads as untouched")
	}
}

// wantDate1 is the calendar day after a "YYYY-MM-DD" string.
func wantDate1(date string) string {
	d, err := time.ParseInLocation("2006-01-02", date, timezone.Argentina)
	if err != nil {
		return date
	}
	return d.AddDate(0, 0, 1).Format("2006-01-02")
}

// argentinaOffsetSeconds is the venue clock's offset at that instant, read from
// the zone rather than written out: Argentina has had no DST since 2009, but a
// literal -10800 in a test is a claim about the future as well as the past.
func argentinaOffsetSeconds(at time.Time) int {
	_, offset := at.In(timezone.Argentina).Zone()
	return offset
}

// str reads a string field out of a decoded payload, failing loudly rather
// than yielding "" — an absent key and an empty one are different bugs.
func str(t *testing.T, body map[string]any, key string) string {
	t.Helper()

	v, ok := body[key].(string)
	if !ok {
		t.Fatalf("%s must be a string on the payload; got %v", key, body[key])
	}
	return v
}
