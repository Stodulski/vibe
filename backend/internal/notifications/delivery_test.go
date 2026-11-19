package notifications

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/mailer"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// reminderService is a Service whose worker reads bookings from the stub and
// whose clock is pinned at now.
func reminderService(t *testing.T, bookings *stubBookings, now time.Time) (*Service, *stubQueue, *stubMailer) {
	t.Helper()
	q, m := newStubQueue(), &stubMailer{}
	s := NewService(q, m, &stubUsers{user: &authstore.User{Email: "owner@example.com"}}, bookings,
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	s.now = func() time.Time { return now }
	s.RegisterWorkers()
	return s, q, m
}

// deliverRaw runs the worker of taskType on a payload as the queue would.
func deliverRaw(t *testing.T, q *stubQueue, taskType string, payload any) error {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return q.handlers[taskType](t.Context(), raw)
}

// queuedReminder runs ReminderDue and returns the payload it queued, so the
// worker below reads exactly what the queue would store.
func queuedReminder(t *testing.T, s *Service, q *stubQueue, rem Reminder) json.RawMessage {
	t.Helper()
	if err := s.ReminderDue(rem); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(q.tasks[len(q.tasks)-1].payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func argentina(day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, timezone.Argentina)
}

// S3: the worker reads the booking back. A reminder whose booking was
// cancelled, or has already begun, while the task waited is not sent; one that
// is still ahead is sent, naming its real start.
func TestTheReminderWorkerSendsOnlyForABookingStillAhead(t *testing.T) {
	start := argentina(19, 18, 0)
	tests := []struct {
		name    string
		status  bookingstore.BookingStatus
		now     time.Time
		wantOut bool
	}{
		{"confirmed and ahead", bookingstore.BookingStatusConfirmed, argentina(19, 16, 0), true},
		{"cancelled since it was queued", bookingstore.BookingStatusCancelled, argentina(19, 16, 0), false},
		{"already begun", bookingstore.BookingStatusConfirmed, argentina(19, 18, 5), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bookings := &stubBookings{booking: &bookingstore.Booking{Status: tt.status, StartsAt: start}}
			s, q, m := reminderService(t, bookings, tt.now)
			raw := queuedReminder(t, s, q, Reminder{Email: "ana@example.com", ComplexName: "Vibe",
				Date: "19/11", StartTime: "18:00 a 19:30", BookingID: uuid.NewString()})

			if err := q.handlers[TaskEmailReminder2h](t.Context(), raw); err != nil {
				t.Fatalf("worker = %v", err)
			}
			if bookings.reads != 1 {
				t.Errorf("the worker must read the booking back; read it %d times", bookings.reads)
			}
			if got := len(m.sent) == 1; got != tt.wantOut {
				t.Fatalf("reminder sent = %v, want %v", got, tt.wantOut)
			}
			if tt.wantOut {
				if when := m.sent[0].args[8]; when != "hoy a las 18:00" {
					t.Errorf("the reminder must name the real start; got %q", when)
				}
			}
		})
	}
}

// A deleted booking is nothing to remind about; any other read failure is
// retried by the queue rather than sending blind or dropping the reminder.
func TestTheReminderWorkerOnAnUnreadableBooking(t *testing.T) {
	s, q, m := reminderService(t, &stubBookings{}, argentina(19, 16, 0))
	raw := queuedReminder(t, s, q, Reminder{Email: "ana@example.com", BookingID: uuid.NewString()})
	if err := q.handlers[TaskEmailReminder2h](t.Context(), raw); err != nil || len(m.sent) != 0 {
		t.Errorf("a missing booking: want nothing sent and no error; got %d sent, %v", len(m.sent), err)
	}

	s, q, m = reminderService(t, &stubBookings{err: errors.New("connection reset")}, argentina(19, 16, 0))
	raw = queuedReminder(t, s, q, Reminder{Email: "ana@example.com", BookingID: uuid.NewString()})
	if err := q.handlers[TaskEmailReminder2h](t.Context(), raw); err == nil || len(m.sent) != 0 {
		t.Errorf("an unreadable booking: want an error and nothing sent; got %d sent, %v", len(m.sent), err)
	}
}

// A reminder queued before the booking id travelled in the payload still
// decodes and is sent, without a read, naming the start from its own label.
func TestALegacyReminderWithoutABookingIDIsStillSent(t *testing.T) {
	bookings := &stubBookings{}
	_, q, m := reminderService(t, bookings, argentina(19, 16, 0))
	legacy := json.RawMessage(`{"email":"ana@example.com","complex_name":"Vibe","court_name":"Cancha 1",` +
		`"date":"19/11","start_time":"18:00 a 19:30","address":"","balance_amount":"$15.000","cancel_url":"","maps_url":""}`)

	if err := q.handlers[TaskEmailReminder2h](t.Context(), legacy); err != nil {
		t.Fatalf("worker = %v", err)
	}
	if len(m.sent) != 1 || m.sent[0].to != "ana@example.com" || bookings.reads != 0 {
		t.Fatalf("want the legacy reminder sent without a read; got %d sent, %d reads", len(m.sent), bookings.reads)
	}
	if when := m.sent[0].args[8]; when != "el 19/11 a las 18:00" {
		t.Errorf("legacy start = %q", when)
	}
}

// A game just past midnight is "mañana" to a client reminded at 22:30.
func TestReminderWhenSaysTomorrowForAGameAfterMidnight(t *testing.T) {
	if got := reminderWhen(Reminder{}, argentina(20, 0, 30), argentina(19, 22, 30)); got != "mañana a las 00:30" {
		t.Errorf("reminderWhen = %q", got)
	}
}

// S7: a client email about a booking carries the complex's phone and replies
// to the owner's current address, resolved at delivery.
func TestClientBookingEmailsCarryTheComplexContact(t *testing.T) {
	s, q, m, users := newTestService(t)
	users.user = &authstore.User{Email: "dueno@vibe.test"}
	s.RegisterWorkers()

	c := fullConfirmation()
	c.ComplexPhone = "+5491122334455"
	c.BookingID = uuid.NewString()
	if err := s.BookingConfirmed(c); err != nil {
		t.Fatal(err)
	}
	for _, task := range q.tasks {
		if task.taskType != TaskEmailBookingConfirmation {
			continue
		}
		if err := deliverRaw(t, q, task.taskType, task.payload); err != nil {
			t.Fatal(err)
		}
	}
	if len(m.sent) != 1 {
		t.Fatalf("want one client email; got %d", len(m.sent))
	}
	got, ok := m.sent[0].args[len(m.sent[0].args)-1].(mailer.Contact)
	if !ok || got.Phone != "+5491122334455" || got.ReplyTo != "dueno@vibe.test" {
		t.Errorf("contact = %+v; want the complex phone and the owner's address as reply-to", got)
	}
}

// S5: a refused enqueue of a critical client notice is returned, not only
// logged, so the caller can report it with the booking.
func TestCriticalClientNoticesReturnARefusedEnqueue(t *testing.T) {
	refused := errors.New("insert failed")
	tests := []struct {
		taskType string
		send     func(*Service) error
	}{
		{TaskEmailBookingConfirmation, func(s *Service) error { return s.BookingConfirmed(fullConfirmation()) }},
		{TaskEmailBookingCancelled, func(s *Service) error { return s.BookingCancelled(fullCancellation()) }},
		{TaskEmailDepositRefunded, func(s *Service) error {
			return s.DepositRefunded(Refund{Email: "ana@example.com", Amount: "$5.000", BookingID: "b", RefundID: "r"})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.taskType, func(t *testing.T) {
			s, q, _, _ := newTestService(t)
			q.failOn = map[string]error{tt.taskType: refused}
			if err := tt.send(s); !errors.Is(err, refused) {
				t.Errorf("want the refusal returned; got %v", err)
			}
			if q.has(tt.taskType) {
				t.Error("a refused task must not read as queued")
			}
		})
	}
}

// S4: a refund that cancelled the booking reaches the client as one email that
// says so, naming the booking.
func TestARefundThatCancelledTheBookingSaysSo(t *testing.T) {
	s, q, m, _ := newTestService(t)
	s.RegisterWorkers()
	if err := s.DepositRefunded(Refund{Email: "ana@example.com", ComplexName: "Vibe", Amount: "$5.000",
		Cancelled: true, CourtName: "Cancha 1", Date: "19/11", StartTime: "18:00 a 19:30",
		BookingID: "b", RefundID: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := deliverRaw(t, q, TaskEmailDepositRefunded, q.tasks[0].payload); err != nil {
		t.Fatal(err)
	}
	if len(m.sent) != 1 || m.sent[0].args[3] != true || m.sent[0].args[4] != "Cancha 1" || !strings.Contains(m.sent[0].args[5].(string), "19/11") {
		t.Errorf("want one email marked cancelled naming the booking; got %+v", m.sent)
	}
}
