package bookings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
)

type altOut struct {
	CourtID   string `json:"court_id"`
	CourtName string `json:"court_name"`
	Conflicts int    `json:"conflicts"`
}

func decodeAlternatives(t *testing.T, w *httptest.ResponseRecorder) []altOut {
	t.Helper()
	var problem struct {
		Alternatives []altOut `json:"alternatives"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	return problem.Alternatives
}

// altCourt adds a court of the fixture's complex, a clone of the chosen one
// unless the caller changes it.
func (f *seriesFx) altCourt(name string, change func(*courtstore.Court)) *courtstore.Court {
	c := *f.courts.court
	c.ID, c.Name = uuid.New(), name
	if change != nil {
		change(&c)
	}
	f.courts.all = append(f.courts.all, &c)
	return &c
}

func (f *seriesFx) occupyCourt(courtID uuid.UUID, d time.Time) {
	id := f.courtID
	f.courtID = courtID
	f.occupy(d, "18:30", 60)
	f.courtID = id
}

func TestAConflictedSeriesOffersSimilarFreeCourtsRankedBySimilarity(t *testing.T) {
	f := newSeriesFixture(t)
	grass, clay := "synthetic_grass", "clay"
	f.courts.court.Sport, f.courts.court.CourtType, f.courts.court.Surface = "padel", "outdoor", &grass
	f.courts.all = []*courtstore.Court{f.courts.court}
	first := f.day(3)
	f.occupy(first.AddDate(0, 0, 14), "18:30", 60)
	f.occupy(first.AddDate(0, 0, 21), "18:30", 60)

	other := f.altCourt("Court 2", func(c *courtstore.Court) { c.Surface = &clay })
	same := f.altCourt("Court 3", nil)
	oneClash := f.altCourt("Court 4", nil)
	f.occupyCourt(oneClash.ID, first.AddDate(0, 0, 14))
	bothClash := f.altCourt("Court 5", nil)
	f.occupyCourt(bothClash.ID, first.AddDate(0, 0, 14))
	f.occupyCourt(bothClash.ID, first.AddDate(0, 0, 21))
	f.altCourt("Court 6", func(c *courtstore.Court) { c.IsActive = false })
	f.altCourt("Court 7", func(c *courtstore.Court) { c.Sport = "tennis" })

	w := f.create(t, first, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d (%s)", w.Code, w.Body.String())
	}
	got := decodeAlternatives(t, w)
	want := []altOut{
		{same.ID.String(), "Court 3", 0},
		{other.ID.String(), "Court 2", 0},
		{oneClash.ID.String(), "Court 4", 1},
	}
	if len(got) != len(want) {
		t.Fatalf("alternatives = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("alternatives[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestAConflictedSeriesWithNoBetterCourtOffersNone(t *testing.T) {
	f := newSeriesFixture(t)
	f.courts.all = []*courtstore.Court{f.courts.court}
	first := f.day(3)
	f.occupy(first.AddDate(0, 0, 14), "18:30", 60)
	busy := f.altCourt("Court 2", nil)
	f.occupyCourt(busy.ID, first.AddDate(0, 0, 14))

	w := f.create(t, first, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", w.Code)
	}
	if got := decodeAlternatives(t, w); len(got) != 0 {
		t.Errorf("alternatives = %+v, want none", got)
	}
}
