// Package admin serves the platform-operator views: cross-tenant statistics,
// the user and complex directories, the audit trail, and the one write it
// allows — enabling or disabling a user account.
//
// Everything here reads across every complex, so every route requires the
// superadmin role.
package admin

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	adminstore "github.com/stodulski/vibe-server/internal/admin/store"
	"github.com/stodulski/vibe-server/internal/audit"
	auditstore "github.com/stodulski/vibe-server/internal/audit/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// defaultPageLimit is the page size when the caller does not ask for one.
const defaultPageLimit = 50

// Store is the platform-wide data this module reads.
type Store interface {
	GetPlatformStats(ctx context.Context) (*adminstore.PlatformStats, error)
	ListUsers(ctx context.Context, search, roleFilter string, filters data.Filters) ([]*adminstore.AdminUserRow, data.Metadata, error)
	GetUserDetail(ctx context.Context, userID uuid.UUID) (*adminstore.AdminUserDetail, error)
	ListComplexes(ctx context.Context, search string, filters data.Filters) ([]*adminstore.AdminComplexRow, data.Metadata, error)
	GetComplexDetail(ctx context.Context, complexID uuid.UUID) (*adminstore.AdminComplexDetail, error)
	ToggleUserActive(ctx context.Context, userID uuid.UUID, isActive bool) error
}

// AuditReader is the platform-wide read of the audit trail this module's
// /admin/audit-log route serves.
//
// It is separate from Store because the trail is its own store: it is written
// by every module and read by a venue's own trail route as well as this one,
// while the rest of Store is read by nobody but the platform operator.
type AuditReader interface {
	ListAuditLogs(ctx context.Context, complexID *uuid.UUID, entityType string, filters data.Filters) ([]*auditstore.AuditLogRow, data.Metadata, error)
}

// UserCache is the cached-session invalidation this module needs after
// changing a user's active flag.
type UserCache interface {
	InvalidateUser(ctx context.Context, id uuid.UUID)
}

// Recorder writes the audit trail.
type Recorder interface {
	Record(e audit.Entry)
}

// Handler serves the admin routes. It decodes, validates, and maps the
// service's domain errors onto HTTP; every rule lives in the Service.
type Handler struct {
	svc          *Service
	respond      *httpx.Refuser
	trustProxies bool
}

// refusals is this module's whole error-to-status table: every domain error of
// its own that is a refusal rather than a fault, and the status and message it
// earns. Everything absent from it — including the sentinels every module
// shares — is answered by internal/httpx.
var refusals = httpx.Refusals{
	ErrSelfToggle: httpx.Conflict("cannot modify your own account status"),
}

// NewHandler returns a Handler. trustProxies must match the deployment: it
// decides whether the audit trail records the forwarded client address or the
// immediate peer.
func NewHandler(svc *Service, respond *httpx.Responder, trustProxies bool) *Handler {
	return &Handler{svc: svc, respond: respond.WithRefusals(refusals), trustProxies: trustProxies}
}

// Routes registers this module's endpoints. Every one exposes data across all
// tenants, so every one is behind the superadmin role as well as authentication.
func (h *Handler) Routes(router httpx.Router, guards httpx.Guards) {
	superAdmin := func(next http.HandlerFunc) http.HandlerFunc {
		return guards.RequireAuth(guards.RequireSuperAdmin(next))
	}

	router.HandlerFunc(http.MethodGet, "/api/v1/admin/stats", superAdmin(h.Stats))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/users", superAdmin(h.ListUsers))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/users/{id}", superAdmin(h.GetUser))
	router.HandlerFunc(http.MethodPatch, "/api/v1/admin/users/{id}/toggle-active", superAdmin(h.ToggleUserActive))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/complexes", superAdmin(h.ListComplexes))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/complexes/{id}", superAdmin(h.GetComplex))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/audit-log", superAdmin(h.ListAuditLogs))
}

// actor reads the operator behind a request, and the address it came from, off
// the request. It is the only thing the audit trail needs that lives on the
// HTTP side.
func (h *Handler) actor(r *http.Request) Actor {
	return Actor{UserID: actingUserID(r), IP: httpx.ClientIP(r, h.trustProxies)}
}

// actingUserID returns the authenticated operator's id, or nil when the
// request somehow carries no user — an audit entry with no actor is still
// worth writing.
func actingUserID(r *http.Request) *uuid.UUID {
	user, ok := httpx.ContextGetAuthenticatedUser(r)
	if !ok {
		return nil
	}
	return &user.ID
}
