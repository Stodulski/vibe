package reporting

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/httpx"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
	"github.com/stodulski/vibe-server/internal/spreadsheet"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/xuri/excelize/v2"
)

// GetMonthlyReport handles GET /api/v1/complexes/:id/reports/monthly, totalling
// the period's payments per method.
//
// It is one cohesive request lifecycle — parse the input, validate the period,
// aggregate the payment and booking stats, respond — and splitting it would
// relocate sequential steps into helpers without reducing what a reader holds
// at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) GetMonthlyReport(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	now := time.Now().In(timezone.Argentina)
	qs := r.URL.Query()
	// Strict, not ReadInt — see H-05. A malformed month/year has to be a 400,
	// not "use the current period and answer 200 anyway".
	month, monthErr := httpx.ReadIntStrict(qs, "month", int(now.Month()))
	year, yearErr := httpx.ReadIntStrict(qs, "year", now.Year())
	if monthErr != nil || yearErr != nil {
		h.respond.BadRequest(w, r, errors.New(httpx.CodeInvalidReportPeriod))
		return
	}

	if err := validateReportPeriod(month, year, now, complex.CreatedAt); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	report, err := h.svc.MonthlyReport(r.Context(), complex.ID, month, year)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"report": report})
}

// periodTotals adds up one period's payments the same way the selected month
// is added up above, so the comparison is between like and like.
//
// Net is what the owner keeps: the amount plus the service fee the client paid
// on top, minus anything refunded.
func periodTotals(summaries []reportstore.PaymentMethodSummary) map[string]any {
	var count, amount, serviceFees, refunded int
	for _, s := range summaries {
		count += s.Count
		amount += s.Amount
		serviceFees += s.ServiceFee
		refunded += s.Refunded
	}
	return map[string]any{
		"count":        count,
		"total":        amount,
		"service_fees": serviceFees,
		"refunded":     refunded,
		"net":          amount + serviceFees - refunded,
	}
}

// defaultMaxExportRows caps the detail sheet.
//
// The bound has to be on rows rather than on wall-clock time, because rows are
// what the export actually consumes: the payment slice, the cells excelize
// holds, and the compressed archive all grow linearly with them, and memory is
// the resource that takes the whole instance down rather than one request. A
// time budget alone would let a large export get most of the way through its
// allocation before being cut off, having already done the damage.
//
// Fifty thousand is roughly five times the pathological ceiling for one venue
// in one month — a twenty-court complex running sixteen slots a day for
// thirty-one days is under ten thousand bookings, and a booking produces a
// handful of payment rows. A period that exceeds this is not a report anyone
// reconciles by hand; it is a signal that something is wrong upstream. So it is
// refused with a code the owner sees, never truncated: a ledger that stops
// silently on the twelfth still looks complete to whoever files it.
//
// The cap is also affordable: an export of exactly this many rows builds and
// compresses in around a second, so the ceiling is not itself a way to occupy
// the process.
const defaultMaxExportRows = 50_000

// ExportBudget bounds the whole export — the query, the build and the
// serialisation.
//
// The request context carries no deadline of its own: http.Server's
// WriteTimeout closes the connection but does not cancel the handler, so
// without this an export keeps allocating for a client that hung up minutes
// ago. The budget therefore has to sit under that timeout, with room left to
// flush what was built; validateBootConfig in cmd/api refuses a configuration
// where it does not, because HTTP_WRITE_TIMEOUT is an operator's knob and this
// is the invariant it can break. A full-cap export measures around a second,
// so the fifty here is headroom for a slow database rather than a target.
//
// It is exported only so that boot check can name it; nothing outside this
// package uses it to do work.
const ExportBudget = 50 * time.Second

// exportRowCheckInterval is how often the row loop looks at the budget.
// Checking every row would cost more than it saves; checking never would make
// the budget decorative.
const exportRowCheckInterval = 512

// The formula-injection escape used by every string cell below lives in
// internal/spreadsheet. It moved there when H-24 found the same defect at the
// other sink that writes spreadsheet cells — the abandoned-lead webhook, which
// posts a caller-supplied email to the owner's Apps Script. Two sinks, one
// definition; see spreadsheet.EscapeFormulaCell for the whole argument,
// including why the fix is escaping here rather than validating at the input.

