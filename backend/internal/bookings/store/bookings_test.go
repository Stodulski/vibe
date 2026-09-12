package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stodulski/vibe-server/internal/data"
)

func TestBooking_StructFields(t *testing.T) {
	now := time.Now()
	createdBy := uuid.New()
	notes := "VIP client"
	b := Booking{
		ID:               uuid.New(),
		ComplexID:        uuid.New(),
		CourtID:          uuid.New(),
		ClientID:         uuid.New(),
		Date:             time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		StartTime:        "09:00",
		DurationMinutes:  90,
		Price:            15000,
		DepositAmount:    7500,
		Status:           "confirmed",
		CollectionStatus: CollectionStatusDepositPaid,
		RefundStatus:     RefundStatusNone,
		ReminderSent2h:   false,
		Notes:            &notes,
		CreatedBy:        &createdBy,
		CreatedAt:        now,
		UpdatedAt:        now,
		CourtName:        "Court 1",
		ClientName:       "John Doe",
		ClientPhone:      "+5491155551234",
	}

	if b.DurationMinutes != 90 {
		t.Errorf("DurationMinutes = %d, want %d", b.DurationMinutes, 90)
	}
	if b.Price != 15000 {
		t.Errorf("Price = %d, want %d", b.Price, 15000)
	}
	if b.DepositAmount != 7500 {
		t.Errorf("DepositAmount = %d, want %d", b.DepositAmount, 7500)
	}
	if b.Status != "confirmed" {
		t.Errorf("Status = %q, want %q", b.Status, "confirmed")
	}
	if b.Notes == nil || *b.Notes != "VIP client" {
		t.Errorf("Notes = %v, want %q", b.Notes, "VIP client")
	}
	if b.CourtName != "Court 1" {
		t.Errorf("CourtName = %q, want %q", b.CourtName, "Court 1")
	}
}

func TestBooking_NilOptionalFields(t *testing.T) {
	b := Booking{
		ID:     uuid.New(),
		Status: "pending",
	}

	if b.Notes != nil {
		t.Error("Notes should be nil")
	}
	if b.CreatedBy != nil {
		t.Error("CreatedBy should be nil")
	}
}

func TestDashboardStats_StructFields(t *testing.T) {
	stats := DashboardStats{
		TodayBookings:      10,
		TodayBookedMinutes: 900,
		TodayRevenue:       150000,
		YesterdayBookings:  8,
		YesterdayRevenue:   120000,
		WeeklyRevenue:      750000,
		MonthlyRevenue:     3000000,
		PendingBookings:    3,
	}

	if stats.TodayBookings != 10 {
		t.Errorf("TodayBookings = %d, want %d", stats.TodayBookings, 10)
	}
	if stats.TodayBookedMinutes != 900 {
		t.Errorf("TodayBookedMinutes = %d, want %d", stats.TodayBookedMinutes, 900)
	}
	if stats.PendingBookings != 3 {
		t.Errorf("PendingBookings = %d, want %d", stats.PendingBookings, 3)
	}
}

func TestRevenueDataPoint_StructFields(t *testing.T) {
	dp := RevenueDataPoint{
		Date:   "2025-06-15",
		Amount: 50000,
	}

	if dp.Amount != 50000 {
		t.Errorf("Amount = %d, want %d", dp.Amount, 50000)
	}
	if dp.Date == "" {
		t.Error("Date should not be empty")
	}
}

func TestOccupancyDataPoint_StructFields(t *testing.T) {
	dp := OccupancyDataPoint{
		DayOfWeek:    1,
		Hour:         14,
		BookingCount: 5,
	}

	if dp.DayOfWeek != 1 {
		t.Errorf("DayOfWeek = %d, want %d", dp.DayOfWeek, 1)
	}
	if dp.Hour != 14 {
		t.Errorf("Hour = %d, want %d", dp.Hour, 14)
	}
	if dp.BookingCount != 5 {
		t.Errorf("BookingCount = %d, want %d", dp.BookingCount, 5)
	}
}

