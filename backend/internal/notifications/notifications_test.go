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
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mailer"
)

type enqueued struct {
	taskType string
	payload  any
	dedupKey string
}

type stubQueue struct {
	tasks    []enqueued
	handlers map[string]func(context.Context, json.RawMessage) error
	// failOn makes TryEnqueue refuse a task type with the error given, the way
	// a failed insert does, and records nothing for it.
	failOn map[string]error
}

func newStubQueue() *stubQueue {
	return &stubQueue{handlers: map[string]func(context.Context, json.RawMessage) error{}}
}

func (q *stubQueue) Enqueue(taskType string, payload any, dedupKey string) {
	q.tasks = append(q.tasks, enqueued{taskType, payload, dedupKey})
}

func (q *stubQueue) TryEnqueue(taskType string, payload any, dedupKey string) error {
	if err := q.failOn[taskType]; err != nil {
		return err
	}
	q.tasks = append(q.tasks, enqueued{taskType, payload, dedupKey})
	return nil
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

func (m *stubMailer) SendBookingConfirmation(_ context.Context, to, complexName, courtName, date, startTime, address, mapsURL, cancelURL, depositAmount, balanceAmount, cancellationLine, paymentState string, contact mailer.Contact) error {
	return m.record("booking_confirmation", to, complexName, courtName, date, startTime, address, mapsURL, cancelURL, depositAmount, balanceAmount, cancellationLine, paymentState, contact)
}
func (m *stubMailer) SendReminder2h(_ context.Context, to, complexName, courtName, date, startTime, when, address, mapsURL, balanceAmount, cancelURL string, contact mailer.Contact) error {
	return m.record("reminder", to, complexName, courtName, date, startTime, address, mapsURL, balanceAmount, cancelURL, when, contact)
}
func (m *stubMailer) SendBookingCancelled(_ context.Context, to, complexName, courtName, date, startTime, refundLine, refundAmount, bookURL string, contact mailer.Contact) error {
	return m.record("cancelled", to, complexName, courtName, date, startTime, refundLine, refundAmount, bookURL, contact)
}
func (m *stubMailer) SendDepositRefunded(_ context.Context, to, complexName, amount, bookURL string, cancelled bool, courtName, date, startTime string, contact mailer.Contact) error {
	return m.record("refunded", to, complexName, amount, bookURL, cancelled, courtName, date, startTime, contact)
}
func (m *stubMailer) SendOwnerNewBooking(_ context.Context, to, complexName, courtName, clientName, date, startTime string) error {
	return m.record("owner_new_booking", to, complexName, courtName, clientName, date, startTime)
}
func (m *stubMailer) SendOwnerBookingCancelled(_ context.Context, to, complexName, courtName, clientName, date, startTime string) error {
	return m.record("owner_booking_cancelled", to, complexName, courtName, clientName, date, startTime)
}
func (m *stubMailer) SendOwnerBookingRefunded(_ context.Context, to, complexName, courtName, clientName, date, startTime string) error {
	return m.record("owner_booking_refunded", to, complexName, courtName, clientName, date, startTime)
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
func (m *stubMailer) SendEmailChangeRequested(_ context.Context, to, firstName, newEmail, confirmURL, expiresIn string) error {
	return m.record("email_change_requested", to, firstName, newEmail, confirmURL, expiresIn)
}

func (m *stubMailer) SendSeriesCreated(_ context.Context, to, complexName, courtName, weekdays, hours, from, until, address, mapsURL string, contact mailer.Contact) error {
	return m.record("series_created", to, complexName, courtName, weekdays, hours, from, until, address, mapsURL, contact)
}
func (m *stubMailer) SendSeriesCancelled(_ context.Context, to, complexName, courtName, weekdays, hours, from string, keptPaid bool, bookURL string, contact mailer.Contact) error {
	return m.record("series_cancelled", to, complexName, courtName, weekdays, hours, from, keptPaid, bookURL, contact)
}

func (m *stubMailer) SendOwnerSeriesDatesLost(_ context.Context, to, complexName, courtName, clientName, weekdays, hours string, lines []string) error {
	return m.record("owner_series_dates_lost", to, complexName, courtName, clientName, weekdays, hours, lines)
}

func (m *stubMailer) SendEventRegistered(_ context.Context, to, firstName, complexName, eventName, date, hours, address, mapsURL, amount, eventURL string, contact mailer.Contact) error {
	return m.record("event_registered", to, firstName, complexName, eventName, date, hours, address, mapsURL, amount, eventURL, contact)
}
func (m *stubMailer) SendEventCancelled(_ context.Context, to, firstName, complexName, eventName, date, hours string, byOwner bool, refundLine, refundAmount, eventsURL string, contact mailer.Contact) error {
	return m.record("event_cancelled", to, firstName, complexName, eventName, date, hours, byOwner, refundLine, refundAmount, eventsURL, contact)
}

func (m *stubMailer) SendWaitlistSlotFreed(_ context.Context, to, complexName, date, bookURL string) error {
	return m.record("waitlist_slot_freed", to, complexName, date, bookURL)
}

func (m *stubMailer) SendWaitlistPlaceFreed(_ context.Context, to, complexName, placeLabel, bookURL string) error {
	return m.record("waitlist_place_freed", to, complexName, placeLabel, bookURL)
}

func (m *stubMailer) SendLifecycleNudge(_ context.Context, to, kind, firstName, complexName, ctaURL, unsubscribeURL, optOutURL string) error {
	return m.record("lifecycle_nudge", to, kind, firstName, complexName, ctaURL, unsubscribeURL, optOutURL)
}

func (m *stubMailer) SendWelcome(_ context.Context, to, firstName string) error {
	return m.record("welcome", to, firstName)
}

func (m *stubMailer) SendPasswordChanged(_ context.Context, to, firstName string) error {
	return m.record("password_changed", to, firstName)
}

func (m *stubMailer) SendEmailChanged(_ context.Context, to, firstName, newEmail string) error {
	return m.record("email_changed", to, firstName, newEmail)
}

func (m *stubMailer) SendAccountDeleted(_ context.Context, to, firstName string) error {
	return m.record("account_deleted", to, firstName)
}

func (m *stubMailer) SendOwnerMPConnected(_ context.Context, to, complexName string) error {
	return m.record("owner_mp_connected", to, complexName)
}

func (m *stubMailer) SendOwnerMPDisconnected(_ context.Context, to, complexName string) error {
	return m.record("owner_mp_disconnected", to, complexName)
}

func (m *stubMailer) SendOwnerMPRefreshFailed(_ context.Context, to, complexName string) error {
	return m.record("owner_mp_refresh_failed", to, complexName)
}

func (m *stubMailer) SendOwnerInstagramReconnect(_ context.Context, to, complexName string) error {
	return m.record("owner_instagram_reconnect", to, complexName)
}

func (m *stubMailer) SendBenefitsCampaign(_ context.Context, to, firstName, segment, _, _, _, _ string) error {
	return m.record("benefits_campaign", to, firstName+"|"+segment)
}

func (m *stubMailer) SendSocialKitCampaign(_ context.Context, to, firstName, imageURL, _, _, _ string) error {
	return m.record("social_kit_campaign", to, firstName+"|"+imageURL)
}

type stubUsers struct {
	user *authstore.User
	err  error
}

func (u *stubUsers) GetByID(context.Context, uuid.UUID) (*authstore.User, error) {
	return u.user, u.err
}

// stubBookings answers the reminder worker's read of a booking: booking is
// returned for any id, or err.
type stubBookings struct {
	booking *bookingstore.Booking
	err     error
	reads   int
}

func (b *stubBookings) GetByID(context.Context, uuid.UUID) (*bookingstore.Booking, error) {
	b.reads++
	if b.err != nil {
		return nil, b.err
	}
	if b.booking == nil {
		return nil, data.ErrRecordNotFound
	}
	return b.booking, nil
}

func newTestService(t *testing.T) (*Service, *stubQueue, *stubMailer, *stubUsers) {
	t.Helper()
	q, m := newStubQueue(), &stubMailer{}
	users := &stubUsers{user: &authstore.User{Email: "owner@example.com"}}
	s := NewService(q, m, users, &stubBookings{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	return s, q, m, users
}

func fullConfirmation() BookingConfirmation {
	return BookingConfirmation{
		Source: SourceOnlineCheckout, Email: "ana@example.com",
		ComplexName: "Vibe", CourtName: "Cancha 1", ClientName: "Ana",
		Date: "04/03", StartTime: "18:00 - 19:30",
		Address:          "Av. Santa Fe 1200, Buenos Aires",
		CancelURL:        "https://vibe.test/cancel",
		MapsURL:          "https://www.google.com/maps/search/?api=1&query=Vibe+Palermo",
		DepositAmount:    "$5.000",
		BalanceAmount:    "$15.000",
		CancellationLine: "con devolución hasta 24 horas antes del turno.",
		OwnerID:          uuid.New().String(),
	}
}

func fullCancellation() Cancellation {
	return Cancellation{
		Email:       "ana@example.com",
		ComplexName: "Vibe", CourtName: "Cancha 1",
		Date: "04/03", StartTime: "18:00 - 19:30",
		RefundLine:   "$5.000 enviados, se acreditan en los próximos días hábiles.",
		RefundAmount: "$5.000",
		BookURL:      "https://vibe.test/vibe/book",
	}
}

func fullRefund() Refund {
	return Refund{
		Email:       "ana@example.com",
		ComplexName: "Vibe", Amount: "$5.000",
		BookURL: "https://vibe.test/vibe/book",
	}
}

// A booking refunded in two parts gets two notices. The deduplication key used to
// be the booking alone, so the queue treated the second part as a redelivery of the
// first and the client was never told about the second refund.
func TestEachPartialRefundOfABookingHasItsOwnNoticeKey(t *testing.T) {
	s, q, _, _ := newTestService(t)
	bookingID := uuid.New().String()

	first := fullRefund()
	first.BookingID, first.RefundID = bookingID, "mp-1:1000"
	second := fullRefund()
	second.BookingID, second.RefundID = bookingID, "mp-1:2000"

	s.DepositRefunded(first)
	s.DepositRefunded(second)

	var keys []string
	for _, task := range q.tasks {
		if task.taskType == TaskEmailDepositRefunded {
			keys = append(keys, task.dedupKey)
		}
	}
	if len(keys) != 2 {
		t.Fatalf("each refund must be enqueued; got %d deposit-refunded emails", len(keys))
	}
	if keys[0] == "" || keys[1] == "" {
		t.Fatalf("a refund notice with no deduplication key is sent twice on a redelivery; got %q and %q", keys[0], keys[1])
	}
	if keys[0] == keys[1] {
		t.Errorf("two partial refunds of one booking share the deduplication key %q, so the second client email is dropped", keys[0])
	}
}

func fullReminder() Reminder {
	return Reminder{
		Email:       "ana@example.com",
		ComplexName: "Vibe", CourtName: "Cancha 1",
		Date: "04/03", StartTime: "18:00 - 19:30",
		Address: "Av. Santa Fe 1200, Buenos Aires", BalanceAmount: "$15.000",
		MapsURL:   "https://www.google.com/maps/search/?api=1&query=Vibe+Palermo",
		CancelURL: "https://vibe.test/vibe/book/cancel?token=abc",
	}
}

// A client who gave no email gets no confirmation, and the owner is told
// regardless: the two recipients are independent.
func TestBookingConfirmedSkipsAClientWithNoEmail(t *testing.T) {
	s, q, _, _ := newTestService(t)

	c := fullConfirmation()
	c.Email = ""
	s.BookingConfirmed(c)

	if q.has(TaskEmailBookingConfirmation) {
		t.Error("a client email was enqueued with no address")
	}
	if !q.has(TaskEmailOwnerNewBooking) {
		t.Error("the owner must be told regardless of the client's email")
	}
}

// The cron flags a booking only when its reminder is queued, so ReminderDue
// must report a refusal from the queue, and the booking stays unflagged.
func TestReminderReturnsTheRefusalFromTheQueue(t *testing.T) {
	s, q, _, _ := newTestService(t)
	refusal := errors.New("database is down")
	q.failOn = map[string]error{TaskEmailReminder2h: refusal}

	err := s.ReminderDue(fullReminder())

	if !errors.Is(err, refusal) {
		t.Fatalf("a refused reminder must fail so the booking stays unflagged; got %v", err)
	}
}

// The owner is told about a sale, not about their own data entry. An owner who
// books a walk-in from the dashboard used to receive an email announcing the
// booking they had just made themselves, every single time.
func TestOwnerIsEmailedOnlyForBookingsTheyDidNotEnter(t *testing.T) {
	t.Run("staff created it from the dashboard", func(t *testing.T) {
		s, q, _, _ := newTestService(t)

		c := fullConfirmation()
		c.Source = SourceStaffCreate
		s.BookingConfirmed(c)

		if q.has(TaskEmailOwnerNewBooking) {
			t.Errorf("the owner was emailed about the booking they just made; got %v", q.types())
		}
		if !q.has(TaskEmailBookingConfirmation) {
			t.Errorf("the client must still be notified by email; got %v", q.types())
		}
	})
}

// A client's own cancellation frees the slot, and the owner is the only one who
// can resell it, so they are told. The owner's panel cancel is their own action,
// and an expired unpaid booking was never announced to them.
func TestClientCancellationEmailsTheOwner(t *testing.T) {
	s, q, _, _ := newTestService(t)

	c := fullCancellation()
	c.Source = SourceClientCancel
	c.OwnerID = uuid.New().String()
	c.ClientName = "Ana Pérez"
	c.BookingID = uuid.New().String()
	s.BookingCancelled(c)

	if !q.has(TaskEmailOwnerBookingCancelled) {
		t.Fatalf("the owner must hear about a client cancellation; got %v", q.types())
	}
	key, _ := q.keyOf(TaskEmailOwnerBookingCancelled)
	if key == "" {
		t.Error("the owner cancellation was enqueued with no deduplication key")
	}
	for _, task := range q.tasks {
		if task.taskType != TaskEmailOwnerBookingCancelled {
			continue
		}
		p, ok := task.payload.(ownerBookingCancelled)
		if !ok {
			t.Fatalf("the owner cancellation payload has type %T", task.payload)
		}
		if p.OwnerID != c.OwnerID || p.ClientName != "Ana Pérez" || p.CourtName != "Cancha 1" ||
			p.ComplexName != "Vibe" || p.Date != "04/03" || p.StartTime != "18:00 - 19:30" {
			t.Errorf("the owner cancellation carries the wrong booking: %+v", p)
		}
	}
}

func TestOnlyAClientCancellationEmailsTheOwner(t *testing.T) {
	for _, source := range []string{SourceOwnerCancel, SourceExpiredUnpaid, ""} {
		t.Run("source "+source, func(t *testing.T) {
			s, q, _, _ := newTestService(t)

			c := fullCancellation()
			c.Source = source
			c.OwnerID = uuid.New().String()
			s.BookingCancelled(c)

			if q.has(TaskEmailOwnerBookingCancelled) {
				t.Errorf("the owner was emailed for a cancellation from %q; got %v", source, q.types())
			}
			if !q.has(TaskEmailBookingCancelled) {
				t.Errorf("the client must still be told; got %v", q.types())
			}
		})
	}
}

func TestClientCancellationNeedsAnOwnerToEmail(t *testing.T) {
	s, q, _, _ := newTestService(t)

	c := fullCancellation()
	c.Source = SourceClientCancel
	s.BookingCancelled(c)

	if q.has(TaskEmailOwnerBookingCancelled) {
		t.Errorf("an owner cancellation was enqueued with no owner to address; got %v", q.types())
	}
}

func TestOwnerIsEmailedAboutAClientCancellation(t *testing.T) {
	s, q, mailer, users := newTestService(t)
	s.RegisterWorkers()

	users.user = &authstore.User{Email: "owner@example.com"}
	payload, _ := json.Marshal(ownerBookingCancelled{
		OwnerID: uuid.New().String(), ComplexName: "Vibe", CourtName: "Cancha 1",
		ClientName: "Ana Pérez", Date: "04/03", StartTime: "18:00 - 19:30",
	})

	if err := q.handlers[TaskEmailOwnerBookingCancelled](t.Context(), payload); err != nil {
		t.Fatalf("delivering the owner cancellation email: %v", err)
	}
	want := []any{"Vibe", "Cancha 1", "Ana Pérez", "04/03", "18:00 - 19:30"}
	if len(mailer.sent) != 1 || mailer.sent[0].kind != "owner_booking_cancelled" ||
		mailer.sent[0].to != "owner@example.com" {
		t.Fatalf("the owner cancellation was not delivered to the owner; got %v", mailer.sent)
	}
	for i, w := range want {
		if mailer.sent[0].args[i] != w {
			t.Errorf("argument %d: want %v; got %v", i, w, mailer.sent[0].args[i])
		}
	}
}

func TestNothingIsEnqueuedWithoutARecipient(t *testing.T) {
	s, q, _, _ := newTestService(t)

	s.BookingCancelled(Cancellation{})
	s.DepositRefunded(Refund{})
	s.ReminderDue(Reminder{})

	if len(q.tasks) != 0 {
		t.Errorf("want nothing enqueued with no recipient; got %v", q.types())
	}
}

func TestWorkersDeliverEachTaskType(t *testing.T) {
	s, q, mailer, _ := newTestService(t)
	s.RegisterWorkers()

	// Every task the service can enqueue must have a worker registered, or it
	// sits in Redis forever.
	for _, taskType := range []string{
		TaskEmailBookingConfirmation, TaskEmailReminder2h, TaskEmailBookingCancelled,
		TaskEmailDepositRefunded, TaskEmailOwnerNewBooking, TaskEmailOwnerBookingCancelled, TaskEmailOwnerBookingRefunded, TaskEmailVerification,
		TaskEmailPasswordReset, TaskEmailDuplicateRegistration, TaskEmailChangeRequested,
		TaskEmailLifecycleNudge, TaskEmailWelcome,
		TaskEmailPasswordChanged, TaskEmailEmailChanged, TaskEmailAccountDeleted,
		TaskEmailOwnerMPConnected, TaskEmailOwnerMPDisconnected, TaskEmailOwnerMPRefreshFailed, TaskEmailOwnerInstagramReconnect,
	} {
		if _, ok := q.handlers[taskType]; !ok {
			t.Errorf("no worker registered for %s", taskType)
		}
	}

	// Drive one email worker end to end.
	payload, _ := json.Marshal(Cancellation{Email: "ana@example.com", ComplexName: "Vibe"})
	if err := q.handlers[TaskEmailBookingCancelled](t.Context(), payload); err != nil {
		t.Fatalf("delivering a cancellation: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].kind != "cancelled" {
		t.Errorf("the cancellation was not delivered; got %v", mailer.sent)
	}

}

// The cancellation and refund email payloads are durable storage too, so they
// carry what their worker reads and nothing more. Enqueueing the whole
// Cancellation would write fields no worker reads into Redis on every cancellation.
func TestCancellationEmailPayloadCarriesOnlyWhatTheEmailNeeds(t *testing.T) {
	s, q, _, _ := newTestService(t)

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
	s, q, mailer, users := newTestService(t)
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
	s, q, _, users := newTestService(t)
	s.RegisterWorkers()

	users.err = errors.New("no such user")
	payload, _ := json.Marshal(ownerNewBooking{OwnerID: uuid.New().String()})

	if err := q.handlers[TaskEmailOwnerNewBooking](t.Context(), payload); err == nil {
		t.Error("want an error so the queue can retry; got nil")
	}
}

// A malformed payload must surface as an error naming the task, not a panic.
func TestWorkersRejectMalformedPayloads(t *testing.T) {
	s, q, _, _ := newTestService(t)
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
	s, q, _, _ := newTestService(t)

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

	// owner_id is read now: the worker resolves it to the reply-to address.
	for _, unwanted := range []string{"phone", "cancel_path", "maps_query", "client_name"} {
		if _, present := payload[unwanted]; present {
			t.Errorf("the email payload persists %q, which the email worker never reads", unwanted)
		}
	}
	for _, required := range []string{"email", "complex_name", "court_name", "date", "start_time", "address", "maps_url", "cancel_url", "deposit_amount", "balance_amount", "cancellation_line", "owner_id"} {
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
		{
			name: "email change requested",
			send: func(s *Service) {
				s.EmailChangeRequested(EmailChangeRequestedEmail{
					To: "ana@example.com", FirstName: "Ana",
					NewEmail: "ana-new@example.com", ConfirmURL: "https://vibe.test/confirm-email-change?t=3",
				})
			},
			taskType: TaskEmailChangeRequested,
			wantKey:  "confirm_url",
			wantURL:  "https://vibe.test/confirm-email-change?t=3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, q, _, _ := newTestService(t)
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
	s, q, mailer, _ := newTestService(t)
	s.RegisterWorkers()

	cases := []struct {
		taskType string
		payload  any
		wantKind string
	}{
		{TaskEmailVerification, VerificationEmail{To: "ana@example.com", FirstName: "Ana", VerifyURL: "https://v"}, "verification"},
		{TaskEmailPasswordReset, PasswordResetEmail{To: "ana@example.com", FirstName: "Ana", ResetURL: "https://r"}, "password_reset"},
		{TaskEmailDuplicateRegistration, DuplicateRegistrationEmail{To: "ana@example.com", FirstName: "Ana", LoginURL: "https://l", ResetURL: "https://r"}, "duplicate_registration"},
		{
			TaskEmailChangeRequested,
			EmailChangeRequestedEmail{
				To: "ana@example.com", FirstName: "Ana", NewEmail: "ana-new@example.com",
				ConfirmURL: "https://c", ExpiresIn: "1 hora",
			},
			"email_change_requested",
		},
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
	s, q, mailer, _ := newTestService(t)
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

	s, q, _, _ := newTestService(t)
	confirmation := fullConfirmation()
	confirmation.BookingID = bookingID
	s.BookingConfirmed(confirmation)

	cancellation := fullCancellation()
	cancellation.BookingID = bookingID
	s.BookingCancelled(cancellation)

	refund := fullRefund()
	refund.BookingID = bookingID
	refund.RefundID = "mp-123:500000"
	s.DepositRefunded(refund)

	reminder := fullReminder()
	reminder.BookingID = bookingID
	s.ReminderDue(reminder)

	s.EmailVerification(VerificationEmail{To: "ana@example.com", VerifyURL: "https://vibe.test/v/tok"})
	s.PasswordReset(PasswordResetEmail{To: "ana@example.com", ResetURL: "https://vibe.test/r/tok"})
	s.DuplicateRegistration(DuplicateRegistrationEmail{To: "ana@example.com", ResetURL: "https://vibe.test/r/tok"})
	s.EmailChangeRequested(EmailChangeRequestedEmail{To: "ana@example.com", NewEmail: "ana-new@example.com", ConfirmURL: "https://vibe.test/confirm-email-change?t=tok"})

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
		s := NewService(q, &stubMailer{},
			&stubUsers{user: &authstore.User{Email: "owner@example.com"}}, &stubBookings{},
			slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
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
		s := NewService(q, &stubMailer{},
			&stubUsers{user: &authstore.User{Email: "owner@example.com"}}, &stubBookings{},
			slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

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

func TestLifecycleNudgeIsDeduplicatedPerKindAndSubjectAndDelivered(t *testing.T) {
	s, q, mailer, _ := newTestService(t)
	s.RegisterWorkers()

	e := LifecycleNudgeEmail{
		Kind: "create_complex", Email: "ana@example.com", FirstName: "Ana",
		CTAURL: "https://app.vibe.test/onboarding", UnsubscribeURL: "https://api.vibe.test/u?token=t",
		OptOutURL: "https://vibe.test/baja?token=t",
	}
	s.LifecycleNudge(e, "owner-1")
	first, ok := q.keyOf(TaskEmailLifecycleNudge)
	if !ok || first == "" {
		t.Fatalf("the nudge was not enqueued with a deduplication key (ok=%v key=%q)", ok, first)
	}

	s.LifecycleNudge(e, "owner-1")
	s.LifecycleNudge(e, "owner-2")
	keys := map[string]int{}
	for _, task := range q.tasks {
		keys[task.dedupKey]++
	}
	if len(keys) != 2 {
		t.Fatalf("keys = %v; the same owner and kind must share one key and another owner must not", keys)
	}

	payload, _ := json.Marshal(e)
	if err := q.handlers[TaskEmailLifecycleNudge](t.Context(), payload); err != nil {
		t.Fatalf("delivering the nudge: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].kind != "lifecycle_nudge" || mailer.sent[0].to != "ana@example.com" {
		t.Fatalf("delivered %+v, want one lifecycle_nudge to the recipient", mailer.sent)
	}
	if got := mailer.sent[0].args[len(mailer.sent[0].args)-1]; got != "https://vibe.test/baja?token=t" {
		t.Fatalf("the opt-out link did not reach the mailer; got %v", got)
	}
}
