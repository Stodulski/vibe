package notifications

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
)

// The dates a series lost in one run reach the owner as one email, each date
// with why, at the owner's own address.
func TestSeriesDatesLostEmailsTheOwnerEveryDateWithItsReason(t *testing.T) {
	s, q, mailer, users := newTestService(t)
	s.RegisterWorkers()
	users.user = &authstore.User{Email: "owner@example.com"}

	ownerID, seriesID := uuid.New().String(), uuid.New().String()
	monday := time.Date(2026, time.November, 23, 0, 0, 0, 0, time.UTC)
	s.SeriesDatesLost(SeriesDatesLost{
		OwnerID: ownerID, ComplexName: "Vibe", CourtName: "Cancha 1", ClientName: "Ana Pérez",
		Weekday: time.Monday, Hours: "18:00 a 19:30", SeriesID: seriesID,
		Dates: []LostDate{{Date: monday, Reason: "overlap"}, {Date: monday.AddDate(0, 0, 7), Reason: "closed_day"}},
	})

	if !slices.Equal(q.types(), []string{TaskEmailOwnerSeriesDatesLost}) {
		t.Fatalf("want one owner notice; got %v", q.types())
	}
	if key, _ := q.keyOf(TaskEmailOwnerSeriesDatesLost); key == "" {
		t.Error("the notice must carry a deduplication key")
	}
	payload, err := json.Marshal(q.tasks[0].payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.handlers[TaskEmailOwnerSeriesDatesLost](t.Context(), payload); err != nil {
		t.Fatalf("delivering: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].to != "owner@example.com" {
		t.Fatalf("the email must go to the owner; got %v", mailer.sent)
	}
	lines, _ := mailer.sent[0].args[len(mailer.sent[0].args)-1].([]string)
	want := []string{"lunes 23/11: el horario ya estaba ocupado", "lunes 30/11: el complejo estaba cerrado ese día"}
	if !slices.Equal(lines, want) {
		t.Errorf("lines = %q, want %q", lines, want)
	}
}

func TestSeriesDatesLostNeedsAnOwnerAndADate(t *testing.T) {
	s, q, _, _ := newTestService(t)

	s.SeriesDatesLost(SeriesDatesLost{SeriesID: uuid.New().String(), Dates: []LostDate{{Date: time.Now(), Reason: "overlap"}}})
	s.SeriesDatesLost(SeriesDatesLost{OwnerID: uuid.New().String(), SeriesID: uuid.New().String()})

	if len(q.tasks) != 0 {
		t.Errorf("nothing to say or nobody to say it to; got %v", q.types())
	}
}
