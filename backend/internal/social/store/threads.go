package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/crypto"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/social"
)

// Threads keeps the one Threads connection. The access token is sealed with the
// credential keyring before it is written and opened when it is read.
type Threads struct {
	DB   *data.DB
	Keys *crypto.Keyring
}

// NewThreads returns a Threads store over db that seals tokens with keys.
func NewThreads(db *data.DB, keys *crypto.Keyring) *Threads {
	return &Threads{DB: db, Keys: keys}
}

// threadsTokenAAD binds a sealed Threads token to its column. A sealed value
// copied into another column, or another table, fails to open.
func threadsTokenAAD() []byte {
	const unitSeparator = "\x1f"
	return []byte("social.threads" + unitSeparator + "v1" + unitSeparator + "access_token")
}

// LoadThreads returns the connection, or ok false when none exists.
func (s *Threads) LoadThreads(ctx context.Context) (social.ThreadsConnection, bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var conn social.ThreadsConnection
	var sealed string
	err := s.DB.QueryRow(ctx, `
		SELECT user_id, username, access_token, connected_at, refreshed_at, expires_at, keyword_search_requested
		FROM social_threads_connection WHERE id = 1`).
		Scan(&conn.UserID, &conn.Username, &sealed, &conn.ConnectedAt, &conn.RefreshedAt, &conn.ExpiresAt, &conn.KeywordSearchRequested)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return social.ThreadsConnection{}, false, nil
	case err != nil:
		return social.ThreadsConnection{}, false, fmt.Errorf("social: read threads connection: %w", err)
	}
	token, err := s.Keys.Open(threadsTokenAAD(), sealed)
	if err != nil {
		return social.ThreadsConnection{}, false, fmt.Errorf("social: open threads token: %w", err)
	}
	conn.AccessToken = token
	return conn, true, nil
}

// SaveThreads replaces the connection. Its token is sealed first, so a missing
// keyring fails here rather than storing the token in plain text.
func (s *Threads) SaveThreads(ctx context.Context, conn social.ThreadsConnection) error {
	sealed, err := s.Keys.Seal(threadsTokenAAD(), conn.AccessToken)
	if err != nil {
		return fmt.Errorf("social: seal threads token: %w", err)
	}
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err = s.DB.Exec(ctx, `
		INSERT INTO social_threads_connection (id, user_id, username, access_token, connected_at, refreshed_at, expires_at, keyword_search_requested)
		VALUES (1, $1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			user_id                 = EXCLUDED.user_id,
			username                = EXCLUDED.username,
			access_token            = EXCLUDED.access_token,
			connected_at            = EXCLUDED.connected_at,
			refreshed_at            = EXCLUDED.refreshed_at,
			expires_at              = EXCLUDED.expires_at,
			keyword_search_requested = EXCLUDED.keyword_search_requested`,
		conn.UserID, conn.Username, sealed, conn.ConnectedAt, conn.RefreshedAt, conn.ExpiresAt, conn.KeywordSearchRequested)
	if err != nil {
		return fmt.Errorf("social: save threads connection: %w", err)
	}
	return nil
}

// DisconnectThreads removes the connection when it belongs to userID, and
// reports the username it removed. Another account's request removes nothing.
func (s *Threads) DisconnectThreads(ctx context.Context, userID string) (string, bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var username string
	err := s.DB.QueryRow(ctx, `
		DELETE FROM social_threads_connection WHERE id = 1 AND user_id = $1
		RETURNING username`, userID).Scan(&username)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("social: disconnect threads: %w", err)
	}
	return username, true, nil
}
