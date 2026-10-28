package bookings

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// While a MercadoPago refund on the booking is still pending, the owner's close-out
// is refused as a conflict, the same status a double-booking gets, and the owner
// reads the sentence in the response. Nothing is audited and the dashboard is not
// told the booking was closed out.
func TestManualRefundIsRefusedAsAConflictWhileAMercadoPagoRefundIsPending(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.Status = "cancelled"
	booking.RefundStatus = bookingstore.RefundStatusPartial
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.payments.manualRefundErr = paymentstore.ErrMercadoPagoRefundPending

	w := httptest.NewRecorder()
	f.handler.ManualRefund(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"bookingID": booking.ID.String()}, `{}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409 while a MercadoPago refund is pending; got %d (%s)", w.Code, w.Body.String())
	}
	if f.payments.manualRefundCalls != 1 {
		t.Fatalf("RecordManualRefund must be asked once, and refuse; got %d calls", f.payments.manualRefundCalls)
	}
	if len(f.audit.entries) != 0 {
		t.Errorf("a refused close-out must not be audited; got %+v", f.audit.entries)
	}

	want := "esta reserva tiene una devolución de MercadoPago pendiente; hasta que se resuelva no se puede cerrar la devolución manual"
	if got := decode(t, w)["detail"]; got != want {
		t.Errorf("the owner must read the reason in Spanish; want %q, got %v", want, got)
	}
}
