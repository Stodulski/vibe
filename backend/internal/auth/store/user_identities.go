package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// UserIdentity links a local account to an external identity provider's
// account — currently only Google (Sign in with Google).
type UserIdentity struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Provider  string
	Subject   string
	Email     *string
	CreatedAt time.Time
}

// Identities implements UserIdentityStore against PostgreSQL.
type Identities struct {
	DB *data.DB
	Q  *db.Queries
}

// Insert records the link between identity.UserID and an external identity.
// It is idempotent: linking the same (provider, subject) pair or the same
// (user_id, provider) pair again is a silent no-op
// (db/migrations/001_init.sql), so a repeated Google sign-in never
// fails on the identity link.
func (m *Identities) Insert(ctx context.Context, identity *UserIdentity) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return m.Q.InsertUserIdentity(ctx, db.InsertUserIdentityParams{
		UserID:   data.UUIDToPg(identity.UserID),
		Provider: identity.Provider,
		Subject:  identity.Subject,
		Email:    data.TextToPg(identity.Email),
	})
}

// GetByProviderSubject returns the identity link for a (provider, subject)
// pair, or ErrRecordNotFound if none exists.
func (m *Identities) GetByProviderSubject(ctx context.Context, provider, subject string) (*UserIdentity, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetUserIdentityByProviderSubject(ctx, db.GetUserIdentityByProviderSubjectParams{
		Provider: provider,
		Subject:  subject,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return userIdentityFromDB(row), nil
}

// GetByUser returns every identity linked to userID, oldest first.
func (m *Identities) GetByUser(ctx context.Context, userID uuid.UUID) ([]*UserIdentity, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.GetUserIdentitiesByUser(ctx, data.UUIDToPg(userID))
	if err != nil {
		return nil, err
	}
	out := make([]*UserIdentity, len(rows))
	for i, row := range rows {
		out[i] = userIdentityFromDB(row)
	}
	return out, nil
}

func userIdentityFromDB(u db.UserIdentity) *UserIdentity {
	return &UserIdentity{
		ID:        data.PgToUUID(u.ID),
		UserID:    data.PgToUUID(u.UserID),
		Provider:  u.Provider,
		Subject:   u.Subject,
		Email:     data.PgToTextPtr(u.Email),
		CreatedAt: data.PgToTime(u.CreatedAt),
	}
}
