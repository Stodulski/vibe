package complexes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/notifications"
)

// MPNotifier queues the owner's emails about a complex's MercadoPago
// connection. notifications.Service is the production implementation. A nil
// MPNotifier sends nothing.
type MPNotifier interface {
	MercadoPagoConnected(n notifications.ConnectionNotice, eventID string)
	MercadoPagoDisconnected(n notifications.ConnectionNotice, eventID string)
	// MercadoPagoRefreshFailed returns the queue's refusal, so an episode is
	// recorded as told only once its notice is queued.
	MercadoPagoRefreshFailed(n notifications.ConnectionNotice, episode string) error
}

// MPRefreshNoticeStore remembers which refresh-failure episode of a complex
// the owner was last told about. complexstore.Store satisfies it.
//
// An episode is one refresh token MercadoPago refused: a successful refresh or
// a reconnect stores a new refresh token, which ends the episode, and the next
// refusal is a new one. It is remembered in the database rather than left to
// the queue's dedup key because the sweep keeps meeting the same refused token
// every 12 hours for as long as the owner does not reconnect, and the queue
// forgets a key a week after its job is done.
type MPRefreshNoticeStore interface {
	MPRefreshFailureNotified(ctx context.Context, complexID uuid.UUID, episode string) (bool, error)
	RecordMPRefreshFailureNotice(ctx context.Context, complexID uuid.UUID, episode string) error
}

// mpNoticeFor reads the venue the notice is about: the owner it goes to and the
// name it gives. The change it reports is already committed, so a failed read
// costs the email and nothing else; it is logged.
func (s *Service) mpNoticeFor(ctx context.Context, complexID uuid.UUID) (notifications.ConnectionNotice, bool) {
	c, err := s.venues.GetByID(ctx, complexID)
	if err != nil || c == nil {
		s.logger.Warn("mp notice: could not read the venue to tell its owner", "error", err, "complex_id", complexID)
		return notifications.ConnectionNotice{}, false
	}
	return connectionNotice(c.ID, c.OwnerID, c.Name), true
}

func connectionNotice(complexID, ownerID uuid.UUID, name string) notifications.ConnectionNotice {
	return notifications.ConnectionNotice{OwnerID: ownerID.String(), ComplexID: complexID.String(), ComplexName: name}
}

// mpConnected tells the owner a connection was stored. Each connection is its
// own event.
func (s *Service) mpConnected(ctx context.Context, complexID uuid.UUID) {
	if s.notify == nil {
		return
	}
	if n, ok := s.mpNoticeFor(ctx, complexID); ok {
		s.notify.MercadoPagoConnected(n, uuid.NewString())
	}
}

// mpDisconnected tells the owner the connection was cleared.
func (s *Service) mpDisconnected(ctx context.Context, complexID uuid.UUID) {
	if s.notify == nil {
		return
	}
	if n, ok := s.mpNoticeFor(ctx, complexID); ok {
		s.notify.MercadoPagoDisconnected(n, uuid.NewString())
	}
}

// mpRefreshRefused tells the owner MercadoPago refused to renew the venue's
// connection, once per episode. The episode is named by a digest of the
// refused refresh token, so the token itself is stored nowhere new.
//
// The order is check, queue, record. A crash between the queue and the record
// costs a second attempt, which the queue's dedup key on the same episode
// absorbs; recording first would let that crash lose the notice.
func (s *Service) mpRefreshRefused(ctx context.Context, c *complexstore.Complex, refreshToken string) {
	if s.notify == nil || s.notices == nil {
		return
	}
	// The refusal is already the caller's answer; telling the owner must not
	// depend on the caller still waiting.
	ctx, cancel := data.DetachedQueryContext(ctx)
	defer cancel()

	sum := sha256.Sum256([]byte(refreshToken))
	episode := hex.EncodeToString(sum[:])

	told, err := s.notices.MPRefreshFailureNotified(ctx, c.ID, episode)
	if err != nil {
		s.logger.Error("mp refresh: could not read whether the owner was told", "error", err, "complex_id", c.ID)
		return
	}
	if told {
		return
	}
	if err := s.notify.MercadoPagoRefreshFailed(connectionNotice(c.ID, c.OwnerID, c.Name), episode); err != nil {
		s.logger.Error("mp refresh: could not queue the owner's notice", "error", err, "complex_id", c.ID)
		return
	}
	if err := s.notices.RecordMPRefreshFailureNotice(ctx, c.ID, episode); err != nil {
		s.logger.Error("mp refresh: could not record that the owner was told", "error", err, "complex_id", c.ID)
	}
}
