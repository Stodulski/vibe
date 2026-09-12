package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stodulski/vibe-server/internal/data"
)

const passwordResetTokenExpiry = 1 * time.Hour

// PasswordResetToken represents a single-use token sent to let a user reset their password.
type PasswordResetToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	CreatedAt time.Time
}

// PasswordResets implements PasswordResetStore against PostgreSQL.
type PasswordResets struct {
	DB *data.DB
}

// InsertWithCooldown creates a new password reset token, atomically clearing tokens older
// than 3 minutes, unless a fresh token was already issued, returning ErrCooldownActive then.
func (m *PasswordResets) InsertWithCooldown(ctx context.Context, userID uuid.UUID, tokenHash []byte) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	expiresAt := time.Now().Add(passwordResetTokenExpiry)

	// Atomic: delete stale tokens (>3 min), insert new only if cooldown passed.
	query := `
		WITH deleted AS (
			DELETE FROM password_reset_tokens
			WHERE user_id = $1
			AND created_at <= NOW() - INTERVAL '3 minutes'
		)
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		SELECT $1, $2, $3
		WHERE NOT EXISTS (
			SELECT 1 FROM password_reset_tokens
			WHERE user_id = $1
			AND created_at > NOW() - INTERVAL '3 minutes'
		)
		RETURNING id`

	var id uuid.UUID
	err := m.DB.QueryRow(ctx, query, userID, tokenHash, expiresAt).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrCooldownActive
		}
		return err
	}
	return nil
}

// GetByHash atomically fetches and deletes the password reset token in one
// query, preventing race conditions and ensuring single-use. Returns
// ErrRecordNotFound if no matching, unexpired token exists.
func (m *PasswordResets) GetByHash(ctx context.Context, tokenHash []byte) (*PasswordResetToken, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	query := `
		DELETE FROM password_reset_tokens
		WHERE token_hash = $1
		AND expires_at > NOW()
		RETURNING id, user_id, token_hash, expires_at, created_at`

	var t PasswordResetToken
	err := m.DB.QueryRow(ctx, query, tokenHash).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return &t, nil
}

// DeleteByUser removes every password reset token belonging to the user.
func (m *PasswordResets) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx, "DELETE FROM password_reset_tokens WHERE user_id = $1", userID)
	return err
}

// DeleteExpired removes every password reset token past its expiry, used by a periodic cleanup job.
func (m *PasswordResets) DeleteExpired(ctx context.Context) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx, "DELETE FROM password_reset_tokens WHERE expires_at <= NOW()")
	return err
}