// ExportPaymentsExcel handles GET /api/v1/complexes/:id/reports/export, building
// a two-sheet workbook: every payment in the period, and the same totals the
// JSON report returns.
//
// The workbook is finished in full before the response status is chosen. It
// used to be written straight at the ResponseWriter, which commits a 200 with
// its first byte — so a failure halfway through arrived as a short .xlsx with a
// JSON error object stapled to the end, and Excel reported a corrupt file
// rather than the server reporting a problem.
func (h *Handler) ExportPaymentsExcel(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	now := time.Now().In(timezone.Argentina)
	qs := r.URL.Query()
	// Strict, not ReadInt — see H-05, same reasoning as GetMonthlyReport.
	month, monthErr := httpx.ReadIntStrict(qs, "month", int(now.Month()))
	year, yearErr := httpx.ReadIntStrict(qs, "year", now.Year())
	if monthErr != nil || yearErr != nil {
		h.respond.BadRequest(w, r, errors.New(httpx.CodeInvalidReportPeriod))
		return
	}

	if err := validateReportPeriod(month, year, now, complex.CreatedAt); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	export, rowCount, err := h.svc.ExportPaymentsExcel(r.Context(), complex, month, year)
	if err != nil {
		if errors.Is(err, ErrExportTooLarge) {
			// Logged with the real count, because the owner is given a code and
			// support needs the number behind it.
			h.respond.LogError(r, fmt.Errorf("export for complex %s %d/%d has %d rows, over the %d cap",
				complex.ID, month, year, rowCount, h.svc.ExportRowCap()))
			h.respond.Refuse(w, r, httpx.Unprocessable(httpx.CodeExportTooLarge))
			return
		}
		h.failExport(w, r, err)
		return
	}

	h.sendExport(w, r, export.Filename, export.Body)
}

// exportSummaries is the three aggregate reads the summary sheet is built from.
// They travel together because they are always needed together, always for the
// same complex, and always for the same period.
type exportSummaries struct {
	byMethod []reportstore.PaymentMethodSummary
	byCourt  []reportstore.PaymentCourtSummary
	previous []reportstore.PaymentMethodSummary
}

// readExportSummaries runs the same aggregate queries the JSON report runs, so
// the file and the screen answer with the same numbers. They are separate
// endpoints; only feeding them from the same queries keeps them from drifting.
//
// The previous month is read here rather than by the caller because it is not a
// period the caller chose — it is this report's own comparison baseline, and
// deriving it beside the queries that consume it keeps the offset in one place.
func (s *Service) readExportSummaries(ctx context.Context, complexID uuid.UUID, from, to time.Time) (exportSummaries, error) {
	byMethod, err := s.reports.PaymentSummaryByMethod(ctx, complexID, from, to)
	if err != nil {
		return exportSummaries{}, err
	}

	byCourt, err := s.reports.PaymentSummaryByCourt(ctx, complexID, from, to)
	if err != nil {
		return exportSummaries{}, err
	}

	prevFrom := from.AddDate(0, -1, 0)
	previous, err := s.reports.PaymentSummaryByMethod(ctx, complexID, prevFrom, prevFrom.AddDate(0, 1, -1))
	if err != nil {
		return exportSummaries{}, err
	}

	return exportSummaries{byMethod: byMethod, byCourt: byCourt, previous: previous}, nil
}

// buildExportWorkbook assembles the whole workbook and serialises it, returning
// the finished bytes. Nothing has been sent when it fails, which is the point:
// serialising is where a malformed workbook surfaces, and finding out here
// costs a 500 rather than a download that stops mid-archive.
func buildExportWorkbook(
	ctx context.Context,
	title string,
	details []reportstore.PaymentDetail,
	summaries []reportstore.PaymentMethodSummary,
	courts []reportstore.PaymentCourtSummary,
	previous []reportstore.PaymentMethodSummary,
) (*bytes.Buffer, error) {
	f := excelize.NewFile()
	// In-memory workbook cleanup; a close error here (e.g. stale sheet references) cannot
	// occur for a freshly created *excelize.File and there is nothing actionable to do with
	// it in a defer.
	defer func() { _ = f.Close() }() //nolint:errcheck // see above: an in-memory workbook's close has nothing actionable to report

	if err := writePaymentSheet(ctx, f, details); err != nil {
		return nil, err
	}
	if err := writeSummarySheet(f, title, summaries, courts, previous); err != nil {
		return nil, err
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("serialising the workbook: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return buf, nil
}

// sendExport writes a finished workbook as a download.
func (h *Handler) sendExport(w http.ResponseWriter, r *http.Request, filename string, buf *bytes.Buffer) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	// Declared, so a connection that drops halfway leaves a short body the
	// client can recognise as an incomplete transfer rather than a finished
	// archive.
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(http.StatusOK)

	if _, err := buf.WriteTo(w); err != nil {
		// The status and the length are already out. Anything written now would
		// land inside the archive the client is reading, so the failure is only
		// recorded.
		h.respond.LogError(r, fmt.Errorf("writing the export: %w", err))
	}
}

// failExport answers an export that could not be produced. Every failure after
// the period has been validated goes through it, so the same context error is
// not a 500 when the query raises it and a 503 when the row loop does.
//
// It is separate from ServerError to keep two cases out of the internal-error
// bucket: a budget expiry, which is this service declining work rather than
// failing at it, and a cancelled request, where there is nobody left to answer.
func (h *Handler) failExport(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.DeadlineExceeded) {
		h.respond.LogError(r, err)
		h.respond.Refuse(w, r, httpx.Unavailable(httpx.CodeExportTimedOut))
		return
	}
	if errors.Is(err, context.Canceled) {
		// The client is gone; there is nobody to answer. Recorded so a rash of
		// these is visible.
		h.respond.LogError(r, err)
		return
	}
	h.respond.ServerError(w, r, err)
}

