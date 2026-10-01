package complexes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mpcred"
)

const (
	// mpRefreshLockPoll is how often a refresh waiting on another one retries
	// the lock. A refresh is one MercadoPago round trip, so the holder is
	// usually done within a second or two.
	mpRefreshLockPoll = 250 * time.Millisecond
	// mpRefreshLockWait bounds that wait. Past it the caller is told the
	// refresh is busy rather than left hanging: checkout is a person waiting.
	mpRefreshLockWait = 15 * time.Second
)

// ErrMPRefreshBusy reports that another refresh of the same venue's credentials
// held the lock for longer than this one was willing to wait.
var ErrMPRefreshBusy = errors.New("another refresh of this venue's MercadoPago credentials is still running")

// RefreshMPCredentials renews a venue's MercadoPago seller credentials and
// returns the access token to use. It is the only place that spends a venue's
// refresh token: the 12-hour sweep and the checkout retry both come through
// here, exported for bookings.
//
// staleAccessToken is the access token the caller found unusable (or, for the
// sweep, the one it read when it listed the venue). The refresh runs under a
// per-venue lock and re-reads the venue first: when the stored access token no
// longer matches the caller's, somebody refreshed in the meantime, and the
// stored token is returned without calling MercadoPago. That matters because
// MercadoPago rotates the refresh token on use — a second caller spending the
// refresh token it loaded earlier gets invalid_grant, or worse overwrites the
// newer tokens with older ones. An empty staleAccessToken skips the comparison
// and always refreshes.
//
// The lock is a lease row, not a session lock, so it costs no pooled
// connection while MercadoPago is being called (see data.LockModel).
//
// Errors the caller may need to tell apart, with errors.Is / errors.As:
//   - mpcred.ErrMPNotConnected, mpcred.ErrMPCredentialUnreadable: there is no
//     refresh token to spend.
//   - *mp.APIError, wrapped; IsMPRefreshRejected reports a 4xx. That is
//     MercadoPago's invalid_grant: the grant was revoked or the refresh token
//     expired, and only reconnecting recovers.
//   - ErrMPRefreshBusy: the lock was not won in time; nothing was refreshed.
//   - mpcred.ErrMPRefreshNotPersisted: MercadoPago issued new tokens and the
//     write failed. The access token is returned alongside the error so
//     checkout can finish the request in hand; the failure has already been
//     logged and sent to Sentry.
func (s *Service) RefreshMPCredentials(ctx context.Context, complexID uuid.UUID, staleAccessToken string) (string, error) {
	release, err := s.lockMPRefresh(ctx, complexID)
	if err != nil {
		return "", err
	}
	defer release()

	c, err := s.venues.GetByID(ctx, complexID)
	if err != nil {
		return "", fmt.Errorf("reloading the venue before refreshing its MercadoPago token: %w", err)
	}

	if current, accessErr := c.SellerAccessToken(); accessErr == nil && staleAccessToken != "" && current != staleAccessToken {
		s.logger.Info("mp refresh: credentials were already refreshed by another caller", "complex_id", complexID)
		return current, nil
	}

	refreshTok, err := c.SellerRefreshToken()
	if err != nil {
		return "", fmt.Errorf("reading the stored MercadoPago refresh token: %w", err)
	}

	newTokens, err := s.oauth.RefreshOAuthToken(ctx, refreshTok)
	if err != nil {
		return "", fmt.Errorf("refreshing the MercadoPago token: %w", err)
	}

	// The new tokens exist only in memory until this write lands, and the old
	// refresh token is already spent, so the write must not die with the
	// caller's request: it is detached from its cancellation, still bounded.
	persistCtx, cancel := data.DetachedQueryContext(ctx)
	defer cancel()
	mpUserID := fmt.Sprintf("%d", newTokens.UserID)
	if err := s.credentials.UpdateMPCredentials(persistCtx, complexID, newTokens.AccessToken, newTokens.RefreshToken, mpUserID, newTokens.ExpiresIn); err != nil {
		s.logger.Error("mp refresh: failed to persist refreshed credentials; the stored refresh token is now dead",
			"error", err, "complex_id", complexID, "complex_name", c.Name)
		sentry.CaptureMessage(fmt.Sprintf("MP OAuth refreshed-credential persist FAILED: complex=%s (%s) error=%v", c.Name, complexID, err))
		return newTokens.AccessToken, fmt.Errorf("%w: %w", mpcred.ErrMPRefreshNotPersisted, err)
	}

	return newTokens.AccessToken, nil
}

// lockMPRefresh takes the venue's refresh lock, waiting for a holder to finish
// for at most refreshLockWait. TryAdvisory only ever answers "taken or not", so
// the wait is a poll. The returned release is safe to defer.
func (s *Service) lockMPRefresh(ctx context.Context, complexID uuid.UUID) (func(), error) {
	if s.locks == nil {
		return nil, errors.New("no lock is configured for the MercadoPago credential refresh")
	}

	key := "mp_refresh:" + complexID.String()
	giveUp := time.NewTimer(s.refreshLockWait)
	defer giveUp.Stop()
	poll := time.NewTicker(s.refreshLockPoll)
	defer poll.Stop()

	for {
		acquired, release, err := s.locks.TryAdvisory(ctx, key)
		if err != nil {
			return nil, err
		}
		if acquired {
			return release, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-giveUp.C:
			return nil, ErrMPRefreshBusy
		case <-poll.C:
		}
	}
}
