package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/whatsapp"
)

type enqueued struct {
	taskType string
	payload  any
	dedupKey string
}

type stubQueue struct {
	tasks    []enqueued
	handlers map[string]func(context.Context, json.RawMessage) error
}

func newStubQueue() *stubQueue {
	return &stubQueue{handlers: map[string]func(context.Context, json.RawMessage) error{}}
}

func (q *stubQueue) Enqueue(taskType string, payload any, dedupKey string) {
	q.tasks = append(q.tasks, enqueued{taskType, payload, dedupKey})
}

// keyOf returns the dedup key the service asked for a task type, and whether
// that type was enqueued at all.
func (q *stubQueue) keyOf(taskType string) (string, bool) {
	for _, t := range q.tasks {
		if t.taskType == taskType {
			return t.dedupKey, true
		}
	}
	return "", false
}

func (q *stubQueue) RegisterHandler(taskType string, h func(context.Context, json.RawMessage) error) {
	q.handlers[taskType] = h
}

// types returns the task types enqueued, in order.
func (q *stubQueue) types() []string {
	out := make([]string, 0, len(q.tasks))
	for _, t := range q.tasks {
		out = append(out, t.taskType)
	}
	return out
}

func (q *stubQueue) has(taskType string) bool {
	for _, t := range q.tasks {
		if t.taskType == taskType {
			return true
		}
	}
	return false
}

type sentMail struct {
	kind string
	to   string
	args []any
}

type stubMailer struct {
	sent []sentMail
	err  error
}

func (m *stubMailer) record(kind, to string, args ...any) error {
	m.sent = append(m.sent, sentMail{kind: kind, to: to, args: args})
	return m.err
}

func (m *stubMailer) SendBookingConfirmation(_ context.Context, to, complexName, courtName, date, startTime, address, mapsURL, cancelURL, depositAmount, balanceAmount, cancellationLine string) error {
	return m.record("booking_confirmation", to, complexName, courtName, date, startTime, address, mapsURL, cancelURL, depositAmount, balanceAmount, cancellationLine)
}
func (m *stubMailer) SendReminder2h(_ context.Context, to, complexName, courtName, date, startTime, address, mapsURL, balanceAmount, cancelURL string) error {
	return m.record("reminder", to, complexName, courtName, date, startTime, address, mapsURL, balanceAmount, cancelURL)
}
func (m *stubMailer) SendBookingCancelled(_ context.Context, to, complexName, courtName, date, startTime, refundLine, refundAmount, bookURL string) error {
	return m.record("cancelled", to, complexName, courtName, date, startTime, refundLine, refundAmount, bookURL)
}
func (m *stubMailer) SendDepositRefunded(_ context.Context, to, complexName, amount, bookURL string) error {
	return m.record("refunded", to, complexName, amount, bookURL)
}
func (m *stubMailer) SendOwnerNewBooking(_ context.Context, to, complexName, courtName, clientName, date, startTime string) error {
	return m.record("owner_new_booking", to, complexName, courtName, clientName, date, startTime)
}
func (m *stubMailer) SendEmailVerification(_ context.Context, to, firstName, verifyURL string) error {
	return m.record("verification", to, firstName, verifyURL)
}
func (m *stubMailer) SendPasswordReset(_ context.Context, to, firstName, resetURL string) error {
	return m.record("password_reset", to, firstName, resetURL)
}
func (m *stubMailer) SendDuplicateRegistration(_ context.Context, to, firstName, loginURL, resetURL string) error {
	return m.record("duplicate_registration", to, firstName, loginURL, resetURL)
}

type sentWA struct {
	to   string
	tmpl whatsapp.TemplateMessage
}

type stubWhatsApp struct {
	sent []sentWA
	err  error
}

func (w *stubWhatsApp) SendTemplate(_ context.Context, to string, tmpl whatsapp.TemplateMessage) error {
	w.sent = append(w.sent, sentWA{to: to, tmpl: tmpl})
	return w.err
}

type stubUsers struct {
	user *authstore.User
	err  error
}

func (u *stubUsers) GetByID(context.Context, uuid.UUID) (*authstore.User, error) {
	return u.user, u.err
}

func newTestService(t *testing.T, whatsappEnabled bool) (*Service, *stubQueue, *stubMailer, *stubWhatsApp, *stubUsers) {
	t.Helper()
	q, m, w := newStubQueue(), &stubMailer{}, &stubWhatsApp{}
	users := &stubUsers{user: &authstore.User{Email: "owner@example.com"}}
	s := NewService(q, m, w, users, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), whatsappEnabled)
	return s, q, m, w, users
}

