package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// MPRefreshFailureNotified reports whether the owner of complexID was already
// emailed about this refresh-failure episode. See migration 043.
func (m *Store) MPRefreshFailureNotified(ctx context.Context, complexID uuid.UUID, episode string) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	told, err := m.Q.GetMPRefreshFailureNotice(ctx, data.UUIDToPg(complexID))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("complexes: read mp refresh notice: %w", err)
	}
	return told == episode, nil
}

// RecordMPRefreshFailureNotice records that the owner of complexID was emailed
// about this refresh-failure episode, replacing the previous one.
func (m *Store) RecordMPRefreshFailureNotice(ctx context.Context, complexID uuid.UUID, episode string) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if err := m.Q.UpsertMPRefreshFailureNotice(ctx, db.UpsertMPRefreshFailureNoticeParams{
		ComplexID: data.UUIDToPg(complexID),
		Episode:   episode,
	}); err != nil {
		return fmt.Errorf("complexes: record mp refresh notice: %w", err)
	}
	return nil
}
