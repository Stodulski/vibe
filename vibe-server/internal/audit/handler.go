package audit

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/validator"
)

// defaultPageSize is the audit-log page a caller gets without asking.
const defaultPageSize = 50

// Reader is the read side of the trail this package writes.
//
// It is declared here, by the consumer, for the same reason Store is: the
// endpoint needs one query, and should not depend on everything the admin
// store happens to expose.
type Reader interface {
	ListAuditLogs(ctx context.Context, complexID *uuid.UUID, entityType string,
		filters data.Filters) ([]*data.AuditLogRow, data.Metadata, error)
}

// Handler serves a tenant's own audit trail.
//
// The trail existed only behind /api/v1/admin/audit-log, which is superadmin
// only — so the record of who changed a venue's bookings, courts and prices was
// readable by the platform and not by the venue. That is backwards for the one
// table whose purpose is accountability: the owner is the person who needs to
// see that a staff account cancelled a booking, and the platform is the party
// the trail should also be able to hold to account.
type Handler struct {
	reader  Reader
	audit   *Recorder
	respond *httpx.Responder
	// trustProxies decides which address is recorded as the caller's.
	trustProxies bool
}

// NewHandler returns a Handler. recorder is the same one the writing modules
// use: a read of the trail is itself an event on the trail.
func NewHandler(reader Reader, recorder *Recorder, respond *httpx.Responder, trustProxies bool) *Handler {
	return &Handler{reader: reader, audit: recorder, respond: respond, trustProxies: trustProxies}
}

// Routes registers the tenant-facing trail. It is scoped to one complex and
// readable only by its owner.
func (h *Handler) Routes(router httpx.Router, guards httpx.Guards) {
	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/audit-log",
		guards.RequireAuth(guards.RequireComplexOwner(h.List)))
}

// List handles GET /api/v1/complexes/:id/audit-log.
//
// The scope comes from the complex the ownership guard put in the context, and
// never from the query string: a complex_id parameter here would be a caller
// naming the tenant whose history they want to read, which is the one thing
// this endpoint must not accept.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	qs := r.URL.Query()
	entityType := httpx.ReadString(qs, "entity_type", "")
	filters := data.Filters{
		Cursor: httpx.ReadString(qs, "cursor", ""),
		Limit:  httpx.ReadInt(qs, "limit", defaultPageSize),
	}

	v := validator.New()
	data.ValidateFilters(v, filters)
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	complexID := complex.ID
	logs, metadata, err := h.reader.ListAuditLogs(r.Context(), &complexID, entityType, filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidCursor):
			h.respond.BadRequest(w, r, fmt.Errorf("invalid cursor value"))
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	// A read of the trail is an event on the trail. Recording it after the
	// query means a failed read is not reported as a completed one.
	//
	//nolint:contextcheck // Record deliberately detaches: the entry describes
	// something that already happened, so it must still be written when the
	// client hangs up mid-response. See Recorder.Record.
	h.record(r, complexID, len(logs), entityType)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"audit_logs": logs,
		"metadata":   metadata,
	})
}

// record writes the entry for one read of the trail.
//
// It records what was read — the scope and how much of it came back — rather
// than the rows themselves: copying the page into new_value would double the
// table on every read and, worse, would mean the trail's own contents get
// rewritten into it under a different actor.
func (h *Handler) record(r *http.Request, complexID uuid.UUID, count int, entityType string) {
	var userID *uuid.UUID
	if user, ok := httpx.ContextGetAuthenticatedUser(r); ok {
		userID = &user.ID
	}

	scope := map[string]any{"returned": count}
	if entityType != "" {
		scope["entity_type"] = entityType
	}

	h.audit.Record(Entry{
		UserID:     userID,
		ComplexID:  &complexID,
		Action:     "read",
		EntityType: "audit_log",
		NewValue:   scope,
		IPAddress:  httpx.ClientIP(r, h.trustProxies),
	})
}
