package reporting

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

type stubReports struct {
	summaries      []reportstore.PaymentMethodSummary
	courtSummaries []reportstore.PaymentCourtSummary
	details        []reportstore.PaymentDetail
	err            error

	// lastFrom and lastTo record the FIRST period the handler asked for,
	// which is the month the owner selected. The handler reads the month
	// before it as well, for the comparison, so recording only the most
	// recent call would leave every assertion here looking at the wrong
	// month — it would still pass, one month off, and say nothing.
	lastFrom, lastTo time.Time
	// periods records every range asked for, in order, so a test can assert
	// that the comparison read the month before and not some other one.
	periods []struct{ from, to time.Time }

	lastCourtFrom, lastCourtTo time.Time
}

func (s *stubReports) PaymentSummaryByMethod(_ context.Context, _ uuid.UUID, from, to time.Time) ([]reportstore.PaymentMethodSummary, error) {
	if len(s.periods) == 0 {
		s.lastFrom, s.lastTo = from, to
	}
	s.periods = append(s.periods, struct{ from, to time.Time }{from, to})
	return s.summaries, s.err
}

// PaymentSummaryByCourt records the range the same way, so a test asserting
// which month was read gets the same answer whichever breakdown it inspects.
func (s *stubReports) PaymentSummaryByCourt(_ context.Context, _ uuid.UUID, from, to time.Time) ([]reportstore.PaymentCourtSummary, error) {
	s.lastCourtFrom, s.lastCourtTo = from, to
	return s.courtSummaries, s.err
}

func (s *stubReports) PaymentDetails(_ context.Context, _ uuid.UUID, from, to time.Time) ([]reportstore.PaymentDetail, error) {
	s.lastFrom, s.lastTo = from, to
	return s.details, s.err
}

// The remaining readers are unused by the report endpoints under test; they
// return zero values so a Handler can be constructed.
type stubBookings struct{}

func (stubBookings) GetDashboardStats(context.Context, uuid.UUID, time.Time) (*bookingstore.DashboardStats, error) {
	return &bookingstore.DashboardStats{}, nil
}
func (stubBookings) GetUpcomingToday(context.Context, uuid.UUID, time.Time, string, int) ([]*bookingstore.Booking, error) {
	return nil, nil
}
func (stubBookings) GetPaymentSummary(context.Context, uuid.UUID, time.Time) (*bookingstore.PaymentSummary, error) {
	return &bookingstore.PaymentSummary{}, nil
}
func (stubBookings) GetRevenueByDay(context.Context, uuid.UUID, time.Time, time.Time) ([]bookingstore.RevenueDataPoint, error) {
	return nil, nil
}
func (stubBookings) GetOccupancyByHourDay(context.Context, uuid.UUID, time.Time, time.Time) ([]bookingstore.OccupancyDataPoint, error) {
	return nil, nil
}

type stubClients struct{}

func (stubClients) CountByComplex(context.Context, uuid.UUID) (int, error) { return 0, nil }
func (stubClients) GetInsights(context.Context, uuid.UUID, time.Time) (*clientstore.ClientInsights, error) {
	return &clientstore.ClientInsights{}, nil
}

type stubCourts struct{}

func (stubCourts) GetByComplex(context.Context, uuid.UUID) ([]*courtstore.Court, error) {
	return nil, nil
}

type stubSchedules struct{}

func (stubSchedules) GetSchedules(context.Context, uuid.UUID) ([]*complexstore.Schedule, error) {
	return nil, nil
}

func newTestHandler(reports PaymentReportReader) *Handler {
	return newTestHandlerWithService(newTestService(reports))
}

// newTestService builds the service the handler under test is backed by, so a
// test that needs to reach past the HTTP layer — the export row cap is the one
// that does — holds the same instance.
func newTestService(reports PaymentReportReader) *Service {
	return NewService(stubBookings{}, stubClients{}, stubCourts{}, stubSchedules{}, reports)
}

func newTestHandlerWithService(svc *Service) *Handler {
	responder := httpx.NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	return NewHandler(svc, responder)
}

