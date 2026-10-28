package bookings

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// cronCandidate builds the enriched booking either sweep would be handed.
func cronCandidate(status bookingstore.BookingStatus) *bookingstore.CronBooking {
	b := &bookingstore.CronBooking{
		ClientEmail: "ana@example.com",
		ClientPhone: "+5491100000000",
		ComplexName: "Vibe Palermo",
		CourtName:   "Cancha 1",
		ComplexSlug: "vibe-palermo",
	}
	b.ID = uuid.New()
	b.ComplexID = uuid.New()
	b.Date = time.Now()
	b.StartTime = "18:00"
	b.StartsAt = slots.At(timezone.Day(b.Date), "18:00")
	b.EndsAt = b.StartsAt.Add(60 * time.Minute)
	b.DurationMinutes = 60
	b.Status = status
	b.CollectionStatus = bookingstore.CollectionStatusUnpaid
	return b
}

// TestTheReminderSweepQueuesBeforeItFlags pins the contract the sweep runs on.
// A candidate is queued before it is flagged, so a flag that cannot be written
// does not cost the client the reminder: the candidate is queued again on the
// next tick, and the job's dedup key turns that second enqueue into a no-op.
// Once a flag write succeeds, it stops every later tick from selecting the
// booking.
func TestTheReminderSweepQueuesBeforeItFlags(t *testing.T) {
	for _, tc := range []struct {
		name        string
		markSentErr error
		mintErr     error
		wantSent    int
		wantLink    bool
	}{
		{name: "reminded with its own cancel link", wantSent: 1, wantLink: true},
		{
			name:     "reminded without one when the mint fails",
			mintErr:  errors.New("database is down"),
			wantSent: 1,
		},
		{
			name:        "reminded, and flagged by a later tick, when the flag cannot be written",
			markSentErr: errors.New("database is down"),
			wantSent:    1,
			wantLink:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			candidate := cronCandidate("confirmed")
			f.store.dueForReminder = []*bookingstore.CronBooking{candidate}
			f.store.markSentErr = tc.markSentErr
			f.linkTokens.mintErr = tc.mintErr

			f.service.Reminder2h(t.Context())

			if tc.markSentErr == nil && len(f.store.markedSent) != 1 {
				t.Errorf("want the reminded booking flagged once; got %d flag write(s)", len(f.store.markedSent))
			}
			if got := len(f.notify.reminders); got != tc.wantSent {
				t.Fatalf("want %d reminder(s) sent; got %d", tc.wantSent, got)
			}
			if tc.wantSent == 0 {
				return
			}
			sent := f.notify.reminders[0]
			if hasLink := sent.CancelURL != ""; hasLink != tc.wantLink {
				t.Errorf("want a cancel link on the reminder: %v; got URL %q", tc.wantLink, sent.CancelURL)
			}
			if sent.Email != candidate.ClientEmail {
				t.Errorf("want the reminder addressed to the client; got %q", sent.Email)
			}
		})
	}
}

// crashAtEnqueue stands in for a worker that dies once the reminder reaches the
// queue. The panic is the crash: nothing after it runs.
type crashAtEnqueue struct{ *stubNotifier }

func (crashAtEnqueue) ReminderDue(notifications.Reminder) error {
	panic("worker died before the reminder was queued")
}

// TestAFailedReminderEnqueueLeavesTheBookingUnflagged is the rule the flag
// depends on: the booking is flagged only when its reminder is queued. A refused
// enqueue leaves it selectable, so the next tick queues it and flags it.
func TestAFailedReminderEnqueueLeavesTheBookingUnflagged(t *testing.T) {
	f := newFixture(t)
	f.store.dueForReminder = []*bookingstore.CronBooking{cronCandidate("confirmed")}
	f.notify.reminderErr = errors.New("database is down")

	f.service.Reminder2h(t.Context())

	if got := f.store.markedSent; len(got) != 0 {
		t.Fatalf("the reminder was refused, so the booking must stay unflagged; flagged %v", got)
	}
	if got := len(f.notify.reminders); got != 0 {
		t.Fatalf("a refused reminder is not sent; got %d", got)
	}

	f.notify.reminderErr = nil
	f.service.Reminder2h(t.Context())

	if got := len(f.store.markedSent); got != 1 {
		t.Errorf("the next tick must queue and flag the booking; got %d flag write(s)", got)
	}
	if got := len(f.notify.reminders); got != 1 {
		t.Errorf("the next tick must queue the reminder; got %d", got)
	}
}

// TestAReminderIsFlaggedOnlyAfterItIsQueued is the reason the sweep queues before
// it flags. Flagged first, a crash between the two leaves a flag that says the
// reminder went out while no job exists, and every later tick skips the booking.
func TestAReminderIsFlaggedOnlyAfterItIsQueued(t *testing.T) {
	f := newFixture(t)
	f.store.dueForReminder = []*bookingstore.CronBooking{cronCandidate("confirmed")}
	f.service.notify = crashAtEnqueue{f.notify}

	func() {
		defer func() { recover() }()
		f.service.Reminder2h(t.Context())
	}()

	if got := f.store.markedSent; len(got) != 0 {
		t.Errorf("the booking was flagged before its reminder was queued, so a crash between the two loses it: flagged %v", got)
	}
}

