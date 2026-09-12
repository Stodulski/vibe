package store

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/mpcred"
)

// SellerAccessToken returns the complex's MercadoPago seller access token.
// It returns mpcred.ErrMPNotConnected if none is stored, or
// mpcred.ErrMPCredentialUnreadable if a value is stored but complexFromDB
// could not decrypt it (wrong or retired key, corruption, tampering) — never a
// value that satisfies a nil/empty check the way a raw ciphertext column
// would.
func (c *Complex) SellerAccessToken() (string, error) {
	if c.mpAccessTokenErr != nil {
		return "", c.mpAccessTokenErr
	}
	if c.mpAccessToken == nil || *c.mpAccessToken == "" {
		return "", mpcred.ErrMPNotConnected
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
		return "", mpcred.ErrMPNotConnected
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

// NewComplexForTest constructs a Complex with its MercadoPago credential
// fields set directly.
//
// It exists for tests outside this package: once the credential fields are
// unexported, no other package can build a composite literal that sets
// them. Every other field on Complex remains exported and can be set by
// direct assignment on the returned pointer, e.g.:
//
//	c := complexstore.NewComplexForTest(id, &accessToken, nil)
//	c.Name = "Vibe"
func NewComplexForTest(id uuid.UUID, mpAccessToken, mpRefreshToken *string) *Complex {
	return &Complex{
		ID:             id,
		mpAccessToken:  mpAccessToken,
		mpRefreshToken: mpRefreshToken,
	}
}

// NewComplexWithUnreadableCredentialForTest constructs a Complex whose
// SellerAccessToken() returns mpcred.ErrMPCredentialUnreadable, simulating a
// stored credential that exists but cannot be decrypted (wrong or retired key,
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
		mpAccessTokenErr: fmt.Errorf("%w: simulated for test", mpcred.ErrMPCredentialUnreadable),
	}
}
