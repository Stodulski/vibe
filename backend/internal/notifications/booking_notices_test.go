package notifications

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// Every booking notice is keyed on the booking and the event it is about, so a
// redelivery asks the queue for the same key (and is dropped) while a second
// event of the same booking (another counter payment) is a new notice.
func TestBookingNoticesAreKeyedOnTheBookingAndTheEvent(t *testing.T) {
	bookingID := uuid.New().String()
	owner := uuid.New().String()

	enqueueAll := func(paymentID string) *stubQueue {
		s, q, _, _ := newTestService(t)
		s.BookingNoShow(NoShow{Email: "ana@example.com", BookingID: bookingID, OwnerID: owner})
		s.PaymentReceived(PaymentReceipt{Email: "ana@example.com", BookingID: bookingID, PaymentID: paymentID, Amount: "$5.000"})
		s.RefundOwedByVenue(RefundOwed{Email: "ana@example.com", OwnerID: owner, BookingID: bookingID, Subject: "attempt-1", Amount: "$5.000"})
		s.OwnerDuplicatePaymentRefunded(OwnerDuplicatePaymentRefund{OwnerID: owner, BookingID: bookingID, RefundID: "mp-2:500000"})
		return q
	}

	first, again := enqueueAll("pay-1"), enqueueAll("pay-1")
	want := []string{
		TaskEmailBookingNoShow, TaskEmailPaymentReceived,
		TaskEmailRefundOwedByVenue, TaskEmailOwnerRefundOwed, TaskEmailOwnerDuplicatePaymentRefunded,
	}
	if got := first.types(); len(got) != len(want) {
		t.Fatalf("want %v enqueued; got %v", want, got)
	}
	for i, task := range first.tasks {
		if task.taskType != want[i] {
			t.Errorf("task %d: want %s; got %s", i, want[i], task.taskType)
		}
		if task.dedupKey == "" {
			t.Errorf("%s has no deduplication key, so a redelivery sends it twice", task.taskType)
		}
		if again.tasks[i].dedupKey != task.dedupKey {
			t.Errorf("%s: the same notice asked for two keys (%q, %q)", task.taskType, task.dedupKey, again.tasks[i].dedupKey)
		}
	}

	second := enqueueAll("pay-2")
	firstKey, _ := first.keyOf(TaskEmailPaymentReceived)
	secondKey, _ := second.keyOf(TaskEmailPaymentReceived)
	if firstKey == secondKey {
		t.Error("two counter payments of one booking share a receipt key, so the second receipt is dropped")
	}
}

// A client with no address is skipped; the owner, who owes the money, is told
// either way.
func TestRefundOwedTellsTheOwnerWhenTheClientHasNoAddress(t *testing.T) {
	s, q, _, _ := newTestService(t)
	s.RefundOwedByVenue(RefundOwed{OwnerID: uuid.New().String(), BookingID: uuid.New().String(), Subject: "a", Amount: "$5.000"})
	if got := q.types(); len(got) != 1 || got[0] != TaskEmailOwnerRefundOwed {
		t.Fatalf("want only the owner's notice; got %v", got)
	}
}

// A notice with no client address enqueues nothing for the client.
func TestClientBookingNoticesNeedAnAddress(t *testing.T) {
	s, q, _, _ := newTestService(t)
	s.BookingNoShow(NoShow{BookingID: uuid.New().String()})
	s.PaymentReceived(PaymentReceipt{BookingID: uuid.New().String(), PaymentID: "p"})
	s.OwnerDuplicatePaymentRefunded(OwnerDuplicatePaymentRefund{BookingID: "b", RefundID: "r"})
	if len(q.tasks) != 0 {
		t.Fatalf("nothing has a recipient, nothing may be enqueued; got %v", q.types())
	}
}

// Each new task type is delivered by its worker to the right template, with
// the owner's address resolved at delivery for the owner's notices.
func TestBookingNoticeWorkersDeliverTheirEmails(t *testing.T) {
	for _, c := range []struct {
		task, kind, to string
		payload        any
	}{
		{TaskEmailBookingNoShow, "booking_no_show", "ana@example.com", bookingNoShowEmail{Email: "ana@example.com"}},
		{TaskEmailPaymentReceived, "payment_received", "ana@example.com", paymentReceivedEmail{Email: "ana@example.com", Amount: "$5.000"}},
		{TaskEmailRefundOwedByVenue, "refund_owed_by_venue", "ana@example.com", refundOwedByVenueEmail{Email: "ana@example.com", Amount: "$5.000"}},
		{TaskEmailOwnerRefundOwed, "owner_refund_owed", "owner@example.com", ownerBookingMoney{OwnerID: uuid.New().String(), Amount: "$5.000"}},
		{TaskEmailOwnerDuplicatePaymentRefunded, "owner_duplicate_payment_refunded", "owner@example.com", ownerBookingMoney{OwnerID: uuid.New().String(), Amount: "$5.000"}},
	} {
		t.Run(c.task, func(t *testing.T) {
			s, q, m, _ := newTestService(t)
			s.RegisterWorkers()
			raw, _ := json.Marshal(c.payload)
			h, ok := q.handlers[c.task]
			if !ok {
				t.Fatalf("no worker registered for %s", c.task)
			}
			if err := h(t.Context(), raw); err != nil {
				t.Fatalf("deliver: %v", err)
			}
			if len(m.sent) != 1 || m.sent[0].kind != c.kind || m.sent[0].to != c.to {
				t.Fatalf("want one %s to %s; got %+v", c.kind, c.to, m.sent)
			}
		})
	}
}

// A refund's reason and the by-hand flag reach the email.
func TestARefundsReasonAndByHandReachTheEmail(t *testing.T) {
	s, q, m, _ := newTestService(t)
	s.RegisterWorkers()
	ref := fullRefund()
	ref.BookingID, ref.RefundID = uuid.New().String(), "manual"
	ref.Reason, ref.ByHand = "El complejo te lo devolvió directamente.", true
	if err := s.DepositRefunded(ref); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(q.tasks[0].payload)
	if err := q.handlers[TaskEmailDepositRefunded](t.Context(), raw); err != nil {
		t.Fatal(err)
	}
	args := m.sent[0].args
	// complexName, amount, bookURL, cancelled, courtName, date, startTime, reason, byHand, contact
	if args[7] != ref.Reason || args[8] != true {
		t.Errorf("want the reason and by-hand flag delivered; got reason=%v byHand=%v", args[7], args[8])
	}
}

// The series email lists the court of each moved date.
func TestSeriesCreatedCarriesTheMovedDates(t *testing.T) {
	s, q, m, _ := newTestService(t)
	s.RegisterWorkers()
	s.SeriesCreated(SeriesCreation{
		Email: "ana@example.com", CourtName: "Cancha 1", SeriesID: uuid.New().String(),
		Moved: []MovedDate{{Date: "23/11", CourtName: "Cancha 3"}},
	})
	raw, _ := json.Marshal(q.tasks[0].payload)
	if err := q.handlers[TaskEmailSeriesCreated](t.Context(), raw); err != nil {
		t.Fatal(err)
	}
	// complexName, courtName, weekdays, hours, from, until, address, mapsURL, moved, contact
	moved, _ := m.sent[0].args[8].([]string)
	if len(moved) != 1 || moved[0] != "23/11: Cancha 3" {
		t.Errorf("want the moved date and its court delivered; got %v", m.sent[0].args[8])
	}
}
