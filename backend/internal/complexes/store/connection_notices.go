package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// The reads the owner's connection notices need: who a notice about a complex
// goes to, and which Instagram connections ran out without being flagged.

// ComplexOwnerAndName returns the owner of a live complex and its name, for a
// notice addressed to that owner. data.ErrRecordNotFound when there is none.
func (m *Store) ComplexOwnerAndName(ctx context.Context, complexID uuid.UUID) (uuid.UUID, string, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetComplexOwnerAndName(ctx, data.UUIDToPg(complexID))
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, "", data.ErrRecordNotFound
	}
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("complexes: complex owner and name: %w", err)
	}
	return data.PgToUUID(row.OwnerID), row.Name, nil
}

// ExpiredInstagramConnection is a connection whose token ran out, with the
// expiry that identifies that token.
type ExpiredInstagramConnection struct {
	ComplexID uuid.UUID
	ExpiresAt time.Time
}

// ListUnflaggedExpiredInstagramConnections lists up to limit connections whose
// token expired by now and that are not flagged for a reconnect yet.
func (m *Store) ListUnflaggedExpiredInstagramConnections(ctx context.Context, now time.Time, limit int) ([]ExpiredInstagramConnection, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListUnflaggedExpiredInstagramConnections(ctx, db.ListUnflaggedExpiredInstagramConnectionsParams{
		Now:      data.TimeToPg(now),
		RowLimit: int32(min(limit, 10_000)), //nolint:gosec // G115: bounded above.
	})
	if err != nil {
		return nil, fmt.Errorf("complexes: list expired instagram connections: %w", err)
	}
	out := make([]ExpiredInstagramConnection, len(rows))
	for i, r := range rows {
		out[i] = ExpiredInstagramConnection{ComplexID: data.PgToUUID(r.ComplexID), ExpiresAt: data.PgToTime(r.ExpiresAt)}
	}
	return out, nil
}
