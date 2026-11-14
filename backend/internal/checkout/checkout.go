// Package checkout is the part of opening a public MercadoPago checkout every
// storefront sale shares, whatever it sells: naming the seller the preference is
// created as, and creating it again with a refreshed credential when the
// seller's access token has expired.
//
// A booking and an open-play event sign-up both open their checkout through it,
// so a rule added here (a refusal, a retry) reaches both.
package checkout

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/mpcred"
)

// ErrNotConnected reports a venue that cannot take money: it has no usable
// MercadoPago credential.
var ErrNotConnected = errors.New("checkout: the complex has no usable MercadoPago credential")

// Preferences is the MercadoPago client's checkout side.
type Preferences interface {
	CreatePreference(ctx context.Context, input mp.CreatePreferenceInput) (*mp.Preference, error)
}

// Refresher renews a seller's MercadoPago credentials and returns the access
// token to use. staleAccessToken is the one the caller just saw rejected; when
// the stored one already differs, somebody else refreshed first and that token
// comes back without another refresh. On mpcred.ErrMPRefreshNotPersisted the
// token is still returned alongside the error.
type Refresher interface {
	RefreshMPCredentials(ctx context.Context, complexID uuid.UUID, staleAccessToken string) (string, error)
}

// Seller is the party a storefront checkout is created as. The checkout is
// created as this seller and nobody else: mp.AsSeller has no arm that reaches
// MercadoPago as the platform, which is where the money would otherwise land.
// It answers ErrNotConnected for a venue with no usable credential.
func Seller(complex *complexstore.Complex) (mp.Caller, error) {
	var seller mp.Caller
	sellerToken, sellerErr := complex.SellerAccessToken()
	if sellerErr == nil {
		seller, sellerErr = mp.AsSeller(sellerToken)
	}
	if sellerErr != nil {
		if errors.Is(sellerErr, mpcred.ErrMPCredentialUnreadable) {
			sentry.CaptureMessage(fmt.Sprintf("SELLER TOKEN UNREADABLE (refusing checkout): complex_id=%s error=%v", complex.ID, sellerErr))
		}
		return seller, ErrNotConnected
	}
	return seller, nil
}

// CreateWithRetry creates a MercadoPago preference, retrying once with a token
// refresh if the seller's access token has expired.
func CreateWithRetry(
	ctx context.Context, prefs Preferences, refresher Refresher, logger *slog.Logger,
	prefInput mp.CreatePreferenceInput, complex *complexstore.Complex,
) (*mp.Preference, error) {
	pref, err := prefs.CreatePreference(ctx, prefInput)
	if err == nil || !mp.IsUnauthorized(err) {
		return pref, err
	}

	// An unreadable access token leaves the stale value empty, which makes the
	// refresh unconditional rather than skipped.
	staleAccess, accessErr := complex.SellerAccessToken()
	if accessErr != nil {
		staleAccess = ""
	}

	logger.Info("public checkout: seller token expired, refreshing", "complex_id", complex.ID)
	accessToken, refreshErr := refresher.RefreshMPCredentials(ctx, complex.ID, staleAccess)
	if refreshErr != nil {
		if !errors.Is(refreshErr, mpcred.ErrMPRefreshNotPersisted) {
			logger.Error("public checkout: failed to refresh seller token", "error", refreshErr, "complex_id", complex.ID)
			return pref, err
		}
		// MercadoPago did issue new tokens; only storing them failed, and the
		// refresh has already alerted. The access token that came back is good
		// for this request, so the client is not failed for a write they have
		// no part in.
		logger.Error("public checkout: refreshed MP credentials were not stored, continuing with the new access token for this request",
			"error", refreshErr, "complex_id", complex.ID)
	}

	refreshedSeller, callerErr := mp.AsSeller(accessToken)
	if callerErr != nil {
		// MercadoPago answered the refresh without an access token.
		// Retrying without one would have meant retrying as the
		// platform, so there is nothing left to retry with: the
		// original 401 is the answer.
		logger.Error("public checkout: the refreshed seller token is unusable, keeping the original failure",
			"error", callerErr, "complex_id", complex.ID)
		return pref, err
	}
	prefInput.Caller = refreshedSeller
	return prefs.CreatePreference(ctx, prefInput)
}
