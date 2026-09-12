package data

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/crypto"
	"github.com/stodulski/vibe-server/internal/db"
	"github.com/stodulski/vibe-server/internal/mpcred"
)

// testKeyring32 returns a base64-encoded 32-byte key filled with fill, so
// tests can build distinct keyring specs without hardcoding opaque literals.
func testKeyring32(t *testing.T, kid string, fill byte) *crypto.Keyring {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = fill
	}
	kr, err := crypto.ParseKeyring(kid + ":" + base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("building test keyring: %v", err)
	}
	return kr
}

// TestComplexFromDB_DecryptsStoredCredential proves complexFromDB's open
// step (Phase 17.2) is a real decrypt, not the Slice 1 identity pass-through:
// a value sealed under the keyring's key comes back as the original
// plaintext through SellerAccessToken/SellerRefreshToken.
func TestComplexFromDB_DecryptsStoredCredential(t *testing.T) {
	id := uuid.New()
	keys := testKeyring32(t, "k1", 0x01)

	sealedAccess, err := keys.Seal(mpcred.AAD(id, mpcred.AccessTokenColumn), "seller-access-token")
	if err != nil {
		t.Fatalf("Seal access: %v", err)
	}
	sealedRefresh, err := keys.Seal(mpcred.AAD(id, mpcred.RefreshTokenColumn), "seller-refresh-token")
	if err != nil {
		t.Fatalf("Seal refresh: %v", err)
	}

	dbComplex := db.Complex{
		ID:             pgtype.UUID{Bytes: id, Valid: true},
		MpAccessToken:  pgtype.Text{String: sealedAccess, Valid: true},
		MpRefreshToken: pgtype.Text{String: sealedRefresh, Valid: true},
	}

	c := complexFromDB(dbComplex, keys)

	gotAccess, err := c.SellerAccessToken()
	if err != nil {
		t.Fatalf("SellerAccessToken: unexpected error: %v", err)
	}
	if gotAccess != "seller-access-token" {
		t.Errorf("SellerAccessToken() = %q, want %q", gotAccess, "seller-access-token")
	}

	gotRefresh, err := c.SellerRefreshToken()
	if err != nil {
		t.Fatalf("SellerRefreshToken: unexpected error: %v", err)
	}
	if gotRefresh != "seller-refresh-token" {
		t.Errorf("SellerRefreshToken() = %q, want %q", gotRefresh, "seller-refresh-token")
	}
}

// TestComplexFromDB_UnreadableCredentialSurfacesAsUnreadable proves the
// real failure mode spec requirement 6 targets: a credential sealed under a
// key the reading keyring no longer holds (a retired key, or corruption)
// must surface as mpcred.ErrMPCredentialUnreadable, never as a value that passes
// the historical nil/empty check.
func TestComplexFromDB_UnreadableCredentialSurfacesAsUnreadable(t *testing.T) {
	id := uuid.New()
	writingKeys := testKeyring32(t, "retired", 0x02)
	readingKeys := testKeyring32(t, "active", 0x03) // does not hold "retired"

	sealed, err := writingKeys.Seal(mpcred.AAD(id, mpcred.AccessTokenColumn), "seller-access-token")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	dbComplex := db.Complex{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		MpAccessToken: pgtype.Text{String: sealed, Valid: true},
	}

	c := complexFromDB(dbComplex, readingKeys)

	_, err = c.SellerAccessToken()
	if !errors.Is(err, mpcred.ErrMPCredentialUnreadable) {
		t.Fatalf("SellerAccessToken(): want mpcred.ErrMPCredentialUnreadable, got %v", err)
	}
	if c.MPConnected() {
		t.Error("MPConnected() must be false for an unreadable credential")
	}
}

// TestComplexFromDB_NoStoredCredentialReturnsNotConnected proves a genuinely
// absent credential is unaffected by the decrypt step — the NULL/empty case
// still reports mpcred.ErrMPNotConnected, not mpcred.ErrMPCredentialUnreadable.
func TestComplexFromDB_NoStoredCredentialReturnsNotConnected(t *testing.T) {
	id := uuid.New()
	keys := testKeyring32(t, "k1", 0x04)

	dbComplex := db.Complex{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		MpAccessToken: pgtype.Text{Valid: false},
	}

	c := complexFromDB(dbComplex, keys)

	_, err := c.SellerAccessToken()
	if !errors.Is(err, mpcred.ErrMPNotConnected) {
		t.Fatalf("SellerAccessToken(): want mpcred.ErrMPNotConnected, got %v", err)
	}
	if c.MPConnected() {
		t.Error("MPConnected() must be false when nothing is stored")
	}
}

// TestComplexFromDB_NilKeyringNeverPassesStoredValueThrough is the guard
// this whole change exists for, exercised at the decode point rather than
// directly against crypto.Keyring: a stored (ciphertext-shaped) value read
// with a nil keyring must never come back as if it were a usable token.
func TestComplexFromDB_NilKeyringNeverPassesStoredValueThrough(t *testing.T) {
	id := uuid.New()

	dbComplex := db.Complex{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		MpAccessToken: pgtype.Text{String: "v1.k1.somepayload", Valid: true},
	}

	c := complexFromDB(dbComplex, nil)

	_, err := c.SellerAccessToken()
	if !errors.Is(err, mpcred.ErrMPCredentialUnreadable) {
		t.Fatalf("SellerAccessToken() with a nil keyring: want mpcred.ErrMPCredentialUnreadable, got %v", err)
	}
}

// TestUpdateMPCredentials_RefusesEmptyBeforeSealing proves the floor
// design.md describes: UpdateMPCredentials never reaches Keyring.Seal with
// an empty value, so a bug in the CHECK constraint is never the only thing
// standing between an empty credential and storage.
func TestUpdateMPCredentials_RefusesEmptyBeforeSealing(t *testing.T) {
	m := &ComplexModel{Keys: testKeyring32(t, "k1", 0x05)}

	tests := []struct {
		name                      string
		access, refresh, mpUserID string
	}{
		{"empty access token", "", "refresh", "user-1"},
		{"empty refresh token", "access", "", "user-1"},
		{"empty user id", "access", "refresh", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.UpdateMPCredentials(t.Context(), uuid.New(), tt.access, tt.refresh, tt.mpUserID, 0)
			if !errors.Is(err, mpcred.ErrMPCredentialEmpty) {
				t.Errorf("UpdateMPCredentials(): want mpcred.ErrMPCredentialEmpty, got %v", err)
			}
		})
	}
}
