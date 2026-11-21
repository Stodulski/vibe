package bookings

import (
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// noticeFixture is a fixture whose complex, court and client can all be read,
// so every notice has something to name. The client's own address is on file;
// a booking that carries a typed address (ContactEmail) wins over it.
func noticeFixture(t *testing.T) (*fixture, uuid.UUID, *bookingstore.Booking) {
	t.Helper()
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	email := "ana@example.com"
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", LastName: "Perez", Email: &email}
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", Slug: "vibe", OwnerID: uuid.New(), Phone: "1155550000"}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Cancha 1"}
	return f, complexID, booking
}

// S1: an owner who cancels through the general edit takes the same path as the
// cancel endpoint: the client is told, the slot goes back to the waitlist and
// the freed-slot listener, and the checkout link of an unpaid booking is
// closed. Before, the edit wrote 'cancelled' and nobody heard of it.
func TestCancellingThroughTheEditTellsTheClientAndFreesTheSlot(t *testing.T) {
	f, complexID, booking := noticeFixture(t)
	booking.CollectionStatus = bookingstore.CollectionStatusUnpaid
	booking.ContactEmail = "typed@example.com"
	pref := "pref-77"
	f.payments.payment = &paymentstore.Payment{BookingID: booking.ID, MPPreferenceID: &pref}

	cancelled := string(bookingstore.BookingStatusCancelled)
	if _, err := f.service.Update(t.Context(), complexID, Actor{}, booking.ID, UpdateInput{Status: &cancelled}); err != nil {
		t.Fatalf("cancelling an unpaid booking through the edit must go through; got %v", err)
	}

	if len(f.notify.cancelled) != 1 {
		t.Fatalf("want one cancellation notice; got %d", len(f.notify.cancelled))
	}
	notice := f.notify.cancelled[0]
	if notice.Email != "typed@example.com" {
		t.Errorf("the notice goes to the address typed for the booking; got %q", notice.Email)
	}
	if notice.Source != notifications.SourceOwnerCancel {
		t.Errorf("the owner cancelled, so the owner is not emailed about it; source %q", notice.Source)
	}
	if notice.BookingID != booking.ID.String() || notice.CourtName != "Cancha 1" {
		t.Errorf("the notice must name the booking and its court; got %+v", notice)
	}
	if notice.RefundLine != refundMessage(paymentstore.RefundNone, 0) {
		t.Errorf("an unpaid booking owes nothing back; got refund line %q", notice.RefundLine)
	}
	if len(f.waitlist.freed) != 1 || len(f.freedSlots.slots) != 1 {
		t.Errorf("the freed slot must reach the waitlist and the listener; got %d and %d",
			len(f.waitlist.freed), len(f.freedSlots.slots))
	}
	if len(f.checkout.expired) != 1 || f.checkout.expired[0] != pref {
		t.Errorf("the unpaid booking's checkout link must be closed; got %v", f.checkout.expired)
	}
	if len(f.refunds.refunded) != 0 {
		t.Error("an unpaid booking must not be sent to the refund path")
	}
}

// S1: a booking whose refund the pipeline already decided (pending here) stays
// cancellable through the edit, and the edit still must not start a second
// refund. The client is told the money is on its way.
func TestCancellingThroughTheEditNeverStartsARefund(t *testing.T) {
	f, complexID, booking := noticeFixture(t)
	booking.RefundStatus = bookingstore.RefundStatusPending

	cancelled := string(bookingstore.BookingStatusCancelled)
	if _, err := f.service.Update(t.Context(), complexID, Actor{}, booking.ID, UpdateInput{Status: &cancelled}); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(f.refunds.refunded) != 0 {
		t.Fatalf("the edit must not start a refund; AutoRefundIfPaid ran %d times", len(f.refunds.refunded))
	}
	if len(f.notify.cancelled) != 1 || f.notify.cancelled[0].RefundLine != refundMessage(paymentstore.RefundQueued, 0) {
		t.Fatalf("want one notice saying the refund is in progress; got %+v", f.notify.cancelled)
	}
}

// S1: resending 'cancelled' for a booking already cancelled (an edit of its
// notes that carries the status) tells nobody anything again.
func TestResendingCancelledThroughTheEditSendsNothing(t *testing.T) {
	f, complexID, booking := noticeFixture(t)
	booking.Status = bookingstore.BookingStatusCancelled
	booking.CollectionStatus = bookingstore.CollectionStatusUnpaid

	cancelled := string(bookingstore.BookingStatusCancelled)
	if _, err := f.service.Update(t.Context(), complexID, Actor{}, booking.ID, UpdateInput{Status: &cancelled}); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(f.notify.cancelled) != 0 || len(f.waitlist.freed) != 0 {
		t.Errorf("nothing changed, nothing may be sent; got %d notices, %d freed", len(f.notify.cancelled), len(f.waitlist.freed))
	}
}