// writePaymentSheet fills the detail sheet, one payment per row.
//
// It uses excelize's streaming writer rather than the ordinary cell API: the
// ordinary one keeps every cell of the sheet as a live struct until the file is
// serialised, which for a full export is the largest thing this process holds.
// The streaming writer spills rows as they are written, so the sheet costs
// roughly the row being built rather than the whole month.
func writePaymentSheet(ctx context.Context, f *excelize.File, details []reportstore.PaymentDetail) error {
	if err := f.SetSheetName("Sheet1", paymentSheetName); err != nil {
		return fmt.Errorf("naming the payment sheet: %w", err)
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"1DB954"}},
	})
	if err != nil {
		return fmt.Errorf("building the header style: %w", err)
	}

	sw, err := f.NewStreamWriter(paymentSheetName)
	if err != nil {
		return fmt.Errorf("opening the payment sheet for streaming: %w", err)
	}

	// Column widths have to be set before the first row is streamed, so they
	// are measured in a pass over the rows rather than accumulated while
	// writing them.
	for i, width := range paymentColumnWidths(details) {
		if err := sw.SetColWidth(i+1, i+1, width); err != nil {
			return fmt.Errorf("setting column width: %w", err)
		}
	}

	header := make([]any, len(paymentSheetHeaders))
	for i, h := range paymentSheetHeaders {
		header[i] = excelize.Cell{StyleID: headerStyle, Value: h}
	}
	if err := sw.SetRow("A1", header); err != nil {
		return fmt.Errorf("writing the header row: %w", err)
	}

	for i, d := range details {
		if i%exportRowCheckInterval == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}

		cell, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return fmt.Errorf("addressing row %d: %w", i+2, err)
		}
		if err := sw.SetRow(cell, paymentRow(d)); err != nil {
			return fmt.Errorf("writing row %d: %w", i+2, err)
		}
	}

	if err := sw.Flush(); err != nil {
		return fmt.Errorf("flushing the payment sheet: %w", err)
	}
	return nil
}

// paymentRow is one payment as the spreadsheet shows it.
//
// Every column is passed through escapeFormulaCell — see H-06 there. Most of
// these columns are not attacker-reachable today (a formatted date, a money
// figure), but ClientName and ClientPhone are free text from the public
// booking form, and the guard is applied uniformly rather than singled out
// per column so a future free-text field does not have to remember to ask
// for it.
func paymentRow(d reportstore.PaymentDetail) []any {
	return []any{
		spreadsheet.EscapeFormulaCell(d.CreatedAt.In(timezone.Argentina).Format("02/01/2006 15:04")),
		spreadsheet.EscapeFormulaCell(timezone.HoursLabel(d.StartsAt, d.EndsAt)),
		spreadsheet.EscapeFormulaCell(d.CourtName),
		spreadsheet.EscapeFormulaCell(d.ClientName),
		spreadsheet.EscapeFormulaCell(d.ClientPhone),
		spreadsheet.EscapeFormulaCell(centavosToARS(d.BookingPrice)),
		spreadsheet.EscapeFormulaCell(centavosToARS(d.Amount)),
		spreadsheet.EscapeFormulaCell(centavosToARS(d.ServiceFee)),
		spreadsheet.EscapeFormulaCell(centavosToARS(d.RefundAmount)),
		spreadsheet.EscapeFormulaCell(methodLabel(d.Method)),
		spreadsheet.EscapeFormulaCell(paymentStatusLabel(d.PaymentStatus)),
		spreadsheet.EscapeFormulaCell(bookingStatusLabel(d.BookingStatus)),
	}
}