func fullConfirmation() BookingConfirmation {
	return BookingConfirmation{
		Source: SourceOnlineCheckout, Email: "ana@example.com", Phone: "+5411",
		ComplexName: "Vibe", CourtName: "Cancha 1", ClientName: "Ana",
		Date: "04/03", StartTime: "18:00 - 19:30",
		Address:   "Av. Santa Fe 1200, Buenos Aires",
		CancelURL: "https://vibe.test/cancel", CancelPath: "cancel/abc", MapsQuery: "Vibe+Palermo",
		MapsURL:          "https://www.google.com/maps/search/?api=1&query=Vibe+Palermo",
		DepositAmount:    "$5.000",
		BalanceAmount:    "$15.000",
		CancellationLine: "con devolución hasta 24 horas antes del turno.",
		OwnerID:          uuid.New().String(),
	}
}

func fullCancellation() Cancellation {
	return Cancellation{
		Email: "ana@example.com", Phone: "+5411",
		ComplexName: "Vibe", CourtName: "Cancha 1",
		Date: "04/03", StartTime: "18:00 - 19:30",
		RefundLine:   "$5.000 enviados, se acreditan en los próximos días hábiles.",
		RefundAmount: "$5.000",
		BookPath:     "vibe/book", BookURL: "https://vibe.test/vibe/book",
	}
}

func fullRefund() Refund {
	return Refund{
		Email: "ana@example.com", Phone: "+5411",
		ComplexName: "Vibe", Amount: "$5.000",
		BookPath: "vibe/book", BookURL: "https://vibe.test/vibe/book",
	}
}

func fullReminder() Reminder {
	return Reminder{
		Email: "ana@example.com", Phone: "+5411",
		ComplexName: "Vibe", CourtName: "Cancha 1",
		Date: "04/03", StartTime: "18:00 - 19:30",
		Address: "Av. Santa Fe 1200, Buenos Aires", BalanceAmount: "$15.000",
		MapsQuery: "Vibe+Palermo", CancelPath: "vibe/book/cancel?token=abc",
		MapsURL:   "https://www.google.com/maps/search/?api=1&query=Vibe+Palermo",
		CancelURL: "https://vibe.test/vibe/book/cancel?token=abc",
	}
}

func TestBookingConfirmedNotifiesEveryChannel(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)

	s.BookingConfirmed(fullConfirmation())

	for _, want := range []string{TaskEmailBookingConfirmation, TaskWABookingConfirmation, TaskEmailOwnerNewBooking} {
		if !q.has(want) {
			t.Errorf("%s was not enqueued; got %v", want, q.types())
		}
	}
}

// A client who gave no email still gets WhatsApp, and the owner is told either
// way — the channels are independent.
func TestBookingConfirmedSkipsOnlyTheMissingChannel(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)

	c := fullConfirmation()
	c.Email = ""
	s.BookingConfirmed(c)

	if q.has(TaskEmailBookingConfirmation) {
		t.Error("a client email was enqueued with no address")
	}
	if !q.has(TaskWABookingConfirmation) {
		t.Error("WhatsApp must still be sent when there is no email")
	}
	if !q.has(TaskEmailOwnerNewBooking) {
		t.Error("the owner must be told regardless of the client's channels")
	}
}

func TestWhatsAppIsSkippedWhenNotConfigured(t *testing.T) {
	s, q, _, _, _ := newTestService(t, false)

	s.BookingConfirmed(fullConfirmation())

	if q.has(TaskWABookingConfirmation) {
		t.Error("WhatsApp was enqueued with the channel disabled")
	}
	if !q.has(TaskEmailBookingConfirmation) {
		t.Error("the email must still go out")
	}
}

// A complex with no coordinates used to lose the WhatsApp channel entirely:
// the maps parameter came out empty and the guard below refused the task, with
// the email still going out to hide it. booklink.MapsQuery now falls back to
// the written address, so this is what the caller hands over.
func TestConfirmationIsNotSkippedForAComplexWithoutCoordinates(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)

	c := fullConfirmation()
	// What booklink.MapsQuery returns for a complex with no latitude.
	c.MapsQuery = "Vibe+Palermo%2C+Av.+Santa+Fe+1200%2C+Buenos+Aires"
	s.BookingConfirmed(c)

	if !q.has(TaskWABookingConfirmation) {
		t.Errorf("a complex with only a written address must still get WhatsApp; got %v", q.types())
	}
}

