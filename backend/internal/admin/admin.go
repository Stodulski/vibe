package admin

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	adminstore "github.com/stodulski/vibe-server/internal/admin/store"
	auditstore "github.com/stodulski/vibe-server/internal/audit/store"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	gen "github.com/stodulski/vibe-server/internal/openapi/gen"
	"github.com/stodulski/vibe-server/internal/validator"
)

// toGenPlatformStats maps the store's platform-wide counters onto the
// generated wire type. Every field lines up one-to-one with no omitempty on
// either side, so this is a plain field copy.
func toGenPlatformStats(s *adminstore.PlatformStats) gen.PlatformStats {
	return gen.PlatformStats{
		TotalUsers:        s.TotalUsers,
		ActiveUsers:       s.ActiveUsers,
		NewUsersMonth:     s.NewUsersMonth,
		TotalComplexes:    s.TotalComplexes,
		NewComplexesMonth: s.NewComplexesMonth,
		TotalCourts:       s.TotalCourts,
		TotalBookings:     s.TotalBookings,
		TotalRevenue:      s.TotalRevenue,
	}
}

// adminUserRowView mirrors gen.AdminUserRow's wire shape, but keeps email as
// a plain string.
//
// WIRE MISMATCH: admin ListUsers email — gen.AdminUserRow.Email
// (openapi_types.Email) runs net/mail.ParseAddress on marshal and errors the
// whole response (turned into a 500 by httpx.Responder.JSON) for any value
// it rejects, including an empty string. The store's email column carries no
// such constraint at the JSON layer — a blank or malformed stored value has
// always gone out as plain text. Kept as a local view with a plain string so
// the wire, and its failure behaviour, stays identical. (Proven by
// TestEveryAdminReadIsRecorded/user_directory, which seeds a row with a
// zero-value email and 500'd under gen.AdminUserRow.)
type adminUserRowView struct {
	ID            uuid.UUID            `json:"id"`
	Email         string               `json:"email"`
	FirstName     string               `json:"first_name"`
	LastName      string               `json:"last_name"`
	Phone         string               `json:"phone"`
	Role          gen.AdminUserRowRole `json:"role"`
	IsActive      bool                 `json:"is_active"`
	EmailVerified bool                 `json:"email_verified"`
	CreatedAt     time.Time            `json:"created_at"`
	ComplexCount  int                  `json:"complex_count"`
}

// toAdminUserRowView maps one directory row onto the wire-preserving local
// view. See adminUserRowView's WIRE MISMATCH note for why this is not
// gen.AdminUserRow.
func toAdminUserRowView(u *adminstore.AdminUserRow) adminUserRowView {
	return adminUserRowView{
		ID:            u.ID,
		Email:         u.Email,
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		Phone:         u.Phone,
		Role:          gen.AdminUserRowRole(u.Role),
		IsActive:      u.IsActive,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
		ComplexCount:  u.ComplexCount,
	}
}

// toAdminUserRowViews maps a page of the user directory, preserving nilness
// so an unset (as opposed to empty) page still serializes as JSON null.
func toAdminUserRowViews(users []*adminstore.AdminUserRow) []adminUserRowView {
	if users == nil {
		return nil
	}
	rows := make([]adminUserRowView, 0, len(users))
	for _, u := range users {
		rows = append(rows, toAdminUserRowView(u))
	}
	return rows
}