// reportRequest builds a request for a period, already carrying a complex that
// has existed long enough to have data.
func reportRequest(t *testing.T, query string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+query, nil)
	return httpx.ContextSetComplex(r, &complexstore.Complex{
		ID:        uuid.New(),
		Name:      "Vibe Palermo",
		CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, timezone.Argentina),
	})
}

func TestMonthlyReportTotalsEachMethodAndTheWhole(t *testing.T) {
	reports := &stubReports{summaries: []reportstore.PaymentMethodSummary{
		{Method: "mercadopago", Count: 3, Amount: 300_000, ServiceFee: 21_000, Refunded: 100_000},
		{Method: "cash", Count: 1, Amount: 100_000, ServiceFee: 0, Refunded: 0},
	}}

	h := newTestHandler(reports)
	w := httptest.NewRecorder()
	h.GetMonthlyReport(w, reportRequest(t, "?month=3&year=2026"))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	var body struct {
		Report struct {
			ByMethod map[string]struct {
				Count    int `json:"count"`
				Total    int `json:"total"`
				Refunded int `json:"refunded"`
				Net      int `json:"net"`
			} `json:"by_method"`
		} `json:"report"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, w.Body.String())
	}

	mp := body.Report.ByMethod["mercadopago"]
	// Net is the amount plus the service fee the client paid on top, less refunds.
	if want := 300_000 + 21_000 - 100_000; mp.Net != want {
		t.Errorf("want net %d; got %d", want, mp.Net)
	}
	if mp.Count != 3 || mp.Refunded != 100_000 {
		t.Errorf("mercadopago row is wrong: %+v", mp)
	}
	if body.Report.ByMethod["cash"].Net != 100_000 {
		t.Errorf("cash net is wrong: %+v", body.Report.ByMethod["cash"])
	}
}

// The period must be resolved in the product's timezone, not UTC, or a report
// for March starts in February.
func TestMonthlyReportAsksForTheWholeRequestedMonth(t *testing.T) {
	reports := &stubReports{}

	h := newTestHandler(reports)
	w := httptest.NewRecorder()
	h.GetMonthlyReport(w, reportRequest(t, "?month=3&year=2026"))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d", w.Code)
	}
	if got := reports.lastFrom.Format("2006-01-02"); got != "2026-03-01" {
		t.Errorf("want the period to start on 2026-03-01; got %s", got)
	}
	if got := reports.lastTo.Format("2006-01-02"); got != "2026-03-31" {
		t.Errorf("want the period to end on 2026-03-31; got %s", got)
	}
	if reports.lastFrom.Location() != timezone.Argentina {
		t.Errorf("the period must be anchored to the product timezone; got %v", reports.lastFrom.Location())
	}
}

func TestMonthlyReportRejectsInvalidPeriods(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, timezone.Argentina)
	created := time.Date(2026, 3, 10, 0, 0, 0, 0, timezone.Argentina)

	tests := []struct {
		name        string
		month, year int
		wantErr     bool
	}{
		{"month zero", 0, 2026, true},
		{"month thirteen", 13, 2026, true},
		{"next year", 6, 2027, true},
		{"later this year", 9, 2026, true},
		{"before the complex existed", 2, 2026, true},
		{"the month it was created", 3, 2026, false},
		{"a month in between", 5, 2026, false},
		{"the current month", 8, 2026, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReportPeriod(tt.month, tt.year, now, created)
			if tt.wantErr && err == nil {
				t.Error("want an error; got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("want no error; got %v", err)
			}
		})
	}
}

func TestMonthlyReportReportsAnInvalidPeriodAsBadRequest(t *testing.T) {
	h := newTestHandler(&stubReports{})
	w := httptest.NewRecorder()
	h.GetMonthlyReport(w, reportRequest(t, "?month=13&year=2026"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400; got %d", w.Code)
	}
}

// H-05: ReadInt silently turned a non-numeric month/year into "the current
// period" and answered 200 with that period's real money, while a
// syntactically valid but out-of-range value (month=13) was correctly
// refused by validateReportPeriod. Both spellings of a malformed period must
// now refuse the same way.
func TestMonthlyReportRejectsANonNumericPeriodInsteadOfDefaulting(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"non-numeric month", "?month=abc&year=2026"},
		{"non-numeric year", "?month=3&year=abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reports := &stubReports{}
			h := newTestHandler(reports)
			w := httptest.NewRecorder()
			h.GetMonthlyReport(w, reportRequest(t, tt.query))

			if w.Code != http.StatusBadRequest {
				t.Fatalf("want 400; got %d (body: %s)", w.Code, w.Body.String())
			}
			// The store must never have been asked for a period at all — the
			// old behaviour's defect was answering 200 with a real period's
			// figures for input nobody sent.
			if len(reports.periods) != 0 {
				t.Errorf("must not query any period for malformed input; queried %v", reports.periods)
			}

			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Error != httpx.CodeInvalidReportPeriod {
				t.Errorf("want %q; got %q", httpx.CodeInvalidReportPeriod, body.Error)
			}
		})
	}
}

// Same defect, same fix, on the export path.
func TestExportRejectsANonNumericPeriodInsteadOfDefaulting(t *testing.T) {
	reports := &stubReports{details: []reportstore.PaymentDetail{{ClientName: "Cliente 0"}}}
	h := newTestHandler(reports)
	w := httptest.NewRecorder()
	h.ExportPaymentsExcel(w, reportRequest(t, "?month=abc&year=2026"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400; got %d (body: %s)", w.Code, w.Body.String())
	}
	if len(reports.periods) != 0 || !reports.lastFrom.IsZero() {
		t.Errorf("must not have read any payment data for malformed input")
	}
}

func TestMonthlyReportReportsStoreFailure(t *testing.T) {
	h := newTestHandler(&stubReports{err: errors.New("db down")})
	w := httptest.NewRecorder()
	h.GetMonthlyReport(w, reportRequest(t, "?month=3&year=2026"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500; got %d", w.Code)
	}
}

func TestReportHandlersRequireTheComplexInContext(t *testing.T) {
	h := newTestHandler(&stubReports{})

	for name, call := range map[string]http.HandlerFunc{
		"monthly": h.GetMonthlyReport,
		"export":  h.ExportPaymentsExcel,
		"stats":   h.GetDashboardStats,
		"revenue": h.GetRevenueChart,
		"clients": h.GetClientInsights,
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			call(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

			if w.Code != http.StatusInternalServerError {
				t.Errorf("want 500 with no complex in context; got %d", w.Code)
			}
		})
	}
}

func TestRevenueChartPeriod(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"absent defaults to week", "", http.StatusOK},
		{"week is allowed", "?period=week", http.StatusOK},
		{"month is allowed", "?period=month", http.StatusOK},
		{"unknown value is rejected", "?period=bogus", http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(&stubReports{})
			w := httptest.NewRecorder()
			h.GetRevenueChart(w, reportRequest(t, tt.query))

			if w.Code != tt.wantStatus {
				t.Fatalf("want %d; got %d (body: %s)", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusUnprocessableEntity {
				var body struct {
					Error map[string]string `json:"error"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if _, ok := body.Error["period"]; !ok {
					t.Errorf("want a validation error on the period field; got %v", body.Error)
				}
			}
		})
	}
}