// Meta rejects a template whose buttons are unbound, so enqueueing it would
// only produce a task that can never succeed.
func TestWhatsAppIsSkippedWhenButtonParametersAreMissing(t *testing.T) {
	for _, missing := range []string{"cancel path", "maps query"} {
		t.Run(missing, func(t *testing.T) {
			s, q, _, _, _ := newTestService(t, true)

			c := fullConfirmation()
			if missing == "cancel path" {
				c.CancelPath = ""
			} else {
				c.MapsQuery = ""
			}
			s.BookingConfirmed(c)

			if q.has(TaskWABookingConfirmation) {
				t.Error("a template with an unbound button was enqueued")
			}
			if !q.has(TaskEmailBookingConfirmation) {
				t.Error("the email must be unaffected")
			}
		})
	}
}

func TestReminderUsesBothChannels(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)

	s.ReminderDue(fullReminder())

	if !q.has(TaskEmailReminder2h) || !q.has(TaskWAReminder2h) {
		t.Errorf("want both reminder channels; got %v", q.types())
	}
}

// The cron mints a fresh access token per reminder, and a mint that fails
// leaves nothing to bind the cancel button to. The email still goes out.
func TestReminderWhatsAppIsSkippedWithoutACancelLink(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)

	rem := fullReminder()
	rem.CancelPath = ""
	s.ReminderDue(rem)

	if q.has(TaskWAReminder2h) {
		t.Error("a reminder template with an unbound button was enqueued")
	}
	if !q.has(TaskEmailReminder2h) {
		t.Error("the reminder email must still go out")
	}
}

// A client who gave a phone number and got their confirmation and reminder over
// WhatsApp used to hear about the cancellation, and about their money coming
// back, only by email — the payloads for those two events had no phone field at
// all, so the channel was not so much disabled as unrepresentable.
func TestCancellationAndRefundReachWhatsApp(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)

	s.BookingCancelled(fullCancellation())
	s.DepositRefunded(fullRefund())

	for _, want := range []string{
		TaskEmailBookingCancelled, TaskWABookingCancelled,
		TaskEmailDepositRefunded, TaskWADepositRefunded,
	} {
		if !q.has(want) {
			t.Errorf("%s was not enqueued; got %v", want, q.types())
		}
	}
}

// The channel gates are unchanged: no phone, or WhatsApp not configured, and
// only the email goes out.
func TestCancellationAndRefundKeepTheirChannelGates(t *testing.T) {
	t.Run("no phone", func(t *testing.T) {
		s, q, _, _, _ := newTestService(t, true)

		cancellation, refund := fullCancellation(), fullRefund()
		cancellation.Phone, refund.Phone = "", ""
		s.BookingCancelled(cancellation)
		s.DepositRefunded(refund)

		if q.has(TaskWABookingCancelled) || q.has(TaskWADepositRefunded) {
			t.Errorf("WhatsApp was enqueued for a client who gave no number; got %v", q.types())
		}
		if len(q.tasks) != 2 {
			t.Errorf("want exactly the two emails; got %v", q.types())
		}
	})

	t.Run("whatsapp not configured", func(t *testing.T) {
		s, q, _, _, _ := newTestService(t, false)

		s.BookingCancelled(fullCancellation())
		s.DepositRefunded(fullRefund())

		if q.has(TaskWABookingCancelled) || q.has(TaskWADepositRefunded) {
			t.Errorf("WhatsApp was enqueued with the channel disabled; got %v", q.types())
		}
	})
}

// The owner is told about a sale, not about their own data entry. An owner who
// books a walk-in from the dashboard used to receive an email announcing the
// booking they had just made themselves, every single time.
func TestOwnerIsEmailedOnlyForBookingsTheyDidNotEnter(t *testing.T) {
	t.Run("online checkout", func(t *testing.T) {
		s, q, _, _, _ := newTestService(t, true)

		c := fullConfirmation()
		c.Source = SourceOnlineCheckout
		s.BookingConfirmed(c)

		if !q.has(TaskEmailOwnerNewBooking) {
			t.Errorf("the owner must hear about an online sale; got %v", q.types())
		}
	})

	t.Run("staff created it from the dashboard", func(t *testing.T) {
		s, q, _, _, _ := newTestService(t, true)

		c := fullConfirmation()
		c.Source = SourceStaffCreate
		s.BookingConfirmed(c)

		if q.has(TaskEmailOwnerNewBooking) {
			t.Errorf("the owner was emailed about the booking they just made; got %v", q.types())
		}
		if !q.has(TaskEmailBookingConfirmation) || !q.has(TaskWABookingConfirmation) {
			t.Errorf("the client must still be notified on both channels; got %v", q.types())
		}
	})
}

