package audit

import (
	"context"

	"github.com/google/uuid"

	auditstore "github.com/stodulski/vibe-server/internal/audit/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// Actor is who is reading the trail, and from where, as the handler read it off
// the request.
type Actor struct {
	// UserID is the authenticated owner, or nil for a system action.
	UserID *uuid.UUID
	// IP is the address the entry records as the caller's.
	IP string
}

// Service serves a tenant its own trail.
//
// The trail existed only behind /api/v1/admin/audit-log, which is superadmin
// only — so the record of who changed a venue's bookings, courts and prices was
// readable by the platform and not by the venue. That is backwards for the one
// table whose purpose is accountability: the owner is the person who needs to
// see that a staff account cancelled a booking, and the platform is the party
// the trail should also be able to hold to account.
//
// The write side is Recorder, which every other module uses and which this
// service reuses: a read of the trail is itself an event on the trail.
type Service struct {
	reader   Reader
	recorder *Recorder
}

// NewService returns a Service. recorder is the same one the writing modules
// use.
func NewService(reader Reader, recorder *Recorder) *Service {
	return &Service{reader: reader, recorder: recorder}
}

// List returns a page of one complex's trail and records the read.
//
// The scope is the complex the caller owns and never a caller-supplied id: a
// complex_id parameter here would be a caller naming the tenant whose history
// they want to read, which is the one thing this must not accept.
//
// Recording happens after the query, so a failed read is not reported as a
// completed one. The entry names what was read — the scope and how much of it
// came back — rather than the rows themselves: copying the page into new_value
// would double the table on every read and, worse, would mean the trail's own
// contents get rewritten into it under a different actor.
func (s *Service) List(ctx context.Context, actor Actor, complexID uuid.UUID, entityType string, filters data.Filters) ([]*auditstore.AuditLogRow, data.Metadata, error) {
	logs, metadata, err := s.reader.ListAuditLogs(ctx, &complexID, entityType, filters)
	if err != nil {
		return nil, data.Metadata{}, err
	}

	scope := map[string]any{"returned": len(logs)}
	if entityType != "" {
		scope["entity_type"] = entityType
	}

	//nolint:contextcheck // Record deliberately detaches: the entry describes
	// something that already happened, so it must still be written when the
	// client hangs up mid-response. See Recorder.Record.
	s.recorder.Record(Entry{
		UserID:     actor.UserID,
		ComplexID:  &complexID,
		Action:     "read",
		EntityType: "audit_log",
		NewValue:   scope,
		IPAddress:  actor.IP,
	})

	return logs, metadata, nil
}

// ListAuditLogs returns a page of the trail with the store's exact signature.
//
// Exported for the admin module, whose platform-wide trail route reads across
// every tenant: its entry point into the trail is this service rather than the
// audit store.
func (s *Service) ListAuditLogs(ctx context.Context, complexID *uuid.UUID, entityType string, filters data.Filters) ([]*auditstore.AuditLogRow, data.Metadata, error) {
	return s.reader.ListAuditLogs(ctx, complexID, entityType, filters)
}
