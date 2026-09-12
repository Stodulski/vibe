package data

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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

// UserIdentityModel implements UserIdentityStore against PostgreSQL.
type UserIdentityModel struct {
	DB *DB
	Q  *db.Queries
}

// Insert records the link between identity.UserID and an external identity.
// It is idempotent: linking the same (provider, subject) pair or the same
// (user_id, provider) pair again is a silent no-op
// (db/migrations/002_user_identities.sql), so a repeated Google sign-in never
// fails on the identity link.
func (m *UserIdentityModel) Insert(ctx context.Context, identity *UserIdentity) error {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	return m.Q.InsertUserIdentity(ctx, db.InsertUserIdentityParams{
		UserID:   UUIDToPg(identity.UserID),
		Provider: identity.Provider,
		Subject:  identity.Subject,
		Email:    TextToPg(identity.Email),
	})
}

// GetByProviderSubject returns the identity link for a (provider, subject)
// pair, or ErrRecordNotFound if none exists.
func (m *UserIdentityModel) GetByProviderSubject(ctx context.Context, provider, subject string) (*UserIdentity, error) {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetUserIdentityByProviderSubject(ctx, db.GetUserIdentityByProviderSubjectParams{
		Provider: provider,
		Subject:  subject,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return userIdentityFromDB(row), nil
}

// GetByUser returns every identity linked to userID, oldest first.
func (m *UserIdentityModel) GetByUser(ctx context.Context, userID uuid.UUID) ([]*UserIdentity, error) {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.GetUserIdentitiesByUser(ctx, UUIDToPg(userID))
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
		ID:        PgToUUID(u.ID),
		UserID:    PgToUUID(u.UserID),
		Provider:  u.Provider,
		Subject:   u.Subject,
		Email:     PgToTextPtr(u.Email),
		CreatedAt: PgToTime(u.CreatedAt),
	}
}