// S2: marking a no-show tells the client, once, at the booking's address.
func TestMarkingANoShowTellsTheClientOnce(t *testing.T) {
	f, complexID, booking := noticeFixture(t)
	booking.StartsAt = time.Now().Add(-2 * time.Hour)
	booking.EndsAt = booking.StartsAt.Add(90 * time.Minute)
	booking.Date = booking.StartsAt

	noShow := string(bookingstore.BookingStatusNoShow)
	if _, err := f.service.Update(t.Context(), complexID, Actor{}, booking.ID, UpdateInput{Status: &noShow}); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(f.notify.noShows) != 1 {
		t.Fatalf("want one no-show notice; got %d", len(f.notify.noShows))
	}
	n := f.notify.noShows[0]
	if n.Email != "ana@example.com" || n.BookingID != booking.ID.String() || n.CourtName != "Cancha 1" || n.ComplexPhone != "1155550000" {
		t.Errorf("the notice must reach the client and name the booking and the venue's contact; got %+v", n)
	}

	// The same status sent again counts nothing and tells nobody again.
	if _, err := f.service.Update(t.Context(), complexID, Actor{}, booking.ID, UpdateInput{Status: &noShow}); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(f.notify.noShows) != 1 {
		t.Errorf("a resent no-show must not notify again; got %d", len(f.notify.noShows))
	}
}

// S3: closing out a manual refund tells the client the amount the owner
// returned, keyed so a retried request is the same notice.
func TestAManualRefundTellsTheClientTheAmountReturned(t *testing.T) {
	f, complexID, booking := noticeFixture(t)
	booking.Status = bookingstore.BookingStatusCancelled
	booking.RefundStatus = bookingstore.RefundStatusPartial
	f.payments.manualRefundAmount = 1_250_000

	if _, err := f.service.ManualRefund(t.Context(), complexID, Actor{}, booking.ID); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(f.notify.refunds) != 1 {
		t.Fatalf("want one refund notice; got %d", len(f.notify.refunds))
	}
	r := f.notify.refunds[0]
	if r.Amount != notifications.FormatARS(1_250_000) || !r.ByHand || r.Email != "ana@example.com" {
		t.Errorf("the notice must quote what was returned by hand to the client; got %+v", r)
	}
	if r.BookingID != booking.ID.String() || r.RefundID == "" {
		t.Errorf("the notice must carry its deduplication subject; got booking %q refund %q", r.BookingID, r.RefundID)
	}
}

// S3: a refused close-out tells nobody anything.
func TestARefusedManualRefundSendsNothing(t *testing.T) {
	f, complexID, booking := noticeFixture(t)
	booking.Status = bookingstore.BookingStatusCancelled
	booking.RefundStatus = bookingstore.RefundStatusPartial
	f.payments.manualRefundErr = paymentstore.ErrNoManualRefundOwed

	if _, err := f.service.ManualRefund(t.Context(), complexID, Actor{}, booking.ID); err == nil {
		t.Fatal("want the refusal")
	}
	if len(f.notify.refunds) != 0 {
		t.Errorf("nothing was returned, nothing may be announced; got %d", len(f.notify.refunds))
	}
}

// S7: a counter payment gets the client a receipt with the amount, how it
// was paid and what is still owed.
func TestACounterPaymentSendsTheClientAReceipt(t *testing.T) {
	f, complexID, booking := noticeFixture(t)
	booking.CollectionStatus = bookingstore.CollectionStatusUnpaid
	booking.Price = 2_000_000

	_, payment, err := f.service.ConfirmPayment(t.Context(), complexID, Actor{}, booking.ID,
		ConfirmPaymentInput{Method: "cash", Amount: 500_000})
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if len(f.notify.receipts) != 1 {
		t.Fatalf("want one receipt; got %d", len(f.notify.receipts))
	}
	r := f.notify.receipts[0]
	if r.Amount != notifications.FormatARS(500_000) || r.Method != "en efectivo" {
		t.Errorf("the receipt must quote the amount and the method; got %+v", r)
	}
	if r.BalanceAmount != notifications.FormatARS(booking.BalanceDue()) || r.BalanceAmount == "" {
		t.Errorf("the receipt must quote what is still owed; got %q (balance %d)", r.BalanceAmount, booking.BalanceDue())
	}
	if r.Email != "ana@example.com" || r.BookingID != booking.ID.String() || r.PaymentID != payment.ID.String() {
		t.Errorf("the receipt must reach the client, keyed on the booking and the payment; got %+v", r)
	}
}

// S7: a refused counter payment sends no receipt.
func TestARefusedCounterPaymentSendsNoReceipt(t *testing.T) {
	f, complexID, booking := noticeFixture(t)
	booking.CollectionStatus = bookingstore.CollectionStatusUnpaid
	f.payments.insertAndConfirmErr = errDatabase

	if _, _, err := f.service.ConfirmPayment(t.Context(), complexID, Actor{}, booking.ID,
		ConfirmPaymentInput{Method: "cash", Amount: 100_000}); err == nil {
		t.Fatal("want the failure")
	}
	if len(f.notify.receipts) != 0 {
		t.Errorf("no payment was recorded, no receipt may be sent; got %d", len(f.notify.receipts))
	}
}
