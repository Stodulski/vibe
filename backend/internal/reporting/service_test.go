package reporting

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// occupancyCourts answers with the given number of active courts, plus one
// retired court that must not count as capacity.
type occupancyCourts struct{ active int }

func (c occupancyCourts) GetByComplex(context.Context, uuid.UUID) ([]*courtstore.Court, error) {
	out := make([]*courtstore.Court, 0, c.active+1)
	for range c.active {
		out = append(out, &courtstore.Court{ID: uuid.New(), IsActive: true})
	}
	return append(out, &courtstore.Court{ID: uuid.New(), IsActive: false}), nil
}

// occupancyBookings answers the occupancy query with one slot's count.
type occupancyBookings struct {
	stubBookings
	count int
}

func (b occupancyBookings) GetOccupancyByHourDay(context.Context, uuid.UUID, time.Time, time.Time) ([]bookingstore.OccupancyDataPoint, error) {
	return []bookingstore.OccupancyDataPoint{{DayOfWeek: 3, Hour: 19, BookingCount: b.count}}, nil
}

// TestServiceOccupancyChart covers the arithmetic that moved out of the
// handler: the denominator is the courts a venue currently trades on times the
// weeks looked at, a retired court is not capacity, and the share never reads
// above 100.
func TestServiceOccupancyChart(t *testing.T) {
	tests := []struct {
		name    string
		courts  int
		weeks   int
		count   int
		wantPct int
	}{
		{name: "half the slots booked", courts: 2, weeks: 4, count: 4, wantPct: 50},
		{name: "every slot booked", courts: 2, weeks: 4, count: 8, wantPct: 100},
		{name: "more bookings than capacity still reads as full", courts: 1, weeks: 1, count: 5, wantPct: 100},
		{name: "no active courts is no capacity, not a division by zero", courts: 0, weeks: 4, count: 3, wantPct: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(
				occupancyBookings{count: tt.count},
				stubClients{},
				occupancyCourts{active: tt.courts},
				stubSchedules{},
				&stubReports{},
				ExportDeps{},
				50*time.Second,
			)

			got, err := svc.OccupancyChart(t.Context(), uuid.New(), time.Now(), tt.weeks)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d slots, want 1", len(got))
			}
			if got[0].Percentage != tt.wantPct {
				t.Errorf("got %d%%, want %d%%", got[0].Percentage, tt.wantPct)
			}
			if got[0].DayOfWeek != 3 || got[0].Hour != 19 {
				t.Errorf("the slot's own coordinates were not carried through: %+v", got[0])
			}
		})
	}
}

// The dashboard's occupancy rate is a day's figure, so it is computed against
// the product's calendar rather than the server's.
func TestServiceDashboardStatsUsesTheProductsCalendar(t *testing.T) {
	var asked time.Time
	svc := NewService(
		dayCapturingBookings{asked: &asked},
		stubClients{},
		occupancyCourts{active: 1},
		stubSchedules{},
		&stubReports{},
		ExportDeps{},
		50*time.Second,
	)

	// 00:30 in Buenos Aires is already the next day in UTC.
	now := time.Date(2026, 3, 12, 0, 30, 0, 0, timezone.Argentina)
	if _, err := svc.DashboardStats(t.Context(), uuid.New(), now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if asked.Day() != 12 || asked.Month() != time.March {
		t.Errorf("the day asked about was %s, want 2026-03-12 — a UTC-anchored day is three hours off", asked.Format(time.RFC3339))
	}
}

type dayCapturingBookings struct {
	stubBookings
	asked *time.Time
}

func (b dayCapturingBookings) GetDashboardStats(_ context.Context, _ uuid.UUID, today time.Time) (*bookingstore.DashboardStats, error) {
	*b.asked = today
	return &bookingstore.DashboardStats{}, nil
}
