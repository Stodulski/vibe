package admin

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/validator"
)

// Stats handles GET /api/v1/admin/stats, returning the platform-wide totals.
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.GetPlatformStats(r.Context())
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.recordRead(r, "platform_stats", nil, nil, nil)

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

	users, metadata, err := h.store.ListUsers(r.Context(), search, role, filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidCursor):
			h.respond.BadRequest(w, r, fmt.Errorf("invalid cursor value"))
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	scope := map[string]any{"returned": len(users)}
	if search != "" {
		scope["search"] = search
	}
	if role != "" {
		scope["role"] = role
	}
	h.recordRead(r, "user", nil, nil, scope)

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

	detail, err := h.store.GetUserDetail(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.recordRead(r, "user", nil, &id, nil)

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

	complexes, metadata, err := h.store.ListComplexes(r.Context(), search, filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidCursor):
			h.respond.BadRequest(w, r, fmt.Errorf("invalid cursor value"))
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	scope := map[string]any{"returned": len(complexes)}
	if search != "" {
		scope["search"] = search
	}
	h.recordRead(r, "complex", nil, nil, scope)

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

	detail, err := h.store.GetComplexDetail(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.recordRead(r, "complex", &id, &id, nil)

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

	logs, metadata, err := h.store.ListAuditLogs(r.Context(), complexID, entityType, filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidCursor):
			h.respond.BadRequest(w, r, fmt.Errorf("invalid cursor value"))
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	logScope := map[string]any{"returned": len(logs)}
	if entityType != "" {
		logScope["entity_type"] = entityType
	}
	h.recordRead(r, "audit_log", complexID, nil, logScope)

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

	currentUser, ok := httpx.ContextGetAuthenticatedUser(r)
	if !ok {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}
	if currentUser.ID == id {
		h.respond.Error(w, r, http.StatusConflict, "cannot modify your own account status")
		return
	}

	err = h.store.ToggleUserActive(r.Context(), id, input.IsActive)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.audit.Record(audit.Entry{
		UserID:     actingUserID(r),
		Action:     "toggle_active",
		EntityType: "user",
		EntityID:   &id,
		NewValue:   map[string]any{"is_active": input.IsActive},
		IPAddress:  httpx.ClientIP(r, h.trustProxies),
	})

	// The user's cached record carries is_active, so a stale copy would keep a
	// deactivated account working until the entry expired.
	h.cache.InvalidateUser(r.Context(), id)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "user updated"})
}
