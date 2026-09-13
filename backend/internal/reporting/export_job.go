package reporting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/jobs"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	"github.com/stodulski/vibe-server/internal/storage"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// TaskExportPayments is the job type the payments export is queued under.
const TaskExportPayments = "export:payments"

// exportMaxAttempts is the budget one export gets. Three, not the column's
// five: every attempt re-reads the same month and rebuilds the same workbook,
// so a fourth is unlikely to tell us anything a third did not, and the owner
// is waiting on a page that polls.
const exportMaxAttempts = 3

// exportDownloadTTL is how long a signed download URL lives.
//
// Fifteen minutes: long enough that an owner who opened the page, answered
// the phone and came back still has a working link, short enough that the URL
// is not worth forwarding. The URL is the whole of the authorization once it
// leaves us, so its lifetime is the whole of the revocation story.
const exportDownloadTTL = 15 * time.Minute

// exportLifetime is how long the object itself is kept.
//
// It is enforced by a lifecycle rule on the private bucket's `exports/`
// prefix, which the owner configures (docs/runbook-exports.md) — nothing here
// deletes it. This constant is what keeps the API honest about that rule: the
// jobs table keeps a finished row for seven days, so without this a status
// read on day three would cheerfully sign a URL for an object the lifecycle
// removed on day one, and the owner would follow it to a 404 from R2.
const exportLifetime = 24 * time.Hour

// The four states an export is reported in. They are the job's four states
// renamed for the client: 'processing' is an implementation word for a claim,
// and 'running' is what the owner's page is showing.
const (
	exportStatusPending = "pending"
	exportStatusRunning = "running"
	exportStatusDone    = "done"
	exportStatusFailed  = "failed"
)

// ErrExportsNotConfigured reports that no private object storage is wired, so
// there is nowhere to put a finished workbook. It mirrors
// complexes.ErrUploadsNotConfigured and answers the same 501: this deployment
// cannot do that, rather than this request was wrong.
var ErrExportsNotConfigured = errors.New("payments exports are not configured")

// ErrExportExpired reports a finished export whose file has passed
// exportLifetime. It is separate from ErrRecordNotFound because the two are
// different answers to the owner: the export was made and is gone, so asking
// again produces a new one.
var ErrExportExpired = errors.New("the export has passed its retention window")

// ExportPaymentsPayload is one queued export, as the job row carries it.
//
// ComplexID is the tenant, and it is the field the store's isolation
// predicate reads (jobs.Store.GetExport): the jobs table has no complex_id
// column, so this is where the tenant lives.
//
// ExportID names the object rather than the job. The job's own id is not
// available when the payload is built — Enqueue assigns it — and the worker
// receives only the payload, so a key derived from the job id could be
// computed by neither side. Name and slug are captured here rather than
// re-read by the worker because they are what the file is titled and called,
// and an export should carry the venue's name as it was when it was asked
// for.
type ExportPaymentsPayload struct {
	ExportID    string `json:"export_id"`
	ComplexID   string `json:"complex_id"`
	ComplexName string `json:"complex_name"`
	ComplexSlug string `json:"complex_slug"`
	Month       int    `json:"month"`
	Year        int    `json:"year"`
}

// ExportStore is the queue as this module uses it: enqueue an export, find the
// one a dedup key already named, free a dead key, and read one export back
// under its tenant.
type ExportStore interface {
	Enqueue(ctx context.Context, jobType string, payload any, runAt time.Time, maxAttempts int, dedupKey string) (uuid.UUID, bool, error)
	GetByDedupKey(ctx context.Context, key string) (*jobs.Job, error)
	ReleaseDedupKey(ctx context.Context, id uuid.UUID) (bool, error)
	GetExport(ctx context.Context, id uuid.UUID, complexID, jobType string) (*jobs.Job, error)
}

// ExportStorage is the private bucket: the worker writes the workbook, the
// status route signs a short-lived URL that reads it.
type ExportStorage interface {
	PutObject(ctx context.Context, obj storage.Object) error
	GeneratePresignedGET(ctx context.Context, key, downloadName string, ttl time.Duration) (string, error)
}

