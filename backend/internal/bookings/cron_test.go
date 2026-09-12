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

// TestTheReminderSweepOnlyRemindsWhatItCanMarkSent is the property the whole
// job hangs on: the flag is what keeps the next five-minute tick from
// reminding the same client again, so a candidate whose flag could not be
// written must not be messaged at all.
func TestTheReminderSweepOnlyRemindsWhatItCanMarkSent(t *testing.T) {
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
			name:        "not reminded at all when the flag cannot be written",
			markSentErr: errors.New("database is down"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			candidate := cronCandidate("confirmed")
			f.store.dueForReminder = []*bookingstore.CronBooking{candidate}
			f.store.markSentErr = tc.markSentErr
			f.linkTokens.mintErr = tc.mintErr

			f.service.Reminder2h(t.Context())

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
	f.store.completed = 5

	f.service.CompletePastBookings(t.Context())
	if f.store.completeCalls != 1 {
		t.Errorf("want the completion sweep to reach the store once; got %d", f.store.completeCalls)
	}

	f.service.CleanLinkTokens(t.Context())
	if len(f.linkTokens.retention) != 1 || f.linkTokens.retention[0] != LinkTokenRetention {
		t.Errorf("want the sweep to pass the package's own retention window; got %v", f.linkTokens.retention)
	}
}
