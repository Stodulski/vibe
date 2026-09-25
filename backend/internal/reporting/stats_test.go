package reporting

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
)

// TestToGenDayMoneyTotalsMapsEveryField proves every DayMoneyTotals figure
// reaches the wire type under its own name, converting the store's int64
// centavo sums to the wire type's int without losing a field along the way
// (HTTP-08: an explicit mapper is the thing under test).
func TestToGenDayMoneyTotalsMapsEveryField(t *testing.T) {
	dm := &bookingstore.DayMoneyTotals{
		Bookings:    150_00,
		BarSales:    80_00,
		OtherIncome: 20_00,
		Expenses:    30_00,
		TotalIncome: 250_00,
		ByMethod:    map[string]int64{"cash": 100_00, "mercadopago": 150_00},
	}

	got := toGenDayMoneyTotals(dm)

	if got.Bookings != 150_00 || got.BarSales != 80_00 || got.OtherIncome != 20_00 ||
		got.Expenses != 30_00 || got.TotalIncome != 250_00 {
		t.Fatalf("figures did not map through: %+v", got)
	}
	if len(got.ByMethod) != 2 || got.ByMethod["cash"] != 100_00 || got.ByMethod["mercadopago"] != 150_00 {
		t.Errorf("by_method did not map through: %+v", got.ByMethod)
	}
}

// TestToGenDayMoneyTotalsHandlesNil covers the defensive nil case: a wire
// body should never carry a nil map for by_method, which oapi-codegen would
// otherwise serialize as JSON null instead of {}.
func TestToGenDayMoneyTotalsHandlesNil(t *testing.T) {
	got := toGenDayMoneyTotals(nil)
	if got.ByMethod == nil {
		t.Error("ByMethod is nil, want an empty map so the wire body serializes {} not null")
	}
}

// dayMoneyBookings answers GetDashboardStats and GetDayMoneyTotals with a
// fixed day-money aggregate, so the service-level test below can prove
// DashboardStats actually carries it onto the response rather than dropping
// it on the floor.
type dayMoneyBookings struct {
	stubBookings
	totals *bookingstore.DayMoneyTotals
}

func (b dayMoneyBookings) GetDayMoneyTotals(context.Context, uuid.UUID, time.Time) (*bookingstore.DayMoneyTotals, error) {
	return b.totals, nil
}

// TestServiceDashboardStatsCarriesDayMoney proves the service's
// DashboardStats aggregate includes the store's day-money totals, not just
// the pre-existing today_revenue/payment_summary figures.
func TestServiceDashboardStatsCarriesDayMoney(t *testing.T) {
	want := &bookingstore.DayMoneyTotals{
		Bookings:    1000,
		BarSales:    500,
		OtherIncome: 200,
		Expenses:    100,
		TotalIncome: 1700,
		ByMethod:    map[string]int64{"cash": 1700},
	}

	svc := NewService(
		dayMoneyBookings{totals: want},
		stubClients{},
		stubCourts{},
		stubSchedules{},
		&stubReports{},
		ExportDeps{},
		50*time.Second,
	)

	dash, err := svc.DashboardStats(t.Context(), uuid.New(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dash.DayMoney != want {
		t.Errorf("DashboardStats.DayMoney = %+v, want the store's totals carried through unchanged", dash.DayMoney)
	}
}
