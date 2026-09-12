package data

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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

// EmailVerificationModel implements EmailVerificationStore against PostgreSQL.
type EmailVerificationModel struct {
	DB *DB
	Q  *db.Queries
}

// Insert creates a new email verification token for the user, expiring after 24 hours.
func (m *EmailVerificationModel) Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte) error {
	_, err := m.Q.InsertEmailVerificationToken(ctx, db.InsertEmailVerificationTokenParams{
		UserID:    UUIDToPg(userID),
		TokenHash: tokenHash,
		ExpiresAt: TimeToPg(time.Now().Add(emailVerificationTokenExpiry)),
	})
	return err
}

// GetByHash returns the email verification token matching tokenHash, or ErrRecordNotFound if none exists.
func (m *EmailVerificationModel) GetByHash(ctx context.Context, tokenHash []byte) (*EmailVerificationToken, error) {
	t, err := m.Q.GetEmailVerificationToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &EmailVerificationToken{
		ID:        PgToUUID(t.ID),
		UserID:    PgToUUID(t.UserID),
		TokenHash: t.TokenHash,
		ExpiresAt: PgToTime(t.ExpiresAt),
		CreatedAt: PgToTime(t.CreatedAt),
	}, nil
}

// GetLatestByUser returns the user's most recently issued email verification token, or ErrRecordNotFound if none exists.
func (m *EmailVerificationModel) GetLatestByUser(ctx context.Context, userID uuid.UUID) (*EmailVerificationToken, error) {
	t, err := m.Q.GetLatestEmailVerificationTokenByUser(ctx, UUIDToPg(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &EmailVerificationToken{
		ID:        PgToUUID(t.ID),
		UserID:    PgToUUID(t.UserID),
		TokenHash: t.TokenHash,
		ExpiresAt: PgToTime(t.ExpiresAt),
		CreatedAt: PgToTime(t.CreatedAt),
	}, nil
}

// InsertWithCooldown creates a new email verification token unless one was already issued
// within the resend cooldown window, returning ErrCooldownActive in that case.
func (m *EmailVerificationModel) InsertWithCooldown(ctx context.Context, userID uuid.UUID, tokenHash []byte) error {
	_, err := m.Q.InsertVerificationTokenWithCooldown(ctx, db.InsertVerificationTokenWithCooldownParams{
		UserID:    UUIDToPg(userID),
		TokenHash: tokenHash,
		ExpiresAt: TimeToPg(time.Now().Add(emailVerificationTokenExpiry)),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCooldownActive
		}
		return err
	}
	return nil
}

// DeleteByUser removes every email verification token belonging to the user.
func (m *EmailVerificationModel) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	return m.Q.DeleteEmailVerificationTokensByUser(ctx, UUIDToPg(userID))
}

// DeleteExpired removes every email verification token past its expiry, used by a periodic cleanup job.
func (m *EmailVerificationModel) DeleteExpired(ctx context.Context) error {
	return m.Q.DeleteExpiredEmailVerificationTokens(ctx)
}
