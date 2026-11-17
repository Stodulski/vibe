package notifications

import (
	"encoding/json"
	"testing"
)

// One place-waitlist entry yields one task, keyed on the entry, so a redelivery
// or a second claim of the same row cannot email the same person twice, while a
// second entry of the same address (another place) gets its own notice.
func TestWaitlistPlaceFreedIsKeyedOnTheEntry(t *testing.T) {
	s, q, _, _ := newTestService(t)

	e := WaitlistPlaceFreedEmail{EntryID: "entry-1", Email: "ana@example.com", ComplexName: "Vibe Padel",
		PlaceLabel: "Cancha 1 el miércoles 18/11 a las 19:00", BookURL: "https://vibe.test/c/vibe-padel"}
	s.WaitlistPlaceFreed(e)
	other := e
	other.EntryID = "entry-2"
	s.WaitlistPlaceFreed(other)

	if len(q.tasks) != 2 {
		t.Fatalf("want one task per entry; got %d", len(q.tasks))
	}
	first, second := q.tasks[0], q.tasks[1]
	if first.taskType != TaskEmailWaitlistPlaceFreed {
		t.Errorf("task type = %q; want %q", first.taskType, TaskEmailWaitlistPlaceFreed)
	}
	if first.dedupKey == "" || first.dedupKey == second.dedupKey {
		t.Errorf("each entry needs its own non-empty dedup key; got %q and %q", first.dedupKey, second.dedupKey)
	}
	if first.payload.(WaitlistPlaceFreedEmail) != e {
		t.Errorf("payload = %+v; want %+v", first.payload, e)
	}
}

// The worker delivers what the payload says, to the entry's address.
func TestWaitlistPlaceFreedWorkerSendsTheEmail(t *testing.T) {
	s, q, mailer, _ := newTestService(t)
	s.RegisterWorkers()

	raw, _ := json.Marshal(WaitlistPlaceFreedEmail{EntryID: "entry-1", Email: "ana@example.com",
		ComplexName: "Vibe Padel", PlaceLabel: "Cancha abierta, miércoles 18/11 20:00", BookURL: "https://vibe.test/e"})
	h, ok := q.handlers[TaskEmailWaitlistPlaceFreed]
	if !ok {
		t.Fatal("no worker registered for the place waitlist notice")
	}
	if err := h(t.Context(), raw); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].kind != "waitlist_place_freed" || mailer.sent[0].to != "ana@example.com" {
		t.Fatalf("want one place notice to ana@example.com; got %+v", mailer.sent)
	}
	if got := mailer.sent[0].args; got[1] != "Cancha abierta, miércoles 18/11 20:00" || got[2] != "https://vibe.test/e" {
		t.Errorf("args = %v", got)
	}
}
