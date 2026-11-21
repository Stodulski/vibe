package complexes

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/notifications"
)

// InstagramNotifier queues the owner's email that the complex's Instagram
// account needs reconnecting. notifications.Service is the production
// implementation. A nil InstagramNotifier sends nothing.
type InstagramNotifier interface {
	InstagramNeedsReconnect(n notifications.ConnectionNotice, episode string)
}

// ComplexOwnerReader resolves who a notice about a complex goes to.
// complexstore.Store satisfies it.
type ComplexOwnerReader interface {
	ComplexOwnerAndName(ctx context.Context, complexID uuid.UUID) (uuid.UUID, string, error)
}

// InstagramNoticeStore is what the token sweep needs to tell owners: who they
// are, and which connections ran out without being flagged yet, so each can be
// flagged one by one and its owner told once. complexstore.Store satisfies it.
type InstagramNoticeStore interface {
	ComplexOwnerReader
	ListUnflaggedExpiredInstagramConnections(ctx context.Context, now time.Time, limit int) ([]complexstore.ExpiredInstagramConnection, error)
}

// instagramReconnectNotice tells the owner a connection now needs a reconnect.
//
// It is called only by the path that flagged the connection: needs_reconnect
// goes from false to true once per token, and a reconnect or a refresh
// replaces the token. The episode is that token's expiry, which is the guard
// every flag already matches on, so the same flag reached by two paths is one
// notice to the queue, and a reconnected account that fails again is a new one.
func instagramReconnectNotice(ctx context.Context, notify InstagramNotifier, owners ComplexOwnerReader, logger *slog.Logger, complexID uuid.UUID, expiresAt time.Time) {
	if notify == nil || owners == nil {
		return
	}
	ownerID, name, err := owners.ComplexOwnerAndName(ctx, complexID)
	if err != nil {
		logger.Warn("instagram: could not read the complex to tell its owner to reconnect", "complex_id", complexID, "error", err)
		return
	}
	notify.InstagramNeedsReconnect(connectionNotice(complexID, ownerID, name), expiresAt.UTC().Format(time.RFC3339Nano))
}
