package notifications

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
)

// A refund MercadoPago issues on its own cancels a booking Vibe did not cancel.
// The owner hears about it with their own copy, enqueued once per booking and
// owner, and nothing else is sent for it through this method.
func TestAMercadoPagoRefundEmailsTheOwnerOnce(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)

	ownerID := uuid.New().String()
	bookingID := uuid.New().String()
	s.OwnerBookingRefunded(OwnerBookingRefund{
		OwnerID: ownerID, ComplexName: "Vibe", CourtName: "Cancha 1", ClientName: "Ana Pérez",
		Date: "04/03", StartTime: "18:00 - 19:30", BookingID: bookingID,
	})

	if !q.has(TaskEmailOwnerBookingRefunded) {
		t.Fatalf("the owner must hear about a booking a refund cancelled; got %v", q.types())
	}
	if len(q.tasks) != 1 {
		t.Errorf("the refund notice goes to the owner alone; got %v", q.types())
	}
	key, _ := q.keyOf(TaskEmailOwnerBookingRefunded)
	if key != dedup(TaskEmailOwnerBookingRefunded, ownerID, bookingID) {
		t.Errorf("the owner refund notice must carry the owner and booking deduplication key; got %q", key)
	}

	p, ok := q.tasks[0].payload.(ownerBookingRefunded)
	if !ok {
		t.Fatalf("the owner refund payload has type %T", q.tasks[0].payload)
	}
	if p.OwnerID != ownerID || p.ClientName != "Ana Pérez" || p.CourtName != "Cancha 1" ||
		p.ComplexName != "Vibe" || p.Date != "04/03" || p.StartTime != "18:00 - 19:30" {
		t.Errorf("the owner refund notice carries the wrong booking: %+v", p)
	}
}

func TestAMercadoPagoRefundNeedsAnOwnerToEmail(t *testing.T) {
	s, q, _, _, _ := newTestService(t, true)

	s.OwnerBookingRefunded(OwnerBookingRefund{CourtName: "Cancha 1", BookingID: uuid.New().String()})

	if q.has(TaskEmailOwnerBookingRefunded) {
		t.Errorf("an owner refund notice was enqueued with no owner to address; got %v", q.types())
	}
}

func TestOwnerIsEmailedAboutAMercadoPagoRefund(t *testing.T) {
	s, q, mailer, _, users := newTestService(t, true)
	s.RegisterWorkers()

	users.user = &authstore.User{Email: "owner@example.com"}
	payload, _ := json.Marshal(ownerBookingRefunded{
		OwnerID: uuid.New().String(), ComplexName: "Vibe", CourtName: "Cancha 1",
		ClientName: "Ana Pérez", Date: "04/03", StartTime: "18:00 - 19:30",
	})

	if err := q.handlers[TaskEmailOwnerBookingRefunded](t.Context(), payload); err != nil {
		t.Fatalf("delivering the owner refund email: %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].kind != "owner_booking_refunded" ||
		mailer.sent[0].to != "owner@example.com" {
		t.Fatalf("the owner refund email was not delivered to the owner; got %v", mailer.sent)
	}
	want := []any{"Vibe", "Cancha 1", "Ana Pérez", "04/03", "18:00 - 19:30"}
	for i, w := range want {
		if mailer.sent[0].args[i] != w {
			t.Errorf("argument %d: want %v; got %v", i, w, mailer.sent[0].args[i])
		}
	}
}
