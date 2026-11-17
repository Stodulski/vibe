package notifications

import (
	"slices"
	"testing"
)

func TestEventNoticesAreEmailOnlyAndNeedAnAddress(t *testing.T) {
	s, q, _, _ := newTestService(t)

	s.EventRegistered(EventRegistration{FirstName: "Ana", EventName: "Canchas abiertas", RegistrationID: "reg-1"})
	s.EventCancelled(EventCancellation{FirstName: "Ana", EventName: "Canchas abiertas", RegistrationID: "reg-1"})
	if len(q.tasks) != 0 {
		t.Fatalf("no email address, nothing to send; got %v", q.types())
	}

	s.EventRegistered(EventRegistration{Email: "ana@example.com", RegistrationID: "reg-1"})
	s.EventCancelled(EventCancellation{Email: "ana@example.com", ByOwner: true, RegistrationID: "reg-1"})
	if got := q.types(); !slices.Equal(got, []string{TaskEmailEventRegistered, TaskEmailEventCancelled}) {
		t.Fatalf("tasks = %v, want the two emails", got)
	}
	for _, task := range []string{TaskEmailEventRegistered, TaskEmailEventCancelled} {
		if key, _ := q.keyOf(task); key == "" {
			t.Errorf("%s must be deduplicated", task)
		}
	}
}

func TestEventCancelledKeyDistinguishesTheRefundThatFollowsTheNotice(t *testing.T) {
	s, q, _, _ := newTestService(t)
	s.EventCancelled(EventCancellation{Email: "ana@example.com", RegistrationID: "reg-1"})
	first, _ := q.keyOf(TaskEmailEventCancelled)
	q.tasks = nil
	s.EventCancelled(EventCancellation{Email: "ana@example.com", RegistrationID: "reg-1", RefundAmount: "$ 8.000"})
	second, _ := q.keyOf(TaskEmailEventCancelled)
	if first == second {
		t.Error("the refund notice must not be swallowed by the cancellation notice's key")
	}
}

func TestEventWorkersDeliverTheirEmails(t *testing.T) {
	s, q, mailer, _ := newTestService(t)
	s.RegisterWorkers()
	for _, task := range []string{TaskEmailEventRegistered, TaskEmailEventCancelled} {
		if _, ok := q.handlers[task]; !ok {
			t.Errorf("no worker registered for %s", task)
		}
	}
	if err := q.handlers[TaskEmailEventRegistered](t.Context(), []byte(`{"email":"ana@example.com","event_name":"Canchas abiertas"}`)); err != nil {
		t.Fatal(err)
	}
	if err := q.handlers[TaskEmailEventCancelled](t.Context(), []byte(`{"email":"ana@example.com","by_owner":true}`)); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 2 || mailer.sent[0].kind != "event_registered" || mailer.sent[1].kind != "event_cancelled" {
		t.Errorf("sent = %v", mailer.sent)
	}
}
