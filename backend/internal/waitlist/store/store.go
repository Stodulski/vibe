// Package store persists waitlist entries: the days a player asked to hear
// about when a slot on them frees up again.
package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// Store is the sqlc-backed waitlist store. Its rows are tenant-scoped, so every
// method runs on a context already scoped to the complex it names, or on the
// cross-tenant bypass for the cleanup sweep.
type Store struct {
	Q *db.Queries
}

// Claimed is one entry that was still waiting for a day when it was claimed.
type Claimed struct {
	ID    uuid.UUID
	Email string
}

// Insert records that email is waiting for day at complexID. A second insert of
// the same address for the same day is a no-op, so joining twice is harmless.
func (s *Store) Insert(ctx context.Context, complexID uuid.UUID, day time.Time, email string) error {
	if err := data.AssertTenant(ctx, complexID); err != nil {
		return err
	}
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return s.Q.InsertWaitlistEntry(ctx, db.InsertWaitlistEntryParams{
		ComplexID: data.UUIDToPg(complexID),
		Date:      data.DateToPg(day),
		Email:     email,
	})
}

// ClaimUnnotified marks every entry still waiting for day at complexID as
// notified and returns them. A row is claimed at most once, so two frees of
// the same day cannot both notify the same person.
func (s *Store) ClaimUnnotified(ctx context.Context, complexID uuid.UUID, day time.Time) ([]Claimed, error) {
	if err := data.AssertTenant(ctx, complexID); err != nil {
		return nil, err
	}
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := s.Q.ClaimWaitlistEntries(ctx, db.ClaimWaitlistEntriesParams{
		ComplexID: data.UUIDToPg(complexID),
		Date:      data.DateToPg(day),
	})
	if err != nil {
		return nil, err
	}

	claimed := make([]Claimed, 0, len(rows))
	for _, row := range rows {
		claimed = append(claimed, Claimed{ID: data.PgToUUID(row.ID), Email: row.Email})
	}
	return claimed, nil
}

// DeleteBefore removes every entry whose day is before today, across all
// complexes, and reports how many it removed. It is the daily cleanup and runs
// on the cross-tenant bypass.
func (s *Store) DeleteBefore(ctx context.Context, today time.Time) (int64, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return s.Q.DeleteWaitlistEntriesBefore(ctx, data.DateToPg(today))
}