// ExportDeps is what an export needs beyond the readers the rest of this
// module uses: somewhere to queue the work and somewhere to put the file.
//
// Both are optional, and both have to be present for the feature to exist —
// see Service.ExportsConfigured. A deployment with no R2 private bucket, and
// the unit suite, both leave them nil.
type ExportDeps struct {
	Store   ExportStore
	Storage ExportStorage
}

// StartedExport is what accepting an export answers with: the id to poll and
// the state it is in right now, which is 'pending' for a fresh one and
// whatever the earlier one reached for a deduplicated request.
type StartedExport struct {
	ID     uuid.UUID
	Status string
}

// PaymentsExportView is one export as the status route reports it.
//
// FailureCode is a machine code, never the job's raw last_error: that is a Go
// error message, and putting it on the wire hands a tenant our internals.
type PaymentsExportView struct {
	ID          uuid.UUID
	Status      string
	DownloadURL string
	ExpiresAt   time.Time
	FailureCode string
}

// ExportsConfigured reports whether this deployment can run exports at all.
//
// Both halves are required and neither substitutes for the other: without the
// queue there is nothing to run the work, and without the private bucket
// there is nowhere to put the result that is not world-readable by key.
func (s *Service) ExportsConfigured() bool {
	return s.exports.Store != nil && s.exports.Storage != nil
}

// StartPaymentsExport queues an export and reports the one that will produce
// it — which, for a second request inside the same Argentina day, is the
// export the first request queued.
func (s *Service) StartPaymentsExport(ctx context.Context, complex *complexstore.Complex, month, year int, now time.Time) (StartedExport, error) {
	if !s.ExportsConfigured() {
		return StartedExport{}, ErrExportsNotConfigured
	}

	key := exportDedupKey(complex.ID, month, year, now)
	payload := ExportPaymentsPayload{
		ExportID:    uuid.NewString(),
		ComplexID:   complex.ID.String(),
		ComplexName: complex.Name,
		ComplexSlug: complex.Slug,
		Month:       month,
		Year:        year,
	}

	id, recorded, err := s.exports.Store.Enqueue(ctx, TaskExportPayments, payload, time.Time{}, exportMaxAttempts, key)
	if err != nil {
		return StartedExport{}, fmt.Errorf("queueing the payments export: %w", err)
	}
	if recorded {
		return StartedExport{ID: id, Status: exportStatusPending}, nil
	}

	return s.resumeExport(ctx, payload, key)
}

// resumeExport answers a deduplicated Enqueue.
//
// Three things can be holding the key, and they are three different answers.
// A pending, running or finished export is the one this request asked for, so
// its id is handed back and the owner's page polls the export that is already
// on its way. A dead-lettered one is a request nobody is serving any more, so
// its key is freed and the work queued again — the dead row stays as the
// record of what failed. And a key held by nothing at all is the retention
// sweep having deleted the row between the Enqueue and this read, which is
// simply a fresh export.
func (s *Service) resumeExport(ctx context.Context, payload ExportPaymentsPayload, key string) (StartedExport, error) {
	existing, err := s.exports.Store.GetByDedupKey(ctx, key)
	switch {
	case errors.Is(err, data.ErrRecordNotFound):
		return s.requeueExport(ctx, payload, key)
	case err != nil:
		return StartedExport{}, fmt.Errorf("reading the queued payments export: %w", err)
	}

	if existing.Status != jobs.StatusFailed {
		return StartedExport{ID: existing.ID, Status: exportStatusOf(existing.Status)}, nil
	}

	freed, err := s.exports.Store.ReleaseDedupKey(ctx, existing.ID)
	if err != nil {
		return StartedExport{}, fmt.Errorf("freeing the failed export's key: %w", err)
	}
	if !freed {
		// Another request got there first and the row is no longer failed;
		// theirs is the live export and this one polls it.
		return StartedExport{ID: existing.ID, Status: exportStatusOf(existing.Status)}, nil
	}
	return s.requeueExport(ctx, payload, key)
}

