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

// RefreshToken represents an issued refresh token, stored by its hash.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	CreatedAt time.Time
	// UsedAt is when the token was rotated; zero for a token still in play.
	// Only GetUsedRefreshToken fills it, so the refresh handler can tell a
	// second browser tab refreshing a moment late from a replayed token.
	UsedAt time.Time
}

// Tokens implements TokenStore against PostgreSQL.
type Tokens struct {
	DB *data.DB
	Q  *db.Queries
}

// InsertRefreshToken stores a new refresh token hash for the user, expiring after ttl.
func (m *Tokens) InsertRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash []byte, ttl time.Duration) error {
	_, err := m.Q.InsertRefreshToken(ctx, db.InsertRefreshTokenParams{
		UserID:    data.UUIDToPg(userID),
		TokenHash: tokenHash,
		ExpiresAt: data.TimeToPg(time.Now().Add(ttl)),
	})
	return err
}

// GetRefreshToken returns the unused refresh token matching tokenHash, or ErrRecordNotFound if none exists.
func (m *Tokens) GetRefreshToken(ctx context.Context, tokenHash []byte) (*RefreshToken, error) {
	dbToken, err := m.Q.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return &RefreshToken{
		ID:        data.PgToUUID(dbToken.ID),
		UserID:    data.PgToUUID(dbToken.UserID),
		TokenHash: dbToken.TokenHash,
		ExpiresAt: data.PgToTime(dbToken.ExpiresAt),
		CreatedAt: data.PgToTime(dbToken.CreatedAt),
	}, nil
}

// MarkRefreshTokenUsed flags the refresh token as consumed, preventing replay.
func (m *Tokens) MarkRefreshTokenUsed(ctx context.Context, tokenHash []byte) error {
	return m.Q.MarkRefreshTokenUsed(ctx, tokenHash)
}

// GetUsedRefreshToken returns an already-consumed refresh token matching tokenHash, used to
// detect token reuse (a signal of a stolen refresh token), or ErrRecordNotFound if none exists.
func (m *Tokens) GetUsedRefreshToken(ctx context.Context, tokenHash []byte) (*RefreshToken, error) {
	dbToken, err := m.Q.GetUsedRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return &RefreshToken{
		ID:        data.PgToUUID(dbToken.ID),
		UserID:    data.PgToUUID(dbToken.UserID),
		TokenHash: dbToken.TokenHash,
		ExpiresAt: data.PgToTime(dbToken.ExpiresAt),
		CreatedAt: data.PgToTime(dbToken.CreatedAt),
		UsedAt:    data.PgToTime(dbToken.UsedAt),
	}, nil
}

// DeleteRefreshToken permanently removes the refresh token matching tokenHash.
func (m *Tokens) DeleteRefreshToken(ctx context.Context, tokenHash []byte) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx, "DELETE FROM refresh_tokens WHERE token_hash = $1", tokenHash)
	return err
}

// DeleteAllForUser revokes every refresh token belonging to the user, used on password change or logout-everywhere.
func (m *Tokens) DeleteAllForUser(ctx context.Context, userID uuid.UUID) error {
	return m.Q.DeleteAllRefreshTokensByUser(ctx, data.UUIDToPg(userID))
}

// DeleteExpired removes every refresh token past its expiry, used by a periodic cleanup job.
func (m *Tokens) DeleteExpired(ctx context.Context) error {
	return m.Q.DeleteExpiredRefreshTokens(ctx)
}
