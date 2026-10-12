// Package store persists registration leads: the people who left a registration
// form before finishing it.
package store

import (
	"context"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// Store is the sqlc-backed lead store. Leads are not tenant data.
type Store struct {
	Q *db.Queries
}

// Upsert keeps a lead. Its inputs are already trimmed, and email already lowercased.
// A repeat capture refreshes the names it has and keeps the ones it does not.
func (s *Store) Upsert(ctx context.Context, email, firstName, lastName, phone string) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return s.Q.UpsertRegistrationLead(ctx, db.UpsertRegistrationLeadParams{
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
		Phone:     phone,
	})
}

// DeleteOlderThan30Days removes every lead captured more than 30 days ago and
// reports how many it removed. It is the daily privacy cleanup.
func (s *Store) DeleteOlderThan30Days(ctx context.Context) (int64, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return s.Q.DeleteRegistrationLeadsOlderThan30Days(ctx)
}