// requeueExport enqueues once more after the key was freed or found free.
//
// A second deduplication here means a concurrent request took the key in
// between, so the answer is whatever holds it now rather than a retry loop:
// two owners clicking at once must not be able to keep each other spinning.
func (s *Service) requeueExport(ctx context.Context, payload ExportPaymentsPayload, key string) (StartedExport, error) {
	id, recorded, err := s.exports.Store.Enqueue(ctx, TaskExportPayments, payload, time.Time{}, exportMaxAttempts, key)
	if err != nil {
		return StartedExport{}, fmt.Errorf("re-queueing the payments export: %w", err)
	}
	if recorded {
		return StartedExport{ID: id, Status: exportStatusPending}, nil
	}

	winner, err := s.exports.Store.GetByDedupKey(ctx, key)
	if err != nil {
		return StartedExport{}, fmt.Errorf("reading the export that took the key: %w", err)
	}
	return StartedExport{ID: winner.ID, Status: exportStatusOf(winner.Status)}, nil
}

// PaymentsExport reads one export back, scoped to the complex it belongs to.
//
// A download URL is minted here rather than stored, because it expires: an
// `expires_at` in a table would be a claim about a signature that was made
// once and is read many times. The signing is a local HMAC, so doing it on
// every poll costs nothing and is always truthful.
func (s *Service) PaymentsExport(ctx context.Context, complexID, exportID uuid.UUID, now time.Time) (PaymentsExportView, error) {
	if !s.ExportsConfigured() {
		return PaymentsExportView{}, ErrExportsNotConfigured
	}

	job, err := s.exports.Store.GetExport(ctx, exportID, complexID.String(), TaskExportPayments)
	if err != nil {
		return PaymentsExportView{}, err
	}

	view := PaymentsExportView{ID: job.ID, Status: exportStatusOf(job.Status)}
	switch view.Status {
	case exportStatusFailed:
		view.FailureCode = exportFailureCode(job.LastError)
		return view, nil
	case exportStatusDone:
	default:
		return view, nil
	}

	// Finished, so the file exists — unless the lifecycle rule has taken it.
	// UpdatedAt is when the row reached 'done', which is when the object was
	// written.
	if now.After(job.UpdatedAt.Add(exportLifetime)) {
		return PaymentsExportView{}, ErrExportExpired
	}

	var payload ExportPaymentsPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return PaymentsExportView{}, fmt.Errorf("reading export %s payload: %w", job.ID, err)
	}

	url, err := s.exports.Storage.GeneratePresignedGET(ctx,
		exportObjectKey(payload), exportFilename(payload), exportDownloadTTL)
	if err != nil {
		return PaymentsExportView{}, fmt.Errorf("signing the export download: %w", err)
	}

	view.DownloadURL = url
	view.ExpiresAt = now.Add(exportDownloadTTL)
	return view, nil
}

// exportDedupKey identifies one piece of export work: this complex, this
// period, today.
//
// The day is in the key on purpose. Without it the first export of a month
// would be the only one ever built for it, and an owner who exported on the
// 3rd and added payments on the 4th would keep downloading the 3rd's file. A
// calendar day is the granularity at which asking again is worth the work,
// and it is Argentina's day because that is the product's calendar.
func exportDedupKey(complexID uuid.UUID, month, year int, now time.Time) string {
	return jobs.DedupKey(TaskExportPayments,
		complexID.String(),
		strconv.Itoa(month),
		strconv.Itoa(year),
		now.In(timezone.Argentina).Format("2006-01-02"),
	)
}

// exportObjectKey is where the workbook lives in the private bucket. It
// matches the `exports/` prefix the 24 hour lifecycle rule is written
// against, and it is derived from the payload alone so the worker that writes
// it and the status route that signs a URL for it cannot disagree.
func exportObjectKey(p ExportPaymentsPayload) string {
	return fmt.Sprintf("exports/%s/%s.xlsx", p.ComplexID, p.ExportID)
}

