//go:build integration

package reporting

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// tenantWatchingReports is the real report store with one question asked of
// every call: which tenant was this read made for?
//
// The worker runs outside the HTTP chain, so nothing upstream of it stamps a
// scope. An unscoped session matches no row-level-security policy, which
// would not fail — it would produce an empty workbook that looks like a month
// with no payments. So the scope is asserted here rather than inferred from
// the rows coming back.
type tenantWatchingReports struct {
	inner  PaymentReportReader
	t      *testing.T
	scopes []uuid.UUID
}

func (r *tenantWatchingReports) record(ctx context.Context) {
	r.t.Helper()
	id, ok := data.TenantFromContext(ctx)
	if !ok {
		r.t.Error("a report read reached the store with no tenant on the context; " +
			"under the tenant policies that reads nothing and the export would be a silently empty workbook")
	}
	r.scopes = append(r.scopes, id)
}

func (r *tenantWatchingReports) PaymentSummaryByMethod(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentMethodSummary, error) {
	r.record(ctx)
	return r.inner.PaymentSummaryByMethod(ctx, complexID, from, to)
}

func (r *tenantWatchingReports) PaymentSummaryByCourt(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentCourtSummary, error) {
	r.record(ctx)
	return r.inner.PaymentSummaryByCourt(ctx, complexID, from, to)
}

func (r *tenantWatchingReports) PaymentDetails(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentDetail, error) {
	r.record(ctx)
	return r.inner.PaymentDetails(ctx, complexID, from, to)
}

func (r *tenantWatchingReports) CashSalesByMethod(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.CashSalesSummary, error) {
	r.record(ctx)
	return r.inner.CashSalesByMethod(ctx, complexID, from, to)
}

func (r *tenantWatchingReports) CashMovementsByCategory(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.CashCategorySummary, error) {
	r.record(ctx)
	return r.inner.CashMovementsByCategory(ctx, complexID, from, to)
}

// detailRows returns the payment sheet of an uploaded workbook.
func detailRows(t *testing.T, body []byte) [][]string {
	t.Helper()

	f, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("the uploaded object is not a readable workbook: %v", err)
	}
	defer func() { _ = f.Close() }()

	rows, err := f.GetRows(paymentSheetName)
	if err != nil {
		t.Fatalf("reading the %q sheet: %v", paymentSheetName, err)
	}
	return rows
}

// TestTheExportWorkerReadsUnderItsOwnTenant runs one attempt against the real
// report store and the real schema: the reads have to be scoped to the
// complex the payload names, and the workbook has to carry that complex's
// payments and nothing else.
func TestTheExportWorkerReadsUnderItsOwnTenant(t *testing.T) {
	f := datatest.Isolated(t)

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	f.CreatePayment(t, booking.ID, 100_000, 7_000, nil)

	reports := &tenantWatchingReports{inner: f.Stores.Reports, t: t}
	bucket := &stubExportStorage{}
	svc := NewService(stubBookings{}, stubClients{}, stubCourts{}, stubSchedules{}, reports,
		ExportDeps{Store: newStubExportStore(), Storage: bucket}, 50*time.Second)

	now := time.Now().In(timezone.Argentina)
	payload := ExportPaymentsPayload{
		ExportID:    uuid.NewString(),
		ComplexID:   f.ComplexID.String(),
		ComplexName: "Test Complex",
		ComplexSlug: "test-complex",
		Month:       int(now.Month()),
		Year:        now.Year(),
	}

	if err := runExportJob(t, svc, payload); err != nil {
		t.Fatalf("the export attempt failed: %v", err)
	}

	if len(reports.scopes) == 0 {
		t.Fatal("the worker made no report reads at all")
	}
	for i, scope := range reports.scopes {
		if scope != f.ComplexID {
			t.Errorf("read %d ran for complex %s, want the payload's own %s", i, scope, f.ComplexID)
		}
	}

	if len(bucket.put) != 1 {
		t.Fatalf("%d objects written; want 1", len(bucket.put))
	}
	if got, want := bucket.put[0].Key, exportObjectKey(payload); got != want {
		t.Errorf("key = %q, want %q", got, want)
	}

	rows := detailRows(t, bucket.put[0].Body)
	if len(rows) != 2 {
		t.Fatalf("the detail sheet has %d rows (header included); want the header and this complex's one payment", len(rows))
	}
	if !strings.Contains(strings.Join(rows[1], "|"), "Ana Diaz") {
		t.Errorf("the fixture's payment is not in the sheet: %v", rows[1])
	}
}

// The same worker, given a complex that is not this fixture's, must come back
// with a workbook of nothing — the isolation is a property of the read, not of
// which rows happen to exist.
func TestTheExportWorkerSeesNothingOfAnotherComplex(t *testing.T) {
	f := datatest.Isolated(t)

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	f.CreatePayment(t, booking.ID, 100_000, 7_000, nil)

	reports := &tenantWatchingReports{inner: f.Stores.Reports, t: t}
	bucket := &stubExportStorage{}
	svc := NewService(stubBookings{}, stubClients{}, stubCourts{}, stubSchedules{}, reports,
		ExportDeps{Store: newStubExportStore(), Storage: bucket}, 50*time.Second)

	stranger := uuid.New()
	now := time.Now().In(timezone.Argentina)
	payload := ExportPaymentsPayload{
		ExportID:    uuid.NewString(),
		ComplexID:   stranger.String(),
		ComplexName: "Otro Complejo",
		ComplexSlug: "otro-complejo",
		Month:       int(now.Month()),
		Year:        now.Year(),
	}

	if err := runExportJob(t, svc, payload); err != nil {
		t.Fatalf("the export attempt failed: %v", err)
	}

	for i, scope := range reports.scopes {
		if scope != stranger {
			t.Errorf("read %d ran for complex %s, want the payload's own %s", i, scope, stranger)
		}
	}

	rows := detailRows(t, bucket.put[0].Body)
	if len(rows) != 1 {
		t.Errorf("the detail sheet has %d rows; want the header alone — this export is for a complex with no payments, "+
			"and anything else here is another tenant's ledger", len(rows))
	}
}