func TestNothingIsEnqueuedWithoutARecipient(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)

	s.BookingCancelled(Cancellation{})
	s.DepositRefunded(Refund{})
	s.ReminderDue(Reminder{})
	s.BookingCancelled(Cancellation{Phone: "+5411"})
	s.DepositRefunded(Refund{Phone: "+5411"})

	if len(q.tasks) != 0 {
		t.Errorf("want nothing enqueued with no recipient; got %v", q.types())
	}
}

func TestWorkersDeliverEachTaskType(t *testing.T) {
	s, q, mailer, wa, _ := newTestService(t, true)
	s.RegisterWorkers()

	// Every task the service can enqueue must have a worker registered, or it
	// sits in Redis forever.
	for _, taskType := range []string{
		TaskEmailBookingConfirmation, TaskEmailReminder2h, TaskEmailBookingCancelled,
		TaskEmailDepositRefunded, TaskEmailOwnerNewBooking, TaskEmailVerification,
		TaskEmailPasswordReset, TaskEmailDuplicateRegistration,
		TaskWABookingConfirmation, TaskWAReminder2h,
		TaskWABookingCancelled, TaskWADepositRefunded,
	} {
		if _, ok := q.handlers[taskType]; !ok {
			t.Errorf("no worker registered for %s", taskType)
		}
	}

	// Drive one email and one WhatsApp worker end to end.
	payload, _ := json.Marshal(Cancellation{Email: "ana@example.com", ComplexName: "Vibe"})
	if err := q.handlers[TaskEmailBookingCancelled](t.Context(), payload); err != nil {
		t.Fatalf("delivering a cancellation: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].kind != "cancelled" {
		t.Errorf("the cancellation was not delivered; got %v", mailer.sent)
	}

	payload, _ = json.Marshal(waReminder{Phone: "+5411", ComplexName: "Vibe"})
	if err := q.handlers[TaskWAReminder2h](t.Context(), payload); err != nil {
		t.Fatalf("delivering a WhatsApp reminder: %v", err)
	}
	if len(wa.sent) != 1 || wa.sent[0].to != "+5411" {
		t.Errorf("the WhatsApp reminder was not delivered; got %v", wa.sent)
	}
}

// Each WhatsApp worker must build its own template. Registering the wrong
// builder for a task type is a mistake nothing else catches: the send succeeds,
// and the client gets a confirmation for a booking that was just cancelled.
func TestWhatsAppWorkersBuildTheRightTemplate(t *testing.T) {
	for _, c := range []struct {
		taskType string
		payload  any
		want     string
	}{
		{TaskWABookingConfirmation, waBookingConfirmation{Phone: "+5411"}, "booking_confirmation"},
		{TaskWAReminder2h, waReminder{Phone: "+5411"}, "reminder_2h"},
		{TaskWABookingCancelled, waBookingCancelled{Phone: "+5411"}, "booking_cancelled"},
		{TaskWADepositRefunded, waDepositRefunded{Phone: "+5411"}, "deposit_returned"},
	} {
		t.Run(c.want, func(t *testing.T) {
			s, q, _, wa, _ := newTestService(t, true)
			s.RegisterWorkers()

			raw, _ := json.Marshal(c.payload)
			if err := q.handlers[c.taskType](t.Context(), raw); err != nil {
				t.Fatalf("delivering %s: %v", c.taskType, err)
			}
			if len(wa.sent) != 1 {
				t.Fatalf("nothing was sent for %s", c.taskType)
			}
			if got := wa.sent[0].tmpl.TemplateName; got != c.want {
				t.Errorf("%s built template %q; want %q", c.taskType, got, c.want)
			}
		})
	}
}

// The cancellation and refund email payloads are durable storage too, so they
// carry what their worker reads and nothing more. Enqueueing the whole
// Cancellation would write the client's phone number and two WhatsApp button
// suffixes into Redis on every cancellation.
func TestCancellationEmailPayloadCarriesOnlyWhatTheEmailNeeds(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)

	s.BookingCancelled(fullCancellation())
	s.DepositRefunded(fullRefund())

	for _, c := range []struct {
		taskType string
		unwanted []string
		required []string
	}{
		{TaskEmailBookingCancelled, []string{"phone", "book_path"}, []string{"email", "refund_line", "refund_amount", "book_url"}},
		{TaskEmailDepositRefunded, []string{"phone", "book_path"}, []string{"email", "amount", "book_url"}},
	} {
		t.Run(c.taskType, func(t *testing.T) {
			var payload map[string]any
			for _, task := range q.tasks {
				if task.taskType != c.taskType {
					continue
				}
				raw, err := json.Marshal(task.payload)
				if err != nil {
					t.Fatalf("marshalling the payload: %v", err)
				}
				if err := json.Unmarshal(raw, &payload); err != nil {
					t.Fatalf("payload is not valid JSON: %v", err)
				}
			}
			if payload == nil {
				t.Fatalf("no %s task was enqueued", c.taskType)
			}
			for _, unwanted := range c.unwanted {
				if _, present := payload[unwanted]; present {
					t.Errorf("the email payload persists %q, which the email worker never reads", unwanted)
				}
			}
			for _, required := range c.required {
				if _, present := payload[required]; !present {
					t.Errorf("the email payload is missing %q", required)
				}
			}
		})
	}
}

