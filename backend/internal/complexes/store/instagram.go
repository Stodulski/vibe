package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/crypto"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// instagramTokenColumn is the column a sealed Instagram token lives in; it is
// part of the AAD.
const instagramTokenColumn = "access_token"

// InstagramConnection is a complex's connected Instagram professional account.
// AccessToken is the opened (plain) long-lived token, set only by the reads
// that need it; the status read leaves it empty.
type InstagramConnection struct {
	ComplexID       uuid.UUID
	InstagramUserID string
	Username        string
	AccountType     string
	AccessToken     string
	ExpiresAt       time.Time
	ConnectedAt     time.Time
	RefreshedAt     time.Time
	NeedsReconnect  bool
}

// DueInstagramToken is one connection the refresh sweep has to renew.
type DueInstagramToken struct {
	ComplexID   uuid.UUID
	AccessToken string
	ExpiresAt   time.Time
}

// InstagramStore keeps the Instagram connections. The token is sealed with the
// credential keyring before it is written and opened when it is read, so a nil
// keyring refuses both rather than storing or returning plain text.
type InstagramStore struct {
	Q    *db.Queries
	Keys *crypto.Keyring
}

// instagramTokenAAD binds a sealed token to its complex and column: a sealed
// value copied to another complex's row fails to open.
func instagramTokenAAD(complexID uuid.UUID) []byte {
	const unitSeparator = "\x1f"
	return []byte("igcred" + unitSeparator + "v1" + unitSeparator + complexID.String() + unitSeparator + instagramTokenColumn)
}

// SaveInstagramConnection creates or replaces the complex's connection,
// sealing its token first. It clears needs_reconnect.
func (s *InstagramStore) SaveInstagramConnection(ctx context.Context, c *InstagramConnection) error {
	if c.AccessToken == "" {
		return errors.New("complexes: instagram token must not be empty")
	}
	sealed, err := s.Keys.Seal(instagramTokenAAD(c.ComplexID), c.AccessToken)
	if err != nil {
		return fmt.Errorf("complexes: seal instagram token: %w", err)
	}
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	err = s.Q.UpsertInstagramConnection(ctx, db.UpsertInstagramConnectionParams{
		ComplexID:       data.UUIDToPg(c.ComplexID),
		InstagramUserID: c.InstagramUserID,
		Username:        c.Username,
		AccountType:     c.AccountType,
		AccessToken:     sealed,
		ExpiresAt:       data.TimeToPg(c.ExpiresAt),
		ConnectedAt:     data.TimeToPg(c.ConnectedAt),
	})
	if err != nil {
		return fmt.Errorf("complexes: save instagram connection: %w", err)
	}
	return nil
}

// GetInstagramConnection returns the complex's connection without its token,
// or data.ErrRecordNotFound when it has none.
func (s *InstagramStore) GetInstagramConnection(ctx context.Context, complexID uuid.UUID) (*InstagramConnection, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	row, err := s.Q.GetInstagramConnection(ctx, data.UUIDToPg(complexID))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, data.ErrRecordNotFound
	case err != nil:
		return nil, fmt.Errorf("complexes: read instagram connection: %w", err)
	}
	return &InstagramConnection{
		ComplexID:       data.PgToUUID(row.ComplexID),
		InstagramUserID: row.InstagramUserID,
		Username:        row.Username,
		AccountType:     row.AccountType,
		ExpiresAt:       data.PgToTime(row.ExpiresAt),
		ConnectedAt:     data.PgToTime(row.ConnectedAt),
		RefreshedAt:     data.PgToTime(row.RefreshedAt),
		NeedsReconnect:  row.NeedsReconnect,
	}, nil
}

// DeleteInstagramConnection removes the complex's connection and reports
// whether there was one. The same statement turns both story switches off.
func (s *InstagramStore) DeleteInstagramConnection(ctx context.Context, complexID uuid.UUID) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	n, err := s.Q.DeleteInstagramConnection(ctx, data.UUIDToPg(complexID))
	if err != nil {
		return false, fmt.Errorf("complexes: delete instagram connection: %w", err)
	}
	return n > 0, nil
}

