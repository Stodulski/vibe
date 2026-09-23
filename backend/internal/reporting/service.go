package reporting

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// upcomingBookingLimit is how many of today's next bookings the dashboard
// lists.
const upcomingBookingLimit = 10

// ErrExportTooLarge reports a period whose detail sheet exceeds the row cap.
// It is refused rather than truncated: a ledger that stops silently on the
// twelfth still looks complete to whoever files it.
var ErrExportTooLarge = errors.New("export exceeds the row cap")

// Service holds this module's arithmetic: the occupancy rates, the period
// totals, and the workbook the export hands back. Every store call the module
// makes goes through it; nothing here touches HTTP.
type Service struct {
	bookings  BookingReader
	clients   ClientReader
	courts    CourtReader
	complexes ScheduleReader
	reports   PaymentReportReader

	// maxExportRows is defaultMaxExportRows, held as a field so a test can
	// exercise the cap boundary — which is an off-by-one in one comparison —
	// without building a fifty-thousand-row workbook to do it.
	maxExportRows int

	// exports is the queue and the private bucket a background export needs.
	// Both may be nil — a deployment with no R2 private bucket, and the unit
	// suite — and the export endpoints then answer 501. See
	// Service.ExportsConfigured.
	exports ExportDeps

	// exportBudget bounds the whole export — the query, the build and the
	// serialisation — so a client that hung up does not leave the handler
	// allocating for it forever (WriteTimeout closes the connection but does
	// not cancel the handler). It is a constructor argument rather than a
	// constant because it has to sit under HTTP_WRITE_TIMEOUT: cmd/api
	// derives it from that timeout and hands it in at wiring time.
	exportBudget time.Duration
}

// NewService returns a Service backed by the given readers, budgeting every
// synchronous export to exportBudget.
//
// exports carries the two dependencies only the background export needs, as
// one value rather than two more positional readers: they are meaningful only
// together (see ExportDeps) and a caller with neither passes the zero value.
func NewService(
	bookings BookingReader,
	clients ClientReader,
	courts CourtReader,
	complexes ScheduleReader,
	reports PaymentReportReader,
	exports ExportDeps,
	exportBudget time.Duration,
) *Service {
	return &Service{
		bookings:      bookings,
		clients:       clients,
		courts:        courts,
		complexes:     complexes,
		reports:       reports,
		exports:       exports,
		maxExportRows: defaultMaxExportRows,
		exportBudget:  exportBudget,
	}
}

// DashboardStats is the headline panel for today.
type DashboardStats struct {
	Stats          *bookingstore.DashboardStats
	TotalClients   int
	OccupancyRate  int
	Upcoming       []*bookingstore.Booking
	PaymentSummary *bookingstore.PaymentSummary
}

// DashboardStats aggregates today's figures for one complex.
//
// The occupancy rate is booked hours over the court-hours the venue is open
// for, capped at 100: a day with more booked minutes than open hours is a
// schedule that changed under a booking, not a venue over capacity.
//
// It is one cohesive read and splitting it would relocate sequential steps into
// helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (s *Service) DashboardStats(ctx context.Context, complexID uuid.UUID, now time.Time) (*DashboardStats, error) {
	today := startOfDay(now)
	nowTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	stats, err := s.bookings.GetDashboardStats(ctx, complexID, today)
	if err != nil {
		return nil, err
	}

	totalClients, err := s.clients.CountByComplex(ctx, complexID)
	if err != nil {
		return nil, err
	}

	// Calculate occupancy rate: (booked hours today) / (total available hours today) × 100.
	courts, err := s.courts.GetByComplex(ctx, complexID)
	if err != nil {
		return nil, err
	}
	activeCourts := countActive(courts)

	schedules, err := s.complexes.GetSchedules(ctx, complexID)
	if err != nil {
		return nil, err
	}

	dayName := slots.DayName(today.Weekday())
	var openHours float64
	for _, sc := range schedules {
		if sc.Day == dayName && !sc.IsClosed {
			openHours = slots.ToHours(sc.CloseTime) - slots.ToHours(sc.OpenTime)
			break
		}
	}

	var occupancyRate int
	totalSlotHours := float64(activeCourts) * openHours
	if totalSlotHours > 0 {
		bookedHours := float64(stats.TodayBookedMinutes) / 60.0
		occupancyRate = min(int((bookedHours/totalSlotHours)*100), 100)
	}

	upcoming, err := s.bookings.GetUpcomingToday(ctx, complexID, today, nowTime, upcomingBookingLimit)
	if err != nil {
		return nil, err
	}

	// Payment summary for today.
	paymentSummary, err := s.bookings.GetPaymentSummary(ctx, complexID, today)
	if err != nil {
		return nil, err
	}

	return &DashboardStats{
		Stats:          stats,
		TotalClients:   totalClients,
		OccupancyRate:  occupancyRate,
		Upcoming:       upcoming,
		PaymentSummary: paymentSummary,
	}, nil
}

