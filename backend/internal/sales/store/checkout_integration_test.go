//go:build integration

package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// artDay is the Argentina calendar day, as bookings.date stores it.
func artDay(at time.Time) time.Time {
	local := at.In(timezone.Argentina)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

// bookingOn inserts a booking on the given day. Fixture bookings default to a
// week ahead; the due list works by day, so these tests place them explicitly.
func bookingOn(t *testing.T, f *datatest.Fixture, day time.Time, opts datatest.BookingOptions) *bookingstore.Booking {
	t.Helper()
	b := f.NewBooking(opts)
	b.Date = day
	if err := f.Stores.Bookings.Insert(f.Scoped(context.Background()), b); err != nil {
		t.Fatalf("inserting booking: %v", err)
	}
	return b
}

func countRows(t *testing.T, f *datatest.Fixture, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.DB.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	return n
}

func paymentsSum(t *testing.T, f *datatest.Fixture, bookingID uuid.UUID) int {
	t.Helper()
	return countRows(t, f, `SELECT COALESCE(SUM(amount), 0)::int FROM payments WHERE booking_id = $1`, bookingID)
}

func stockOf(t *testing.T, f *datatest.Fixture, productID uuid.UUID) int {
	t.Helper()
	return countRows(t, f, `SELECT stock_on_hand FROM products WHERE id = $1`, productID)
}

// TestIntegration_CheckoutBookingOnlyCollectsTheBalanceAsOneCounterPayment:
// a checkout with no product lines has no sale. It records the balance as one
// payment through the booking payment logic, moves the booking to fully_paid,
// and returns the booking line for the response.
func TestIntegration_CheckoutBookingOnlyCollectsTheBalanceAsOneCounterPayment(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	openSession(t, f, 0)
	booking := bookingOn(t, f, artDay(time.Now()), datatest.BookingOptions{StartTime: "18:00", EndTime: "19:00"})

	res, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID, nil,
		[]salestore.BookingPaymentInput{{BookingID: booking.ID, Amount: 350_000}}, "cash", nil)
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if res.Sale != nil {
		t.Fatalf("booking-only checkout created a sale: %+v", res.Sale)
	}
	if len(res.BookingLines) != 1 || res.BookingLines[0].Amount != 350_000 || res.BookingLines[0].BookingID != booking.ID {
		t.Fatalf("want one booking line for 350000; got %+v", res.BookingLines)
	}
	if res.BookingLines[0].CourtName == "" || res.BookingLines[0].ClientName == "" {
		t.Errorf("booking line lacks its court or client summary: %+v", res.BookingLines[0])
	}
	if _, _, refund := f.ReadBookingState(t, booking.ID); refund != "none" {
		t.Errorf("refund status changed: %q", refund)
	}
	if got := countRows(t, f, `SELECT COUNT(*)::int FROM sales WHERE complex_id = $1`, f.ComplexID); got != 0 {
		t.Errorf("want no sale rows; got %d", got)
	}
	if _, collection, _ := f.ReadBookingState(t, booking.ID); collection != "fully_paid" {
		t.Errorf("collection = %q; want fully_paid", collection)
	}
	if got := paymentsSum(t, f, booking.ID); got != 350_000 {
		t.Errorf("payments for the booking sum to %d; want exactly the balance 350000", got)
	}
}

