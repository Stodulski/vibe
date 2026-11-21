package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stodulski/vibe-server/internal/mailer"
)

func (m *stubMailer) SendOwnerEventRegistered(_ context.Context, to string, n mailer.OwnerEventNotice) error {
	return m.record("owner_event_registered", to, n)
}
func (m *stubMailer) SendOwnerEventCancelled(_ context.Context, to string, n mailer.OwnerEventNotice) error {
	return m.record("owner_event_cancelled", to, n)
}
func (m *stubMailer) SendEventRefunded(_ context.Context, to, firstName, complexName, eventName, date, amount, eventURL string, contact mailer.Contact) error {
	return m.record("event_refunded", to, firstName, complexName, eventName, date, amount, eventURL, contact)
}
func (m *stubMailer) SendEventReminder(_ context.Context, to, firstName, complexName, eventName, when, address, mapsURL, eventURL string, contact mailer.Contact) error {
	return m.record("event_reminder", to, firstName, complexName, eventName, when, address, mapsURL, eventURL, contact)
}
func (m *stubMailer) SendTournamentNotice(_ context.Context, to string, n mailer.TournamentNotice, contact mailer.Contact) error {
	return m.record("tournament_notice", to, n, contact)
}

func TestOwnerEventNoticesGoToTheOwnerOncePerRegistration(t *testing.T) {
	s, q, _, _ := newTestService(t)
	n := OwnerEventNotice{OwnerID: "owner-1", EventName: "Canchas abiertas", RegistrationID: "reg-1"}

	s.OwnerEventRegistered(n)
	s.OwnerEventCancelled(n)
	s.OwnerEventRegistered(OwnerEventNotice{RegistrationID: "reg-1"})
	if len(q.tasks) != 2 {
		t.Fatalf("tasks = %v, want one registered and one cancelled; no owner, no notice", q.types())
	}
	regKey, _ := q.keyOf(TaskEmailOwnerEventRegistered)
	cancelKey, _ := q.keyOf(TaskEmailOwnerEventCancelled)
	if regKey == "" || cancelKey == "" || regKey == cancelKey {
		t.Fatalf("keys = %q, %q: each notice must be deduplicated on its own", regKey, cancelKey)
	}

	q.tasks = nil
	s.OwnerEventRegistered(OwnerEventNotice{OwnerID: "owner-1", RegistrationID: "reg-2"})
	if other, _ := q.keyOf(TaskEmailOwnerEventRegistered); other == regKey {
		t.Error("another registration must not share the first one's key")
	}
}

func TestOwnerEventWorkerWritesToTheOwnersCurrentAddress(t *testing.T) {
	s, q, m, users := newTestService(t)
	s.RegisterWorkers()
	users.user.Email = "new-owner@example.com"
	payload, _ := json.Marshal(ownerEventEmail{OwnerID: "0193a0a0-0000-7000-8000-000000000001", ParticipantName: "Ana Pérez"})
	for _, task := range []string{TaskEmailOwnerEventRegistered, TaskEmailOwnerEventCancelled} {
		if err := q.handlers[task](t.Context(), payload); err != nil {
			t.Fatalf("%s: %v", task, err)
		}
	}
	if len(m.sent) != 2 || m.sent[0].to != "new-owner@example.com" || m.sent[0].kind != "owner_event_registered" || m.sent[1].kind != "owner_event_cancelled" {
		t.Errorf("sent = %+v", m.sent)
	}
}

func TestEventRefundedIsOneNoticePerRefund(t *testing.T) {
	s, q, _, _ := newTestService(t)
	s.EventRefunded(EventRefund{Email: "ana@example.com", Amount: "$ 2.000", RegistrationID: "reg-1", RefundedTotal: 200000})
	first, _ := q.keyOf(TaskEmailEventRefunded)
	s.EventRefunded(EventRefund{Email: "ana@example.com", Amount: "$ 2.000", RegistrationID: "reg-1", RefundedTotal: 200000})
	if q.tasks[1].dedupKey != first || first == "" {
		t.Fatalf("a redelivered refund must carry the same key; got %q then %q", first, q.tasks[1].dedupKey)
	}
	s.EventRefunded(EventRefund{Email: "ana@example.com", Amount: "$ 1.000", RegistrationID: "reg-1", RefundedTotal: 300000})
	if q.tasks[2].dedupKey == first {
		t.Error("a second refund of the same registration must not be swallowed by the first one's key")
	}
	s.EventRefunded(EventRefund{Amount: "$ 1.000", RegistrationID: "reg-1"})
	if len(q.tasks) != 3 {
		t.Error("no address, nothing to send")
	}
}

