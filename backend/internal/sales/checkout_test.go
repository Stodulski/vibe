package sales

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	salestore "github.com/stodulski/vibe-server/internal/sales/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// stubPublisher counts the booking-changed signals a checkout sends.
type stubPublisher struct{ calls int }

func (p *stubPublisher) PublishBookingChanged(uuid.UUID) { p.calls++ }

func balanceLine(bookingID uuid.UUID, amount int) *salestore.BookingPaymentLine {
	start := time.Date(2026, 10, 12, 22, 0, 0, 0, time.UTC) // 19:00 in Argentina
	return &salestore.BookingPaymentLine{
		PaymentID:  uuid.New(),
		BookingID:  bookingID,
		Amount:     amount,
		Date:       time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
		StartsAt:   start,
		EndsAt:     start.Add(time.Hour),
		CourtName:  "Cancha 2",
		ClientName: "Juan Perez",
	}
}

func checkoutRequest(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, body))
	return w
}

func TestCheckoutBookingOnlyReturnsANullSaleAndTheBalanceLines(t *testing.T) {
	bookingID := uuid.New()
	store := &stubStore{checkoutRes: &salestore.CheckoutResult{BookingLines: []*salestore.BookingPaymentLine{balanceLine(bookingID, 350_000)}}}
	h, rec := newTestHandler(store)

	w := checkoutRequest(t, h, `{"items":[],"booking_payments":[{"booking_id":"`+bookingID.String()+`","amount":350000}],"method":"cash"}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["sale"] != nil {
		t.Errorf("a booking-only checkout must answer sale null; got %v", body["sale"])
	}
	lines, ok := body["booking_payments"].([]any)
	if !ok || len(lines) != 1 {
		t.Fatalf("want one booking_payments entry; got %v", body["booking_payments"])
	}
	line := lines[0].(map[string]any)
	if line["amount"] != float64(350_000) || line["start_time"] != "19:00" || line["client_name"] != "Juan Perez" {
		t.Errorf("booking line = %v; want amount 350000 starting 19:00 for Juan Perez", line)
	}
	if len(store.checkoutItems) != 0 || len(store.checkoutBookings) != 1 || store.checkoutBookings[0].Amount != 350_000 {
		t.Errorf("store got items=%v bookings=%v", store.checkoutItems, store.checkoutBookings)
	}
	if len(rec.entries) != 1 || rec.entries[0].Action != "confirm_payment" || rec.entries[0].EntityType != "booking" {
		t.Errorf("want one confirm_payment audit on the booking; got %+v", rec.entries)
	}
}

func TestCheckoutMixedReturnsTheSaleWithItsBalanceLines(t *testing.T) {
	bookingID := uuid.New()
	sale := &salestore.Sale{ID: uuid.New(), Total: 1_600}
	store := &stubStore{checkoutRes: &salestore.CheckoutResult{
		Sale:         sale,
		Items:        []*salestore.SaleItem{{ID: uuid.New(), SaleID: sale.ID}},
		BookingLines: []*salestore.BookingPaymentLine{balanceLine(bookingID, 350_000)},
	}}
	h, rec := newTestHandler(store)

	w := checkoutRequest(t, h, `{"items":[{"product_id":"`+uuid.New().String()+`","quantity":2}],"booking_payments":[{"booking_id":"`+bookingID.String()+`","amount":350000}],"method":"transfer"}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	saleBody := decode(t, w)["sale"].(map[string]any)
	if saleBody["total"] != float64(1_600) {
		t.Errorf("sale total = %v; want the products total 1600", saleBody["total"])
	}
	if lines := saleBody["booking_payments"].([]any); len(lines) != 1 {
		t.Errorf("sale should carry its balance line; got %v", lines)
	}
	if len(rec.entries) != 2 || rec.entries[0].Action != "create" || rec.entries[1].Action != "confirm_payment" {
		t.Errorf("want a sale create audit then a booking confirm_payment audit; got %+v", rec.entries)
	}
}

func TestCheckoutNeedsAtLeastOneLine(t *testing.T) {
	h, _ := newTestHandler(&stubStore{})
	w := checkoutRequest(t, h, `{"items":[],"method":"cash"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if _, ok := fieldError(decode(t, w), "items"); !ok {
		t.Errorf("want a field error on items; got %s", w.Body.String())
	}
}

func TestCheckoutValidatesEachBalanceEntry(t *testing.T) {
	id := uuid.New().String()
	tests := []struct {
		name, body, field string
	}{
		{"duplicate booking", `{"items":[],"booking_payments":[{"booking_id":"` + id + `","amount":1},{"booking_id":"` + id + `","amount":1}],"method":"cash"}`, "booking_payments[1].booking_id"},
		{"missing amount", `{"items":[],"booking_payments":[{"booking_id":"` + id + `"}],"method":"cash"}`, "booking_payments[0].amount"},
		{"zero amount", `{"items":[],"booking_payments":[{"booking_id":"` + id + `","amount":0}],"method":"cash"}`, "booking_payments[0].amount"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newTestHandler(&stubStore{})
			w := checkoutRequest(t, h, tt.body)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if _, ok := fieldError(decode(t, w), tt.field); !ok {
				t.Errorf("want a field error on %s; got %s", tt.field, w.Body.String())
			}
		})
	}
}

func TestCheckoutRefusalsAnswerTheirStatusAndRecordNothing(t *testing.T) {
	id := uuid.New()
	body := `{"items":[],"booking_payments":[{"booking_id":"` + id.String() + `","amount":350000}],"method":"cash"}`
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"stale balance", salestore.ErrBookingBalanceMismatch, http.StatusConflict},
		{"already paid", salestore.ErrBookingAlreadyPaid, http.StatusConflict},
		{"not payable", salestore.ErrBookingNotPayable, http.StatusConflict},
		{"no open session", salestore.ErrNoOpenCashSession, http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubStore{createErr: tt.err}
			h, rec := newTestHandler(store)
			w := checkoutRequest(t, h, body)
			if w.Code != tt.want {
				t.Fatalf("want %d; got %d (%s)", tt.want, w.Code, w.Body.String())
			}
			if len(rec.entries) != 0 {
				t.Errorf("a refused checkout must audit nothing; got %+v", rec.entries)
			}
		})
	}
}

func TestCheckoutUnknownBookingNamesItsIndex(t *testing.T) {
	id := uuid.New()
	store := &stubStore{createErr: &salestore.ErrUnknownBooking{BookingID: id}}
	h, _ := newTestHandler(store)
	w := checkoutRequest(t, h, `{"items":[],"booking_payments":[{"booking_id":"`+id.String()+`","amount":350000}],"method":"cash"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if _, ok := fieldError(decode(t, w), "booking_payments[0].booking_id"); !ok {
		t.Errorf("want the unknown booking named at its index; got %s", w.Body.String())
	}
}

func TestCheckoutPublishesOneBookingChangeWhenABalanceIsCollected(t *testing.T) {
	store := &stubStore{checkoutRes: &salestore.CheckoutResult{BookingLines: []*salestore.BookingPaymentLine{balanceLine(uuid.New(), 1)}}}
	pub := &stubPublisher{}
	svc := newTestService(store, &stubRecorder{}).WithBookingPublisher(pub)

	if _, err := svc.Checkout(t.Context(), uuid.New(), testActor(), uuid.New(), CheckoutInput{
		BookingPayments: []salestore.BookingPaymentInput{{BookingID: uuid.New(), Amount: 1}},
		Method:          "cash",
	}); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if pub.calls != 1 {
		t.Errorf("want one booking-changed signal; got %d", pub.calls)
	}
}

// TestBookingsDueReadsTodayInArgentina pins the day boundary: at 23:30 on the
// 12th in Buenos Aires it is already the 13th in UTC, and the list must still be
// the 12th's bookings.
func TestBookingsDueReadsTodayInArgentina(t *testing.T) {
	store := &stubStore{}
	svc := newTestService(store, &stubRecorder{})
	svc.now = func() time.Time {
		return time.Date(2026, 10, 12, 23, 30, 0, 0, timezone.Argentina)
	}

	day, _, err := svc.BookingsDue(t.Context(), uuid.New())
	if err != nil {
		t.Fatalf("BookingsDue: %v", err)
	}
	if want := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC); !day.Equal(want) {
		t.Fatalf("day = %s; want 2026-10-12", day)
	}
	if !store.balancesDay.Equal(day) {
		t.Errorf("store read day %s; want %s", store.balancesDay, day)
	}
}

func TestBookingsDueAnswersTheRemainingBalances(t *testing.T) {
	store := &stubStore{balancesDue: []*salestore.BalanceDue{{
		BookingID:  uuid.New(),
		Date:       time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
		StartsAt:   time.Date(2026, 10, 12, 22, 0, 0, 0, time.UTC),
		EndsAt:     time.Date(2026, 10, 12, 23, 0, 0, 0, time.UTC),
		CourtName:  "Cancha 2",
		ClientName: "Juan Perez",
		Status:     "confirmed",
		Total:      500_000,
		Paid:       150_000,
		Remaining:  350_000,
	}}}
	h, _ := newTestHandler(store)

	w := httptest.NewRecorder()
	h.BookingsDue(w, ownerRequest(t, http.MethodGet, "/", uuid.New(), nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if _, ok := body["date"].(string); !ok {
		t.Errorf("want the day the list was read for; got %v", body["date"])
	}
	rows := body["bookings"].([]any)
	if len(rows) != 1 {
		t.Fatalf("want one booking due; got %v", rows)
	}
	row := rows[0].(map[string]any)
	if row["remaining"] != float64(350_000) || row["paid"] != float64(150_000) || row["start_time"] != "19:00" || row["end_time"] != "20:00" {
		t.Errorf("row = %v; want remaining 350000, paid 150000, 19:00-20:00", row)
	}
}

func TestCheckoutErrorsKeepTheirSentinel(t *testing.T) {
	store := &stubStore{createErr: salestore.ErrBookingBalanceMismatch}
	svc := newTestService(store, &stubRecorder{})
	_, err := svc.Checkout(t.Context(), uuid.New(), testActor(), uuid.New(), CheckoutInput{
		BookingPayments: []salestore.BookingPaymentInput{{BookingID: uuid.New(), Amount: 1}},
		Method:          "cash",
	})
	if !errors.Is(err, salestore.ErrBookingBalanceMismatch) {
		t.Fatalf("want ErrBookingBalanceMismatch; got %v", err)
	}
}