// TestIntegration_CheckoutMixedCountsTheSaleOnceAndLinksTheBalance: products
// and a balance in one checkout. The sale's cash income carries the products
// only; the balance lives once, in its payments row, and is linked to the sale.
func TestIntegration_CheckoutMixedCountsTheSaleOnceAndLinksTheBalance(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	product := insertProduct(t, f, "Paddle rental grip", 800, true)
	openSession(t, f, 0)
	booking := bookingOn(t, f, artDay(time.Now()), datatest.BookingOptions{StartTime: "18:00", EndTime: "19:00"})

	res, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 2}},
		[]salestore.BookingPaymentInput{{BookingID: booking.ID, Amount: 350_000}}, "transfer", nil)
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if res.Sale == nil || res.Sale.Total != 1_600 {
		t.Fatalf("want a sale totalling the products (1600); got %+v", res.Sale)
	}
	if got := countRows(t, f, `SELECT amount FROM cash_movements WHERE id = $1`, res.Sale.CashMovementID); got != 1_600 {
		t.Errorf("sale cash income = %d; want the products total only (1600)", got)
	}
	if got := paymentsSum(t, f, booking.ID); got != 350_000 {
		t.Errorf("booking payments sum to %d; want 350000 once", got)
	}
	lines, err := f.Stores.Sales.ListBookingPaymentsBySaleIDs(ctx, f.ComplexID, []uuid.UUID{res.Sale.ID})
	if err != nil {
		t.Fatalf("ListBookingPaymentsBySaleIDs: %v", err)
	}
	if got := lines[res.Sale.ID]; len(got) != 1 || got[0].Amount != 350_000 || got[0].BookingID != booking.ID {
		t.Fatalf("sale should link the balance once; got %+v", got)
	}
	if _, collection, _ := f.ReadBookingState(t, booking.ID); collection != "fully_paid" {
		t.Errorf("collection = %q; want fully_paid", collection)
	}
}

// TestIntegration_CheckoutRefusesAStaleBalanceAndRecordsNothing: an amount
// that differs from the booking's remaining balance is refused, and the whole
// checkout rolls back, products included.
func TestIntegration_CheckoutRefusesAStaleBalanceAndRecordsNothing(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	product := insertProduct(t, f, "Stale cart snack", 300, true)
	openSession(t, f, 0)
	if _, _, _, err := f.Stores.Sales.Create(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 5}}, "cash", nil); err != nil {
		t.Fatalf("seed stock: %v", err)
	}
	before := stockOf(t, f, product.ID)
	booking := bookingOn(t, f, artDay(time.Now()), datatest.BookingOptions{StartTime: "18:00", EndTime: "19:00"})

	_, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 2}},
		[]salestore.BookingPaymentInput{{BookingID: booking.ID, Amount: 300_000}}, "cash", nil)
	if !errors.Is(err, salestore.ErrBookingBalanceMismatch) {
		t.Fatalf("want ErrBookingBalanceMismatch; got %v", err)
	}
	if got := stockOf(t, f, product.ID); got != before {
		t.Errorf("stock moved to %d on a refused checkout; want %d", got, before)
	}
	if got := paymentsSum(t, f, booking.ID); got != 0 {
		t.Errorf("a refused checkout recorded %d of payments", got)
	}
	if _, collection, _ := f.ReadBookingState(t, booking.ID); collection != "deposit_paid" {
		t.Errorf("collection = %q; want the booking untouched at deposit_paid", collection)
	}
}

// TestIntegration_CheckoutRefusesABookingThatCannotTakeAPayment covers the
// refusals that answer without recording anything. A pending booking is
// confirmed from the booking panel, never by a checkout.
func TestIntegration_CheckoutRefusesABookingThatCannotTakeAPayment(t *testing.T) {
	tests := []struct {
		name    string
		opts    datatest.BookingOptions
		amount  int
		wantErr error
	}{
		{"fully paid", datatest.BookingOptions{Status: "confirmed", CollectionStatus: "fully_paid", DepositAmount: 500_000}, 0, salestore.ErrBookingAlreadyPaid},
		{"cancelled", datatest.BookingOptions{Status: "cancelled", CollectionStatus: "deposit_paid"}, 350_000, salestore.ErrBookingNotPayable},
		{"no-show", datatest.BookingOptions{Status: "no_show", CollectionStatus: "unpaid"}, 500_000, salestore.ErrBookingNotPayable},
		{"pending", datatest.BookingOptions{Status: "pending", CollectionStatus: "unpaid"}, 500_000, salestore.ErrBookingNotPayable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := datatest.Isolated(t)
			ctx := f.Scoped(context.Background())
			openSession(t, f, 0)
			tt.opts.StartTime, tt.opts.EndTime = "18:00", "19:00"
			booking := bookingOn(t, f, artDay(time.Now()), tt.opts)
			amount := tt.amount
			if amount == 0 {
				amount = 1
			}
			_, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID, nil,
				[]salestore.BookingPaymentInput{{BookingID: booking.ID, Amount: amount}}, "cash", nil)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("want %v; got %v", tt.wantErr, err)
			}
			if got := paymentsSum(t, f, booking.ID); got != 0 {
				t.Errorf("a refused checkout recorded %d of payments", got)
			}
		})
	}
}

