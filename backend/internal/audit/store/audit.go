// Package store persists and reads the audit trail: who changed what, when,
// and from where.
//
// It is its own store rather than a corner of the admin one because the two
// have no caller in common any more. The trail is written by every module
// through internal/audit's recorder, and read by both the platform-wide admin
// view and a venue's own trail — while the rest of the admin store is read by
// nobody but the platform operator.
package store

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
)

// AuditLogRow represents an audit log entry for admin listing.
type AuditLogRow struct {
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

// CursorKey returns the (created-at, ID) pair used to build a pagination cursor for this row.
func (l *AuditLogRow) CursorKey() (time.Time, uuid.UUID) { return l.CreatedAt, l.ID }

// Store reads and writes the audit trail.
type Store struct {
	DB *data.DB
}

// listAuditLogsSQL is the audit-log page query.
//
// It is a constant rather than a literal inside the method so the plan assertion
// in admin_integration_test.go can EXPLAIN the exact statement this store issues.
// Its unscoped form — complex_id NULL, the default superadmin view — is served by
// idx_audit_log_created_at; before that index existed it seq-scanned
// the whole table and blew past the QueryContext budget at roughly 372,000 rows.
const listAuditLogsSQL = `
	SELECT a.id, a.user_id, u.email, a.complex_id, a.action, a.entity_type,
	       a.entity_id, a.ip_address::text, a.created_at
	FROM audit_log a
	LEFT JOIN users u ON u.id = a.user_id
	WHERE ($1::uuid IS NULL OR a.complex_id = $1)
	  AND ($2 = '' OR a.entity_type = $2)
	  AND (NOT $3 OR (a.created_at, a.id) < ($4, $5))
	ORDER BY a.created_at DESC, a.id DESC
	LIMIT $6
`

// ListAuditLogs returns a paginated, optionally complex- and entity-type-filtered list of audit log entries.
func (m *Store) ListAuditLogs(ctx context.Context, complexID *uuid.UUID, entityType string, filters data.Filters) ([]*AuditLogRow, data.Metadata, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	cursorTime, cursorID, err := filters.ParseCursor()
	if err != nil {
		return nil, data.Metadata{}, err
	}

	hasCursor := !cursorTime.IsZero()
	fetchLimit := filters.Limit + 1

	var cID *uuid.UUID
	if complexID != nil {
		cID = complexID
	}

	rows, err := m.DB.Query(ctx, listAuditLogsSQL, cID, entityType, hasCursor, cursorTime, cursorID, fetchLimit)
	if err != nil {
		return nil, data.Metadata{}, err
	}
	defer rows.Close()

	logs := make([]*AuditLogRow, 0, filters.Limit)
	for rows.Next() {
		var l AuditLogRow
		err := rows.Scan(
			&l.ID, &l.UserID, &l.UserEmail, &l.ComplexID, &l.Action, &l.EntityType,
			&l.EntityID, &l.IPAddress, &l.CreatedAt,
		)
		if err != nil {
			return nil, data.Metadata{}, err
		}
		logs = append(logs, &l)
	}
	if err := rows.Err(); err != nil {
		return nil, data.Metadata{}, err
	}

	logs, meta := data.TrimPage(logs, filters.Limit, data.BuildTimestampCursor)
	return logs, meta, nil
}

// InsertAuditLog records an admin or system action.
//
// oldJSON and newJSON are already-encoded JSON, or nil for "no value", which is
// stored as SQL NULL. This method used to take `any` and marshal here, but it
// runs on a background goroutine while the caller still owns the struct it
// handed over: encoding moved to the caller's goroutine (internal/audit.Record)
// so the snapshot is taken before anything can be scheduled against it.
func (m *Store) InsertAuditLog(ctx context.Context, userID, complexID *uuid.UUID, action, entityType string, entityID *uuid.UUID, oldJSON, newJSON []byte, ipAddr string) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var ip *netip.Addr
	if ipAddr != "" {
		if parsed, parseErr := netip.ParseAddr(ipAddr); parseErr == nil {
			ip = &parsed
		}
	}

	// The pool runs in pgx's QueryExecModeExec (cmd/api/main.go), which never
	// asks the server for parameter types and instead infers them from the Go
	// values: a []byte is sent as bytea, and Postgres refuses to read bytea as
	// json ("invalid input syntax for type json"). Every audit row used to fail
	// that way — silently but for one log line per action — so the trail was
	// empty. The payloads travel as text with an explicit cast, and "no value"
	// as an untyped nil, which is the one thing that reaches the driver as NULL.
	_, err := m.DB.Exec(ctx, `
		INSERT INTO audit_log (user_id, complex_id, action, entity_type, entity_id, old_value, new_value, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8)
	`, userID, complexID, action, entityType, entityID, jsonTextOrNull(oldJSON), jsonTextOrNull(newJSON), ip)
	return err
}

// jsonTextOrNull hands an encoded JSON payload to the driver as text (cast to
// jsonb in the statement), or as NULL when there is no value.
func jsonTextOrNull(encoded []byte) any {
	if len(encoded) == 0 {
		return nil
	}
	return string(encoded)
}
