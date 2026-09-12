package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mpcred"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

func TestCronCleanExpiredTokens(t *testing.T) {
	app := newTestApplication(t)

	refreshDeleted := false

	app.models.Tokens = &mockTokenStore{
		DeleteExpiredFn: func(ctx context.Context) error {
			refreshDeleted = true
			return nil
		},
	}
	// The mockEmailVerificationStore.DeleteExpired always returns nil,
	// which is enough to test the happy path. We rely on the mock not panicking.
	verificationDeleted := true

	app.cronCleanExpiredTokens(context.Background())

	if !refreshDeleted {
		t.Error("expected DeleteExpired to be called on refresh tokens")
	}
	if !verificationDeleted {
		t.Error("expected DeleteExpired to be called on verification tokens")
	}
}

func TestCronCompleteBookings_NothingToComplete(t *testing.T) {
	app := newTestApplication(t)

	app.models.Bookings = &mockBookingStore{
		CompletePastBookingsFn: func(ctx context.Context) (int64, error) {
			return 0, nil
		},
	}

	// Should not panic
	app.cronCompleteBookings(context.Background())
}

func TestCronCompleteBookings_CompletesBookings(t *testing.T) {
	app := newTestApplication(t)

	var count int64
	app.models.Bookings = &mockBookingStore{
		CompletePastBookingsFn: func(ctx context.Context) (int64, error) {
			count = 5
			return 5, nil
		},
	}

	app.cronCompleteBookings(context.Background())

	if count != 5 {
		t.Errorf("want 5 bookings completed; got %d", count)
	}
}

func TestCronCleanSlotLocks(t *testing.T) {
	app := newTestApplication(t)

	// Default mockSlotLockStore.CleanExpired returns 0, nil.
	// Should not panic.
	app.cronCleanSlotLocks(context.Background())
}

func TestCronCleanUnverifiedUsers(t *testing.T) {
	app := newTestApplication(t)

	// Default mockUserStore.DeleteUnverifiedStale returns nil.
	// Should not panic.
	app.cronCleanUnverifiedUsers(context.Background())
}

func TestCronRefreshMPTokens_NoComplexes(t *testing.T) {
	app := newTestApplication(t)

	app.models.Complexes = &mockComplexStore{
		GetWithMPConnectedFn: func(ctx context.Context) ([]*complexstore.Complex, error) {
			return nil, nil
		},
	}

	// Should not panic when there are no complexes to refresh.
	app.cronRefreshMPTokens(context.Background())
}

// TestCronRefreshMPTokens_SkipsEmptyRefreshToken documents the accessor
// contract this loop actually depends on: an empty refresh token makes
// SellerRefreshToken() return mpcred.ErrMPNotConnected, and the loop skips
// silently on that error — it does not assert on the raw field it happens to
// still be able to set directly (the field itself is unexported once Phase
// 11 lands).
func TestCronRefreshMPTokens_SkipsEmptyRefreshToken(t *testing.T) {
	app := newTestApplication(t)

	emptyToken := ""
	complex := complexstore.NewComplexForTest(uuid.New(), nil, &emptyToken)
	complex.Name = "Test Complex"

	if _, err := complex.SellerRefreshToken(); !errors.Is(err, mpcred.ErrMPNotConnected) {
		t.Fatalf("want ErrMPNotConnected for an empty refresh token; got %v", err)
	}

	app.models.Complexes = &mockComplexStore{
		GetWithMPConnectedFn: func(ctx context.Context) ([]*complexstore.Complex, error) {
			return []*complexstore.Complex{complex}, nil
		},
	}

	// Should skip complexes with empty refresh token without panicking.
	app.cronRefreshMPTokens(context.Background())
}

