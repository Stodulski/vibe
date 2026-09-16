package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/data"
)

// linkTokenByteLength is how much entropy MintLinkToken draws for a plaintext
// token: 32 random bytes, base64.RawURLEncoding-encoded to 43 characters —
// the same shape generateRefreshToken/hashToken (internal/auth/tokens.go)
// use for a refresh token, duplicated here rather than imported —
// internal/data must not depend on internal/auth. Chosen over the house
// uuid.New() idiom deliberately: a UUID-shaped token would be
// indistinguishable from a booking id in any URL or log, which is the
// confusion this change exists to end (design.md Decision 1).
const linkTokenByteLength = 32

// HashLinkToken hashes a plaintext booking link token the same way
// hashToken (internal/auth/tokens.go) hashes every single-use token —
// a refresh token, an email verification, a password reset, an email
// change: SHA-256, stored raw.
func HashLinkToken(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}

// MintLinkToken generates a fresh plaintext token and inserts its hash within
// tx, so a caller that already holds a transaction (Store.InsertSafe)
// can mint atomically with the booking it belongs to — a crash between the
// two commits would otherwise strand a booking with no usable link on any of
// its public routes.
func MintLinkToken(ctx context.Context, tx pgx.Tx, bookingID uuid.UUID, expiresAt time.Time) (string, error) {
	buf := make([]byte, linkTokenByteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate booking link token: %w", err)
	}
	plaintext := base64.RawURLEncoding.EncodeToString(buf)

	_, err := tx.Exec(ctx, `
		INSERT INTO booking_link_tokens (booking_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`,
		data.UUIDToPg(bookingID), HashLinkToken(plaintext), data.TimeToPg(expiresAt),
	)
	if err != nil {
		return "", fmt.Errorf("insert booking link token: %w", err)
	}
	return plaintext, nil
}
