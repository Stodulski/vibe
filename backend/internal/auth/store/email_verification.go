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

const emailVerificationTokenExpiry = 24 * time.Hour

// EmailVerificationToken represents a single-use token sent to confirm a user's email address.
type EmailVerificationToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	CreatedAt time.Time
}

// EmailVerifications implements EmailVerificationStore against PostgreSQL.
type EmailVerifications struct {
	DB *data.DB
	Q  *db.Queries
}

// Insert creates a new email verification token for the user, expiring after 24 hours.
func (m *EmailVerifications) Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.Q.InsertEmailVerificationToken(ctx, db.InsertEmailVerificationTokenParams{
		UserID:    data.UUIDToPg(userID),
		TokenHash: tokenHash,
		ExpiresAt: data.TimeToPg(time.Now().Add(emailVerificationTokenExpiry)),
	})
	return err
}

// GetByHash returns the email verification token matching tokenHash, or ErrRecordNotFound if none exists.
func (m *EmailVerifications) GetByHash(ctx context.Context, tokenHash []byte) (*EmailVerificationToken, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	t, err := m.Q.GetEmailVerificationToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return &EmailVerificationToken{
		ID:        data.PgToUUID(t.ID),
		UserID:    data.PgToUUID(t.UserID),
		TokenHash: t.TokenHash,
		ExpiresAt: data.PgToTime(t.ExpiresAt),
		CreatedAt: data.PgToTime(t.CreatedAt),
	}, nil
}

// GetLatestByUser returns the user's most recently issued email verification token, or ErrRecordNotFound if none exists.
func (m *EmailVerifications) GetLatestByUser(ctx context.Context, userID uuid.UUID) (*EmailVerificationToken, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	t, err := m.Q.GetLatestEmailVerificationTokenByUser(ctx, data.UUIDToPg(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return &EmailVerificationToken{
		ID:        data.PgToUUID(t.ID),
		UserID:    data.PgToUUID(t.UserID),
		TokenHash: t.TokenHash,
		ExpiresAt: data.PgToTime(t.ExpiresAt),
		CreatedAt: data.PgToTime(t.CreatedAt),
	}, nil
}

// InsertWithCooldown creates a new email verification token unless one was already issued
// within the resend cooldown window, returning ErrCooldownActive in that case.
func (m *EmailVerifications) InsertWithCooldown(ctx context.Context, userID uuid.UUID, tokenHash []byte) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.Q.InsertVerificationTokenWithCooldown(ctx, db.InsertVerificationTokenWithCooldownParams{
		UserID:    data.UUIDToPg(userID),
		TokenHash: tokenHash,
		ExpiresAt: data.TimeToPg(time.Now().Add(emailVerificationTokenExpiry)),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrCooldownActive
		}
		return err
	}
	return nil
}

// DeleteByUser removes every email verification token belonging to the user.
func (m *EmailVerifications) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return m.Q.DeleteEmailVerificationTokensByUser(ctx, data.UUIDToPg(userID))
}

// DeleteExpired removes every email verification token past its expiry, used by a periodic cleanup job.
func (m *EmailVerifications) DeleteExpired(ctx context.Context) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return m.Q.DeleteExpiredEmailVerificationTokens(ctx)
}