// exportFilename is the name the owner's browser saves it under — the same
// name the synchronous export has always produced.
func exportFilename(p ExportPaymentsPayload) string {
	return fmt.Sprintf("pagos_%s_%d_%d.xlsx", p.ComplexSlug, p.Month, p.Year)
}

// exportStatusPath is the status resource for one export, which is both the
// Location of an accepted POST and the `instance` of any problem raised about
// that export.
func exportStatusPath(complexID, exportID uuid.UUID) string {
	return fmt.Sprintf("/api/v1/complexes/%s/reports/exports/%s", complexID, exportID)
}

// exportStatusOf maps a job's state onto the export's.
func exportStatusOf(jobStatus string) string {
	switch jobStatus {
	case jobs.StatusProcessing:
		return exportStatusRunning
	case jobs.StatusDone:
		return exportStatusDone
	case jobs.StatusFailed:
		return exportStatusFailed
	default:
		return exportStatusPending
	}
}

// exportFailureCode turns a dead-lettered job's last_error into something the
// frontend can map to a sentence.
//
// Only the refusal the owner can act on is named. Everything else — a store
// failure, a serialisation error, an expired job — is one code, because
// last_error is a Go error message and the alternative to collapsing it is
// publishing our internals to a tenant.
func exportFailureCode(lastError string) string {
	if strings.HasPrefix(lastError, httpx.CodeExportTooLarge) {
		return httpx.CodeExportTooLarge
	}
	return httpx.CodeExportFailed
}

// exportProblem builds the Problem embedded in a failed export's 200 body.
//
// It is assembled here rather than through the Responder because that writes
// a Problem AS the response, and this one travels inside a successful one.
// Every field still comes from the same places the Responder's would: the
// kind's own URI and title, the request's path as `instance` — which is the
// status resource itself, since this is the only route that reports on a
// specific export — and the request id, so a failed export is traceable to
// the log line the worker left.
func exportProblem(r *http.Request, status int, kind httpx.Kind, code string) httpx.Problem {
	return httpx.Problem{
		Type:      kind.URI(),
		Title:     kind.Title(),
		Status:    status,
		Detail:    code,
		Instance:  r.URL.Path,
		RequestID: httpx.ContextGetRequestID(r),
	}
}

// exportFailureProblem is the Problem for a failure code: the row of the
// status table that says which HTTP status and which kind a given refusal
// would have been answered with, had it been answered synchronously.
func exportFailureProblem(r *http.Request, code string) httpx.Problem {
	if code == httpx.CodeExportTooLarge {
		return exportProblem(r, http.StatusUnprocessableEntity, httpx.KindValidation, code)
	}
	return exportProblem(r, http.StatusInternalServerError, httpx.KindInternal, code)
}

// toGenProblem maps an httpx.Problem onto the generated wire type field by
// field, for the same reason toGenMonthlyReport does it: a JSON round trip
// would let a renamed field vanish silently, and an explicit mapping fails to
// compile instead.
func toGenProblem(p httpx.Problem) *gen.Problem {
	out := &gen.Problem{Type: p.Type, Title: p.Title, Status: p.Status}
	if p.Detail != "" {
		out.Detail = &p.Detail
	}
	if p.Instance != "" {
		out.Instance = &p.Instance
	}
	if p.RequestID != "" {
		out.RequestId = &p.RequestID
	}
	return out
}

// toGenPaymentsExport maps one export onto the wire type, attaching the
// embedded problem when it failed.
func toGenPaymentsExport(r *http.Request, view PaymentsExportView) gen.PaymentsExport {
	out := gen.PaymentsExport{Id: view.ID, Status: gen.PaymentsExportStatus(view.Status)}
	if view.DownloadURL != "" {
		out.DownloadUrl = &view.DownloadURL
		expires := view.ExpiresAt
		out.ExpiresAt = &expires
	}
	if view.FailureCode != "" {
		out.Error = toGenProblem(exportFailureProblem(r, view.FailureCode))
	}
	return out
}
