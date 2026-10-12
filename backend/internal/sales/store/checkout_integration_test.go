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
	"github.com/stodulski/vibe-server/internal/data"
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
	if got := countRows(t, f, `SELECT amount FROM cash_movements WHERE id = $1`, *res.Sale.CashMovementID); got != 1_600 {
		t.Errorf("sale cash income = %d; want the products total only (1600)", got)
	}
	if got := paymentsSum(t, f, booking.ID); got != 350_000 {
		t.Errorf("booking payments sum to %d; want 350000 once", got)
	}
	lines, err := f.Stores.Sales.ListBookingPaymentsByEntryIDs(ctx, f.ComplexID, []uuid.UUID{res.Sale.ID})
	if err != nil {
		t.Fatalf("ListBookingPaymentsByEntryIDs: %v", err)
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

// listSales reads every page of the sales list through the store, the way the
// sell page pages through it, newest first. The guard bounds a cursor that
// never ends.
func listSales(t *testing.T, f *datatest.Fixture, sessionID *uuid.UUID, limit int) []*salestore.Sale {
	t.Helper()
	ctx := f.Scoped(context.Background())
	var out []*salestore.Sale
	cursor := ""
	for range 100 {
		page, meta, err := f.Stores.Sales.ListByComplex(ctx, f.ComplexID, sessionID, data.Filters{Limit: limit, Cursor: cursor})
		if err != nil {
			t.Fatalf("listing sales: %v", err)
		}
		out = append(out, page...)
		if !meta.HasMore {
			return out
		}
		cursor = meta.NextCursor
	}
	t.Fatal("the sales list never ended")
	return nil
}

// TestIntegration_ListShowsABalanceOnlyCheckoutBesideTheProductSales: a
// checkout that collected only court balances has no sale row, yet the
// session's sales list shows it between the product sales around it, with its
// balance lines, method and total, and it writes no cash income of its own.
func TestIntegration_ListShowsABalanceOnlyCheckoutBesideTheProductSales(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	product := insertProduct(t, f, "Tubo de pelotas", 800, true)
	session := openSession(t, f, 0)
	booking := bookingOn(t, f, artDay(time.Now()), datatest.BookingOptions{StartTime: "18:00", EndTime: "19:00"})

	before, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 1}}, nil, "cash", nil)
	if err != nil {
		t.Fatalf("product checkout: %v", err)
	}
	cashBefore := countRows(t, f, `SELECT COUNT(*)::int FROM cash_movements WHERE session_id = $1`, session.ID)

	if _, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID, nil,
		[]salestore.BookingPaymentInput{{BookingID: booking.ID, Amount: 350_000}}, "transfer", nil); err != nil {
		t.Fatalf("balance-only checkout: %v", err)
	}
	if got := countRows(t, f, `SELECT COUNT(*)::int FROM cash_movements WHERE session_id = $1`, session.ID); got != cashBefore {
		t.Errorf("a balance-only checkout wrote %d cash movements; want none", got-cashBefore)
	}

	after, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 2}}, nil, "cash", nil)
	if err != nil {
		t.Fatalf("second product checkout: %v", err)
	}

	entries := listSales(t, f, &session.ID, 20)
	if len(entries) != 3 {
		t.Fatalf("list holds %d entries; want the two sales and the balance checkout", len(entries))
	}
	if entries[0].ID != after.Sale.ID || entries[2].ID != before.Sale.ID {
		t.Fatalf("list is not newest first around the balance checkout: %v, %v, %v", entries[0].ID, entries[1].ID, entries[2].ID)
	}

	balance := entries[1]
	if balance.CashMovementID != nil {
		t.Errorf("balance-only entry names a cash movement %v; want none", balance.CashMovementID)
	}
	if balance.Total != 350_000 || balance.Method != "transfer" || balance.SessionID != session.ID {
		t.Errorf("balance-only entry = total %d method %q session %v; want 350000, transfer, the session", balance.Total, balance.Method, balance.SessionID)
	}
	if balance.IsVoided() {
		t.Error("balance-only entry reads as voided")
	}
	items, err := f.Stores.Sales.ListItemsBySaleIDs(ctx, f.ComplexID, []uuid.UUID{balance.ID})
	if err != nil {
		t.Fatalf("items of the balance-only entry: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("balance-only entry has %d product lines; want none", len(items))
	}
	lines, err := f.Stores.Sales.ListBookingPaymentsByEntryIDs(ctx, f.ComplexID, []uuid.UUID{balance.ID})
	if err != nil {
		t.Fatalf("balance lines of the entry: %v", err)
	}
	if got := lines[balance.ID]; len(got) != 1 || got[0].Amount != 350_000 || got[0].BookingID != booking.ID {
		t.Errorf("balance-only entry lines = %+v; want the one 350000 balance of the booking", got)
	}

	if got := listSales(t, f, nil, 20); len(got) != 3 {
		t.Errorf("complex-wide list holds %d entries; want 3", len(got))
	}
}