func TestCronBooking_StructFields(t *testing.T) {
	mpToken := "access-token-123"
	mpRefresh := "refresh-token-456"
	cb := CronBooking{
		Booking: Booking{
			ID:     uuid.New(),
			Status: "confirmed",
		},
		ClientPhone:       "+5491155551234",
		ClientEmail:       "client@example.com",
		ComplexName:       "Padel Club",
		CourtName:         "Court A",
		mpAccessToken:     &mpToken,
		mpRefreshToken:    &mpRefresh,
		CancellationHours: 24,
	}

	if cb.ClientPhone != "+5491155551234" {
		t.Errorf("ClientPhone = %q, want %q", cb.ClientPhone, "+5491155551234")
	}
	if cb.ComplexName != "Padel Club" {
		t.Errorf("ComplexName = %q, want %q", cb.ComplexName, "Padel Club")
	}
	if cb.CancellationHours != 24 {
		t.Errorf("CancellationHours = %d, want %d", cb.CancellationHours, 24)
	}
	if cb.mpAccessToken == nil || *cb.mpAccessToken != "access-token-123" {
		t.Errorf("mpAccessToken unexpected value")
	}
	if cb.Status != "confirmed" {
		t.Errorf("embedded Booking.Status = %q, want %q", cb.Status, "confirmed")
	}
}

func TestPaymentSummary_StructFields(t *testing.T) {
	summary := PaymentSummary{
		ByStatus: map[string]PaymentStatusBreakdown{
			"deposit_paid": {Count: 5, Total: 37500},
			"fully_paid":   {Count: 3, Total: 45000},
		},
		ByMethod: map[string]int{
			"mercadopago": 60000,
			"cash":        22500,
		},
	}

	if summary.ByStatus["deposit_paid"].Count != 5 {
		t.Errorf("deposit_paid count = %d, want %d", summary.ByStatus["deposit_paid"].Count, 5)
	}
	if summary.ByMethod["mercadopago"] != 60000 {
		t.Errorf("mercadopago amount = %d, want %d", summary.ByMethod["mercadopago"], 60000)
	}
}

// TestBookingModel_RequiresDB documents that Store methods all require a database.
func TestBookingModel_RequiresDB(t *testing.T) {
	t.Skip("Store methods all require *pgxpool.Pool and *db.Queries")
}

// The refusal a client can act on depends on this mapping, and the code that
// carries it moved: the exclusion constraint retired idx_bookings_no_double, so a
// double booking is now refused by an exclusion constraint (23P01) rather than
// a unique index (23505). Both are checked, against the SQLSTATE PostgreSQL
// actually emits for each.
func TestIsSlotAlreadySold(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"exclusion violation — what bookings_no_overlapping_span raises today", &pgconn.PgError{Code: "23P01"}, true},
		{"unique violation — what the dropped index used to raise", &pgconn.PgError{Code: "23505"}, true},
		{"wrapped, because callers see it through fmt.Errorf", fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23P01"}), true},
		{"a check constraint is a different refusal and must surface", &pgconn.PgError{Code: "23514"}, false},
		{"a foreign key violation is not a taken slot", &pgconn.PgError{Code: "23503"}, false},
		{"a plain error is not a database refusal at all", errors.New("connection reset"), false},
		{"no error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSlotAlreadySold(tt.err); got != tt.want {
				t.Errorf("isSlotAlreadySold = %v, want %v", got, tt.want)
			}
		})
	}
}

// The store refuses a write whose row belongs to another tenant, and one whose
// caller declared no tenant at all. Neither refusal depends on the HTTP chain
// having run, which is the point: the chain is where every other check lives.
func TestTheBookingWritesAssertTheirTenant(t *testing.T) {
	own, other := uuid.New(), uuid.New()
	store := &Store{}

	b := &Booking{ComplexID: own}

	if err := store.Insert(data.ContextWithTenant(context.Background(), other), b); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("Insert for another tenant: want ErrRecordNotFound; got %v", err)
	}
	if err := store.Insert(context.Background(), b); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("Insert with no tenant on the context: want ErrRecordNotFound; got %v", err)
	}
	if err := store.Update(data.ContextWithTenant(context.Background(), other), b); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("Update for another tenant: want ErrRecordNotFound; got %v", err)
	}
	if err := store.InsertSafe(data.ContextWithTenant(context.Background(), other), b); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("InsertSafe for another tenant: want ErrRecordNotFound; got %v", err)
	}
	if err := store.CancelFutureByComplex(data.ContextWithTenant(context.Background(), other), own); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("CancelFutureByComplex for another tenant: want ErrRecordNotFound; got %v", err)
	}
}
