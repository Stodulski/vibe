package admin

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/validator"
)

// Stats handles GET /api/v1/admin/stats, returning the platform-wide totals.
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.Stats(r.Context(), h.actor(r))
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"stats": stats})
}

// ListUsers handles GET /api/v1/admin/users, the operator's user directory.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	search := httpx.ReadString(qs, "search", "")
	role := httpx.ReadString(qs, "role", "")

	v := validator.New()
	v.Check(len(search) <= 200, "search", "must not be more than 200 characters")
	if role != "" {
		v.Check(role == "owner" || role == "client" || role == "superadmin", "role", "invalid role value")
	}

	filters := data.Filters{
		Cursor: httpx.ReadString(qs, "cursor", ""),
		Limit:  httpx.ReadInt(qs, "limit", 50),
	}

	data.ValidateFilters(v, filters)
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	users, metadata, err := h.svc.ListUsers(r.Context(), h.actor(r), search, role, filters)
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
		"users":    users,
		"metadata": metadata,
	})
}

// GetUser handles GET /api/v1/admin/users/:id, returning one account with the
// complexes and activity attached to it.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ReadUUIDParam(r, "id")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	detail, err := h.svc.GetUser(r.Context(), h.actor(r), id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"user":      detail.User,
		"complexes": detail.Complexes,
	})
}

// ListComplexes handles GET /api/v1/admin/complexes, the operator's directory
// of every complex on the platform.
func (h *Handler) ListComplexes(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	search := httpx.ReadString(qs, "search", "")

	filters := data.Filters{
		Cursor: httpx.ReadString(qs, "cursor", ""),
		Limit:  httpx.ReadInt(qs, "limit", 50),
	}

	v := validator.New()
	v.Check(len(search) <= 200, "search", "must not be more than 200 characters")
	data.ValidateFilters(v, filters)
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	complexes, metadata, err := h.svc.ListComplexes(r.Context(), h.actor(r), search, filters)
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
		"complexes": complexes,
		"metadata":  metadata,
	})
}

// GetComplex handles GET /api/v1/admin/complexes/:id, returning one complex
// with its owner and usage.
func (h *Handler) GetComplex(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ReadUUIDParam(r, "id")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	detail, err := h.svc.GetComplex(r.Context(), h.actor(r), id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"complex":        detail.Complex,
		"owner_name":     detail.OwnerName,
		"owner_email":    detail.OwnerEmail,
		"courts_count":   detail.CourtsCount,
		"clients_count":  detail.ClientsCount,
		"bookings_count": detail.BookingsCount,
		"total_revenue":  detail.TotalRevenue,
	})
}

// ListAuditLogs handles GET /api/v1/admin/audit-log, the platform-wide audit
// trail, optionally narrowed to one complex or entity type.
func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	entityType := httpx.ReadString(qs, "entity_type", "")

	var complexID *uuid.UUID
	if raw := httpx.ReadString(qs, "complex_id", ""); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			h.respond.BadRequest(w, r, fmt.Errorf("invalid complex_id"))
			return
		}
		complexID = &id
	}

	filters := data.Filters{
		Cursor: httpx.ReadString(qs, "cursor", ""),
		Limit:  httpx.ReadInt(qs, "limit", 50),
	}

	v := validator.New()
	data.ValidateFilters(v, filters)
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	logs, metadata, err := h.svc.ListAuditLogs(r.Context(), h.actor(r), complexID, entityType, filters)
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
		"audit_logs": logs,
		"metadata":   metadata,
	})
}

// ToggleUserActive handles PATCH /api/v1/admin/users/:id/toggle-active, the
// only write this module allows.
func (h *Handler) ToggleUserActive(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ReadUUIDParam(r, "id")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	var input struct {
		IsActive bool `json:"is_active"`
	}

	err = httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	if _, ok := httpx.ContextGetAuthenticatedUser(r); !ok {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	err = h.svc.ToggleUserActive(r.Context(), h.actor(r), id, input.IsActive)
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "user updated"})
}
