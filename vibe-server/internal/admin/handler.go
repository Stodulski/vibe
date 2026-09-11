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

	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// defaultPageLimit is the page size when the caller does not ask for one.
const defaultPageLimit = 50

// Store is the platform-wide data this module reads.
type Store interface {
	GetPlatformStats(ctx context.Context) (*data.PlatformStats, error)
	ListUsers(ctx context.Context, search, roleFilter string, filters data.Filters) ([]*data.AdminUserRow, data.Metadata, error)
	GetUserDetail(ctx context.Context, userID uuid.UUID) (*data.AdminUserDetail, error)
	ListComplexes(ctx context.Context, search string, filters data.Filters) ([]*data.AdminComplexRow, data.Metadata, error)
	GetComplexDetail(ctx context.Context, complexID uuid.UUID) (*data.AdminComplexDetail, error)
	ListAuditLogs(ctx context.Context, complexID *uuid.UUID, entityType string, filters data.Filters) ([]*data.AuditLogRow, data.Metadata, error)
	ToggleUserActive(ctx context.Context, userID uuid.UUID, isActive bool) error
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

// Handler serves the admin routes.
type Handler struct {
	store        Store
	cache        UserCache
	audit        Recorder
	respond      *httpx.Responder
	trustProxies bool
}

// NewHandler returns a Handler. trustProxies must match the deployment: it
// decides whether the audit trail records the forwarded client address or the
// immediate peer.
func NewHandler(store Store, cache UserCache, recorder Recorder, respond *httpx.Responder, trustProxies bool) *Handler {
	return &Handler{store: store, cache: cache, audit: recorder, respond: respond, trustProxies: trustProxies}
}

// Routes registers this module's endpoints. Every one exposes data across all
// tenants, so every one is behind the superadmin role as well as authentication.
func (h *Handler) Routes(router httpx.Router, guards httpx.Guards) {
	superAdmin := func(next http.HandlerFunc) http.HandlerFunc {
		return guards.RequireAuth(guards.RequireSuperAdmin(next))
	}

	router.HandlerFunc(http.MethodGet, "/api/v1/admin/stats", superAdmin(h.Stats))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/users", superAdmin(h.ListUsers))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/users/:id", superAdmin(h.GetUser))
	router.HandlerFunc(http.MethodPatch, "/api/v1/admin/users/:id/toggle-active", superAdmin(h.ToggleUserActive))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/complexes", superAdmin(h.ListComplexes))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/complexes/:id", superAdmin(h.GetComplex))
	router.HandlerFunc(http.MethodGet, "/api/v1/admin/audit-log", superAdmin(h.ListAuditLogs))
}

// recordRead writes the audit entry for one platform-operator read.
//
// Every read here crosses tenants, and until this existed none of them left a
// trace: the account that can see every venue's revenue, every owner's contact
// details and the whole audit trail was the only actor on the platform whose
// activity was invisible. A trail that records what operators change but not
// what they look at cannot answer the question an operator is most likely to
// be asked.
//
// complexID is set whenever the read is about one tenant, so the entry lands
// in that tenant's own trail as well — being able to see that the platform
// read your records is the part that makes this accountability rather than
// bookkeeping.
//
// The entry names what was read, not what came back: copying the rows into
// new_value would re-publish the very data the read exposed into a second
// table, and would grow the audit log by the size of every page anybody views.
func (h *Handler) recordRead(r *http.Request, entityType string, complexID, entityID *uuid.UUID, scope map[string]any) {
	// An empty scope is handed over as an untyped nil, not as a nil map: a nil
	// map inside an `any` is not nil, so it would encode to the four bytes
	// "null" and be stored as a JSON null where the column means "no value".
	var value any
	if len(scope) > 0 {
		value = scope
	}

	h.audit.Record(audit.Entry{
		UserID:     actingUserID(r),
		ComplexID:  complexID,
		Action:     "read",
		EntityType: entityType,
		EntityID:   entityID,
		NewValue:   value,
		IPAddress:  httpx.ClientIP(r, h.trustProxies),
	})
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
