package courts

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// At 00:10 a venue trading 20:00-02:00 is still in last night's window: its
// 00:30 and 01:00 starts are on today's calendar but belong to yesterday's
// grid, and POST /book accepts them (it allows the grid day one short of
// today). Availability refused yesterday's date outright, so the storefront
// could never list the rest of a grid day that was still open. It now answers
// for yesterday, and the instant check leaves out whatever has begun.
func TestAvailabilityListsYesterdaysGridAfterMidnight(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	today := timezone.Today()
	yesterday := today.AddDate(0, 0, -1).Format("2006-01-02")

	complexes := &stubComplexes{complex: activeComplex(complexID), schedules: everyDaySchedule("20:00", "02:00")}
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		prices: everyDayBand(courtID, "20:00", "02:00", 500_000),
	}
	h, _ := newTestHandler(store, &stubBookings{}, complexes)
	tenPast := time.Date(today.Year(), today.Month(), today.Day(), 0, 10, 0, 0, timezone.Argentina)
	h.svc.now = func() time.Time { return tenPast }

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?date="+yesterday+"&duration=60", nil)
	w := httptest.NewRecorder()
	h.Availability(w, withSlug(r, "vibe"))
	if w.Code != http.StatusOK {
		t.Fatalf("yesterday's grid must still be readable after midnight; got %d (%s)", w.Code, w.Body.String())
	}

	grid, _ := decode(t, w)["availability"].(map[string]any)
	courtsOut, _ := grid["courts"].([]any)
	if len(courtsOut) != 1 {
		t.Fatalf("want one court; got %d", len(courtsOut))
	}
	first, _ := courtsOut[0].(map[string]any)
	slotsOut, _ := first["slots"].([]any)

	checked := map[string]bool{}
	for _, raw := range slotsOut {
		s, _ := raw.(map[string]any)
		start, _ := s["start_time"].(string)
		switch start {
		case "21:00", "00:00":
			checked[start] = true
			if s["available"] != false {
				t.Errorf("%s has begun at 00:10; it must not be for sale; got %v", start, s)
			}
		case "00:30", "01:00":
			checked[start] = true
			if s["available"] != true {
				t.Errorf("%s of last night's window has not begun at 00:10; it must be for sale; got %v", start, s)
			}
		}
	}
	if len(checked) != 4 {
		t.Fatalf("the grid did not publish the hours around midnight; checked %v", checked)
	}
}