func TestExportProducesAWorkbook(t *testing.T) {
	reports := &stubReports{
		details: []reportstore.PaymentDetail{{
			CreatedAt: time.Date(2026, 3, 4, 18, 30, 0, 0, timezone.Argentina),
			StartsAt:  time.Date(2026, 3, 4, 18, 0, 0, 0, timezone.Argentina),
			EndsAt:    time.Date(2026, 3, 4, 19, 30, 0, 0, timezone.Argentina),
			CourtName: "Cancha 1", ClientName: "Ana Perez", ClientPhone: "+5411",
			BookingPrice: 500_000, Amount: 150_000, ServiceFee: 100_000, RefundAmount: 0,
			Method: "mercadopago", PaymentStatus: "deposit_paid", BookingStatus: "confirmed",
		}},
		summaries: []reportstore.PaymentMethodSummary{
			{Method: "mercadopago", Count: 1, Amount: 150_000, ServiceFee: 100_000},
		},
	}

	h := newTestHandler(reports)
	w := httptest.NewRecorder()
	h.ExportPaymentsExcel(w, reportRequest(t, "?month=3&year=2026"))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "spreadsheet") {
		t.Errorf("want a spreadsheet content type; got %q", got)
	}
	if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment") {
		t.Errorf("the export must download rather than render; got %q", got)
	}
	// A .xlsx file is a zip archive, so it starts with the zip magic bytes.
	if body := w.Body.Bytes(); len(body) < 4 || string(body[:2]) != "PK" {
		t.Errorf("the body is not a workbook; first bytes %q", string(body[:min(4, len(body))]))
	}
}

