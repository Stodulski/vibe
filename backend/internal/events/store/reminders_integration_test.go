//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/data/datatest"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

func dueIDs(t *testing.T, s interface {
	DueReminders(context.Context, time.Time) ([]eventstore.DueReminder, error)
}, now time.Time) map[uuid.UUID]eventstore.DueReminder {
	t.Helper()
	due, err := s.DueReminders(data.ContextWithTenantBypass(context.Background()), now)
	if err != nil {
		t.Fatal(err)
	}
	out := map[uuid.UUID]eventstore.DueReminder{}
	for _, d := range due {
		out[d.RegistrationID] = d
	}
	return out
}

// S12: a paid spot is due its reminder inside the day before its event, read on
// Argentina's clock, and never again once marked; a spot whose event was
// cancelled, or still unpaid, is not reminded.
func TestDueRemindersSelectsPaidSpotsInsideTheDayBefore(t *testing.T) {
	f := datatest.Isolated(t)
	ev := createEvent(t, f, "19:00", 120)
	start := time.Date(ev.Date.Year(), ev.Date.Month(), ev.Date.Day(), 19, 0, 0, 0, timezone.Argentina)

	paid, err := signUp(f, ev, "+5491100000101")
	if err != nil {
		t.Fatal(err)
	}
	if err := pay(f, paid, "mp-rem-1"); err != nil {
		t.Fatal(err)
	}
	held, err := signUp(f, ev, "+5491100000102")
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := dueIDs(t, f.Stores.Events, start.Add(-30*time.Hour))[paid.ID]; ok {
		t.Error("30 hours out is not the day before")
	}
	due := dueIDs(t, f.Stores.Events, start.Add(-20*time.Hour))
	got, ok := due[paid.ID]
	if !ok || !got.StartsAt.Equal(start) || !got.HasTime {
		t.Fatalf("20 hours out the paid spot is due, starting at %v; got %+v (due=%v)", start, got, ok)
	}
	if _, ok := due[held.ID]; ok {
		t.Error("an unpaid hold is not reminded")
	}
	if _, ok := dueIDs(t, f.Stores.Events, start.Add(time.Minute))[paid.ID]; ok {
		t.Error("a started event is not reminded")
	}

	if err := f.Stores.Events.MarkReminded(data.ContextWithTenantBypass(context.Background()), paid.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := dueIDs(t, f.Stores.Events, start.Add(-20*time.Hour))[paid.ID]; ok {
		t.Error("a marked spot is not reminded twice")
	}
}

// S12: a spot paid inside the day before just got its confirmation, and an event
// the owner cancelled has nothing to remind.
func TestDueRemindersSkipsLatePaymentsAndCancelledEvents(t *testing.T) {
	f := datatest.Isolated(t)
	ev := createEvent(t, f, "19:00", 120)
	start := time.Date(ev.Date.Year(), ev.Date.Month(), ev.Date.Day(), 19, 0, 0, 0, timezone.Argentina)
	reg, err := signUp(f, ev, "+5491100000103")
	if err != nil {
		t.Fatal(err)
	}
	if err := pay(f, reg, "mp-rem-2"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Exec(context.Background(),
		`UPDATE open_play_registrations SET confirmed_at = $2 WHERE id = $1`, reg.ID, start.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, ok := dueIDs(t, f.Stores.Events, start.Add(-time.Hour))[reg.ID]; ok {
		t.Error("a spot paid inside the day before is not reminded")
	}

	if _, err := f.DB.Exec(context.Background(),
		`UPDATE open_play_registrations SET confirmed_at = $2 WHERE id = $1`, reg.ID, start.Add(-72*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.Stores.Events.CancelEvent(f.Scoped(context.Background()), ev.ID, f.UserID, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := dueIDs(t, f.Stores.Events, start.Add(-20*time.Hour))[reg.ID]; ok {
		t.Error("a cancelled event is not reminded")
	}
}