// Two different extra payments of the same amount on one sign-up are two
// refunds: each payment's refund is its own notice.
func TestEventRefundedTellsEachPaymentsRefundApart(t *testing.T) {
	s, q, _, _ := newTestService(t)
	s.EventRefunded(EventRefund{Email: "ana@example.com", Amount: "$ 8.200", RegistrationID: "reg-1", MPPaymentID: "mp-1", RefundedTotal: 820000})
	s.EventRefunded(EventRefund{Email: "ana@example.com", Amount: "$ 8.200", RegistrationID: "reg-1", MPPaymentID: "mp-2", RefundedTotal: 820000})
	if len(q.tasks) != 2 || q.tasks[0].dedupKey == q.tasks[1].dedupKey {
		t.Fatalf("two payments refunded must be two notices; got %+v", q.tasks)
	}
	s.EventRefunded(EventRefund{Email: "ana@example.com", Amount: "$ 8.200", RegistrationID: "reg-1", MPPaymentID: "mp-1", RefundedTotal: 820000})
	if q.tasks[2].dedupKey != q.tasks[0].dedupKey {
		t.Error("the same payment's refund told again must carry the same key")
	}
}

func TestEventReminderReportsTheQueuesRefusal(t *testing.T) {
	s, q, _, _ := newTestService(t)
	if err := s.EventReminderDue(EventReminder{RegistrationID: "reg-1"}); err != nil || len(q.tasks) != 0 {
		t.Fatalf("no address: err=%v tasks=%v", err, q.types())
	}
	q.failOn = map[string]error{TaskEmailEventReminder: errors.New("db down")}
	if err := s.EventReminderDue(EventReminder{Email: "ana@example.com", RegistrationID: "reg-1"}); err == nil {
		t.Fatal("a refused reminder must be reported, so the sweep tries again")
	}
	q.failOn = nil
	if err := s.EventReminderDue(EventReminder{Email: "ana@example.com", RegistrationID: "reg-1"}); err != nil {
		t.Fatal(err)
	}
	if key, _ := q.keyOf(TaskEmailEventReminder); key == "" {
		t.Error("the reminder must be deduplicated")
	}
}

func TestTournamentNoticeIsDeduplicatedOnItsSubject(t *testing.T) {
	s, q, _, _ := newTestService(t)
	n := TournamentNotice{Email: "ana@example.com", Subject: "match-1:2026-11-22T19:00"}
	n.Kind = mailer.TournamentMatchScheduled
	if err := s.TournamentNoticeDue(n); err != nil {
		t.Fatal(err)
	}
	n.Subject = "match-1:2026-11-22T21:00"
	if err := s.TournamentNoticeDue(n); err != nil {
		t.Fatal(err)
	}
	if len(q.tasks) != 2 || q.tasks[0].dedupKey == "" || q.tasks[0].dedupKey == q.tasks[1].dedupKey {
		t.Fatalf("a moved match must be a notice of its own; tasks = %+v", q.tasks)
	}
}

func TestEventNoticeWorkersAreRegistered(t *testing.T) {
	s, q, m, _ := newTestService(t)
	s.RegisterWorkers()
	cases := map[string]string{
		TaskEmailEventRefunded:    `{"email":"ana@example.com","amount":"$ 2.000"}`,
		TaskEmailEventReminder:    `{"email":"ana@example.com","when":"mañana a las 19:00"}`,
		TaskEmailTournamentNotice: `{"email":"ana@example.com","kind":"match_reminder","court":"Cancha 1"}`,
	}
	for task, raw := range cases {
		h, ok := q.handlers[task]
		if !ok {
			t.Fatalf("no worker registered for %s", task)
		}
		if err := h(t.Context(), []byte(raw)); err != nil {
			t.Fatalf("%s: %v", task, err)
		}
	}
	if len(m.sent) != 3 {
		t.Fatalf("sent = %+v", m.sent)
	}
	for _, s := range m.sent {
		if s.kind == "tournament_notice" && s.args[0].(mailer.TournamentNotice).Court != "Cancha 1" {
			t.Errorf("the tournament notice lost its court: %+v", s.args[0])
		}
	}
}