// The owner's address is looked up at delivery, not captured at enqueue, so a
// task queued before an address change still reaches the current one.
func TestOwnerEmailIsResolvedAtDeliveryTime(t *testing.T) {
	s, q, mailer, _, users := newTestService(t, true)
	s.RegisterWorkers()

	users.user = &authstore.User{Email: "new-owner@example.com"}
	payload, _ := json.Marshal(ownerNewBooking{OwnerID: uuid.New().String(), ComplexName: "Vibe"})

	if err := q.handlers[TaskEmailOwnerNewBooking](t.Context(), payload); err != nil {
		t.Fatalf("delivering the owner email: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].to != "new-owner@example.com" {
		t.Errorf("the owner email went to the wrong address; got %v", mailer.sent)
	}
}

func TestOwnerEmailFailsLoudlyOnAnUnknownOwner(t *testing.T) {
	s, q, _, _, users := newTestService(t, true)
	s.RegisterWorkers()

	users.err = errors.New("no such user")
	payload, _ := json.Marshal(ownerNewBooking{OwnerID: uuid.New().String()})

	if err := q.handlers[TaskEmailOwnerNewBooking](t.Context(), payload); err == nil {
		t.Error("want an error so the queue can retry; got nil")
	}
}

// A malformed payload must surface as an error naming the task, not a panic.
func TestWorkersRejectMalformedPayloads(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)
	s.RegisterWorkers()

	err := q.handlers[TaskEmailReminder2h](t.Context(), json.RawMessage(`{"email":`))
	if err == nil {
		t.Fatal("want an error for a malformed payload; got nil")
	}
	if !bytes.Contains([]byte(err.Error()), []byte(TaskEmailReminder2h)) {
		t.Errorf("the error should name the task; got %q", err.Error())
	}
}

// The email payload is durable storage, so it must carry what the email worker
// reads and nothing more. Enqueueing the whole BookingConfirmation put the
// client's phone number and the owner id into Redis on every booking.
func TestBookingEmailPayloadCarriesOnlyWhatTheEmailNeeds(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)

	s.BookingConfirmed(fullConfirmation())

	var payload map[string]any
	for _, task := range q.tasks {
		if task.taskType == TaskEmailBookingConfirmation {
			raw, err := json.Marshal(task.payload)
			if err != nil {
				t.Fatalf("marshalling the payload: %v", err)
			}
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatalf("payload is not valid JSON: %v", err)
			}
		}
	}
	if payload == nil {
		t.Fatal("no email task was enqueued")
	}

	for _, unwanted := range []string{"phone", "owner_id", "cancel_path", "maps_query", "client_name"} {
		if _, present := payload[unwanted]; present {
			t.Errorf("the email payload persists %q, which the email worker never reads", unwanted)
		}
	}
	for _, required := range []string{"email", "complex_name", "court_name", "date", "start_time", "address", "maps_url", "cancel_url", "deposit_amount", "balance_amount", "cancellation_line"} {
		if _, present := payload[required]; !present {
			t.Errorf("the email payload is missing %q", required)
		}
	}
}