// H-06: client_first_name on the PUBLIC booking form (no account required)
// reached the exported cell unescaped. RPT-10 booked with the first name
// "=cmd|' /C calc'!A0", downloaded the export, and the stored cell came back
// as exactly that string — a spreadsheet opening the file reads a leading
// '=' as a formula, not as the text someone typed. This exercises the whole
// handler, not just escapeFormulaCell in isolation, so a regression that
// wires the guard out of the write path (rather than breaking the guard
// itself) is still caught.
func TestExportNeutralizesAFormulaPayloadInTheClientName(t *testing.T) {
	for _, trigger := range []string{"=", "+", "-", "@"} {
		t.Run(trigger, func(t *testing.T) {
			payload := trigger + `cmd|' /C calc'!A0`
			reports := &stubReports{
				details: []reportstore.PaymentDetail{{
					CreatedAt: time.Date(2026, 3, 4, 18, 30, 0, 0, timezone.Argentina),
					StartsAt:  time.Date(2026, 3, 4, 18, 0, 0, 0, timezone.Argentina),
					EndsAt:    time.Date(2026, 3, 4, 19, 30, 0, 0, timezone.Argentina),
					CourtName: "Cancha 1", ClientName: payload, ClientPhone: "+5411",
					BookingPrice: 500_000, Amount: 150_000, ServiceFee: 100_000, RefundAmount: 0,
					Method: "mercadopago", PaymentStatus: "deposit_paid", BookingStatus: "confirmed",
				}},
			}

			h := newTestHandler(reports)
			w := httptest.NewRecorder()
			h.ExportPaymentsExcel(w, reportRequest(t, "?month=3&year=2026"))

			if w.Code != http.StatusOK {
				t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
			}

			f := openExport(t, w)
			rows, err := f.GetRows("Pagos")
			if err != nil {
				t.Fatalf("reading the payment sheet: %v", err)
			}
			if len(rows) < 2 {
				t.Fatalf("want a data row; got %v", rows)
			}

			cell := rows[1][3] // the "Cliente" column
			if len(cell) == 0 || strings.ContainsRune("=+-@", rune(cell[0])) {
				t.Errorf("the stored cell must not start with a formula-trigger character; got %q", cell)
			}
			if !strings.Contains(cell, payload) {
				t.Errorf("the escaped cell must still carry the original text; got %q, want it to contain %q", cell, payload)
			}
		})
	}
}

// The escape's own table moved with it to internal/spreadsheet when H-24 found
// the second sink that needs it. What stays here is the test above, which is
// the one this package owes: it drives the whole export handler, so a change
// that wires the guard out of the write path is still caught even though the
// guard itself is now somebody else's unit.

func TestCentavosToARS(t *testing.T) {
	tests := map[int]string{
		0:       "$0.00",
		1:       "$0.01",
		100:     "$1.00",
		150_000: "$1500.00",
	}

	for centavos, want := range tests {
		if got := centavosToARS(centavos); got != want {
			t.Errorf("centavosToARS(%d) = %q; want %q", centavos, got, want)
		}
	}
}

// An unrecognised value is passed through rather than blanked, so a new
// payment method added to the database still shows something in the export.
func TestLabelsFallBackToTheRawValue(t *testing.T) {
	if got := methodLabel("crypto"); got != "crypto" {
		t.Errorf("want the raw value back; got %q", got)
	}
	if got := paymentStatusLabel("chargeback"); got != "chargeback" {
		t.Errorf("want the raw value back; got %q", got)
	}
	if got := bookingStatusLabel("rescheduled"); got != "rescheduled" {
		t.Errorf("want the raw value back; got %q", got)
	}
}