// TestIntegration_ListPagesThroughBalanceOnlyCheckoutsInOrder: the sales list
// is one keyset page sequence. A page of one entry at a time must visit every
// entry, product sales and balance checkouts alike, in the same order a single
// page returns them.
func TestIntegration_ListPagesThroughBalanceOnlyCheckoutsInOrder(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	product := insertProduct(t, f, "Overgrip", 800, true)
	openSession(t, f, 0)
	booking := bookingOn(t, f, artDay(time.Now()), datatest.BookingOptions{StartTime: "18:00", EndTime: "19:00"})

	if _, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 1}}, nil, "cash", nil); err != nil {
		t.Fatalf("product checkout: %v", err)
	}
	if _, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID, nil,
		[]salestore.BookingPaymentInput{{BookingID: booking.ID, Amount: 350_000}}, "cash", nil); err != nil {
		t.Fatalf("balance-only checkout: %v", err)
	}
	if _, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID,
		[]salestore.ItemInput{{ProductID: product.ID, Quantity: 1}}, nil, "cash", nil); err != nil {
		t.Fatalf("second product checkout: %v", err)
	}

	whole := listSales(t, f, nil, 20)
	paged := listSales(t, f, nil, 1)
	if len(whole) != 3 || len(paged) != 3 {
		t.Fatalf("whole list %d entries, paged list %d; want 3 and 3", len(whole), len(paged))
	}
	for i := range whole {
		if paged[i].ID != whole[i].ID {
			t.Errorf("page %d holds entry %v; the single page holds %v", i, paged[i].ID, whole[i].ID)
		}
	}
}

// TestIntegration_VoidRefusesABalanceOnlyCheckout: the balance-only entry
// answers the same refusal as a sale that collected a balance, and its lines
// stay as they were.
func TestIntegration_VoidRefusesABalanceOnlyCheckout(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	openSession(t, f, 0)
	booking := bookingOn(t, f, artDay(time.Now()), datatest.BookingOptions{StartTime: "18:00", EndTime: "19:00"})
	if _, err := f.Stores.Sales.Checkout(ctx, f.ComplexID, f.UserID, nil,
		[]salestore.BookingPaymentInput{{BookingID: booking.ID, Amount: 350_000}}, "cash", nil); err != nil {
		t.Fatalf("balance-only checkout: %v", err)
	}
	entries := listSales(t, f, nil, 20)
	if len(entries) != 1 {
		t.Fatalf("list holds %d entries; want the balance checkout", len(entries))
	}

	if _, err := f.Stores.Sales.Void(ctx, f.ComplexID, entries[0].ID, f.UserID, nil); !errors.Is(err, salestore.ErrSaleHasBookingPayments) {
		t.Fatalf("void of a balance-only checkout = %v; want ErrSaleHasBookingPayments", err)
	}
	got, err := f.Stores.Sales.GetByID(ctx, f.ComplexID, entries[0].ID)
	if err != nil {
		t.Fatalf("reading the balance-only entry after a refused void: %v", err)
	}
	if got.IsVoided() || got.Total != 350_000 {
		t.Errorf("refused void changed the entry: voided=%v total=%d", got.IsVoided(), got.Total)
	}
}
