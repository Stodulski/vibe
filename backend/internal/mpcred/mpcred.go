// Package mpcred is the encryption boundary for a stored MercadoPago seller
// credential: the additional authenticated data that binds a sealed value to
// the row and column it was written for, the decode step every reader goes
// through, and the sentinels a caller switches on.
//
// It sits below the stores that hold such a credential — a complex and the
// enriched booking rows the cron paths build — so that both decode it the same
// way without one importing the other.
package mpcred

import (
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/crypto"
)

// Column names bound into a sealed MercadoPago credential's AAD (see AAD).
// The stores use these exact literals at seal and open time; cmd/mpcredkey
// uses them too when converting or rotating credentials outside the running
// server. Sealing or opening with any other spelling of the column name
// produces an AAD mismatch and Open fails.
const (
	// AccessTokenColumn is the column a sealed seller access token lives in.
	AccessTokenColumn = "mp_access_token"
	// RefreshTokenColumn is the column a sealed seller refresh token lives in.
	RefreshTokenColumn = "mp_refresh_token"
)

// AAD builds the additional authenticated data that binds a sealed
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
func AAD(complexID uuid.UUID, column string) []byte {
	const unitSeparator = "\x1f"
	return []byte("mpcred" + unitSeparator + "v1" + unitSeparator + complexID.String() + unitSeparator + column)
}

// Open is the shared decode step behind every reader of a stored credential:
// given the raw stored column value, it returns either the decrypted
// plaintext or a wrapped ErrMPCredentialUnreadable — never the raw
// ciphertext, and never a bare crypto-package error a caller might mistake
// for something else. A nil or empty raw value is "nothing stored", not a
// failure: it returns (nil, nil), which the seller-token accessors read as
// ErrMPNotConnected.
func Open(keys *crypto.Keyring, complexID uuid.UUID, column string, raw *string) (*string, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}

	plain, err := keys.Open(AAD(complexID, column), *raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMPCredentialUnreadable, err)
	}
	return &plain, nil
}

// Sentinel errors returned by the MercadoPago credential accessors.
var (
	// ErrMPNotConnected is returned when a complex (or an enriched booking
	// row) has no MercadoPago credential stored — nil or empty.
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