// The three account emails each carry exactly their own link. Merging them into
// one struct with optional URLs let a reset task be enqueued with a
// verification link and no reset link, and still compile.
func TestAccountEmailsCarryTheirOwnLink(t *testing.T) {
	tests := []struct {
		name     string
		send     func(*Service)
		taskType string
		wantURL  string
		wantKey  string
	}{
		{
			name: "verification",
			send: func(s *Service) {
				s.EmailVerification(VerificationEmail{To: "ana@example.com", FirstName: "Ana", VerifyURL: "https://vibe.test/verify?t=1"})
			},
			taskType: TaskEmailVerification,
			wantKey:  "verify_url",
			wantURL:  "https://vibe.test/verify?t=1",
		},
		{
			name: "password reset",
			send: func(s *Service) {
				s.PasswordReset(PasswordResetEmail{To: "ana@example.com", FirstName: "Ana", ResetURL: "https://vibe.test/reset?t=2"})
			},
			taskType: TaskEmailPasswordReset,
			wantKey:  "reset_url",
			wantURL:  "https://vibe.test/reset?t=2",
		},
		{
			name: "duplicate registration",
			send: func(s *Service) {
				s.DuplicateRegistration(DuplicateRegistrationEmail{
					To: "ana@example.com", FirstName: "Ana",
					LoginURL: "https://vibe.test/login", ResetURL: "https://vibe.test/forgot",
				})
			},
			taskType: TaskEmailDuplicateRegistration,
			wantKey:  "login_url",
			wantURL:  "https://vibe.test/login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, q, _, _, _ := newTestService(t, false)
			tt.send(s)

			if len(q.tasks) != 1 || q.tasks[0].taskType != tt.taskType {
				t.Fatalf("want one %s task; got %v", tt.taskType, q.types())
			}

			raw, _ := json.Marshal(q.tasks[0].payload)
			var payload map[string]any
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatalf("payload is not valid JSON: %v", err)
			}
			if payload[tt.wantKey] != tt.wantURL {
				t.Errorf("want %s = %q; got %v", tt.wantKey, tt.wantURL, payload[tt.wantKey])
			}
			if payload["to"] != "ana@example.com" {
				t.Errorf("the recipient was lost; got %v", payload["to"])
			}
		})
	}
}

// The account emails are the ones that gate access to an account, so each is
// driven end to end through its worker.
func TestAccountEmailWorkersDeliver(t *testing.T) {
	s, q, mailer, _, _ := newTestService(t, false)
	s.RegisterWorkers()

	cases := []struct {
		taskType string
		payload  any
		wantKind string
	}{
		{TaskEmailVerification, VerificationEmail{To: "ana@example.com", FirstName: "Ana", VerifyURL: "https://v"}, "verification"},
		{TaskEmailPasswordReset, PasswordResetEmail{To: "ana@example.com", FirstName: "Ana", ResetURL: "https://r"}, "password_reset"},
		{TaskEmailDuplicateRegistration, DuplicateRegistrationEmail{To: "ana@example.com", FirstName: "Ana", LoginURL: "https://l", ResetURL: "https://r"}, "duplicate_registration"},
	}

	for _, c := range cases {
		t.Run(c.wantKind, func(t *testing.T) {
			before := len(mailer.sent)
			raw, _ := json.Marshal(c.payload)

			if err := q.handlers[c.taskType](t.Context(), raw); err != nil {
				t.Fatalf("delivering %s: %v", c.taskType, err)
			}
			if len(mailer.sent) != before+1 {
				t.Fatalf("nothing was sent for %s", c.taskType)
			}

			sent := mailer.sent[len(mailer.sent)-1]
			if sent.kind != c.wantKind {
				t.Errorf("want kind %q; got %q", c.wantKind, sent.kind)
			}
			if sent.to != "ana@example.com" {
				t.Errorf("want the recipient; got %q", sent.to)
			}
			// The link is the whole point of these messages: an empty one
			// makes the email useless and the account unrecoverable.
			for _, arg := range sent.args[1:] {
				if s, isString := arg.(string); isString && s == "" {
					t.Errorf("%s was delivered with an empty link: %v", c.wantKind, sent.args)
				}
			}
		})
	}
}

