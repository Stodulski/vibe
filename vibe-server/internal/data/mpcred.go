package data

import (
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/crypto"
)

// Column names bound into a sealed MercadoPago credential's AAD (see
// MPCredAAD). internal/data uses these exact literals at seal and open
// time; cmd/mpcredkey uses them too when converting or rotating credentials
// outside the running server. Sealing or opening with any other spelling of
// the column name produces an AAD mismatch and Open fails.
const (
	MPAccessTokenColumn  = "mp_access_token"
	MPRefreshTokenColumn = "mp_refresh_token"
)

// MPCredAAD builds the additional authenticated data that binds a sealed
// MercadoPago credential to the exact row and column it was written for:
// "mpcred" 0x1F "v1" 0x1F complexID 0x1F column. AES-GCM authenticates this
// value unmodified at Open time, so copying a sealed value between
// complexes, or from mp_access_token into mp_refresh_token (or vice versa),
// fails to decrypt instead of silently succeeding against the wrong row or
// column.
//
// Exported for cmd/mpcredkey, which seals and re-keys credentials outside
// the running server and must reproduce this exact byte sequence or every
// row it touches fails to open.
func MPCredAAD(complexID uuid.UUID, column string) []byte {
	const unitSeparator = "\x1f"
	return []byte("mpcred" + unitSeparator + "v1" + unitSeparator + complexID.String() + unitSeparator + column)
}

// openMPCredential is the shared decode step behind complexFromDB and
// scanCronBookings: given the raw stored column value, it returns either the
// decrypted plaintext or a wrapped ErrMPCredentialUnreadable — never the raw
// ciphertext, and never a bare crypto-package error a caller might mistake
// for something else. A nil or empty raw value is "nothing stored", not a
// failure: it returns (nil, nil), which SellerAccessToken/SellerRefreshToken
// read as ErrMPNotConnected.
func openMPCredential(keys *crypto.Keyring, complexID uuid.UUID, column string, raw *string) (*string, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}

	plain, err := keys.Open(MPCredAAD(complexID, column), *raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMPCredentialUnreadable, err)
	}
	return &plain, nil
}

// Sentinel errors returned by the MercadoPago credential accessors below.
var (
	// ErrMPNotConnected is returned when a complex (or CronBooking) has no
	// MercadoPago credential stored — nil or empty.
	ErrMPNotConnected = errors.New("data: MercadoPago is not connected for this complex")
	// ErrMPCredentialUnreadable is returned when a stored credential exists
	// but could not be turned back into its original plaintext (Slice 2:
	// wrong or retired key, corruption, tampering). Unreachable in Slice 1,
	// where opening a credential is the identity function — declared now so
	// the accessor signatures do not change again once Slice 2 wires a real
	// decrypt in behind them.
	ErrMPCredentialUnreadable = errors.New("data: stored MercadoPago credential could not be read")
	// ErrMPCredentialEmpty is returned by UpdateMPCredentials (Slice 2) when
	// asked to seal an empty value, before any encryption is attempted — a
	// floor beneath the encryption boundary, not the guard itself. Declared
	// now for signature stability; unreachable until Slice 2 wires sealing.
	ErrMPCredentialEmpty = errors.New("data: MercadoPago credential must not be empty")
)

// SellerAccessToken returns the complex's MercadoPago seller access token.
// It returns ErrMPNotConnected if none is stored, or ErrMPCredentialUnreadable
// if a value is stored but complexFromDB could not decrypt it (wrong or
// retired key, corruption, tampering) — never a value that satisfies a
// nil/empty check the way a raw ciphertext column would.
func (c *Complex) SellerAccessToken() (string, error) {
	if c.mpAccessTokenErr != nil {
		return "", c.mpAccessTokenErr
	}
	if c.mpAccessToken == nil || *c.mpAccessToken == "" {
		return "", ErrMPNotConnected
	}
	return *c.mpAccessToken, nil
}

// SellerRefreshToken returns the complex's MercadoPago seller refresh token.
// Same contract as SellerAccessToken.
func (c *Complex) SellerRefreshToken() (string, error) {
	if c.mpRefreshTokenErr != nil {
		return "", c.mpRefreshTokenErr
	}
	if c.mpRefreshToken == nil || *c.mpRefreshToken == "" {
		return "", ErrMPNotConnected
	}
	return *c.mpRefreshToken, nil
}

// MPConnected reports whether this complex currently has a usable
// MercadoPago connection. It reports false for every error
// SellerAccessToken can return — "not connected" and, once Slice 2 makes
// opening a real decrypt, "stored but unreadable" both mean this venue
// cannot take payments right now.
func (c *Complex) MPConnected() bool {
	_, err := c.SellerAccessToken()
	return err == nil
}

// SellerAccessToken returns the CronBooking's MercadoPago seller access
// token. Same contract as (*Complex).SellerAccessToken — CronBooking is a
// second, independent decode surface for the same credential
// (internal/data/bookings.go's scanCronBookings), so it needs its own
// accessor rather than sharing Complex's.
func (b *CronBooking) SellerAccessToken() (string, error) {
	if b.mpAccessTokenErr != nil {
		return "", b.mpAccessTokenErr
	}
	if b.mpAccessToken == nil || *b.mpAccessToken == "" {
		return "", ErrMPNotConnected
	}
	return *b.mpAccessToken, nil
}

// NewComplexForTest constructs a Complex with its MercadoPago credential
// fields set directly.
//
// It exists for tests outside internal/data: once the credential fields are
// unexported, no other package can build a composite literal that sets
// them. Every other field on Complex remains exported and can be set by
// direct assignment on the returned pointer, e.g.:
//
//	c := data.NewComplexForTest(id, &accessToken, nil)
//	c.Name = "Vibe"
func NewComplexForTest(id uuid.UUID, mpAccessToken, mpRefreshToken *string) *Complex {
	return &Complex{
		ID:             id,
		mpAccessToken:  mpAccessToken,
		mpRefreshToken: mpRefreshToken,
	}
}

// NewComplexWithUnreadableCredentialForTest constructs a Complex whose
// SellerAccessToken() returns ErrMPCredentialUnreadable, simulating a stored
// credential that exists but cannot be decrypted (wrong or retired key,
// corruption, tampering).
//
// It exists so cross-package tests can exercise the credential-unreadable
// arm of the mutation-verified money-path test (spec requirement 6) without
// depending on internal/crypto directly — exactly the "test harness that
// makes the credential accessor return a decrypt-failure outcome" the spec
// itself describes as the intended way to assert this behaviour.
func NewComplexWithUnreadableCredentialForTest(id uuid.UUID) *Complex {
	return &Complex{
		ID:               id,
		mpAccessTokenErr: fmt.Errorf("%w: simulated for test", ErrMPCredentialUnreadable),
	}
}