// TestIntegration_CheckoutWithNoOpenSessionRefuses: a checkout is cash-session
// work, so it is refused with nothing open, balances included.
func TestIntegration_CheckoutWithNoOpenSessionRefuses(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	booking := bookingOn(t, f, artDay(time.Now()), datatest.BookingOptions{StartTime: "18:00", EndTime: "19:00"})

	_, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID, nil,
		[]salestore.BookingPaymentInput{{BookingID: booking.ID, Amount: 350_000}}, "cash", nil)
	if !errors.Is(err, salestore.ErrNoOpenCashSession) {
		t.Fatalf("want ErrNoOpenCashSession; got %v", err)
	}
	if got := paymentsSum(t, f, booking.ID); got != 0 {
		t.Errorf("a refused checkout recorded %d of payments", got)
	}
}

// TestIntegration_TwoCheckoutsForOneBalanceLetOneWin: two cashiers cannot both
// collect the same balance. The second waits on the booking row, re-reads it
// fully paid, and is refused.
func TestIntegration_TwoCheckoutsForOneBalanceLetOneWin(t *testing.T) {
	// Two goroutines need two connections: only the shared fixture has a pool.
	f := datatest.Shared(t)
	ctx := f.Scoped(context.Background())
	openSession(t, f, 0)
	booking := bookingOn(t, f, artDay(time.Now()), datatest.BookingOptions{StartTime: "18:00", EndTime: "19:00"})

	var wg sync.WaitGroup
	errs := make([]error, 2)
	start := make(chan struct{})
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID, nil,
				[]salestore.BookingPaymentInput{{BookingID: booking.ID, Amount: 350_000}}, "cash", nil)
		}(i)
	}
	close(start)
	wg.Wait()

	won, lost := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case errors.Is(err, salestore.ErrBookingAlreadyPaid):
			lost++
		default:
			t.Fatalf("unexpected error from a racing checkout: %v", err)
		}
	}
	if won != 1 || lost != 1 {
		t.Fatalf("want one win and one ErrBookingAlreadyPaid; got %d wins, %d losses", won, lost)
	}
	if got := paymentsSum(t, f, booking.ID); got != 350_000 {
		t.Errorf("the balance was collected %d times in total; want exactly 350000", got)
	}
}

// TestIntegration_VoidRefusesASaleThatCollectedABalance: no reversal exists for
// a booking payment, so a sale carrying one cannot be voided. Nothing moves.
func TestIntegration_VoidRefusesASaleThatCollectedABalance(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	product := insertProduct(t, f, "Voidable grip", 800, true)
	openSession(t, f, 0)
	booking := bookingOn(t, f, artDay(time.Now()), datatest.BookingOptions{StartTime: "18:00", EndTime: "19:00"})
	res, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 1}},
		[]salestore.BookingPaymentInput{{BookingID: booking.ID, Amount: 350_000}}, "cash", nil)
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	stock := stockOf(t, f, product.ID)

	_, err = f.Stores.Sales.Void(ctx, f.ComplexID, res.Sale.ID, f.UserID, nil)
	if !errors.Is(err, salestore.ErrSaleHasBookingPayments) {
		t.Fatalf("want ErrSaleHasBookingPayments; got %v", err)
	}
	if got, _ := f.Stores.Sales.GetByID(ctx, f.ComplexID, res.Sale.ID); got.IsVoided() {
		t.Error("the refused void still voided the sale")
	}
	if got := stockOf(t, f, product.ID); got != stock {
		t.Errorf("a refused void moved stock to %d; want %d", got, stock)
	}
}