// A delivery failure has to reach the queue so it can retry, not be swallowed.
func TestDeliveryFailuresPropagateToTheQueue(t *testing.T) {
	s, q, mailer, _, _ := newTestService(t, false)
	s.RegisterWorkers()
	mailer.err = errors.New("smtp unavailable")

	raw, _ := json.Marshal(PasswordResetEmail{To: "ana@example.com", ResetURL: "https://r"})
	if err := q.handlers[TaskEmailPasswordReset](t.Context(), raw); err == nil {
		t.Error("want the failure returned so the queue retries; got nil")
	}
}

// ---------------------------------------------------------------------------
// Deduplication (JOB-04)
// ---------------------------------------------------------------------------

// TestEachDeliveryCarriesADeduplicationKey is the enqueue-side half of JOB-04.
// The queue is at-least-once, so the only thing standing between a redelivered
// MercadoPago webhook and a second confirmation email is a key the service
// asks for here. A task enqueued with an empty key is one that will be sent
// twice, which is why every one of them is listed.
func TestEachDeliveryCarriesADeduplicationKey(t *testing.T) {
	bookingID := uuid.New().String()

	s, q, _, _, _ := newTestService(t, true)
	confirmation := fullConfirmation()
	confirmation.BookingID = bookingID
	s.BookingConfirmed(confirmation)

	cancellation := fullCancellation()
	cancellation.BookingID = bookingID
	s.BookingCancelled(cancellation)

	refund := fullRefund()
	refund.BookingID = bookingID
	s.DepositRefunded(refund)

	reminder := fullReminder()
	reminder.BookingID = bookingID
	s.ReminderDue(reminder)

	s.EmailVerification(VerificationEmail{To: "ana@example.com", VerifyURL: "https://vibe.test/v/tok"})
	s.PasswordReset(PasswordResetEmail{To: "ana@example.com", ResetURL: "https://vibe.test/r/tok"})
	s.DuplicateRegistration(DuplicateRegistrationEmail{To: "ana@example.com", ResetURL: "https://vibe.test/r/tok"})

	for _, taskType := range q.types() {
		key, ok := q.keyOf(taskType)
		if !ok {
			t.Fatalf("%s: enqueued task vanished from the stub", taskType)
		}
		if key == "" {
			t.Errorf("%s was enqueued with no deduplication key, so a redelivery sends it twice", taskType)
		}
	}
}

// TestTheSameDeliveryTwiceAsksForTheSameKey is what makes the key worth
// having: two runs of the same notification have to agree, or the queue has
// nothing to match on and both are recorded.
func TestTheSameDeliveryTwiceAsksForTheSameKey(t *testing.T) {
	confirmation := fullConfirmation()
	confirmation.BookingID = uuid.New().String()

	first, second := newStubQueue(), newStubQueue()
	for _, q := range []*stubQueue{first, second} {
		s := NewService(q, &stubMailer{}, &stubWhatsApp{},
			&stubUsers{user: &authstore.User{Email: "owner@example.com"}},
			slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), true)
		s.BookingConfirmed(confirmation)
	}

	for _, taskType := range first.types() {
		a, _ := first.keyOf(taskType)
		b, _ := second.keyOf(taskType)
		if a != b {
			t.Errorf("%s: key %q on the first delivery and %q on the second; a redelivery would be recorded as new work",
				taskType, a, b)
		}
	}
}

// TestTwoClientsOfOneBookingDoNotShareAKey is the failure the empty-key rule
// in dedup exists to avoid, stated as a property: the key has to separate
// recipients, or the second client's confirmation is silently dropped as a
// duplicate of the first's.
func TestTwoClientsOfOneBookingDoNotShareAKey(t *testing.T) {
	bookingID := uuid.New().String()

	keys := make(map[string]string, 2)
	for _, email := range []string{"ana@example.com", "beto@example.com"} {
		q := newStubQueue()
		s := NewService(q, &stubMailer{}, &stubWhatsApp{},
			&stubUsers{user: &authstore.User{Email: "owner@example.com"}},
			slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), false)

		confirmation := fullConfirmation()
		confirmation.BookingID, confirmation.Email = bookingID, email
		s.BookingConfirmed(confirmation)

		key, ok := q.keyOf(TaskEmailBookingConfirmation)
		if !ok {
			t.Fatalf("%s: no confirmation was enqueued", email)
		}
		keys[email] = key
	}

	if keys["ana@example.com"] == keys["beto@example.com"] {
		t.Error("two recipients of one booking got the same deduplication key; one of them never gets their email")
	}
}
