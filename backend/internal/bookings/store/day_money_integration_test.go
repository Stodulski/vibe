//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// argentinaNoonOffset returns noon, in Argentina's wall-clock, on the day
// dayOffset days from today (0 = today, -1 = yesterday) — a time comfortably
// inside the calendar day on either side of midnight, in either the
// server's or the database's timezone.
func argentinaNoonOffset(dayOffset int) time.Time {
	today := timezone.Today()
	d := today.AddDate(0, 0, dayOffset)
	return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, timezone.Argentina)
}

// insertCashSession opens a session directly (bypassing the cashbox service,
// which this package must not depend on) so the test can control opened_at
// and land movements against it.
func insertCashSession(t *testing.T, f *datatest.Fixture) (sessionID uuid.UUID) {
	t.Helper()

	err := f.DB.QueryRow(context.Background(),
		`INSERT INTO cash_sessions (complex_id, opened_by, opening_cash, opening_note)
		 VALUES ($1, $2, 0, NULL) RETURNING id`,
		f.ComplexID, f.UserID,
	).Scan(&sessionID)
	if err != nil {
		t.Fatalf("opening cash session: %v", err)
	}
	return sessionID
}

// insertMovement writes a cash_movements row directly, at the given created_at,
// bypassing the cashbox service's own validation — this package tests the
// day-money query's SQL, not the movement lifecycle, which cashbox's own
// integration tests already cover.
func insertMovement(t *testing.T, f *datatest.Fixture, sessionID uuid.UUID, kind, category, method string, amount int, createdAt time.Time, voidsID *uuid.UUID) (id uuid.UUID) {
	t.Helper()

	err := f.DB.QueryRow(context.Background(),
		`INSERT INTO cash_movements (complex_id, session_id, kind, category, method, amount, created_at, created_by, voids_movement_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		f.ComplexID, sessionID, kind, category, method, amount, createdAt, f.UserID, voidsID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("inserting %s/%s movement: %v", kind, category, err)
	}
	return id
}

func backdatePaymentCreatedAt(t *testing.T, f *datatest.Fixture, paymentID uuid.UUID, when time.Time) {
	t.Helper()

	tag, err := f.DB.Exec(context.Background(),
		`UPDATE payments SET created_at = $2 WHERE id = $1`, paymentID, when)
	if err != nil {
		t.Fatalf("backdating payment %s: %v", paymentID, err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("backdating payment %s: want 1 row affected, got %d", paymentID, tag.RowsAffected())
	}
}

// TestGetDayMoneyTotalsCombinesBookingsAndMovementsNettingVoids is the
// acceptance scenario from odd/tasks/dashboard-today-card.md: a counter cash
// booking payment, a transfer booking payment, a bar sale, a manual income
// and an expense all made today, a voided movement that must net to zero,
// and yesterday's data, which must not appear at all.
func TestGetDayMoneyTotalsCombinesBookingsAndMovementsNettingVoids(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	todayNoon := argentinaNoonOffset(0)
	yesterdayNoon := argentinaNoonOffset(-1)

	// A counter cash booking payment today.
	cashBooking := f.CreateBooking(t, datatest.BookingOptions{StartTime: "10:00", EndTime: "11:00"})
	cashPayment := f.CreatePayment(t, cashBooking.ID, 10_000, 0, nil)
	backdatePaymentCreatedAt(t, f, cashPayment.ID, todayNoon)

	// A transfer booking payment today — CreatePayment only builds cash/mercadopago,
	// so this one is written directly with method='transfer'.
	transferBooking := f.CreateBooking(t, datatest.BookingOptions{StartTime: "11:00", EndTime: "12:00"})
	var transferPaymentID uuid.UUID
	if err := f.DB.QueryRow(ctx,
		`INSERT INTO payments (booking_id, complex_id, amount, service_fee, method, status, created_at)
		 VALUES ($1, $2, $3, 0, 'transfer', 'deposit_paid', $4) RETURNING id`,
		transferBooking.ID, f.ComplexID, 20_000, todayNoon,
	).Scan(&transferPaymentID); err != nil {
		t.Fatalf("inserting transfer payment: %v", err)
	}

	session := insertCashSession(t, f)

	// A bar sale today, cash.
	insertMovement(t, f, session, "income", "sale", "cash", 5_000, todayNoon, nil)

	// A manual income today, transfer.
	insertMovement(t, f, session, "income", "classes", "transfer", 3_000, todayNoon, nil)

	// An expense today, cash.
	insertMovement(t, f, session, "expense", "supplies", "cash", 1_000, todayNoon, nil)

	// A voided movement today: the original bar sale plus its void must net
	// to zero and not appear in bar_sales or by_method at all.
	voidedID := insertMovement(t, f, session, "income", "sale", "cash", 7_000, todayNoon, nil)
	insertMovement(t, f, session, "expense", "sale", "cash", 7_000, todayNoon, &voidedID)

	// Yesterday's data — a booking payment and a bar sale — must be excluded
	// entirely.
	yesterdayBooking := f.CreateBooking(t, datatest.BookingOptions{StartTime: "12:00", EndTime: "13:00"})
	yesterdayPayment := f.CreatePayment(t, yesterdayBooking.ID, 99_000, 0, nil)
	backdatePaymentCreatedAt(t, f, yesterdayPayment.ID, yesterdayNoon)
	insertMovement(t, f, session, "income", "sale", "cash", 88_000, yesterdayNoon, nil)

	got, err := f.Stores.Bookings.GetDayMoneyTotals(ctx, f.ComplexID, timezone.Today())
	if err != nil {
		t.Fatalf("GetDayMoneyTotals: %v", err)
	}

	if got.Bookings != 30_000 {
		t.Errorf("Bookings = %d, want 30000 (10000 cash + 20000 transfer, yesterday's 99000 excluded)", got.Bookings)
	}
	if got.BarSales != 5_000 {
		t.Errorf("BarSales = %d, want 5000 — the voided 7000 sale must net to zero and yesterday's 88000 must be excluded", got.BarSales)
	}
	if got.OtherIncome != 3_000 {
		t.Errorf("OtherIncome = %d, want 3000", got.OtherIncome)
	}
	if got.Expenses != 1_000 {
		t.Errorf("Expenses = %d, want 1000", got.Expenses)
	}
	if got.TotalIncome != 38_000 {
		t.Errorf("TotalIncome = %d, want 38000 (30000 bookings + 5000 bar sales + 3000 other income)", got.TotalIncome)
	}

	wantByMethod := map[string]int64{"cash": 15_000, "transfer": 23_000}
	for method, want := range wantByMethod {
		if got.ByMethod[method] != want {
			t.Errorf("ByMethod[%q] = %d, want %d", method, got.ByMethod[method], want)
		}
	}
	if _, ok := got.ByMethod["mercadopago"]; ok && got.ByMethod["mercadopago"] != 0 {
		t.Errorf("ByMethod[mercadopago] = %d, want 0 or absent — no mercadopago activity today", got.ByMethod["mercadopago"])
	}
}
