package payments

import (
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// refundFixture returns a fixture where a booking in the given status has a
// paid payment, and its complex, court and client are all readable, so a
// refund that cancels the booking can name everything it announces.
func refundFixture(t *testing.T, status string) (*fixture, *bookingstore.Booking, *paymentstore.Payment, uuid.UUID) {
	t.Helper()

	f := newFixture(t)
	complexID := uuid.New()
	ownerID := uuid.New()
	booking, payment := paidBooking(complexID)
	booking.Status = bookingstore.BookingStatus(status)
	f.bookings.booking = booking
	f.payments.byBooking = payment

	complex := linkedComplex(complexID, "")
	complex.OwnerID = ownerID
	complex.Name = "Vibe Palermo"
	complex.Slug = "vibe-palermo"
	f.complexes.complex = complex

	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Cancha 1"}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", LastName: "Pérez"}
	return f, booking, payment, ownerID
}

// A refund MercadoPago issues on its own side, for the whole payment, is the
// one refund that cancels a live booking and frees a slot nobody here decided
// on. The owner is told with their own copy, and the waitlist hears the day.
// Full refunds and chargebacks both take this path.
func TestAMercadoPagoRefundThatCancelsALiveBookingEmailsTheOwnerAndFreesTheSlot(t *testing.T) {
	for _, mpStatus := range []string{"refunded", "charged_back"} {
		t.Run(mpStatus, func(t *testing.T) {
			f, booking, payment, ownerID := refundFixture(t, "confirmed")

			err := f.service.processRefundedPayment(t.Context(), payment, &mp.Payment{
				ID: 123, Status: mpStatus, TransactionAmount: 2500, TransactionAmountRefunded: 2500,
			}, "mp-123")
			if err != nil {
				t.Fatalf("processing the refund: %v", err)
			}

			if len(f.notify.ownerRefunds) != 1 {
				t.Fatalf("the owner must be told once that MercadoPago's refund cancelled the booking; got %d", len(f.notify.ownerRefunds))
			}
			got := f.notify.ownerRefunds[0]
			if got.OwnerID != ownerID.String() || got.BookingID != booking.ID.String() {
				t.Errorf("the owner notice names the wrong owner or booking: %+v", got)
			}
			if got.ComplexName != "Vibe Palermo" || got.CourtName != "Cancha 1" || got.ClientName != "Ana Pérez" {
				t.Errorf("the owner notice must name the complex, court and client; got %+v", got)
			}
			if got.Date != booking.Date.Format("02/01") {
				t.Errorf("the owner notice must carry the booking's day; want %q, got %q", booking.Date.Format("02/01"), got.Date)
			}

			if len(f.waitlist.freed) != 1 {
				t.Fatalf("the freed slot must be handed to the waitlist once; got %d", len(f.waitlist.freed))
			}
			freed := f.waitlist.freed[0]
			if freed.ComplexID != booking.ComplexID || freed.ComplexSlug != "vibe-palermo" || freed.ComplexName != "Vibe Palermo" {
				t.Errorf("the waitlist must be told which complex freed the slot; got %+v", freed)
			}
			if !freed.Day.Equal(booking.Date) {
				t.Errorf("the waitlist must be told the freed day; want %v, got %v", booking.Date, freed.Day)
			}
		})
	}
}

// A partial refund leaves the booking standing, so nothing was freed and
// nobody is told it was cancelled.
func TestAPartialMercadoPagoRefundCancelsNothingSoTellsNobodyAboutACancellation(t *testing.T) {
	f, _, payment, _ := refundFixture(t, "confirmed")

	err := f.service.processRefundedPayment(t.Context(), payment, &mp.Payment{
		ID: 123, Status: "refunded", TransactionAmount: 2500, TransactionAmountRefunded: 1000,
	}, "mp-123")
	if err != nil {
		t.Fatalf("processing the partial refund: %v", err)
	}

	if len(f.notify.ownerRefunds) != 0 {
		t.Errorf("a partial refund cancels nothing; the owner must not be told; got %d", len(f.notify.ownerRefunds))
	}
	if len(f.waitlist.freed) != 0 {
		t.Errorf("a partial refund frees no slot; the waitlist must not hear it; got %d", len(f.waitlist.freed))
	}
}