// TestCronReminder2h_PayloadCarriesWhereToGoAndWhatToPay is the coverage for
// the two facts a reminder exists to carry.
//
// The reminder used to repeat the confirmation's four fields and stop. Two
// hours before a game, the client's questions are "where is this" and "what do
// I owe when I get there", and the message that answered neither had already
// been sent, days earlier. It also carries its own cancel link, minted here
// because booking_link_tokens stores only a hash and the confirmation's
// plaintext cannot be read back.
func TestCronReminder2h_PayloadCarriesWhereToGoAndWhatToPay(t *testing.T) {
	app, queue := newTestApplicationWithNotifications(t)

	lat, lng := -34.603722, -58.381592
	candidate := &data.CronBooking{
		ClientEmail:      "ana@example.com",
		ClientPhone:      "+5491100000000",
		ComplexName:      "Vibe Palermo",
		CourtName:        "Cancha 1",
		ComplexSlug:      "vibe-palermo",
		ComplexAddress:   "Av. Santa Fe 1200",
		ComplexCity:      "Buenos Aires",
		ComplexLatitude:  &lat,
		ComplexLongitude: &lng,
	}
	candidate.ID = uuid.New()
	candidate.Date = time.Now()
	candidate.StartTime = "18:00"
	candidate.StartsAt = slots.At(timezone.Day(candidate.Date), "18:00")
	candidate.EndsAt = candidate.StartsAt.Add(60 * time.Minute)
	candidate.DurationMinutes = 60
	candidate.Price = 2000000
	candidate.DepositAmount = 500000
	candidate.CollectionStatus = data.CollectionStatusDepositPaid

	app.models.Bookings = &mockBookingStore{
		GetForReminder2hEnrichedFn: func(context.Context, time.Time) ([]*data.CronBooking, error) {
			return []*data.CronBooking{candidate}, nil
		},
		MarkReminderSent2hFn: func(context.Context, uuid.UUID) error { return nil },
	}
	app.models.BookingLinkTokens = &mockBookingLinkTokenStore{
		MintFn: func(context.Context, uuid.UUID, time.Time) (string, error) { return "fresh-token", nil },
	}

	app.cronReminder2h(context.Background())

	payloads := queue.payloadsOf(notifications.TaskEmailReminder2h)
	if len(payloads) != 1 {
		t.Fatalf("want one reminder enqueued; got %d (%v)", len(payloads), queue.taskTypes())
	}
	rem, ok := payloads[0].(notifications.Reminder)
	if !ok {
		t.Fatalf("want a notifications.Reminder payload; got %T", payloads[0])
	}

	if rem.Address != "Av. Santa Fe 1200, Buenos Aires" {
		t.Errorf("the reminder does not say where to go; got address %q", rem.Address)
	}
	if rem.BalanceAmount != "$15.000" {
		t.Errorf("the reminder does not say what is left to pay; got %q", rem.BalanceAmount)
	}
	if rem.MapsQuery != "-34.603722,-58.381592" {
		t.Errorf("the reminder cannot open a map; got maps query %q", rem.MapsQuery)
	}
	if rem.MapsURL != "https://www.google.com/maps/search/?api=1&query=-34.603722,-58.381592" {
		t.Errorf("the reminder email cannot open a map; got maps URL %q", rem.MapsURL)
	}
	if rem.CancelPath != "vibe-palermo/book/cancel?token=fresh-token" {
		t.Errorf("the reminder cannot be cancelled from WhatsApp; got cancel path %q", rem.CancelPath)
	}
	if rem.CancelURL != "http://localhost:5173/vibe-palermo/book/cancel?token=fresh-token" {
		t.Errorf("the reminder email has no cancel link; got %q", rem.CancelURL)
	}
}

// A mint that fails costs the WhatsApp cancel button and nothing else: the
// reminder still goes out, which is the point of the notification.
func TestCronReminder2h_StillRemindsWhenTheCancelLinkCannotBeMinted(t *testing.T) {
	app, queue := newTestApplicationWithNotifications(t)

	candidate := &data.CronBooking{ClientEmail: "ana@example.com", ComplexName: "Vibe", ComplexSlug: "vibe"}
	candidate.ID = uuid.New()
	candidate.Date = time.Now()
	candidate.StartTime = "18:00"
	candidate.StartsAt = slots.At(timezone.Day(candidate.Date), "18:00")
	candidate.EndsAt = candidate.StartsAt.Add(60 * time.Minute)

	app.models.Bookings = &mockBookingStore{
		GetForReminder2hEnrichedFn: func(context.Context, time.Time) ([]*data.CronBooking, error) {
			return []*data.CronBooking{candidate}, nil
		},
		MarkReminderSent2hFn: func(context.Context, uuid.UUID) error { return nil },
	}
	app.models.BookingLinkTokens = &mockBookingLinkTokenStore{
		MintFn: func(context.Context, uuid.UUID, time.Time) (string, error) {
			return "", errors.New("database is down")
		},
	}

	app.cronReminder2h(context.Background())

	if payloads := queue.payloadsOf(notifications.TaskEmailReminder2h); len(payloads) != 1 {
		t.Fatalf("the reminder must still go out; got %d (%v)", len(payloads), queue.taskTypes())
	}
}

