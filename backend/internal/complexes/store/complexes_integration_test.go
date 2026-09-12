//go:build integration

package store_test

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/crypto"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/mpcred"
)

// TestIntegration_RawMPAccessTokenColumnIsNeverAUsableToken is spec
// requirement 1's scenario against a real database: after
// UpdateMPCredentials seals a credential, a raw SELECT against the column —
// bypassing the credential accessor entirely — must never yield the value a
// caller stored, only the v1 envelope.
func TestIntegration_RawMPAccessTokenColumnIsNeverAUsableToken(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	const plainAccess = "seller-access-token-raw-check"
	const plainRefresh = "seller-refresh-token-raw-check"

	if err := f.Stores.Complexes.UpdateMPCredentials(ctx, f.ComplexID, plainAccess, plainRefresh, "mp-user-raw-check", 0); err != nil {
		t.Fatalf("UpdateMPCredentials: %v", err)
	}

	var rawAccess, rawRefresh string
	err := f.DB.QueryRow(ctx,
		`SELECT mp_access_token, mp_refresh_token FROM complexes WHERE id = $1`, f.ComplexID,
	).Scan(&rawAccess, &rawRefresh)
	if err != nil {
		t.Fatalf("reading raw columns: %v", err)
	}

	if rawAccess == plainAccess {
		t.Error("raw mp_access_token column equals the plaintext value that was stored — it is not encrypted")
	}
	if rawRefresh == plainRefresh {
		t.Error("raw mp_refresh_token column equals the plaintext value that was stored — it is not encrypted")
	}
	if !strings.HasPrefix(rawAccess, "v1.") {
		t.Errorf("raw mp_access_token = %q, want a v1.<kid>.<payload> envelope", rawAccess)
	}
	if !strings.HasPrefix(rawRefresh, "v1.") {
		t.Errorf("raw mp_refresh_token = %q, want a v1.<kid>.<payload> envelope", rawRefresh)
	}

	// The round trip through the real accessor must still recover the
	// original plaintext — the point is that only this path can.
	complex, err := f.Stores.Complexes.GetByID(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got, err := complex.SellerAccessToken(); err != nil || got != plainAccess {
		t.Errorf("SellerAccessToken() = (%q, %v), want (%q, nil)", got, err, plainAccess)
	}
}

// TestIntegration_GetWithMPConnectedSurfacesUnreadableRows proves Phase 8's
// SQL predicate deletion (the mp_refresh_token empty-string check removed, only IS NOT NULL
// remains) is safe end to end: GetWithMPConnected returns both a readable
// row and a row sealed under a key this process's keyring does not hold,
// and the Go-side accessor — not the SQL query — is what tells them apart.
func TestIntegration_GetWithMPConnectedSurfacesUnreadableRows(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	if err := f.Stores.Complexes.UpdateMPCredentials(ctx, f.ComplexID, "readable-access", "readable-refresh", "mp-user-readable", 0); err != nil {
		t.Fatalf("UpdateMPCredentials (readable complex): %v", err)
	}

	unreadableID := insertSecondComplex(f, t, "gwmc-unreadable")
	sealUnderForeignKey(f, t, unreadableID)

	complexes, err := f.Stores.Complexes.GetWithMPConnected(ctx)
	if err != nil {
		t.Fatalf("GetWithMPConnected: %v", err)
	}

	byID := make(map[string]*complexstore.Complex, len(complexes))
	for _, c := range complexes {
		byID[c.ID.String()] = c
	}

	readable, ok := byID[f.ComplexID.String()]
	if !ok {
		t.Fatalf("GetWithMPConnected did not return the readable complex %s — the SQL predicate must still surface it", f.ComplexID)
	}
	if tok, err := readable.SellerAccessToken(); err != nil || tok != "readable-access" {
		t.Errorf("readable complex SellerAccessToken() = (%q, %v), want (\"readable-access\", nil)", tok, err)
	}

	unreadable, ok := byID[unreadableID.String()]
	if !ok {
		t.Fatalf("GetWithMPConnected did not return the unreadable complex %s — presence (not readability) is all the SQL predicate may filter on", unreadableID)
	}
	if _, err := unreadable.SellerAccessToken(); !errors.Is(err, mpcred.ErrMPCredentialUnreadable) {
		t.Errorf("unreadable complex SellerAccessToken(): want mpcred.ErrMPCredentialUnreadable, got %v", err)
	}
	if unreadable.MPConnected() {
		t.Error("a row sealed under a key this keyring does not hold must not report MPConnected() == true")
	}
}

// TestIntegration_ListComplexesNeedingMPRefresh proves the query
// cronRefreshMPTokens now filters on: a connected complex whose token has no
// recorded expiry, or expires within 30 days, is due for a refresh; one
// whose token still has 100 days left is not.
func TestIntegration_ListComplexesNeedingMPRefresh(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	// The fixture's own complex: connected, but UpdateMPCredentials was
	// never told an expiry (expiresIn=0) — mp_token_expires_at stays NULL,
	// which must read as "needs a refresh".
	if err := f.Stores.Complexes.UpdateMPCredentials(ctx, f.ComplexID, "null-expiry-access", "null-expiry-refresh", "mp-user-null-expiry", 0); err != nil {
		t.Fatalf("UpdateMPCredentials (null expiry): %v", err)
	}

	soonID := insertSecondComplex(f, t, "lcnmr-soon")
	if err := f.Stores.Complexes.UpdateMPCredentials(ctx, soonID, "soon-access", "soon-refresh", "mp-user-soon", 10*24*3600); err != nil {
		t.Fatalf("UpdateMPCredentials (expires in 10 days): %v", err)
	}

	freshID := insertSecondComplex(f, t, "lcnmr-fresh")
	if err := f.Stores.Complexes.UpdateMPCredentials(ctx, freshID, "fresh-access", "fresh-refresh", "mp-user-fresh", 100*24*3600); err != nil {
		t.Fatalf("UpdateMPCredentials (expires in 100 days): %v", err)
	}

	complexes, err := f.Stores.Complexes.ListComplexesNeedingMPRefresh(ctx)
	if err != nil {
		t.Fatalf("ListComplexesNeedingMPRefresh: %v", err)
	}

	needsRefresh := make(map[string]bool, len(complexes))
	for _, c := range complexes {
		needsRefresh[c.ID.String()] = true
	}

	if !needsRefresh[f.ComplexID.String()] {
		t.Errorf("a complex with no recorded token expiry (NULL) must be returned as needing a refresh")
	}
	if !needsRefresh[soonID.String()] {
		t.Errorf("a complex whose token expires in 10 days (within the 30-day window) must be returned")
	}
	if needsRefresh[freshID.String()] {
		t.Errorf("a complex whose token expires in 100 days (outside the 30-day window) must NOT be returned")
	}
}

// TestIntegration_UpdateRefusesALostUpdate is H-14 against a real database:
// two editors who both loaded the row before either wrote must not be able to
// silently erase one another's change. UpdateComplex's `updated_at = $17`
// precondition is what makes that true — this test is the proof that the SQL
// actually enforces it, not just that complexstore.Store.Update compiles against
// the right shape.
//
// Simulates "one owner in two tabs" (CPX-17): both tabs GET the same row,
// each edits a different field, and the second save must not overwrite the
// first's — which is exactly the failure CPX-17 observed before this fix.
func TestIntegration_UpdateRefusesALostUpdate(t *testing.T) {
	f := datatest.Shared(t)
	ctx := context.Background()

	firstTab, err := f.Stores.Complexes.GetByID(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("GetByID (first tab): %v", err)
	}
	secondTab, err := f.Stores.Complexes.GetByID(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("GetByID (second tab): %v", err)
	}

	// The first tab saves. This must succeed and move updated_at forward.
	firstTab.City = "Concurrency City"
	if err := f.Stores.Complexes.Update(ctx, firstTab, nil); err != nil {
		t.Fatalf("first Update (should win the race): %v", err)
	}

	// The second tab still holds the updated_at it read before the first
	// tab's write landed. Its save must be refused rather than silently
	// overwrite the first tab's change with the stale row it has in memory.
	secondTab.Province = "Concurrency Province"
	err = f.Stores.Complexes.Update(ctx, secondTab, nil)
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("second Update (stale updated_at) = %v, want ErrRecordNotFound", err)
	}

	final, err := f.Stores.Complexes.GetByID(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("GetByID (final): %v", err)
	}
	if final.City != "Concurrency City" {
		t.Errorf("the winning writer's change was lost; City = %q, want %q", final.City, "Concurrency City")
	}
	if final.Province == "Concurrency Province" {
		t.Error("the losing writer's change landed anyway; the updated_at precondition did not hold")
	}

	// The losing tab can retry against the now-current row and succeed.
	secondTab.UpdatedAt = final.UpdatedAt
	if err := f.Stores.Complexes.Update(ctx, secondTab, nil); err != nil {
		t.Fatalf("retry after refresh: %v", err)
	}
}