// paymentColumnWidths sizes each column to its widest value, headers included.
func paymentColumnWidths(details []reportstore.PaymentDetail) []float64 {
	widths := make([]float64, len(paymentSheetHeaders))
	for i, header := range paymentSheetHeaders {
		widths[i] = float64(len(header)) + 2
	}

	for _, d := range details {
		for i, v := range paymentRow(d) {
			s, ok := v.(string)
			if !ok {
				continue
			}
			if width := float64(len(s)) + 2; width > widths[i] {
				widths[i] = width
			}
		}
	}

	return widths
}

// writeSummarySheet fills the totals sheet: one row per payment method, then
// the whole. It is bounded by the number of payment methods, so it is written
// with the ordinary cell API.
func writeSummarySheet(
	f *excelize.File,
	title string,
	summaries []reportstore.PaymentMethodSummary,
	courts []reportstore.PaymentCourtSummary,
	previous []reportstore.PaymentMethodSummary,
) error {
	if _, err := f.NewSheet(summarySheetName); err != nil {
		return fmt.Errorf("creating the summary sheet: %w", err)
	}

	styles, err := newSummaryStyles(f)
	if err != nil {
		return err
	}

	if err := f.SetCellValue(summarySheetName, "A1", spreadsheet.EscapeFormulaCell(title)); err != nil {
		return fmt.Errorf("writing the title: %w", err)
	}
	if err := f.SetCellStyle(summarySheetName, "A1", "A1", styles.title); err != nil {
		return fmt.Errorf("styling the title: %w", err)
	}

	if err := writeSummaryRow(f, 3, summarySheetHeaders, styles.header); err != nil {
		return err
	}

	row, err := writeMethodBlock(f, 4, summaries, styles)
	if err != nil {
		return err
	}

	// The same figures the screen shows, in the same order, because an owner
	// reconciling a month reads one against the other. The report and the
	// spreadsheet are two separate code paths and nothing forces them to
	// agree; what keeps them honest is that both are fed from the same
	// queries.
	if row, err = writeCourtBlock(f, row, courts, styles); err != nil {
		return err
	}
	if err := writePreviousBlock(f, row, previous, styles); err != nil {
		return err
	}

	return setSummaryColumnWidths(f)
}

// writeMethodBlock writes one row per payment method, then the total row, and
// returns the next free row — the same shape as the court and previous blocks
// that follow it.
//
// The totals are accumulated from exactly the rows written here. Summing the
// same slice anywhere else is what lets a table and its own total disagree.
func writeMethodBlock(f *excelize.File, row int, summaries []reportstore.PaymentMethodSummary, styles summaryStyles) (int, error) {
	var totalCount, totalAmount, totalServiceFees, totalRefunded int

	for _, sum := range summaries {
		net := sum.Amount + sum.ServiceFee - sum.Refunded
		totalCount += sum.Count
		totalAmount += sum.Amount
		totalServiceFees += sum.ServiceFee
		totalRefunded += sum.Refunded

		values := []string{
			methodLabel(sum.Method),
			strconv.Itoa(sum.Count),
			centavosToARS(sum.Amount),
			centavosToARS(sum.Refunded),
			centavosToARS(net),
		}
		if err := writeSummaryRow(f, row, values, 0); err != nil {
			return 0, err
		}
		row++
	}

	totals := []string{
		"Total",
		strconv.Itoa(totalCount),
		centavosToARS(totalAmount),
		centavosToARS(totalRefunded),
		centavosToARS(totalAmount + totalServiceFees - totalRefunded),
	}
	if err := writeSummaryRow(f, row, totals, styles.bold); err != nil {
		return 0, err
	}

	return row + 2, nil
}