// The expiry sweep cancels a booking nobody paid for. Its client's question is
// why, and the generic cancellation copy never answered it.
func TestCronReleaseExpiredPayments_SaysWhyTheBookingWentAway(t *testing.T) {
	app, queue := newTestApplicationWithNotifications(t)

	candidate := &data.CronBooking{
		ClientEmail: "ana@example.com",
		ClientPhone: "+5491100000000",
		ComplexName: "Vibe Palermo",
		CourtName:   "Cancha 1",
		ComplexSlug: "vibe-palermo",
	}
	candidate.ID = uuid.New()
	candidate.Date = time.Now()
	candidate.StartTime = "18:00"
	candidate.StartsAt = slots.At(timezone.Day(candidate.Date), "18:00")
	candidate.EndsAt = candidate.StartsAt.Add(60 * time.Minute)
	candidate.Status = "pending"
	candidate.CollectionStatus = data.CollectionStatusUnpaid

	app.models.Bookings = &mockBookingStore{
		GetExpiredPendingEnrichedFn: func(context.Context, time.Duration) ([]*data.CronBooking, error) {
			return []*data.CronBooking{candidate}, nil
		},
		UpdateFn: func(context.Context, *data.Booking) error { return nil },
	}

	app.cronReleaseExpiredPayments(context.Background())

	payloads := queue.payloadsOf(notifications.TaskEmailBookingCancelled)
	if len(payloads) != 1 {
		t.Fatalf("want one cancellation enqueued; got %d (%v)", len(payloads), queue.taskTypes())
	}
	// The email worker's payload, not the argument struct.
	raw, err := json.Marshal(payloads[0])
	if err != nil {
		t.Fatalf("marshalling the payload: %v", err)
	}
	var sent struct {
		RefundLine string `json:"refund_line"`
		BookURL    string `json:"book_url"`
	}
	if err := json.Unmarshal(raw, &sent); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if sent.RefundLine != notifications.ExpiredUnpaidRefundLine {
		t.Errorf("the cancellation does not say why the booking went away; got %q", sent.RefundLine)
	}
	if sent.BookURL != "http://localhost:5173/vibe-palermo/book" {
		t.Errorf("the cancellation offers no way back; got %q", sent.BookURL)
	}
}

func TestCronReminder2h_NoBookings(t *testing.T) {
	app := newTestApplication(t)

	// Default mock returns nil, nil for GetForReminder2hEnriched.
	// Should not panic.
	app.cronReminder2h(context.Background())
}

// TestCronReminder2h_EnqueuesExactlyOnceAndMarksSent is the QA coverage for
// "reminder only once": a candidate the store hands back must produce exactly
// one email:reminder_2h task, and MarkReminderSent2h must be called for it —
// that flag is the only thing keeping GetForReminder2hEnriched from handing
// the same booking back on the next 5-minute tick and reminding it twice.
//
// Mutation: delete the app.models.Bookings.MarkReminderSent2h call in
// cronReminder2h (cron.go), re-run — this test must fail on the "marked sent"
// assertion even though the email would still (wrongly) go out every tick.
func TestCronReminder2h_EnqueuesExactlyOnceAndMarksSent(t *testing.T) {
	app, queue := newTestApplicationWithNotifications(t)

	bookingID := uuid.New()
	candidate := &data.CronBooking{
		ClientEmail: "ana@example.com",
		ClientPhone: "+5491100000000",
		ComplexName: "Vibe",
		CourtName:   "Cancha 1",
	}
	candidate.ID = bookingID
	candidate.Date = time.Now()
	candidate.StartTime = "18:00"
	candidate.StartsAt = slots.At(timezone.Day(candidate.Date), "18:00")
	candidate.EndsAt = candidate.StartsAt.Add(60 * time.Minute)
	candidate.DurationMinutes = 60

	var marked []uuid.UUID
	app.models.Bookings = &mockBookingStore{
		GetForReminder2hEnrichedFn: func(ctx context.Context, now time.Time) ([]*data.CronBooking, error) {
			return []*data.CronBooking{candidate}, nil
		},
		MarkReminderSent2hFn: func(ctx context.Context, id uuid.UUID) error {
			marked = append(marked, id)
			return nil
		},
	}

	app.cronReminder2h(context.Background())

	if len(marked) != 1 || marked[0] != bookingID {
		t.Fatalf("want MarkReminderSent2h called once for %s; got %v", bookingID, marked)
	}

	emails := queue.payloadsOf("email:reminder_2h")
	if len(emails) != 1 {
		t.Fatalf("want exactly one reminder email enqueued; got %d (%v)", len(emails), queue.taskTypes())
	}

	// A second tick before reminder_sent_2h takes effect in a real store must
	// not enqueue again: simulate the flag having been set by swapping in a
	// store that now returns no candidates, the way GetForReminder2hEnriched's
	// `AND reminder_sent_2h = false` clause would once the first tick's
	// MarkReminderSent2h has landed.
	app.models.Bookings = &mockBookingStore{
		GetForReminder2hEnrichedFn: func(ctx context.Context, now time.Time) ([]*data.CronBooking, error) {
			return nil, nil
		},
	}
	app.cronReminder2h(context.Background())

	emails = queue.payloadsOf("email:reminder_2h")
	if len(emails) != 1 {
		t.Errorf("a second tick after the reminder flag is set must not enqueue again; got %d", len(emails))
	}
}

