package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stodulski/vibe-server/internal/data"
)

// EmailChangeTokenExpiry follows the password-reset TTL convention (see
// passwordResetTokenExpiry): long enough to open an inbox, short enough that
// a leaked link is not a standing way to move an account's recovery channel.
//
// Exported so the mailer can render the displayed expiry from this single
// constant (see notifications.EmailChangeRequestedEmail.ExpiresIn) instead of
// a second, hand-copied "1 hora" that can drift from it.
const EmailChangeTokenExpiry = 1 * time.Hour

// EmailChangeRequest is the account's pending request to move users.email to
// a new address. Its existence is not itself an email change: only consuming
// its token (auth.Service.ConfirmEmailChange) writes users.email, because a
// live session alone is not proof of control over the new address — see
// db/migrations/001_init.sql for the full reasoning.
type EmailChangeRequest struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	NewEmail  string
	TokenHash []byte
	ExpiresAt time.Time
	CreatedAt time.Time
}

// EmailChanges implements auth.EmailChangeStore against PostgreSQL.
type EmailChanges struct {
	DB *data.DB
}

// Put creates the account's pending email-change request, replacing any
// previous one: a second PUT /auth/me asking for a different address must
// invalidate the first request's link rather than leave two live tokens
// racing to decide the account's next address. The UNIQUE constraint on
// user_id (db/migrations/001_init.sql) is what ON CONFLICT upserts against.
func (m *EmailChanges) Put(ctx context.Context, userID uuid.UUID, newEmail string, tokenHash []byte) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	expiresAt := time.Now().Add(EmailChangeTokenExpiry)

	_, err := m.DB.Exec(ctx, `
		INSERT INTO email_change_requests (user_id, new_email, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE
		SET new_email  = EXCLUDED.new_email,
		    token_hash = EXCLUDED.token_hash,
		    expires_at = EXCLUDED.expires_at,
		    created_at = NOW()`,
		userID, newEmail, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("auth: put email change request: %w", err)
	}
	return nil
}

// Peek reads the pending request the token names without consuming it.
//
// Unlike PasswordResets.GetByHash, this does not delete on read: see
// auth.Service.ConfirmEmailChange for why the token is validated and applied
// before it is spent, rather than the other way around. An unknown or expired
// token answers data.ErrRecordNotFound — one outcome for both, like every
// other token this module checks.
func (m *EmailChanges) Peek(ctx context.Context, tokenHash []byte) (*EmailChangeRequest, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	query := `
		SELECT id, user_id, new_email, token_hash, expires_at, created_at
		FROM email_change_requests
		WHERE token_hash = $1
		AND expires_at > NOW()`

	var t EmailChangeRequest
	err := m.DB.QueryRow(ctx, query, tokenHash).
		Scan(&t.ID, &t.UserID, &t.NewEmail, &t.TokenHash, &t.ExpiresAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, fmt.Errorf("auth: peek email change request: %w", err)
	}
	return &t, nil
}

// Consume deletes the pending request by its token hash, once the caller has
// finished validating it and has already applied the change it authorizes.
// data.ErrRecordNotFound means the row is already gone: either this exact
// request was consumed concurrently (a second click of the same link), or a
// newer request replaced it first (Put's ON CONFLICT (user_id) DO UPDATE) —
// in which case the older address this call just applied to users.email is
// already in place, and the newer request stays pending under its own token.
// Both are a known, accepted edge case: see ConfirmEmailChange, which treats
// this as a benign race rather than a failure, since the change it guarded
// has already committed by the time this runs.
func (m *EmailChanges) Consume(ctx context.Context, tokenHash []byte) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	tag, err := m.DB.Exec(ctx, "DELETE FROM email_change_requests WHERE token_hash = $1", tokenHash)
	if err != nil {
		return fmt.Errorf("auth: consume email change request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return data.ErrRecordNotFound
	}
	return nil
}

// GetPendingByUser returns the account's live pending request, or
// data.ErrRecordNotFound when it has none — what GET and PUT /auth/me read
// to answer pending_email.
func (m *EmailChanges) GetPendingByUser(ctx context.Context, userID uuid.UUID) (*EmailChangeRequest, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	query := `
		SELECT id, user_id, new_email, token_hash, expires_at, created_at
		FROM email_change_requests
		WHERE user_id = $1 AND expires_at > NOW()`

	var t EmailChangeRequest
	err := m.DB.QueryRow(ctx, query, userID).
		Scan(&t.ID, &t.UserID, &t.NewEmail, &t.TokenHash, &t.ExpiresAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, fmt.Errorf("auth: get pending email change request: %w", err)
	}
	return &t, nil
}

// DeleteExpired removes every pending request past its expiry, used by a
// periodic cleanup job — the same posture as PasswordResets.DeleteExpired.
func (m *EmailChanges) DeleteExpired(ctx context.Context) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx, "DELETE FROM email_change_requests WHERE expires_at <= NOW()")
	return err
}