// TestTheExpirySweepClosesTheCheckoutBeforeItFreesTheSlot pins the order the
// job exists for: a client whose link is still live when the slot goes back on
// sale can pay for hours somebody else now owns.
func TestTheExpirySweepClosesTheCheckoutBeforeItFreesTheSlot(t *testing.T) {
	f := newFixture(t)
	candidate := cronCandidate("pending")
	preferenceID := "pref-1"
	f.store.expiredPending = []*bookingstore.CronBooking{candidate}
	f.payments.payment = &paymentstore.Payment{
		BookingID:      candidate.ID,
		ComplexID:      candidate.ComplexID,
		MPPreferenceID: &preferenceID,
	}

	f.service.ReleaseExpiredPayments(t.Context())

	if len(f.checkout.expired) != 1 || f.checkout.expired[0] != preferenceID {
		t.Fatalf("want the checkout link expired once; got %v", f.checkout.expired)
	}
	if len(f.store.updated) != 1 || f.store.updated[0].Status != "cancelled" {
		t.Fatalf("want the booking cancelled; got %v", f.store.updated)
	}
	if len(f.notify.cancelled) != 1 {
		t.Fatalf("want one cancellation message; got %d", len(f.notify.cancelled))
	}
	if got := f.notify.cancelled[0].RefundLine; got != notifications.ExpiredUnpaidRefundLine {
		t.Errorf("want the expiry's own refund line, which says why the booking went away; got %q", got)
	}
	if len(f.realtime.published) != 1 {
		t.Errorf("want the owner's dashboards told; got %d broadcasts", len(f.realtime.published))
	}
}

// An expired booking was never paid, so the owner was never told it existed and
// the sweep does not email them. It still marks the cancellation as the sweep's,
// so no owner notice can be derived from it by accident.
func TestTheExpirySweepDoesNotTellTheOwner(t *testing.T) {
	f := newFixture(t)
	candidate := cronCandidate("pending")
	f.store.expiredPending = []*bookingstore.CronBooking{candidate}

	f.service.ReleaseExpiredPayments(t.Context())

	if len(f.notify.cancelled) != 1 {
		t.Fatalf("want one cancellation message to the client; got %d", len(f.notify.cancelled))
	}
	if got := f.notify.cancelled[0].Source; got != notifications.SourceExpiredUnpaid {
		t.Errorf("want the expiry marked as the sweep's own; got source %q", got)
	}
	if f.notify.cancelled[0].OwnerID != "" {
		t.Errorf("the expiry must not carry an owner to email; got %q", f.notify.cancelled[0].OwnerID)
	}
}

// TestTheExpirySweepFinishesTheBatchAfterOneFailure documents that one booking
// the database refuses does not cost the rest of the batch their slots.
func TestTheExpirySweepFinishesTheBatchAfterOneFailure(t *testing.T) {
	f := newFixture(t)
	failing, ok := cronCandidate("pending"), cronCandidate("pending")
	f.store.expiredPending = []*bookingstore.CronBooking{failing, ok}
	f.store.onUpdate = func() {}

	attempts := 0
	f.store.updateErrFor = func(b *bookingstore.Booking) error {
		attempts++
		if b.ID == failing.ID {
			return errors.New("simulated db failure")
		}
		return nil
	}

	f.service.ReleaseExpiredPayments(t.Context())

	if attempts != 2 {
		t.Fatalf("want both bookings attempted; got %d", attempts)
	}
	if len(f.store.updated) != 1 || f.store.updated[0].ID != ok.ID {
		t.Errorf("want the second booking cancelled despite the first one failing; got %v", f.store.updated)
	}
}

// TestTheHousekeepingSweepsReportWhatTheyDid covers the two jobs whose whole
// body is one store call: the sweep that completes played bookings and the one
// that drops the link tokens of terminal ones.
func TestTheHousekeepingSweepsReportWhatTheyDid(t *testing.T) {
	f := newFixture(t)
	first, second := uuid.New(), uuid.New()
	f.store.completed = []uuid.UUID{first, first, second}

	f.service.CompletePastBookings(t.Context())
	if f.store.completeCalls != 1 {
		t.Errorf("want the completion sweep to reach the store once; got %d", f.store.completeCalls)
	}

	f.service.CleanLinkTokens(t.Context())
	if len(f.linkTokens.retention) != 1 || f.linkTokens.retention[0] != LinkTokenRetention {
		t.Errorf("want the sweep to pass the package's own retention window; got %v", f.linkTokens.retention)
	}
}

// Completing a played booking changes its complex's owner calendar, and the
// owner's open dashboard only shows the change after a broadcast. A sweep that
// completes several bookings in one complex tells that complex once, not once
// per booking, and every complex it touched is told.
func TestCompletingPastBookingsTellsEachAffectedComplexOnce(t *testing.T) {
	f := newFixture(t)
	first, second := uuid.New(), uuid.New()
	f.store.completed = []uuid.UUID{first, first, second}

	f.service.CompletePastBookings(t.Context())

	if len(f.realtime.published) != 2 || f.realtime.published[0] != first || f.realtime.published[1] != second {
		t.Errorf("want each affected complex told once, in sweep order; got %v", f.realtime.published)
	}
}