// ListInstagramTokensDueForRefresh returns, opened, the tokens of live
// connections that expire before refreshBefore and were last refreshed before
// refreshedBefore. A token that cannot be opened is reported in skipped and
// left out, so one bad row does not stop the sweep.
func (s *InstagramStore) ListInstagramTokensDueForRefresh(ctx context.Context, now, refreshBefore, refreshedBefore time.Time, limit int) (due []DueInstagramToken, skipped []uuid.UUID, err error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	rows, err := s.Q.ListInstagramConnectionsDueForRefresh(ctx, db.ListInstagramConnectionsDueForRefreshParams{
		Now:             data.TimeToPg(now),
		RefreshBefore:   data.TimeToPg(refreshBefore),
		RefreshedBefore: data.TimeToPg(refreshedBefore),
		RowLimit:        int32(min(limit, 10_000)), //nolint:gosec // G115: bounded above.
	})
	if err != nil {
		return nil, nil, fmt.Errorf("complexes: list instagram tokens due: %w", err)
	}
	for _, row := range rows {
		complexID := data.PgToUUID(row.ComplexID)
		token, openErr := s.Keys.Open(instagramTokenAAD(complexID), row.AccessToken)
		if openErr != nil {
			skipped = append(skipped, complexID)
			continue
		}
		due = append(due, DueInstagramToken{ComplexID: complexID, AccessToken: token, ExpiresAt: data.PgToTime(row.ExpiresAt)})
	}
	return due, skipped, nil
}

// UpdateInstagramToken stores a refreshed token for the connection whose
// expiry is still oldExpiresAt. It reports false when the row changed in the
// meantime (reconnected, disconnected or marked), and then writes nothing.
func (s *InstagramStore) UpdateInstagramToken(ctx context.Context, complexID uuid.UUID, oldExpiresAt time.Time, token string, expiresAt, refreshedAt time.Time) (bool, error) {
	if token == "" {
		return false, errors.New("complexes: instagram token must not be empty")
	}
	sealed, err := s.Keys.Seal(instagramTokenAAD(complexID), token)
	if err != nil {
		return false, fmt.Errorf("complexes: seal instagram token: %w", err)
	}
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	n, err := s.Q.UpdateInstagramToken(ctx, db.UpdateInstagramTokenParams{
		AccessToken:  sealed,
		ExpiresAt:    data.TimeToPg(expiresAt),
		RefreshedAt:  data.TimeToPg(refreshedAt),
		ComplexID:    data.UUIDToPg(complexID),
		OldExpiresAt: data.TimeToPg(oldExpiresAt),
	})
	if err != nil {
		return false, fmt.Errorf("complexes: update instagram token: %w", err)
	}
	return n > 0, nil
}

// MarkExpiredInstagramConnections flags every connection whose token has
// expired as needing a reconnect, and returns how many it flagged.
func (s *InstagramStore) MarkExpiredInstagramConnections(ctx context.Context, now time.Time) (int64, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	n, err := s.Q.MarkExpiredInstagramConnections(ctx, data.TimeToPg(now))
	if err != nil {
		return 0, fmt.Errorf("complexes: mark expired instagram connections: %w", err)
	}
	return n, nil
}

// MarkInstagramNeedsReconnect flags the connection whose refresh Meta refused,
// unless a reconnect replaced its token since (oldExpiresAt no longer matches).
func (s *InstagramStore) MarkInstagramNeedsReconnect(ctx context.Context, complexID uuid.UUID, oldExpiresAt time.Time) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	n, err := s.Q.MarkInstagramNeedsReconnect(ctx, db.MarkInstagramNeedsReconnectParams{
		ComplexID:    data.UUIDToPg(complexID),
		OldExpiresAt: data.TimeToPg(oldExpiresAt),
	})
	if err != nil {
		return false, fmt.Errorf("complexes: mark instagram reconnect: %w", err)
	}
	return n > 0, nil
}

// InstagramComplexOwner returns the owner of a live complex.
func (s *InstagramStore) InstagramComplexOwner(ctx context.Context, complexID uuid.UUID) (uuid.UUID, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	owner, err := s.Q.GetInstagramComplexOwner(ctx, data.UUIDToPg(complexID))
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, data.ErrRecordNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("complexes: instagram complex owner: %w", err)
	}
	return data.PgToUUID(owner), nil
}