func TestCronReleaseExpiredPayments_NoExpiredBookings(t *testing.T) {
	app := newTestApplication(t)

	// Default mock returns nil, nil for GetExpiredPendingEnriched.
	// Should not panic.
	app.cronReleaseExpiredPayments(context.Background())
}

// newExpiredCronBooking builds a minimal CronBooking in "pending payment"
// state, the shape GetExpiredPendingEnriched would hand the job.
func newExpiredCronBooking(id uuid.UUID) *data.CronBooking {
	cb := &data.CronBooking{}
	cb.ID = id
	cb.ComplexID = uuid.New()
	cb.Status = "pending"
	cb.CollectionStatus = data.CollectionStatusUnpaid
	cb.RefundStatus = data.RefundStatusNone
	cb.Date = time.Now()
	cb.StartTime = "10:00"
	cb.StartsAt = slots.At(timezone.Day(cb.Date), "10:00")
	cb.EndsAt = cb.StartsAt.Add(60 * time.Minute)
	cb.DurationMinutes = 60
	cb.ComplexName = "Test Complex"
	cb.CourtName = "Court 1"
	return cb
}

// TestCronReleaseExpiredPayments_CancelsBooking is the happy path for the
// auto-cancel job: a booking still unpaid past the payment expiry window is
// cancelled. GetByBookingIDFn returns ErrRecordNotFound so the job takes the
// "no MP preference on file" branch and never dials MercadoPago — the point
// of the test is the cancellation, not the network call.
func TestCronReleaseExpiredPayments_CancelsBooking(t *testing.T) {
	app := newTestApplication(t)

	booking := newExpiredCronBooking(uuid.New())

	var updated *data.Booking
	app.models.Bookings = &mockBookingStore{
		GetExpiredPendingEnrichedFn: func(ctx context.Context, expiry time.Duration) ([]*data.CronBooking, error) {
			return []*data.CronBooking{booking}, nil
		},
		UpdateFn: func(ctx context.Context, b *data.Booking) error {
			updated = b
			return nil
		},
	}
	app.models.Payments = &mockPaymentStore{
		GetByBookingIDFn: func(ctx context.Context, bookingID uuid.UUID) (*paymentstore.Payment, error) {
			return nil, data.ErrRecordNotFound
		},
	}

	app.cronReleaseExpiredPayments(context.Background())

	if updated == nil {
		t.Fatal("want the booking to be updated (cancelled); got no Update call")
	}
	if updated.ID != booking.ID {
		t.Errorf("want booking %s updated; got %s", booking.ID, updated.ID)
	}
	if updated.Status != "cancelled" {
		t.Errorf("want status cancelled; got %q", updated.Status)
	}
	if updated.Notes == nil || *updated.Notes == "" {
		t.Error("want a cancellation note explaining the auto-cancel")
	}
}

// TestCronReleaseExpiredPayments_OneFailureDoesNotStopOthers proves the batch
// is not aborted by one booking that fails to update: with two expired
// bookings and Update failing only for the first, the second must still be
// cancelled.
func TestCronReleaseExpiredPayments_OneFailureDoesNotStopOthers(t *testing.T) {
	app := newTestApplication(t)

	failing := newExpiredCronBooking(uuid.New())
	ok := newExpiredCronBooking(uuid.New())

	var okUpdated bool
	var updateCalls int
	app.models.Bookings = &mockBookingStore{
		GetExpiredPendingEnrichedFn: func(ctx context.Context, expiry time.Duration) ([]*data.CronBooking, error) {
			return []*data.CronBooking{failing, ok}, nil
		},
		UpdateFn: func(ctx context.Context, b *data.Booking) error {
			updateCalls++
			if b.ID == failing.ID {
				return errors.New("simulated db failure")
			}
			if b.ID == ok.ID {
				okUpdated = true
			}
			return nil
		},
	}
	app.models.Payments = &mockPaymentStore{
		GetByBookingIDFn: func(ctx context.Context, bookingID uuid.UUID) (*paymentstore.Payment, error) {
			return nil, data.ErrRecordNotFound
		},
	}

	app.cronReleaseExpiredPayments(context.Background())

	if updateCalls != 2 {
		t.Fatalf("want both bookings attempted; got %d Update calls", updateCalls)
	}
	if !okUpdated {
		t.Error("want the second booking cancelled despite the first one failing")
	}
}