// insertSecondComplex creates one more complex under the fixture's owner,
// with no MercadoPago credential yet — sealUnderForeignKey fills that in.
// Registered for cleanup the same way newTestFixture's own complex is.
func insertSecondComplex(f *datatest.Fixture, t *testing.T, slugSuffix string) (id uuid.UUID) {
	t.Helper()

	err := f.DB.QueryRow(context.Background(), `
		INSERT INTO complexes (owner_id, name, slug, address, city, province, phone)
		VALUES ($1, 'Second Complex', $2, 'Av. Siempreviva 743', 'Rosario', 'Santa Fe', '+5491100000002')
		RETURNING id`,
		f.UserID, "test-complex-"+slugSuffix,
	).Scan(&id)
	if err != nil {
		t.Fatalf("creating second complex: %v", err)
	}

	t.Cleanup(func() {
		if _, err := f.DB.Exec(context.Background(), `DELETE FROM complexes WHERE id = $1`, id); err != nil {
			t.Errorf("deleting second complex: %v", err)
		}
	})

	return id
}

// sealUnderForeignKey writes a credential sealed under a keyring the
// fixture's own Models was never given — simulating a row encrypted under a
// key that has since been retired and dropped.
func sealUnderForeignKey(f *datatest.Fixture, t *testing.T, complexID uuid.UUID) {
	t.Helper()

	key := make([]byte, 32)
	for i := range key {
		key[i] = 0xCD
	}
	foreign, err := crypto.ParseKeyring("foreign:" + base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("building foreign keyring: %v", err)
	}

	sealedAccess, err := foreign.Seal(mpcred.AAD(complexID, mpcred.AccessTokenColumn), "will-not-open")
	if err != nil {
		t.Fatalf("sealing under foreign key: %v", err)
	}
	sealedRefresh, err := foreign.Seal(mpcred.AAD(complexID, mpcred.RefreshTokenColumn), "will-not-open-either")
	if err != nil {
		t.Fatalf("sealing under foreign key: %v", err)
	}

	_, err = f.DB.Exec(context.Background(),
		`UPDATE complexes SET mp_access_token = $1, mp_refresh_token = $2, mp_user_id = $3 WHERE id = $4`,
		sealedAccess, sealedRefresh, "mp-user-unreadable", complexID)
	if err != nil {
		t.Fatalf("writing foreign-sealed credential: %v", err)
	}
}

