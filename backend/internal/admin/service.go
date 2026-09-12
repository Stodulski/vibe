package admin

import (
	"context"
	"errors"

	"github.com/google/uuid"

	adminstore "github.com/stodulski/vibe-server/internal/admin/store"
	"github.com/stodulski/vibe-server/internal/audit"
	auditstore "github.com/stodulski/vibe-server/internal/audit/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// ErrSelfToggle reports that an operator tried to change their own account's
// active flag. It is refused outright: an operator who deactivates themselves
// locks the platform's only administrative account out of the platform.
var ErrSelfToggle = errors.New("cannot modify your own account status")

// Actor is the platform operator behind a request, as the handler read it off.
// The service needs it for the audit trail and for nothing else.
type Actor struct {
	// UserID is the authenticated operator, or nil when the request somehow
	// carries no user — an audit entry with no actor is still worth writing.
	UserID *uuid.UUID
	// IP is the address the audit trail records.
	IP string
}

// Service holds this module's rules. There is one write here and a great many
// cross-tenant reads, and the rule that matters most is that every one of those
// reads is written down.
type Service struct {
	store     Store
	auditLogs AuditReader
	cache     UserCache
	audit     Recorder
}

// NewService returns a Service backed by the given stores.
func NewService(store Store, auditLogs AuditReader, cache UserCache, recorder Recorder) *Service {
	return &Service{store: store, auditLogs: auditLogs, cache: cache, audit: recorder}
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
func (s *Service) recordRead(actor Actor, entityType string, complexID, entityID *uuid.UUID, scope map[string]any) {
	// An empty scope is handed over as an untyped nil, not as a nil map: a nil
	// map inside an `any` is not nil, so it would encode to the four bytes
	// "null" and be stored as a JSON null where the column means "no value".
	var value any
	if len(scope) > 0 {
		value = scope
	}

	s.audit.Record(audit.Entry{
		UserID:     actor.UserID,
		ComplexID:  complexID,
		Action:     "read",
		EntityType: entityType,
		EntityID:   entityID,
		NewValue:   value,
		IPAddress:  actor.IP,
	})
}

// Stats returns the platform-wide totals and records the read.
func (s *Service) Stats(ctx context.Context, actor Actor) (*adminstore.PlatformStats, error) {
	stats, err := s.store.GetPlatformStats(ctx)
	if err != nil {
		return nil, err
	}

	s.recordRead(actor, "platform_stats", nil, nil, nil)
	return stats, nil
}

// ListUsers returns a page of the operator's user directory and records what
// was asked for, not what came back.
func (s *Service) ListUsers(ctx context.Context, actor Actor, search, role string, filters data.Filters) ([]*adminstore.AdminUserRow, data.Metadata, error) {
	users, metadata, err := s.store.ListUsers(ctx, search, role, filters)
	if err != nil {
		return nil, data.Metadata{}, err
	}

	scope := map[string]any{"returned": len(users)}
	if search != "" {
		scope["search"] = search
	}
	if role != "" {
		scope["role"] = role
	}
	s.recordRead(actor, "user", nil, nil, scope)

	return users, metadata, nil
}

// GetUser returns one account with the complexes and activity attached to it,
// and records that the operator looked at it.
func (s *Service) GetUser(ctx context.Context, actor Actor, userID uuid.UUID) (*adminstore.AdminUserDetail, error) {
	detail, err := s.store.GetUserDetail(ctx, userID)
	if err != nil {
		return nil, err
	}

	s.recordRead(actor, "user", nil, &userID, nil)
	return detail, nil
}

// ListComplexes returns a page of the platform's venue directory.
func (s *Service) ListComplexes(ctx context.Context, actor Actor, search string, filters data.Filters) ([]*adminstore.AdminComplexRow, data.Metadata, error) {
	complexes, metadata, err := s.store.ListComplexes(ctx, search, filters)
	if err != nil {
		return nil, data.Metadata{}, err
	}

	scope := map[string]any{"returned": len(complexes)}
	if search != "" {
		scope["search"] = search
	}
	s.recordRead(actor, "complex", nil, nil, scope)

	return complexes, metadata, nil
}

// GetComplex returns one venue with its owner and usage. The entry is scoped to
// that venue, so it lands in the venue's own trail as well as the platform's.
func (s *Service) GetComplex(ctx context.Context, actor Actor, complexID uuid.UUID) (*adminstore.AdminComplexDetail, error) {
	detail, err := s.store.GetComplexDetail(ctx, complexID)
	if err != nil {
		return nil, err
	}

	s.recordRead(actor, "complex", &complexID, &complexID, nil)
	return detail, nil
}

// ListAuditLogs returns a page of the platform-wide trail, optionally narrowed
// to one complex or entity type — and records that read in the same trail.
func (s *Service) ListAuditLogs(ctx context.Context, actor Actor, complexID *uuid.UUID, entityType string, filters data.Filters) ([]*auditstore.AuditLogRow, data.Metadata, error) {
	logs, metadata, err := s.auditLogs.ListAuditLogs(ctx, complexID, entityType, filters)
	if err != nil {
		return nil, data.Metadata{}, err
	}

	scope := map[string]any{"returned": len(logs)}
	if entityType != "" {
		scope["entity_type"] = entityType
	}
	s.recordRead(actor, "audit_log", complexID, nil, scope)

	return logs, metadata, nil
}

// ToggleUserActive enables or disables an account — the only write this module
// allows. An operator may not turn it on themselves.
func (s *Service) ToggleUserActive(ctx context.Context, actor Actor, userID uuid.UUID, isActive bool) error {
	if actor.UserID != nil && *actor.UserID == userID {
		return ErrSelfToggle
	}

	if err := s.store.ToggleUserActive(ctx, userID, isActive); err != nil {
		return err
	}

	s.audit.Record(audit.Entry{
		UserID:     actor.UserID,
		Action:     "toggle_active",
		EntityType: "user",
		EntityID:   &userID,
		NewValue:   map[string]any{"is_active": isActive},
		IPAddress:  actor.IP,
	})

	// The user's cached record carries is_active, so a stale copy would keep a
	// deactivated account working until the entry expired.
	s.cache.InvalidateUser(ctx, userID)

	return nil
}