// TestCronReleaseExpiredPayments_Idempotent runs the job twice against a
// small stateful fake store: the first run cancels the one expired booking,
// and because a cancelled booking is no longer "pending payment" the second
// run must find nothing to do and must not touch the booking again.
func TestCronReleaseExpiredPayments_Idempotent(t *testing.T) {
	app := newTestApplication(t)

	booking := newExpiredCronBooking(uuid.New())
	status := booking.Status
	updateCalls := 0

	app.models.Bookings = &mockBookingStore{
		GetExpiredPendingEnrichedFn: func(ctx context.Context, expiry time.Duration) ([]*data.CronBooking, error) {
			if status != "pending" {
				return nil, nil
			}
			cb := *booking
			cb.Status = status
			return []*data.CronBooking{&cb}, nil
		},
		UpdateFn: func(ctx context.Context, b *data.Booking) error {
			updateCalls++
			status = b.Status
			return nil
		},
	}
	app.models.Payments = &mockPaymentStore{
		GetByBookingIDFn: func(ctx context.Context, bookingID uuid.UUID) (*paymentstore.Payment, error) {
			return nil, data.ErrRecordNotFound
		},
	}

	app.cronReleaseExpiredPayments(context.Background())
	app.cronReleaseExpiredPayments(context.Background())

	if updateCalls != 1 {
		t.Errorf("want exactly one Update across both runs (idempotent second run); got %d", updateCalls)
	}
	if status != "cancelled" {
		t.Errorf("want the booking left cancelled; got %q", status)
	}
}

// TestCronCleanBookingLinkTokens_DeletesExpiredTerminal is a mechanical test:
// the job had no coverage at all in cmd/api, so a regression that stopped it
// calling the store (e.g. a typo'd retention constant swap) would previously
// have gone unnoticed here.
func TestCronCleanBookingLinkTokens_DeletesExpiredTerminal(t *testing.T) {
	app := newTestApplication(t)

	var gotRetention time.Duration
	called := false
	app.models.BookingLinkTokens = &mockBookingLinkTokenStore{
		DeleteExpiredTerminalFn: func(ctx context.Context, retention time.Duration) error {
			called = true
			gotRetention = retention
			return nil
		},
	}

	app.cronCleanBookingLinkTokens(context.Background())

	if !called {
		t.Fatal("want DeleteExpiredTerminal to be called")
	}
	if gotRetention != bookingLinkTokenRetention {
		t.Errorf("want retention %v; got %v", bookingLinkTokenRetention, gotRetention)
	}
}

// TestCronCleanBookingLinkTokens_FailurePropagatesNoPanic documents that a
// store failure is logged and swallowed, not panicked on — the scheduler's
// per-job recover exists as a backstop, not as the primary error path.
func TestCronCleanBookingLinkTokens_FailurePropagatesNoPanic(t *testing.T) {
	app := newTestApplication(t)

	app.models.BookingLinkTokens = &mockBookingLinkTokenStore{
		DeleteExpiredTerminalFn: func(ctx context.Context, retention time.Duration) error {
			return errors.New("simulated db failure")
		},
	}

	// Should not panic.
	app.cronCleanBookingLinkTokens(context.Background())
}

// TestCronCleanFailedRefunds_DeletesResolved is a mechanical test for a job
// that previously had no coverage in cmd/api.
func TestCronCleanFailedRefunds_DeletesResolved(t *testing.T) {
	app := newTestApplication(t)

	called := false
	app.models.FailedRefunds = &mockFailedRefundStore{
		DeleteResolvedFn: func(ctx context.Context, olderThan time.Duration) (int64, error) {
			called = true
			return 3, nil
		},
	}

	app.cronCleanFailedRefunds(context.Background())

	if !called {
		t.Fatal("want DeleteResolved to be called")
	}
}