// RevenueChart returns daily revenue over the requested period, which is either
// the last week or the last thirty days.
func (s *Service) RevenueChart(ctx context.Context, complexID uuid.UUID, now time.Time, period string) ([]bookingstore.RevenueDataPoint, error) {
	today := startOfDay(now)

	from := today.AddDate(0, 0, -6)
	if period == "month" {
		from = today.AddDate(0, 0, -29)
	}

	return s.bookings.GetRevenueByDay(ctx, complexID, from, today)
}

// OccupancySlot is the share of one (weekday, hour) slot that was booked.
type OccupancySlot struct {
	DayOfWeek  int `json:"day_of_week"`
	Hour       int `json:"hour"`
	Percentage int `json:"percentage"`
}

// OccupancyChart returns the share of open court-hours booked, by hour and
// weekday, over the given number of weeks.
//
// The denominator is the active courts times the weeks looked at: that is how
// many bookings one (weekday, hour) slot could have held.
func (s *Service) OccupancyChart(ctx context.Context, complexID uuid.UUID, now time.Time, weeks int) ([]OccupancySlot, error) {
	today := startOfDay(now)
	from := today.AddDate(0, 0, -7*weeks)

	rawData, err := s.bookings.GetOccupancyByHourDay(ctx, complexID, from, today)
	if err != nil {
		return nil, err
	}

	courts, err := s.courts.GetByComplex(ctx, complexID)
	if err != nil {
		return nil, err
	}

	maxPerSlot := countActive(courts) * weeks

	result := make([]OccupancySlot, len(rawData))
	for i, dp := range rawData {
		pct := 0
		if maxPerSlot > 0 {
			pct = min((dp.BookingCount*100)/maxPerSlot, 100)
		}
		result[i] = OccupancySlot{DayOfWeek: dp.DayOfWeek, Hour: dp.Hour, Percentage: pct}
	}
	return result, nil
}

// ClientInsights returns the retention and frequency panel.
func (s *Service) ClientInsights(ctx context.Context, complexID uuid.UUID, now time.Time) (*clientstore.ClientInsights, error) {
	return s.clients.GetInsights(ctx, complexID, startOfDay(now))
}

// MonthlySummary is one row of a MonthlyReport's payment totals — a single
// method, the whole period, or the month before — carried as a typed value
// between Service.MonthlyReport and toGenMonthlyReport rather than through a
// map[string]any and a JSON round trip, so an unknown key cannot be dropped
// silently and a type mismatch cannot surface as a 500 instead of a compile
// error. ServiceFees is only meaningful on Totals/PreviousTotals: a by-method
// row never carries it, matching gen.MonthlyReportSummary.ServiceFees being
// a nil-omitted pointer there.
type MonthlySummary struct {
	Count       int
	Total       int
	Refunded    int
	Net         int
	ServiceFees int
}

// MonthlyCourtSummary is one court's row in a MonthlyReport.
type MonthlyCourtSummary struct {
	CourtID   string
	CourtName string
	Count     int
	Total     int
	Refunded  int
	Net       int
}

// MonthlyReport is a period's payment totals: per method, per court, the
// whole period, and the month before for comparison.
type MonthlyReport struct {
	Month          int
	Year           int
	ByMethod       map[string]MonthlySummary
	ByCourt        []MonthlyCourtSummary
	Totals         MonthlySummary
	PreviousTotals MonthlySummary
}

// MonthlyReport totals a period's payments per method and per court, alongside
// the month before.
//
// That comparison is read through the same query, so the two months are counted
// identically. A period that predates the complex simply has no payments and
// totals zero — the period validation is not consulted for it, because the
// owner asked about THIS month and the comparison is context rather than a
// second request they made.
//
// It is one cohesive read — three queries and the arithmetic that turns them
// into the report — and splitting it would relocate sequential steps into
// helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (s *Service) MonthlyReport(ctx context.Context, complexID uuid.UUID, month, year int) (MonthlyReport, error) {
	from := periodStart(month, year)
	to := from.AddDate(0, 1, -1)

	summaries, err := s.reports.PaymentSummaryByMethod(ctx, complexID, from, to)
	if err != nil {
		return MonthlyReport{}, err
	}

	courts, err := s.reports.PaymentSummaryByCourt(ctx, complexID, from, to)
	if err != nil {
		return MonthlyReport{}, err
	}

	prevFrom := from.AddDate(0, -1, 0)
	prevSummaries, err := s.reports.PaymentSummaryByMethod(ctx, complexID, prevFrom, prevFrom.AddDate(0, 1, -1))
	if err != nil {
		return MonthlyReport{}, err
	}

	byMethod := make(map[string]MonthlySummary, len(summaries))
	var totalCount, totalAmount, totalServiceFees, totalRefunded int

	for _, sm := range summaries {
		// Net is what the owner keeps: the amount plus the service fee the
		// client paid on top, minus anything refunded.
		byMethod[sm.Method] = MonthlySummary{
			Count:    sm.Count,
			Total:    sm.Amount,
			Refunded: sm.Refunded,
			Net:      sm.Amount + sm.ServiceFee - sm.Refunded,
		}

		totalCount += sm.Count
		totalAmount += sm.Amount
		totalServiceFees += sm.ServiceFee
		totalRefunded += sm.Refunded
	}

	byCourt := make([]MonthlyCourtSummary, 0, len(courts))
	for _, c := range courts {
		byCourt = append(byCourt, MonthlyCourtSummary{
			CourtID:   c.CourtID,
			CourtName: c.CourtName,
			Count:     c.Count,
			Total:     c.Amount,
			Refunded:  c.Refunded,
			Net:       c.Amount + c.ServiceFee - c.Refunded,
		})
	}

	return MonthlyReport{
		Month:    month,
		Year:     year,
		ByMethod: byMethod,
		ByCourt:  byCourt,
		Totals: MonthlySummary{
			Count:       totalCount,
			Total:       totalAmount,
			ServiceFees: totalServiceFees,
			Refunded:    totalRefunded,
			Net:         totalAmount + totalServiceFees - totalRefunded,
		},
		PreviousTotals: periodTotals(prevSummaries),
	}, nil
}