// adminComplexRowView mirrors gen.AdminComplexRow's wire shape, but keeps
// owner_email as a plain string. See adminUserRowView's WIRE MISMATCH note —
// the same openapi_types.Email marshal failure applies to OwnerEmail here.
type adminComplexRowView struct {
	ID          uuid.UUID `json:"id"`
	OwnerID     uuid.UUID `json:"owner_id"`
	OwnerName   string    `json:"owner_name"`
	OwnerEmail  string    `json:"owner_email"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	City        string    `json:"city"`
	IsActive    bool      `json:"is_active"`
	CourtsCount int       `json:"courts_count"`
	MpConnected bool      `json:"mp_connected"`
	CreatedAt   time.Time `json:"created_at"`
}

// toAdminComplexRowView maps one directory row onto the wire-preserving local
// view. See adminComplexRowView's WIRE MISMATCH note for why this is not
// gen.AdminComplexRow.
func toAdminComplexRowView(c *adminstore.AdminComplexRow) adminComplexRowView {
	return adminComplexRowView{
		ID:          c.ID,
		OwnerID:     c.OwnerID,
		OwnerName:   c.OwnerName,
		OwnerEmail:  c.OwnerEmail,
		Name:        c.Name,
		Slug:        c.Slug,
		City:        c.City,
		IsActive:    c.IsActive,
		CourtsCount: c.CourtsCount,
		MpConnected: c.MPConnected,
		CreatedAt:   c.CreatedAt,
	}
}

// toAdminComplexRowViews maps a page of the complex directory, preserving
// nilness so an unset (as opposed to empty) page still serializes as JSON null.
func toAdminComplexRowViews(complexes []*adminstore.AdminComplexRow) []adminComplexRowView {
	if complexes == nil {
		return nil
	}
	rows := make([]adminComplexRowView, 0, len(complexes))
	for _, c := range complexes {
		rows = append(rows, toAdminComplexRowView(c))
	}
	return rows
}

// auditLogRowView mirrors gen.AuditLogRow's wire shape, but keeps
// user_email as a plain *string. See adminUserRowView's WIRE MISMATCH note —
// the same openapi_types.Email marshal failure applies to a non-nil UserEmail
// here, and the audit trail is exactly the place a bad or legacy stored email
// must not take the whole read down.
type auditLogRowView struct {
	ID         uuid.UUID  `json:"id"`
	UserID     *uuid.UUID `json:"user_id"`
	UserEmail  *string    `json:"user_email"`
	ComplexID  *uuid.UUID `json:"complex_id"`
	Action     string     `json:"action"`
	EntityType string     `json:"entity_type"`
	EntityID   *uuid.UUID `json:"entity_id"`
	IPAddress  *string    `json:"ip_address"`
	CreatedAt  time.Time  `json:"created_at"`
}

// toAuditLogRowView maps one audit trail row onto the wire-preserving local
// view. See auditLogRowView's WIRE MISMATCH note for why this is not
// gen.AuditLogRow. Every pointer field lacks omitempty on both sides, so a
// nil pointer keeps serializing as JSON null rather than being dropped.
func toAuditLogRowView(l *auditstore.AuditLogRow) auditLogRowView {
	return auditLogRowView{
		ID:         l.ID,
		UserID:     l.UserID,
		UserEmail:  l.UserEmail,
		ComplexID:  l.ComplexID,
		Action:     l.Action,
		EntityType: l.EntityType,
		EntityID:   l.EntityID,
		IPAddress:  l.IPAddress,
		CreatedAt:  l.CreatedAt,
	}
}

// toAuditLogRowViews maps a page of the audit trail, preserving nilness so an
// unset (as opposed to empty) page still serializes as JSON null.
func toAuditLogRowViews(logs []*auditstore.AuditLogRow) []auditLogRowView {
	if logs == nil {
		return nil
	}
	rows := make([]auditLogRowView, 0, len(logs))
	for _, l := range logs {
		rows = append(rows, toAuditLogRowView(l))
	}
	return rows
}

// toGenMetadata maps the store's pagination metadata onto the generated wire
// type. data.Metadata omits NextCursor/TotalCount from JSON on their zero
// value (both tagged omitempty on a non-pointer); gen.Metadata carries the
// same fields as pointers, also omitempty, so nil reproduces the same
// omitted-key behaviour.
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

// adminUserView mirrors gen.User's wire shape, but keeps email as a plain
// string. See adminUserRowView's WIRE MISMATCH note — the same
// openapi_types.Email marshal failure applies to gen.User.Email here.
type adminUserView struct {
	ID            uuid.UUID    `json:"id"`
	Email         string       `json:"email"`
	FirstName     string       `json:"first_name"`
	LastName      string       `json:"last_name"`
	Phone         string       `json:"phone"`
	Role          gen.UserRole `json:"role"`
	IsActive      bool         `json:"is_active"`
	EmailVerified bool         `json:"email_verified"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// toAdminUserView maps a platform account onto the wire-preserving local
// view. See adminUserView's WIRE MISMATCH note for why this is not gen.User.
func toAdminUserView(u *authstore.User) adminUserView {
	return adminUserView{
		ID:            u.ID,
		Email:         u.Email,
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		Phone:         u.Phone,
		Role:          gen.UserRole(u.Role),
		IsActive:      u.IsActive,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

// adminComplexView mirrors complexstore.Complex's wire shape field for field.
//
// WIRE MISMATCH: admin GetUser/GetComplex complex — the generated gen.Complex
// cannot reproduce today's shape: it has no `version` field at all, and it
// tags email/logo_url/cover_url/latitude/longitude as omitempty where the
// store type (and today's actual response) always emits them, `null` when
// unset. It also narrows latitude/longitude to float32, which would truncate
// precision the store carries as float64. Kept as a local type instead of
// gen.Complex so the wire stays byte-identical.
type adminComplexView struct {
	ID                uuid.UUID  `json:"id"`
	OwnerID           uuid.UUID  `json:"owner_id"`
	Name              string     `json:"name"`
	Slug              string     `json:"slug"`
	Address           string     `json:"address"`
	City              string     `json:"city"`
	Province          string     `json:"province"`
	CountryCode       string     `json:"country_code"`
	Currency          string     `json:"currency"`
	Phone             string     `json:"phone"`
	Email             *string    `json:"email"`
	LogoURL           *string    `json:"logo_url"`
	CoverURL          *string    `json:"cover_url"`
	DepositPercentage int        `json:"deposit_percentage"`
	CancellationHours int        `json:"cancellation_hours"`
	Latitude          *float64   `json:"latitude"`
	Longitude         *float64   `json:"longitude"`
	IsActive          bool       `json:"is_active"`
	Amenities         []string   `json:"amenities"`
	CourtCount        *int       `json:"court_count,omitempty"`
	PaymentsEnabled   bool       `json:"payments_enabled"`
	MPUserID          *string    `json:"mp_user_id,omitempty"`
	Version           int        `json:"version"`
	MPTokenExpiresAt  *time.Time `json:"mp_token_expires_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// toAdminComplexView maps a complex onto the wire-preserving local view. See
// adminComplexView's WIRE MISMATCH note for why this is not gen.Complex.
func toAdminComplexView(c *complexstore.Complex) adminComplexView {
	return adminComplexView{
		ID:                c.ID,
		OwnerID:           c.OwnerID,
		Name:              c.Name,
		Slug:              c.Slug,
		Address:           c.Address,
		City:              c.City,
		Province:          c.Province,
		CountryCode:       c.CountryCode,
		Currency:          c.Currency,
		Phone:             c.Phone,
		Email:             c.Email,
		LogoURL:           c.LogoURL,
		CoverURL:          c.CoverURL,
		DepositPercentage: c.DepositPercentage,
		CancellationHours: c.CancellationHours,
		Latitude:          c.Latitude,
		Longitude:         c.Longitude,
		IsActive:          c.IsActive,
		Amenities:         c.Amenities,
		CourtCount:        c.CourtCount,
		PaymentsEnabled:   c.PaymentsEnabled,
		MPUserID:          c.MPUserID,
		Version:           c.Version,
		MPTokenExpiresAt:  c.MPTokenExpiresAt,
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}
}

// toAdminComplexViews maps a slice of complexes, preserving nilness so an
// unset (as opposed to empty) slice still serializes as JSON null.
func toAdminComplexViews(complexes []*complexstore.Complex) []adminComplexView {
	if complexes == nil {
		return nil
	}
	views := make([]adminComplexView, 0, len(complexes))
	for _, c := range complexes {
		views = append(views, toAdminComplexView(c))
	}
	return views
}

// Stats handles GET /api/v1/admin/stats, returning the platform-wide totals.
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.Stats(r.Context(), h.actor(r))
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	var out *gen.PlatformStats
	if stats != nil {
		mapped := toGenPlatformStats(stats)
		out = &mapped
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"stats": out})
}

// ListUsers handles GET /api/v1/admin/users, the operator's user directory.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	search := httpx.ReadString(qs, "search", "")
	role := httpx.ReadString(qs, "role", "")

	v := validator.New()
	v.Check(len(search) <= 200, "search", "must not be more than 200 characters")
	if role != "" {
		v.Check(gen.AdminListUsersParamsRole(role).Valid(), "role", "invalid role value")
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
		"users":    toAdminUserRowViews(users),
		"metadata": toGenMetadata(metadata),
	})
}

// GetUser handles GET /api/v1/admin/users/{id}, returning one account with the
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

	var user *adminUserView
	if detail.User != nil {
		mapped := toAdminUserView(detail.User)
		user = &mapped
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"user":      user,
		"complexes": toAdminComplexViews(detail.Complexes),
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
		"complexes": toAdminComplexRowViews(complexes),
		"metadata":  toGenMetadata(metadata),
	})
}

// GetComplex handles GET /api/v1/admin/complexes/{id}, returning one complex
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

	var complex *adminComplexView
	if detail.Complex != nil {
		mapped := toAdminComplexView(detail.Complex)
		complex = &mapped
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"complex":        complex,
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
		"audit_logs": toAuditLogRowViews(logs),
		"metadata":   toGenMetadata(metadata),
	})
}

// ToggleUserActive handles PATCH /api/v1/admin/users/{id}/toggle-active, the
// only write this module allows.
func (h *Handler) ToggleUserActive(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ReadUUIDParam(r, "id")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	var input gen.AdminToggleUserActiveJSONBody

	err = httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	if _, ok := httpx.ContextGetAuthenticatedUser(r); !ok {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	isActive := input.IsActive != nil && *input.IsActive

	err = h.svc.ToggleUserActive(r.Context(), h.actor(r), id, isActive)
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "user updated"})
}