// TestIntegration_SlugOfASoftDeletedComplexStaysTaken pins the agreement
// between the availability check and the uniqueness constraint.
//
// complexes.slug is UNIQUE across the whole table, so a soft-deleted venue
// keeps its slug. SlugExists used to exclude deleted rows, which made it the
// only place that answered "free" about a name the database would refuse: the
// handler validated, inserted, and got a 23505 back as a 500.
//
// Both halves are asserted here, because fixing only the check would leave the
// race (another request taking the slug in between) landing as a 500 again.
func TestIntegration_SlugOfASoftDeletedComplexStaysTaken(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	var slug string
	err := f.DB.QueryRow(ctx, `SELECT slug FROM complexes WHERE id = $1`, f.ComplexID).Scan(&slug)
	if err != nil {
		t.Fatalf("reading the fixture's slug: %v", err)
	}

	// SoftDeleteCascade, not SoftDelete: the store no longer offers a way to
	// stamp a complex without closing its courts (the soft-delete cascade). The count it
	// returns is not what this test is about, and every assertion below is
	// unchanged — the slug stays taken, which is the decision this pins.
	if _, err := f.Stores.Complexes.SoftDeleteCascade(ctx, f.ComplexID); err != nil {
		t.Fatalf("SoftDeleteCascade: %v", err)
	}

	// The complex is gone from every live-row query...
	if _, err := f.Stores.Complexes.GetBySlug(ctx, slug); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a soft-deleted complex is still served publicly: %v", err)
	}

	// ...but its slug is not free, because the constraint says it is not.
	taken, err := f.Stores.Complexes.SlugExists(ctx, slug)
	if err != nil {
		t.Fatalf("SlugExists: %v", err)
	}
	if !taken {
		t.Error("SlugExists reported a soft-deleted complex's slug as free; the unique " +
			"constraint will refuse the insert that follows")
	}

	// And the insert that a stale check would let through is a named domain
	// error, not a raw constraint violation.
	// CancellationHours is spelled out even though this test is about slugs.
	// Insert names the column, so an omitted field here is Go's zero rather than
	// the schema's DEFAULT 24, and complexes_cancellation_hours_range refuses zero — the check
	// would fire before the unique constraint and this test would report the
	// wrong error. That is not an inconvenience of the constraint; it is the
	// constraint catching the same shape of omission that put two
	// cancellation_hours = 0 rows in the development database.
	err = f.Stores.Complexes.Insert(ctx, &complexstore.Complex{
		OwnerID:           f.UserID,
		Name:              "Reuses the deleted slug",
		Slug:              slug,
		Address:           "Av. Siempreviva 742",
		City:              "Rosario",
		Province:          "Santa Fe",
		CountryCode:       "AR",
		Currency:          "ARS",
		Phone:             "+5491100000002",
		CancellationHours: 24,
	})
	if !errors.Is(err, complexstore.ErrDuplicateSlug) {
		t.Errorf("Insert on a taken slug returned %v; want ErrDuplicateSlug", err)
	}
}