func TestKnownLabelsAreTranslated(t *testing.T) {
	if got := methodLabel("cash"); got != "Efectivo" {
		t.Errorf(`want "Efectivo"; got %q`, got)
	}
	if got := paymentStatusLabel("deposit_paid"); got != "Seña pagada" {
		t.Errorf(`want "Seña pagada"; got %q`, got)
	}
	if got := bookingStatusLabel("no_show"); got != "No se presentó" {
		t.Errorf(`want "No se presentó"; got %q`, got)
	}
}

// --- Export robustness -------------------------------------------------------

// exportDetails builds n distinct payment rows.
func exportDetails(n int) []reportstore.PaymentDetail {
	details := make([]reportstore.PaymentDetail, n)
	for i := range details {
		details[i] = reportstore.PaymentDetail{
			CreatedAt:     time.Date(2026, 3, 1, 9, 0, 0, 0, timezone.Argentina).Add(time.Duration(i) * time.Minute),
			StartsAt:      time.Date(2026, 3, 1, 18, 0, 0, 0, timezone.Argentina),
			EndsAt:        time.Date(2026, 3, 1, 19, 30, 0, 0, timezone.Argentina),
			CourtName:     fmt.Sprintf("Cancha %d", i%4+1),
			ClientName:    fmt.Sprintf("Cliente %d", i),
			ClientPhone:   "+5411",
			BookingPrice:  500_000,
			Amount:        150_000 + i,
			ServiceFee:    1_000,
			Method:        "mercadopago",
			PaymentStatus: "deposit_paid",
			BookingStatus: "confirmed",
		}
	}
	return details
}

