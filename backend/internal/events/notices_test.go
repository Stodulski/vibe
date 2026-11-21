package events

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/events/notice"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// S9: a paid participant giving up their spot is told to the owner, once, naming
// who and which event. It is the participant's action, so the owner hears of it.
func TestAPaidParticipantsCancellationIsToldToTheOwner(t *testing.T) {
	f := newFixture(t)
	f.payments.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 820_000}
	token := signedUpAndPaid(t, f, 48*time.Hour, 72*time.Hour)
	reg := f.store.byToken[token]

	if _, err := f.svc.CancelRegistration(t.Context(), visitor(), token); err != nil {
		t.Fatal(err)
	}
	if len(f.notify.ownerCancelled) != 1 {
		t.Fatalf("owner notices = %+v, want one", f.notify.ownerCancelled)
	}
	got := f.notify.ownerCancelled[0]
	if got.OwnerID != f.complex.OwnerID.String() || got.RegistrationID != reg.ID.String() ||
		got.EventName != f.store.event.Name || !strings.Contains(got.ParticipantName, "Ana") {
		t.Errorf("owner notice = %+v", got)
	}
}

// S9: a spot that was never paid was never announced to the owner, so its
// release is not either; and the owner cancelling the event is their own action.
func TestTheOwnerIsNotToldOfUnpaidSpotsOrTheirOwnCancellation(t *testing.T) {
	f := newFixture(t)
	held, err := f.svc.SignUp(t.Context(), visitor(), "vibe", f.store.event.ID, signUpInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CancelRegistration(t.Context(), visitor(), held.Token); err != nil {
		t.Fatal(err)
	}

	token := signedUpAndPaid(t, f, 48*time.Hour, 72*time.Hour)
	f.store.paid = []uuid.UUID{f.store.byToken[token].ID}
	if _, err := f.svc.Cancel(t.Context(), f.complex, f.ownerActor(), f.store.event.ID, nil); err != nil {
		t.Fatal(err)
	}
	if len(f.notify.ownerCancelled) != 0 {
		t.Errorf("owner notices = %+v, want none", f.notify.ownerCancelled)
	}
}

// S10: a hold nobody paid in time is told to the person who left an email, as an
// expired storefront booking is: their spot was released and nothing is owed.
func TestAnExpiredHoldIsToldToThePersonWhoLeftAnEmail(t *testing.T) {
	f := newFixture(t)
	email := "ana@example.com"
	withEmail := &eventstore.Registration{ID: uuid.New(), EventID: f.store.event.ID, ComplexID: f.complex.ID,
		FirstName: "Ana", Email: &email, Status: eventstore.RegistrationHeld, Price: 800_000}
	noEmail := &eventstore.Registration{ID: uuid.New(), EventID: f.store.event.ID, ComplexID: f.complex.ID,
		FirstName: "Beto", Status: eventstore.RegistrationHeld, Price: 800_000}
	f.store.remember(withEmail, "a")
	f.store.remember(noEmail, "b")
	f.store.expired = []eventstore.ExpiredHold{
		{ID: withEmail.ID, ComplexID: f.complex.ID, EventID: f.store.event.ID},
		{ID: noEmail.ID, ComplexID: f.complex.ID, EventID: f.store.event.ID},
	}

	f.svc.ExpireHolds(t.Context())
	if len(f.notify.cancelled) != 1 {
		t.Fatalf("notices = %+v, want only the person with an email", f.notify.cancelled)
	}
	got := f.notify.cancelled[0]
	if got.Email != email || got.RegistrationID != withEmail.ID.String() || got.ByOwner ||
		got.RefundLine != notice.LineExpired || got.RefundAmount != "" {
		t.Errorf("notice = %+v", got)
	}
	if len(f.notify.ownerCancelled) != 0 {
		t.Error("an unpaid hold was never announced to the owner")
	}
}

// S12: the day-before reminder is queued once per registration, then marked, and
// carries when the event starts as the person reads it.
func TestTheReminderSweepQueuesEachDueReminderOnce(t *testing.T) {
	f := newFixture(t)
	start := time.Now().Add(20 * time.Hour).In(timezoneAR())
	token := signedUpAndPaid(t, f, 72*time.Hour, 20*time.Hour)
	reg := f.store.byToken[token]
	email := "ana@example.com"
	reg.Email = &email
	f.store.due = []eventstore.DueReminder{{RegistrationID: reg.ID, StartsAt: start, HasTime: true}}

	f.svc.RemindUpcoming(t.Context())
	f.svc.RemindUpcoming(t.Context())
	if len(f.notify.reminders) != 1 || len(f.store.reminded) != 1 || f.store.reminded[0] != reg.ID {
		t.Fatalf("reminders = %+v marked = %v, want one of each", f.notify.reminders, f.store.reminded)
	}
	got := f.notify.reminders[0]
	if got.Email != email || got.RegistrationID != reg.ID.String() || !strings.Contains(got.When, start.Format("15:04")) ||
		!strings.Contains(got.When, start.Format("02/01")) {
		t.Errorf("reminder = %+v", got)
	}
}

// S12: a reminder the queue refused is not marked, so the next tick tries again.
func TestARefusedReminderIsNotMarked(t *testing.T) {
	f := newFixture(t)
	token := signedUpAndPaid(t, f, 72*time.Hour, 20*time.Hour)
	reg := f.store.byToken[token]
	email := "ana@example.com"
	reg.Email = &email
	f.store.due = []eventstore.DueReminder{{RegistrationID: reg.ID, StartsAt: time.Now().Add(20 * time.Hour), HasTime: true}}
	f.notify.refuseReminders = true

	f.svc.RemindUpcoming(t.Context())
	if len(f.store.reminded) != 0 {
		t.Fatalf("a refused reminder must stay unmarked; marked = %v", f.store.reminded)
	}
	f.notify.refuseReminders = false
	f.svc.RemindUpcoming(t.Context())
	if len(f.notify.reminders) != 1 || len(f.store.reminded) != 1 {
		t.Errorf("the next tick must send it; reminders = %d marked = %v", len(f.notify.reminders), f.store.reminded)
	}
}

// S12 + A4: a tournament pair's partner who left an email is reminded too, under
// their own name.
func TestATournamentReminderReachesThePartnerToo(t *testing.T) {
	f := newFixture(t)
	tid := uuid.New()
	f.store.event.TournamentID = &tid
	token := signedUpAndPaid(t, f, 72*time.Hour, 20*time.Hour)
	reg := f.store.byToken[token]
	email, partner := "ana@example.com", "beto@example.com"
	reg.Email = &email
	f.store.due = []eventstore.DueReminder{{RegistrationID: reg.ID, StartsAt: time.Now().Add(20 * time.Hour),
		PartnerEmail: &partner, PartnerFirstName: "Beto"}}

	f.svc.RemindUpcoming(t.Context())
	if len(f.notify.reminders) != 2 || f.notify.reminders[1].Email != partner || f.notify.reminders[1].FirstName != "Beto" {
		t.Fatalf("reminders = %+v", f.notify.reminders)
	}
	if f.notify.reminders[0].RegistrationID == f.notify.reminders[1].RegistrationID && f.notify.reminders[0].Email == f.notify.reminders[1].Email {
		t.Error("each player needs their own reminder")
	}
}
