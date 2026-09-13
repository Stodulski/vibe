package reporting

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/jobs"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
)

// recordingQueue captures the handler a registration wires up, so a test can
// run one attempt the way the pool would — with a raw payload — rather than
// calling the unexported function underneath it.
type recordingQueue struct {
	handlers map[string]func(context.Context, json.RawMessage) error
}

func newRecordingQueue() *recordingQueue {
	return &recordingQueue{handlers: map[string]func(context.Context, json.RawMessage) error{}}
}

func (q *recordingQueue) RegisterHandler(taskType string, h func(context.Context, json.RawMessage) error) {
	q.handlers[taskType] = h
}

// runExportJob registers the worker against svc and runs one attempt with
// payload, exactly as the pool does.
func runExportJob(t *testing.T, svc *Service, payload ExportPaymentsPayload) error {
	t.Helper()

	q := newRecordingQueue()
	svc.RegisterExportWorker(q)

	handler, ok := q.handlers[TaskExportPayments]
	if !ok {
		t.Fatalf("no handler was registered for %q; a claimed export would sit in the queue forever", TaskExportPayments)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return handler(t.Context(), raw)
}

// workerFixture is a service whose reads are stubbed and whose bucket records
// what was written.
func workerFixture(reports PaymentReportReader) (*Service, *stubExportStorage) {
	bucket := &stubExportStorage{}
	svc := NewService(stubBookings{}, stubClients{}, stubCourts{}, stubSchedules{}, reports,
		ExportDeps{Store: newStubExportStore(), Storage: bucket}, 50*time.Second)
	return svc, bucket
}

func samplePayload() ExportPaymentsPayload {
	return ExportPaymentsPayload{
		ExportID:    uuid.NewString(),
		ComplexID:   uuid.NewString(),
		ComplexName: "Vibe Palermo",
		ComplexSlug: "vibe-palermo",
		Month:       9,
		Year:        2026,
	}
}

func TestTheWorkerUploadsTheWorkbookAsADownload(t *testing.T) {
	svc, bucket := workerFixture(&stubReports{details: []reportstore.PaymentDetail{
		{CourtName: "Cancha 1", ClientName: "Ana", Amount: 100_000, Method: "cash"},
	}})
	payload := samplePayload()

	if err := runExportJob(t, svc, payload); err != nil {
		t.Fatalf("the export attempt failed: %v", err)
	}

	if len(bucket.put) != 1 {
		t.Fatalf("%d objects written; want exactly 1", len(bucket.put))
	}
	obj := bucket.put[0]

	if want := exportObjectKey(payload); obj.Key != want {
		t.Errorf("key = %q, want %q — the status route derives the same key from the same payload and would sign a URL for nothing",
			obj.Key, want)
	}
	if !strings.HasPrefix(obj.Key, "exports/") {
		t.Errorf("key = %q, want the exports/ prefix the 24 hour lifecycle rule is written against", obj.Key)
	}
	if obj.ContentType != exportContentType {
		t.Errorf("content type = %q, want the spreadsheet type; an object with the wrong one downloads as an opaque stream", obj.ContentType)
	}
	if want := `attachment; filename="pagos_vibe-palermo_9_2026.xlsx"`; obj.ContentDisposition != want {
		t.Errorf("content disposition = %q, want %q", obj.ContentDisposition, want)
	}
	if len(obj.Body) == 0 {
		t.Error("an empty workbook was uploaded")
	}
}

// A month over the cap reads the same way on every attempt, so it dead-letters
// now — and carries the code the status route turns back into a sentence.
func TestAnOversizedPeriodDeadLettersWithItsCode(t *testing.T) {
	details := make([]reportstore.PaymentDetail, defaultMaxExportRows+1)
	svc, bucket := workerFixture(&stubReports{details: details})

	err := runExportJob(t, svc, samplePayload())
	if err == nil {
		t.Fatal("an oversized period was exported rather than refused")
	}
	if got := jobs.Classify(err); got != jobs.OutcomePermanent {
		t.Errorf("outcome = %v, want OutcomePermanent: retrying re-reads the same month for the same answer", got)
	}
	if code := exportFailureCode(err.Error()); code != httpx.CodeExportTooLarge {
		t.Errorf("the status route would report %q, want %q", code, httpx.CodeExportTooLarge)
	}
	if len(bucket.put) != 0 {
		t.Error("a refused export still wrote an object")
	}
}

// A store failure is transient: the month is fine, the database was not.
func TestAReadFailureComesBackOnTheBackoff(t *testing.T) {
	svc, _ := workerFixture(&stubReports{err: context.DeadlineExceeded})

	err := runExportJob(t, svc, samplePayload())
	if err == nil {
		t.Fatal("a failing read reported success")
	}
	if got := jobs.Classify(err); got != jobs.OutcomeRetry {
		t.Errorf("outcome = %v, want OutcomeRetry", got)
	}
}

// A payload that names no complex can never be run, whoever picks it up.
func TestAPayloadWithNoComplexDeadLetters(t *testing.T) {
	svc, _ := workerFixture(&stubReports{})
	payload := samplePayload()
	payload.ComplexID = "not-a-uuid"

	err := runExportJob(t, svc, payload)
	if got := jobs.Classify(err); got != jobs.OutcomePermanent {
		t.Errorf("outcome = %v, want OutcomePermanent for %v", got, err)
	}
}

// Nothing was tried, so the attempt is not spent: an instance that comes up
// before its bucket is configured must not burn a job's whole budget.
func TestAnUnconfiguredBucketDoesNotSpendAnAttempt(t *testing.T) {
	svc := NewService(stubBookings{}, stubClients{}, stubCourts{}, stubSchedules{}, &stubReports{},
		ExportDeps{}, 50*time.Second)

	err := runExportJob(t, svc, samplePayload())
	if got := jobs.Classify(err); got != jobs.OutcomeNotAttempted {
		t.Errorf("outcome = %v, want OutcomeNotAttempted for %v", got, err)
	}
}