// openExport reads a recorded response back as a workbook, which is the only
// way to tell a well-formed export from a plausible-looking prefix of one.
func openExport(t *testing.T, w *httptest.ResponseRecorder) *excelize.File {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("the response is not a readable workbook: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// The streaming rewrite has to carry every payment and the same totals the
// cell-by-cell version produced, so the workbook is read back rather than
// sniffed for its zip header.
func TestExportWorkbookCarriesEveryPaymentAndTheTotals(t *testing.T) {
	// More than exportRowCheckInterval, so the streamed loop is exercised
	// across a budget check rather than only before the first one.
	const payments = 600
	reports := &stubReports{
		details: exportDetails(payments),
		summaries: []reportstore.PaymentMethodSummary{
			{Method: "mercadopago", Count: 200, Amount: 200_000, ServiceFee: 5_000, Refunded: 1_000},
			{Method: "cash", Count: 50, Amount: 50_000, ServiceFee: 0, Refunded: 0},
		},
	}

	h := newTestHandler(reports)
	w := httptest.NewRecorder()
	h.ExportPaymentsExcel(w, reportRequest(t, "?month=3&year=2026"))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	f := openExport(t, w)

	rows, err := f.GetRows("Pagos")
	if err != nil {
		t.Fatalf("reading the payment sheet: %v", err)
	}
	if want := payments + 1; len(rows) != want { // + the header
		t.Errorf("want %d rows on the payment sheet; got %d", want, len(rows))
	}
	if len(rows) > 0 && rows[0][0] != "Fecha" {
		t.Errorf("the header row is missing; got %v", rows[0])
	}
	if len(rows) > 1 && rows[1][3] != "Cliente 0" {
		t.Errorf("the first payment is wrong; got %v", rows[1])
	}
	if last := rows[len(rows)-1]; last[3] != fmt.Sprintf("Cliente %d", payments-1) {
		t.Errorf("the last payment is wrong; got %v", last)
	}

	summary, err := f.GetRows("Resumen")
	if err != nil {
		t.Fatalf("reading the summary sheet: %v", err)
	}
	// Found by its label, not by being last. The summary sheet grew a per-court
	// block and a previous-month block below the method table, and a test that
	// reads the final row would follow whatever gets appended next rather than
	// the row it means.
	var totals []string
	for _, row := range summary {
		if len(row) > 0 && row[0] == "Total" {
			totals = row
			break
		}
	}
	if totals == nil {
		t.Fatalf("the totals row is missing from the summary sheet; got %v", summary)
	}
	// 250_000 taken + 5_000 in fees - 1_000 refunded.
	if totals[1] != "250" || totals[4] != centavosToARS(254_000) {
		t.Errorf("the totals row does not add up; got %v", totals)
	}
}

// A period with more payments than the export will build is refused, not
// truncated: an owner reconciling a month cannot see that a ledger stopped
// early, and a file that looks complete is worse than one that never arrives.
func TestExportRefusesAPeriodOverTheRowCap(t *testing.T) {
	reports := &stubReports{details: exportDetails(testExportCap + 1)}

	svc := newTestService(reports)
	svc.maxExportRows = testExportCap
	h := newTestHandlerWithService(svc)
	w := httptest.NewRecorder()
	h.ExportPaymentsExcel(w, reportRequest(t, "?month=3&year=2026"))

	if w.Code != http.StatusUnprocessableEntity {
		// The body is not printed in full: past the cap it would be an
		// archive, which is itself the failure being reported.
		t.Fatalf("want 422 over the row cap; got %d (%d bytes starting %q)",
			w.Code, w.Body.Len(), w.Body.String()[:min(48, w.Body.Len())])
	}
	assertNotADownload(t, w)

	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	// The owner has to be told which limit they hit, or the refusal is
	// indistinguishable from the export being broken.
	if body.Error != httpx.CodeExportTooLarge {
		t.Errorf("want the %q code; got %q", httpx.CodeExportTooLarge, body.Error)
	}
}

// testExportCap stands in for the production cap. The property under test is
// the comparison — a period exactly on the cap must pass, one row over must not
// — and that is the same property at three rows as at fifty thousand, without
// spending seventeen seconds of every test run rebuilding the evidence that a
// full-sized workbook fits in its budget.
const testExportCap = 3

// The cap is a ceiling, not a target: a period sitting exactly on it still
// exports in full. This is the off-by-one guard — a cap applied with >= would
// refuse the largest legal export, and nothing else here would notice.
func TestExportAtTheRowCapStillSucceeds(t *testing.T) {
	reports := &stubReports{details: exportDetails(testExportCap)}

	svc := newTestService(reports)
	svc.maxExportRows = testExportCap
	h := newTestHandlerWithService(svc)
	w := httptest.NewRecorder()
	h.ExportPaymentsExcel(w, reportRequest(t, "?month=3&year=2026"))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 at exactly the cap; got %d (%s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment") {
		t.Errorf("want a download; got %q", got)
	}
}

// assertNotADownload checks that a failed export did not answer as a
// spreadsheet. The export used to set the download headers before it began
// writing, so an error arrived as a file named .xlsx holding a JSON error
// object — or, once bytes were out, a short archive with that object appended.
func assertNotADownload(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()

	if got := w.Header().Get("Content-Disposition"); got != "" {
		t.Errorf("a failed export must not offer a download; Content-Disposition was %q", got)
	}
	if got := w.Header().Get("Content-Type"); strings.Contains(got, "spreadsheet") {
		t.Errorf("a failed export must not claim to be a spreadsheet; Content-Type was %q", got)
	}
	if body := w.Body.Bytes(); bytes.HasPrefix(body, []byte("PK")) {
		t.Errorf("a failed export must not send archive bytes; body starts %q", string(body[:min(4, len(body))]))
	}
	if !json.Valid(w.Body.Bytes()) {
		t.Errorf("a failed export must answer with a JSON error; got %q", w.Body.String())
	}
}

// An export that runs out of budget is answered as a refusal, before anything
// that looks like a file has been sent.
func TestAnExportOverItsBudgetFailsBeforeTheStatusGoesOut(t *testing.T) {
	reports := &stubReports{details: exportDetails(1000)}

	h := newTestHandler(reports)
	w := httptest.NewRecorder()

	// A deadline already in the past: the export's own budget inherits it, so
	// the row loop sees an expired context on its first check. The deadline is
	// layered onto the request's own context rather than replacing it, which
	// would drop the complex the handler reads from there.
	r := reportRequest(t, "?month=3&year=2026")
	ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(-time.Second))
	defer cancel()

	h.ExportPaymentsExcel(w, r.WithContext(ctx))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when the budget is spent; got %d (%s)", w.Code, w.Body.String())
	}
	assertNotADownload(t, w)
}