// TestIntegration_ListBalancesDueFiltersByDayStatusAndBalance pins the sell
// page's list: one day, owed money only, sorted by start time, with the status
// rules (only confirmed and completed bookings; a pending one is confirmed from
// the booking panel first).
func TestIntegration_ListBalancesDueFiltersByDayStatusAndBalance(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	today := artDay(time.Now())
	tomorrow := today.AddDate(0, 0, 1)

	deposit := bookingOn(t, f, today, datatest.BookingOptions{StartTime: "18:00", EndTime: "19:00", Price: 500_000, DepositAmount: 150_000})
	unpaid := bookingOn(t, f, today, datatest.BookingOptions{StartTime: "17:00", EndTime: "18:00", Status: "confirmed", CollectionStatus: "unpaid", DepositAmount: 0, Price: 500_000})
	bookingOn(t, f, today, datatest.BookingOptions{StartTime: "16:00", EndTime: "17:00", Status: "pending", CollectionStatus: "unpaid", DepositAmount: 0, Price: 500_000})
	bookingOn(t, f, today, datatest.BookingOptions{StartTime: "19:00", EndTime: "20:00", Status: "confirmed", CollectionStatus: "fully_paid", DepositAmount: 500_000, Price: 500_000})
	bookingOn(t, f, today, datatest.BookingOptions{StartTime: "20:00", EndTime: "21:00", Status: "cancelled", CollectionStatus: "deposit_paid", DepositAmount: 150_000, Price: 500_000})
	bookingOn(t, f, today, datatest.BookingOptions{StartTime: "21:00", EndTime: "22:00", Status: "no_show", CollectionStatus: "unpaid", DepositAmount: 0, Price: 500_000})
	bookingOn(t, f, today, datatest.BookingOptions{StartTime: "22:00", EndTime: "23:00", Status: "completed", CollectionStatus: "deposit_paid", DepositAmount: 500_000, Price: 500_000})
	bookingOn(t, f, tomorrow, datatest.BookingOptions{StartTime: "18:00", EndTime: "19:00", Price: 500_000, DepositAmount: 150_000})

	due, err := f.Stores.Sales.ListBalancesDue(ctx, f.ComplexID, today)
	if err != nil {
		t.Fatalf("ListBalancesDue: %v", err)
	}
	want := []struct {
		id        uuid.UUID
		remaining int
	}{
		{unpaid.ID, 500_000},
		{deposit.ID, 350_000},
	}
	if len(due) != len(want) {
		t.Fatalf("want %d bookings due; got %d: %+v", len(want), len(due), due)
	}
	for i, w := range want {
		if due[i].BookingID != w.id || due[i].Remaining != w.remaining {
			t.Errorf("row %d = %s remaining %d; want %s remaining %d", i, due[i].BookingID, due[i].Remaining, w.id, w.remaining)
		}
	}
}

// TestIntegration_CheckoutCountsTheBalanceOnceInExpectedCash reads the two
// figures expected cash is built from (cashbox.Service.buildSummary): cash
// booking payments in the session window, and the session's sale income. The
// balance must appear in exactly one of them.
func TestIntegration_CheckoutCountsTheBalanceOnceInExpectedCash(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	product := insertProduct(t, f, "Expected cash snack", 800, false)
	session := openSession(t, f, 0)
	booking := bookingOn(t, f, artDay(time.Now()), datatest.BookingOptions{StartTime: "18:00", EndTime: "19:00"})

	if _, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 2}},
		[]salestore.BookingPaymentInput{{BookingID: booking.ID, Amount: 350_000}}, "cash", nil); err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	methods, err := f.Stores.Reports.PaymentSummaryByMethodWindow(ctx, f.ComplexID, session.OpenedAt, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("PaymentSummaryByMethodWindow: %v", err)
	}
	cashBookings := 0
	for _, m := range methods {
		if m.Method == "cash" {
			cashBookings += m.Amount + m.ServiceFee
		}
	}
	if cashBookings != 350_000 {
		t.Errorf("cash booking payments in the session = %d; want the balance once (350000)", cashBookings)
	}
	cashSales := countRows(t, f, `SELECT COALESCE(SUM(amount), 0)::int FROM cash_movements WHERE session_id = $1 AND kind = 'income' AND category = 'sale'`, session.ID)
	if cashSales != 1_600 {
		t.Errorf("sale income in the session = %d; want the products only (1600)", cashSales)
	}
}
