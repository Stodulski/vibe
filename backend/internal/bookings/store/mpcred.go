package store

import (
	"github.com/stodulski/vibe-server/internal/mpcred"
)

// SellerAccessToken returns the CronBooking's MercadoPago seller access
// token. Same contract as (*Complex).SellerAccessToken — CronBooking is a
// second, independent decode surface for the same credential
// (scanCronBookings in bookings.go), so it needs its own accessor rather than
// sharing Complex's.
func (b *CronBooking) SellerAccessToken() (string, error) {
	if b.mpAccessTokenErr != nil {
		return "", b.mpAccessTokenErr
	}
	if b.mpAccessToken == nil || *b.mpAccessToken == "" {
		return "", mpcred.ErrMPNotConnected
	}
	return *b.mpAccessToken, nil
}

// SetSellerAccessTokenForTest sets the seller credential a CronBooking reads,
// for tests outside this package: the credential fields are unexported, so no
// other package can build a CronBooking whose venue is connected.
func (b *CronBooking) SetSellerAccessTokenForTest(token string) {
	b.mpAccessToken, b.mpAccessTokenErr = &token, nil
}
