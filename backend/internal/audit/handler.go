package audit

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	auditstore "github.com/stodulski/vibe-server/internal/audit/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
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
		filters data.Filters) ([]*auditstore.AuditLogRow, data.Metadata, error)
}

// Handler serves a tenant's own audit trail. It decodes, validates, and maps
// errors onto HTTP; the read and the entry it writes live in the Service.
type Handler struct {
	svc     *Service
	respond *httpx.Responder
	// trustProxies decides which address is recorded as the caller's.
	trustProxies bool
}

// NewHandler returns a Handler backed by the given service.
func NewHandler(svc *Service, respond *httpx.Responder, trustProxies bool) *Handler {
	return &Handler{svc: svc, respond: respond, trustProxies: trustProxies}
}

// Routes registers the tenant-facing trail. It is scoped to one complex and
// readable only by its owner.
func (h *Handler) Routes(router httpx.Router, guards httpx.Guards) {
	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/{id}/audit-log",
		guards.RequireAuth(guards.RequireComplexOwner(h.List)))
}

// List handles GET /api/v1/complexes/{id}/audit-log.
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

	// A read of the trail is an event on the trail, and the service records it
	// after the query so that a failed read is not reported as a completed one.
	logs, metadata, err := h.svc.List(r.Context(), h.actor(r), complex.ID, entityType, filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidCursor):
			h.respond.BadRequest(w, r, fmt.Errorf("invalid cursor value"))
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"audit_logs": toGenAuditLogRows(logs),
		"metadata":   toGenMetadata(metadata),
	})
}

// toGenAuditLogRows maps a page of store rows onto the generated wire type,
// field by field, so the store's sqlc-adjacent struct never reaches encoding
// directly (HTTP-08). Nil in is nil out: the store returns a non-nil,
// possibly empty slice for a real query and a nil one from a zero-value
// stub, and the response preserves that distinction (`[]` vs `null`)
// exactly as the store type serializing directly used to.
func toGenAuditLogRows(logs []*auditstore.AuditLogRow) []gen.AuditLogRow {
	if logs == nil {
		return nil
	}
	rows := make([]gen.AuditLogRow, len(logs))
	for i, l := range logs {
		rows[i] = toGenAuditLogRow(l)
	}
	return rows
}

// toGenAuditLogRow maps one store row onto gen.AuditLogRow. Every field lines
// up one-to-one, UserEmail included: the OpenAPI document declares it a plain
// string (format: email was dropped — see the doc-fix note in
// docs/auditoria-backend-2026-09-11, it made oapi-codegen emit
// openapi_types.Email, which fails to marshal any stored value that is not a
// valid net/mail address, including empty/legacy data), so it needs no
// conversion from the store's own *string.
func toGenAuditLogRow(l *auditstore.AuditLogRow) gen.AuditLogRow {
	return gen.AuditLogRow{
		Action:     l.Action,
		ComplexId:  l.ComplexID,
		CreatedAt:  l.CreatedAt,
		EntityId:   l.EntityID,
		EntityType: l.EntityType,
		Id:         l.ID,
		IpAddress:  l.IPAddress,
		UserEmail:  l.UserEmail,
		UserId:     l.UserID,
	}
}

// toGenMetadata maps data.Metadata's zero-value-means-absent fields onto
// gen.Metadata's pointer-plus-omitempty fields, reproducing the same
// wire behaviour: an empty cursor or a zero total count is an absent key on
// both sides.
func toGenMetadata(m data.Metadata) gen.Metadata {
	meta := gen.Metadata{HasMore: m.HasMore}
	if m.NextCursor != "" {
		cursor := m.NextCursor
		meta.NextCursor = &cursor
	}
	if m.TotalCount != 0 {
		total := m.TotalCount
		meta.TotalCount = &total
	}
	return meta
}

// actor reads who is asking, and from where, off the request. It is the only
// thing the entry needs that lives on the HTTP side.
func (h *Handler) actor(r *http.Request) Actor {
	var userID *uuid.UUID
	if user, ok := httpx.ContextGetAuthenticatedUser(r); ok {
		userID = &user.ID
	}
	return Actor{UserID: userID, IP: httpx.ClientIP(r, h.trustProxies)}
}
