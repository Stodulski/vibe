package bookings

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// racingStore stands in for the database while an owner's edit races a second
// writer. GetByID hands out a snapshot, as a read taken before the other writer
// committed does; onRead then plays that writer and changes the row. Update
// writes the caller's whole struct over the row, which is what the unconditional
// UPDATE does. UpdateIfUnchanged writes only while the row still carries the
// updated_at the owner read.
type racingStore struct {
	*stubStore
	row    *bookingstore.Booking
	onRead func(row *bookingstore.Booking)
}

func (r *racingStore) GetByID(_ context.Context, _ uuid.UUID) (*bookingstore.Booking, error) {
	snapshot := *r.row
	if r.onRead != nil {
		r.onRead(r.row)
	}
	return &snapshot, nil
}

func (r *racingStore) Update(_ context.Context, b *bookingstore.Booking) error {
	*r.row = *b
	return nil
}

func (r *racingStore) UpdateIfUnchanged(_ context.Context, b *bookingstore.Booking) error {
	if !r.row.UpdatedAt.Equal(b.UpdatedAt) {
		return data.ErrEditConflict
	}
	*r.row = *b
	return nil
}

// A stale owner edit must not write back over a cancellation that committed
// after the owner read the booking. The owner saw it confirmed and asks for
// no_show; the client cancelled it in between and recorded a refund intent.
// The write must be refused with a conflict, and the cancellation must stand.
func TestOwnerEditDoesNotRevertAConcurrentCancellation(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	row := futureBooking(complexID)
	row.UpdatedAt = time.Now().Add(-time.Minute)

	racing := &racingStore{stubStore: f.store, row: row}
	racing.onRead = func(current *bookingstore.Booking) {
		intent := time.Now()
		current.Status = bookingstore.BookingStatus("cancelled")
		current.RefundIntentAt = &intent
		current.UpdatedAt = intent
	}
	f.service.store = racing

	w := httptest.NewRecorder()
	f.handler.Update(w, ownerRequest(t, http.MethodPut, "/", complexID,
		map[string]string{"bookingID": row.ID.String()}, `{"status":"no_show"}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("a stale owner edit must be answered 409; got %d (%s)", w.Code, w.Body.String())
	}
	if row.Status != "cancelled" || row.RefundIntentAt == nil {
		t.Errorf("the concurrent cancellation was reverted: status %q, refund intent %v",
			row.Status, row.RefundIntentAt)
	}
}

// The owner's counter payment and the booking it pays for are one write. When
// the payment cannot be recorded, no booking may be left behind with a 201 that
// tells the owner the money was taken.
func TestOwnerCreateRecordsNothingWhenItsCounterPaymentFails(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	f.store.counterPaymentErr = errors.New("payments: insert failed")

	date := time.Now().In(timezone.Argentina).AddDate(0, 0, 7).Format("2006-01-02")
	body := fmt.Sprintf(`{"court_id":%q,"date":%q,"start_time":"18:00","duration_minutes":60,`+
		`"price":20000,"payment_option":"full","payment_method":"cash",`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000"}`,
		courtID, date)

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID, nil, body))

	if w.Code == http.StatusCreated {
		t.Fatalf("a booking whose counter payment failed must not be answered 201; got %d (%s)",
			w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Errorf("the booking must not be recorded without its payment; got %d", len(f.store.inserted))
	}
	if len(f.notify.confirmed) != 0 {
		t.Error("no confirmation may be sent for a booking that was not recorded")
	}
}

// The amount an owner confirms at the counter may not exceed what the booking
// still owes. Here the deposit is 100000 of a 500000 price, so 400000 is the
// balance and anything above it is refused.
func TestConfirmPaymentRefusesAnAmountAboveTheOutstandingBalance(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		wantCode int
	}{
		{"one peso above the balance is refused", "400001", http.StatusUnprocessableEntity},
		{"exactly the balance is accepted", "400000", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking := futureBooking(complexID)
			booking.Status = "pending"
			booking.DepositAmount = 100_000
			f.store.booking = booking
			f.linkResolver.booking = booking

			w := httptest.NewRecorder()
			body := fmt.Sprintf(`{"method":"cash","amount":%s}`, tt.amount)
			f.handler.ConfirmPayment(w, ownerRequest(t, http.MethodPost, "/", complexID,
				map[string]string{"bookingID": booking.ID.String()}, body))

			if w.Code != tt.wantCode {
				t.Fatalf("want %d; got %d (%s)", tt.wantCode, w.Code, w.Body.String())
			}
			if tt.wantCode != http.StatusOK && len(f.payments.confirmed) != 0 {
				t.Error("no payment may be confirmed above the outstanding balance")
			}
		})
	}
}

// A no-show is settled by the forfeited deposit, not by collecting the balance
// at the counter: the sales checkout refuses the same booking for the same
// reason (salestore.ErrBookingNotPayable). Confirming a payment on it is refused.
func TestConfirmPaymentRefusesANoShowBooking(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.Status = "no_show"
	f.store.booking = booking
	f.linkResolver.booking = booking

	w := httptest.NewRecorder()
	f.handler.ConfirmPayment(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"bookingID": booking.ID.String()}, `{"method":"cash","amount":100000}`))

	if w.Code != http.StatusConflict {
		t.Errorf("a no-show booking must refuse a counter payment with 409; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.payments.confirmed) != 0 {
		t.Error("no payment may be confirmed on a no-show booking")
	}
}
