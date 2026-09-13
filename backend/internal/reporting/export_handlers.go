package reporting

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// startedExportBody is the 202's `export` object: the id to poll, the state it
// is in, and where to poll it. The status URL is served rather than assembled
// by the frontend, so a route change is one edit here instead of one in each
// client.
type startedExportBody struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	StatusURL string `json:"status_url"`
}

// CreatePaymentsExport handles POST /api/v1/complexes/{id}/reports/exports:
// it queues the workbook and answers immediately.
//
// The synchronous export (ExportPaymentsExcel) builds the file inside the
// request, which is why it needs a write-timeout-derived budget and answers
// 503 when it runs out. This one owes the request nothing but an id, so a
// month that takes ten seconds to compile is no longer a month the owner
// cannot export.
//
// A second POST for the same complex, period and Argentina day answers with
// the FIRST export's id rather than queueing another — see exportDedupKey.
func (h *Handler) CreatePaymentsExport(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	// Checked before the body is read, so a deployment that cannot export
	// says so rather than validating a period it will refuse anyway.
	if !h.svc.ExportsConfigured() {
		h.respond.Refuse(w, r, httpx.NotImplemented(ErrExportsNotConfigured.Error()))
		return
	}

	now := time.Now().In(timezone.Argentina)
	month, year, ok := h.readExportPeriod(w, r, now)
	if !ok {
		return
	}

	if err := validateReportPeriod(month, year, now, complex.CreatedAt); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	export, err := h.svc.StartPaymentsExport(r.Context(), complex, month, year, now)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	statusURL := exportStatusPath(complex.ID, export.ID)
	w.Header().Set("Location", statusURL)
	h.respond.JSON(w, r, http.StatusAccepted, httpx.Envelope{
		"export": startedExportBody{
			ID:        export.ID.String(),
			Status:    export.Status,
			StatusURL: statusURL,
		},
	})
}

// readExportPeriod reads the optional body's period, defaulting to the current
// Argentina month.
//
// The body is optional in the document, and an absent one is the ordinary
// request: the dashboard exports the month it is showing, and "the month it
// is showing" is usually this one. ReadJSON refuses an empty body — rightly,
// for every endpoint that requires one — so emptiness is checked first rather
// than the refusal being pattern-matched afterwards. That check cannot stop
// at r.ContentLength == 0: a request sent with Transfer-Encoding: chunked
// (curl --data-binary @- from an empty pipe, some proxies, some HTTP clients)
// carries no Content-Length at all, so net/http reports -1 rather than 0 even
// when the body turns out to hold zero bytes once read. bodyIsEmpty covers
// that case, and http.NoBody, alongside the plain ContentLength == 0 case.
func (h *Handler) readExportPeriod(w http.ResponseWriter, r *http.Request, now time.Time) (month, year int, ok bool) {
	month, year = int(now.Month()), now.Year()

	empty, err := bodyIsEmpty(r)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return 0, 0, false
	}
	if empty {
		return month, year, true
	}

	var input gen.ReportingCreatePaymentsExportJSONBody
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return 0, 0, false
	}
	if input.Month != nil {
		month = *input.Month
	}
	if input.Year != nil {
		year = *input.Year
	}
	return month, year, true
}

// bodyIsEmpty reports whether r carries no body worth decoding: an absent
// body (r.ContentLength == 0), the http.NoBody sentinel, or a chunked body
// (r.ContentLength == -1, whose length is unknown until read) that turns out
// to hold zero bytes. A positive ContentLength is trusted without reading
// ahead. A chunked body found to be non-empty has its first byte read back
// onto r.Body, unconsumed, so a later httpx.ReadJSON(r) still decodes the
// whole thing.
func bodyIsEmpty(r *http.Request) (bool, error) {
	if r.Body == nil || r.Body == http.NoBody || r.ContentLength == 0 {
		return true, nil
	}
	if r.ContentLength > 0 {
		return false, nil
	}

	var first [1]byte
	n, err := r.Body.Read(first[:])
	if n == 0 {
		if err != nil && !errors.Is(err, io.EOF) {
			return false, fmt.Errorf("reporting: read request body: %w", err)
		}
		return true, nil
	}

	r.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(first[:n]), r.Body), r.Body}
	return false, nil
}

// GetPaymentsExport handles
// GET /api/v1/complexes/{id}/reports/exports/{exportID}.
//
// A failed export is a 200 carrying `status: failed` and an embedded problem,
// not a 4xx: the request to READ the export succeeded, and what it found was
// an export that failed. Answering 422 here would make a polling client treat
// a perfectly good status read as its own failure.
func (h *Handler) GetPaymentsExport(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	if !h.svc.ExportsConfigured() {
		h.respond.Refuse(w, r, httpx.NotImplemented(ErrExportsNotConfigured.Error()))
		return
	}

	exportID, err := httpx.ReadUUIDParam(r, "exportID")
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	view, err := h.svc.PaymentsExport(r.Context(), complex.ID, exportID, time.Now())
	if err != nil {
		if errors.Is(err, ErrExportExpired) {
			// 410 rather than 404: this export existed and its file has been
			// collected, so asking again produces a new one. A 404 would read
			// as "there was never such an export".
			h.respond.Refuse(w, r, httpx.Gone(httpx.CodeExportExpired))
			return
		}
		// A foreign or unknown export id arrives here as ErrRecordNotFound,
		// which DomainError answers 404 — the same answer a cross-tenant read
		// gets, so the route cannot be used to probe another complex's ids.
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"export": toGenPaymentsExport(r, view)})
}