// writeCourtBlock adds the per-court breakdown under the method table and
// returns the next free row.
func writeCourtBlock(f *excelize.File, row int, courts []reportstore.PaymentCourtSummary, styles summaryStyles) (int, error) {
	if len(courts) == 0 {
		return row, nil
	}

	if err := writeSummaryRow(f, row, []string{courtBlockTitle}, styles.bold); err != nil {
		return 0, err
	}
	row++
	if err := writeSummaryRow(f, row, courtSummaryHeaders, styles.header); err != nil {
		return 0, err
	}
	row++

	for _, c := range courts {
		name := c.CourtName
		if name == "" {
			name = deletedCourtLabel
		}
		values := []string{
			name,
			strconv.Itoa(c.Count),
			centavosToARS(c.Amount),
			centavosToARS(c.Refunded),
			centavosToARS(c.Amount + c.ServiceFee - c.Refunded),
		}
		if err := writeSummaryRow(f, row, values, 0); err != nil {
			return 0, err
		}
		row++
	}

	return row + 1, nil
}

// writePreviousBlock adds the month before's totals, so the file carries the
// same baseline the screen shows rather than a figure with nothing to read it
// against.
func writePreviousBlock(f *excelize.File, row int, previous []reportstore.PaymentMethodSummary, styles summaryStyles) error {
	var count, amount, serviceFees, refunded int
	for _, s := range previous {
		count += s.Count
		amount += s.Amount
		serviceFees += s.ServiceFee
		refunded += s.Refunded
	}

	if err := writeSummaryRow(f, row, []string{previousBlockTitle}, styles.bold); err != nil {
		return err
	}
	return writeSummaryRow(f, row+1, []string{
		previousBlockTitle,
		strconv.Itoa(count),
		centavosToARS(amount),
		centavosToARS(refunded),
		centavosToARS(amount + serviceFees - refunded),
	}, 0)
}

// setSummaryColumnWidths applies the fixed widths to the summary sheet.
func setSummaryColumnWidths(f *excelize.File) error {
	for i, width := range summaryColumnWidths {
		name, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return fmt.Errorf("naming summary column %d: %w", i+1, err)
		}
		if err := f.SetColWidth(summarySheetName, name, name, width); err != nil {
			return fmt.Errorf("setting summary column width: %w", err)
		}
	}
	return nil
}

// summaryStyles are the three cell styles the summary sheet uses.
type summaryStyles struct {
	title  int
	header int
	bold   int
}

// newSummaryStyles registers them against the workbook.
func newSummaryStyles(f *excelize.File) (summaryStyles, error) {
	title, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}})
	if err != nil {
		return summaryStyles{}, fmt.Errorf("building the title style: %w", err)
	}
	header, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"1DB954"}},
	})
	if err != nil {
		return summaryStyles{}, fmt.Errorf("building the header style: %w", err)
	}
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return summaryStyles{}, fmt.Errorf("building the totals style: %w", err)
	}
	return summaryStyles{title: title, header: header, bold: bold}, nil
}

// summaryColumnWidths are fixed: the summary sheet's contents are labels and
// money, both of known length.
var summaryColumnWidths = []float64{16, 12, 16, 16, 16}

// writeSummaryRow writes one row of the summary sheet, applying styleID when it
// is non-zero.
//
// Every value is passed through escapeFormulaCell — see H-06. Headers and
// static labels never trigger it in practice, but a court name (writeCourtBlock)
// is owner-entered text and flows through this same path, so the guard sits
// here once rather than at each caller.
func writeSummaryRow(f *excelize.File, row int, values []string, styleID int) error {
	for i, v := range values {
		cell, err := excelize.CoordinatesToCellName(i+1, row)
		if err != nil {
			return fmt.Errorf("addressing summary cell %d,%d: %w", i+1, row, err)
		}
		if err := f.SetCellValue(summarySheetName, cell, spreadsheet.EscapeFormulaCell(v)); err != nil {
			return fmt.Errorf("writing summary cell %s: %w", cell, err)
		}
		if styleID != 0 {
			if err := f.SetCellStyle(summarySheetName, cell, cell, styleID); err != nil {
				return fmt.Errorf("styling summary cell %s: %w", cell, err)
			}
		}
	}
	return nil
}

// validateReportPeriod checks that month/year is valid, not in the future,
// and not before the complex was created.
func validateReportPeriod(month, year int, now time.Time, complexCreatedAt time.Time) error {
	if month < 1 || month > 12 {
		return errors.New(httpx.CodeMonthOutOfRange)
	}

	// Cannot export future periods.
	if year > now.Year() || (year == now.Year() && month > int(now.Month())) {
		return errors.New(httpx.CodeFuturePeriod)
	}

	// Cannot export before the complex existed.
	created := complexCreatedAt.In(timezone.Argentina)
	if year < created.Year() || (year == created.Year() && month < int(created.Month())) {
		return errors.New(httpx.CodeBeforeComplexExisted)
	}

	return nil
}