// The summary sheet is fetched after the payment rows, so a failure there lands
// at the point the old code had already decided the response was a file.
func TestAFailureAfterTheDetailSheetIsNotAnsweredAsADownload(t *testing.T) {
	reports := &failAfterDetails{details: exportDetails(10)}

	h := newTestHandler(reports)
	w := httptest.NewRecorder()
	h.ExportPaymentsExcel(w, reportRequest(t, "?month=3&year=2026"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500; got %d (%s)", w.Code, w.Body.String())
	}
	assertNotADownload(t, w)
}

// failAfterDetails answers the payment query and then fails the summary one.
type failAfterDetails struct {
	details []reportstore.PaymentDetail
}

func (f *failAfterDetails) PaymentDetails(context.Context, uuid.UUID, time.Time, time.Time) ([]reportstore.PaymentDetail, error) {
	return f.details, nil
}

func (f *failAfterDetails) PaymentSummaryByMethod(context.Context, uuid.UUID, time.Time, time.Time) ([]reportstore.PaymentMethodSummary, error) {
	return nil, errors.New("the summary query failed")
}

func (f *failAfterDetails) PaymentSummaryByCourt(context.Context, uuid.UUID, time.Time, time.Time) ([]reportstore.PaymentCourtSummary, error) {
	return nil, errors.New("the summary query failed")
}

// The export declares its length, which is what lets a client tell a complete
// download from a connection that dropped halfway. Without it the transfer is
// chunked and a truncated archive looks finished.
func TestExportDeclaresItsLength(t *testing.T) {
	reports := &stubReports{details: exportDetails(50)}

	h := newTestHandler(reports)
	w := httptest.NewRecorder()
	h.ExportPaymentsExcel(w, reportRequest(t, "?month=3&year=2026"))

	declared := w.Header().Get("Content-Length")
	if declared == "" {
		t.Fatal("the export did not declare a Content-Length")
	}
	if declared != strconv.Itoa(w.Body.Len()) {
		t.Errorf("declared %s bytes but sent %d", declared, w.Body.Len())
	}
}

// A budget spent inside the query is the same event as a budget spent inside
// the row loop, and has to be reported the same way. Answering one as a 500 and
// the other as a 503 sends whoever reads the logs looking for two problems.
func TestABudgetExpiryDuringTheQueryIsNotAnInternalError(t *testing.T) {
	reports := &stubReports{err: context.DeadlineExceeded}

	h := newTestHandler(reports)
	w := httptest.NewRecorder()
	h.ExportPaymentsExcel(w, reportRequest(t, "?month=3&year=2026"))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503; got %d (%s)", w.Code, w.Body.String())
	}
	assertNotADownload(t, w)
}

// The spreadsheet carries the same blocks the screen shows.
//
// This test exists because the report and the export are two endpoints with two
// code paths, each totalling the month on its own. Nothing in the type system
// makes them agree; what keeps them honest is that both are fed from the same
// queries, and this is what would notice if one of them stopped being.
func TestTheExportCarriesTheCourtBreakdownAndThePreviousMonth(t *testing.T) {
	reports := &stubReports{
		summaries: []reportstore.PaymentMethodSummary{
			{Method: "cash", Count: 2, Amount: 100_000, ServiceFee: 0, Refunded: 0},
		},
		courtSummaries: []reportstore.PaymentCourtSummary{
			{CourtID: "c1", CourtName: "Cancha 1", Count: 2, Amount: 100_000},
			{CourtID: "c2", CourtName: "", Count: 1, Amount: 40_000},
		},
		details: []reportstore.PaymentDetail{{ClientName: "Cliente 0"}},
	}
	h := newTestHandler(reports)

	w := httptest.NewRecorder()
	h.ExportPaymentsExcel(w, reportRequest(t, "?month=3&year=2026"))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("opening the workbook: %v", err)
	}
	summary, err := f.GetRows("Resumen")
	if err != nil {
		t.Fatalf("reading the summary sheet: %v", err)
	}

	var flat []string
	for _, row := range summary {
		flat = append(flat, strings.Join(row, "|"))
	}
	joined := strings.Join(flat, "\n")

	for _, want := range []string{courtBlockTitle, "Cancha 1", deletedCourtLabel, previousBlockTitle} {
		if !strings.Contains(joined, want) {
			t.Errorf("the summary sheet must carry %q; got:\n%s", want, joined)
		}
	}
}