// Export is a finished workbook and the name it downloads as.
type Export struct {
	Filename string
	Body     *bytes.Buffer
}

// ExportPaymentsExcel builds the two-sheet workbook: every payment in the
// period, and the same totals the JSON report returns.
//
// rowCount is the size of the detail sheet, meaningful alongside
// ErrExportTooLarge: the caller logs the real number behind the code the owner
// is given.
//
// The workbook is finished in full before it is handed back. It used to be
// written straight at the ResponseWriter, which commits a 200 with its first
// byte — so a failure halfway through arrived as a short .xlsx with a JSON
// error object stapled to the end, and Excel reported a corrupt file rather
// than the server reporting a problem.
func (s *Service) ExportPaymentsExcel(ctx context.Context, complex *complexstore.Complex, month, year int) (export *Export, rowCount int, err error) {
	ctx, cancel := context.WithTimeout(ctx, s.exportBudget)
	defer cancel()

	return s.buildPaymentsExport(ctx, complex.ID, complex.Name, complex.Slug, month, year)
}

// buildPaymentsExport is the workbook itself, with no budget of its own.
//
// The budget lives at the caller because the two callers have different ones
// and neither is this function's business: the synchronous handler bounds the
// build by a slice of the HTTP write timeout, because a client is holding a
// connection open for it, while the background worker runs under the job
// pool's own per-attempt timeout, because nobody is. A timeout in here would
// be a third one, silently the tightest.
//
// It takes the complex's id, name and slug rather than the record, because
// the worker has a payload rather than a row — and those three fields are the
// whole of what a workbook needs from one.
func (s *Service) buildPaymentsExport(ctx context.Context, complexID uuid.UUID, complexName, complexSlug string, month, year int) (export *Export, rowCount int, err error) {
	from := periodStart(month, year)
	to := from.AddDate(0, 1, -1)

	details, err := s.reports.PaymentDetails(ctx, complexID, from, to)
	if err != nil {
		return nil, 0, err
	}

	if len(details) > s.maxExportRows {
		return nil, len(details), ErrExportTooLarge
	}

	sums, err := s.readExportSummaries(ctx, complexID, from, to)
	if err != nil {
		return nil, len(details), err
	}

	buf, err := buildExportWorkbook(ctx, reportTitle(complexName, month, year), details,
		sums.byMethod, sums.byCourt, sums.previous, sums.cashSales, sums.cashByCategory)
	if err != nil {
		return nil, len(details), err
	}

	return &Export{
		Filename: fmt.Sprintf("pagos_%s_%d_%d.xlsx", complexSlug, month, year),
		Body:     buf,
	}, len(details), nil
}

// ExportRowCap is the largest detail sheet this service will build. It is
// exported for the caller's log line, which names the cap the refused export
// was measured against.
func (s *Service) ExportRowCap() int { return s.maxExportRows }

// startOfDay is midnight on the product's calendar, which is Argentina's. Every
// figure here is a day's figure, and a day is a calendar question.
func startOfDay(now time.Time) time.Time {
	local := now.In(timezone.Argentina)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, timezone.Argentina)
}

// periodStart is the first instant of a report's month, on the same calendar.
func periodStart(month, year int) time.Time {
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, timezone.Argentina)
}

// countActive counts the courts a complex currently trades on. A retired court
// is not capacity.
func countActive(courts []*courtstore.Court) int {
	n := 0
	for _, c := range courts {
		if c.IsActive {
			n++
		}
	}
	return n
}