// When Vibe cancelled the booking first, the owner already heard about it
// (a client cancellation) or chose it (their own cancel), and the slot was
// already handed to the waitlist. The webhook for the money coming back must
// not announce any of that again.
func TestAMercadoPagoRefundOfABookingVibeAlreadyCancelledDoesNotEmailTheOwnerAgain(t *testing.T) {
	f, _, payment, _ := refundFixture(t, "cancelled")

	err := f.service.processRefundedPayment(t.Context(), payment, &mp.Payment{
		ID: 123, Status: "refunded", TransactionAmount: 2500, TransactionAmountRefunded: 2500,
	}, "mp-123")
	if err != nil {
		t.Fatalf("processing the refund of a cancelled booking: %v", err)
	}

	if len(f.notify.ownerRefunds) != 0 {
		t.Errorf("the booking was cancelled before this webhook; the owner must not be told again; got %d", len(f.notify.ownerRefunds))
	}
	if len(f.waitlist.freed) != 0 {
		t.Errorf("the slot was freed when the booking was cancelled; the waitlist must not hear it twice; got %d", len(f.waitlist.freed))
	}
}

// A played or no-show booking is never cancelled by a refund, so its slot is
// not freed and the owner is not told anything was cancelled.
func TestAMercadoPagoRefundOfAPlayedBookingFreesNoSlot(t *testing.T) {
	for _, status := range []string{"completed", "no_show"} {
		t.Run(status, func(t *testing.T) {
			f, _, payment, _ := refundFixture(t, status)

			err := f.service.processRefundedPayment(t.Context(), payment, &mp.Payment{
				ID: 123, Status: "refunded", TransactionAmount: 2500, TransactionAmountRefunded: 2500,
			}, "mp-123")
			if err != nil {
				t.Fatalf("processing the refund of a played booking: %v", err)
			}

			if len(f.notify.ownerRefunds) != 0 || len(f.waitlist.freed) != 0 {
				t.Errorf("a %s booking is not cancelled, so nobody is told and no slot is freed; owner=%d waitlist=%d",
					status, len(f.notify.ownerRefunds), len(f.waitlist.freed))
			}
		})
	}
}

// When the cancellation cannot be written, nothing was cancelled, so the owner
// hears nothing and the slot is not handed out. The webhook is retried.
func TestAMercadoPagoRefundThatFailsToCancelTellsNobody(t *testing.T) {
	f, _, payment, _ := refundFixture(t, "confirmed")
	f.payments.confirmErr = errDatabase

	err := f.service.processRefundedPayment(t.Context(), payment, &mp.Payment{
		ID: 123, Status: "refunded", TransactionAmount: 2500, TransactionAmountRefunded: 2500,
	}, "mp-123")
	if err == nil {
		t.Fatal("a cancellation the database refused must be reported so the webhook is retried")
	}

	if len(f.notify.ownerRefunds) != 0 || len(f.waitlist.freed) != 0 {
		t.Errorf("nothing was cancelled, so nobody is told and no slot is freed; owner=%d waitlist=%d",
			len(f.notify.ownerRefunds), len(f.waitlist.freed))
	}
}

// The no-payment-record path cancels a live booking the same way, so it owes
// the owner the same notice and the waitlist the same day.
func TestAMercadoPagoRefundWithNoPaymentRecordAlsoEmailsTheOwnerAndFreesTheSlot(t *testing.T) {
	f, booking, _, ownerID := refundFixture(t, "confirmed")

	err := f.service.processRefundedPaymentFromBooking(t.Context(), booking, &mp.Payment{
		ID: 123, Status: "charged_back", TransactionAmount: 2500,
	}, "mp-123")
	if err != nil {
		t.Fatalf("cancelling after an unrecorded refund: %v", err)
	}

	if len(f.notify.ownerRefunds) != 1 || f.notify.ownerRefunds[0].OwnerID != ownerID.String() {
		t.Fatalf("the owner must be told when an unrecorded refund cancels a live booking; got %+v", f.notify.ownerRefunds)
	}
	if len(f.waitlist.freed) != 1 {
		t.Fatalf("the freed slot must reach the waitlist on this path too; got %d", len(f.waitlist.freed))
	}
}
