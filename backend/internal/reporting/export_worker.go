package reporting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/jobs"
	"github.com/stodulski/vibe-server/internal/storage"
)

// exportContentType is the media type the workbook is stored and served as.
// It is set on the object rather than only on the signed URL, so the file is
// correct however it is later read.
const exportContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// Queue is the registration side of the durable work queue, as this module
// needs it. It is the same surface notifications.Queue registers through,
// declared here rather than imported so reporting does not depend on the
// notifications package to run a job of its own.
type Queue interface {
	RegisterHandler(taskType string, h func(ctx context.Context, payload json.RawMessage) error)
}

// RegisterExportWorker wires TaskExportPayments to the function that builds
// and uploads the workbook. It must be called before the pool is started, on
// every instance that consumes.
//
// It shares the one pool rather than running a second one. Pool.work claims
// with no type filter, so a second pool would take notification jobs off the
// same table and run them with no handler registered. TaskExportPayments
// still gets its own attempt timeout rather than the pool's shared 10s
// JobTimeout, though: cmd/api registers this type in the pool's
// jobs.Config.Timeouts, keyed to exportBudgetFor's synchronous-route budget
// plus exportUploadAllowance, so a large export gets the same headroom the
// deprecated synchronous route has, plus time for the R2 upload that route
// never has to do. Raising JobTimeout itself would have widened the lease
// for every other type sharing this pool.
func (s *Service) RegisterExportWorker(q Queue) {
	q.RegisterHandler(TaskExportPayments, func(ctx context.Context, raw json.RawMessage) error {
		var payload ExportPaymentsPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			// Retrying re-reads the same bytes for the same answer.
			return fmt.Errorf("%s: unmarshal payload: %w: %w", TaskExportPayments, err, jobs.ErrPermanent)
		}
		return s.runPaymentsExport(ctx, payload)
	})
}

// runPaymentsExport is one attempt: read the month, build the workbook, put it
// in the private bucket.
//
// The three failure shapes are deliberately different. A payload that cannot
// name a complex, and a period too large to export, are permanent: both
// re-read identically on every attempt, so they dead-letter now rather than
// five attempts from now, and the status route turns the dead letter into a
// code the owner can read. A missing bucket is not-attempted, because nothing
// was tried and the deployment may be mid-configuration. Everything else —
// the database, R2 — is transient and comes back on the backoff, bounded by
// the pool's 24 hour MaxAge.
func (s *Service) runPaymentsExport(ctx context.Context, payload ExportPaymentsPayload) error {
	if !s.ExportsConfigured() {
		return fmt.Errorf("%s: %w: %w", TaskExportPayments, ErrExportsNotConfigured, jobs.ErrNotAttempted)
	}

	complexID, err := uuid.Parse(payload.ComplexID)
	if err != nil {
		return fmt.Errorf("%s: payload names no complex: %w: %w", TaskExportPayments, err, jobs.ErrPermanent)
	}

	// The worker runs outside the HTTP chain, so nothing has stamped a tenant
	// on this context — and an unscoped session matches no tenant policy at
	// all, which would make the export a silently empty workbook rather than
	// an error. The scope is the complex the payload names, never the bypass:
	// this job reads one venue's ledger and has no business seeing another's.
	ctx = data.ContextWithTenant(ctx, complexID)

	export, rows, err := s.buildPaymentsExport(ctx, complexID,
		payload.ComplexName, payload.ComplexSlug, payload.Month, payload.Year)
	switch {
	case errors.Is(err, ErrExportTooLarge):
		// The code leads the message because the status route reads it back
		// out of last_error; see exportFailureCode.
		return fmt.Errorf("%s: %d rows over the %d cap: %w",
			httpx.CodeExportTooLarge, rows, s.maxExportRows, jobs.ErrPermanent)
	case err != nil:
		return fmt.Errorf("%s: building the workbook: %w", TaskExportPayments, err)
	}

	// In memory rather than through a temp file: the row cap already bounds
	// the workbook to a few megabytes, and a temp file would add a cleanup
	// path that has to survive a process the pool may kill mid-attempt.
	return s.exports.Storage.PutObject(ctx, storage.Object{
		Key:                exportObjectKey(payload),
		ContentType:        exportContentType,
		ContentDisposition: fmt.Sprintf(`attachment; filename="%s"`, export.Filename),
		Body:               export.Body.Bytes(),
	})
}
