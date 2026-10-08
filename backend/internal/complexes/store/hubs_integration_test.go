//go:build integration

package store_test

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	sqldb "github.com/stodulski/vibe-server/internal/db"
)

// TestIntegration_ListActiveComplexesForHubsKeepsOnlyLiveSwitchedOnComplexes
// proves the hub read against a real database: a complex that is switched off
// or soft-deleted is absent, and a complex's sports come only from its
// switched-on, live courts, each sport once.
func TestIntegration_ListActiveComplexesForHubsKeepsOnlyLiveSwitchedOnComplexes(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	// The hub read is the concrete store's method, not the ComplexStore
	// interface's, so the test builds the store the same way stores.New does.
	store := &complexstore.Store{DB: f.DB, Q: sqldb.New(f.DB)}

	live := insertHubComplex(ctx, t, f, "Club Live", "Banfield", true)
	off := insertHubComplex(ctx, t, f, "Club Off", "Banfield", false)
	gone := insertHubComplex(ctx, t, f, "Club Gone", "Banfield", true)
	if _, err := f.DB.Exec(ctx, `UPDATE complexes SET deleted_at = now() WHERE id = $1`, gone); err != nil {
		t.Fatalf("soft-deleting a complex: %v", err)
	}

	insertHubCourt(ctx, t, f, live, "Cancha 1", "padel", true)
	insertHubCourt(ctx, t, f, live, "Cancha 2", "padel", true)
	insertHubCourt(ctx, t, f, live, "Cancha 3", "tennis", true)
	insertHubCourt(ctx, t, f, live, "Cancha 4", "soccer", false)
	softDeleted := insertHubCourt(ctx, t, f, live, "Cancha 5", "hockey", true)
	if _, err := f.DB.Exec(ctx, `UPDATE courts SET deleted_at = now() WHERE id = $1`, softDeleted); err != nil {
		t.Fatalf("soft-deleting a court: %v", err)
	}

	rows, err := store.ListActiveComplexesForHubs(ctx)
	if err != nil {
		t.Fatalf("ListActiveComplexesForHubs: %v", err)
	}

	byID := make(map[uuid.UUID]complexstore.HubComplex, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}

	if _, ok := byID[off]; ok {
		t.Error("a switched-off complex was listed for its city hub")
	}
	if _, ok := byID[gone]; ok {
		t.Error("a soft-deleted complex was listed for its city hub")
	}

	got, ok := byID[live]
	if !ok {
		t.Fatal("the live, switched-on complex was not listed")
	}
	if got.City != "Banfield" || got.Name != "Club Live" {
		t.Errorf("listed the complex as %q in %q; want Club Live in Banfield", got.Name, got.City)
	}
	// Two padel courts collapse to one padel; the switched-off and the
	// soft-deleted court contribute nothing.
	if want := []string{"padel", "tennis"}; !slices.Equal(got.Sports, want) {
		t.Errorf("sports = %v; want %v", got.Sports, want)
	}
}

// insertHubComplex inserts a complex in the given city, owned by a fresh owner
// (a complex's owner may hold only one live complex), and returns its id.
func insertHubComplex(ctx context.Context, t *testing.T, f *datatest.Fixture, name, city string, active bool) uuid.UUID {
	t.Helper()

	var owner uuid.UUID
	if err := f.DB.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, first_name, last_name, phone, role, email_verified)
		VALUES ($1, $2, 'Owner', 'Hub', '+5491100000000', 'owner', true)
		RETURNING id`,
		"hub-owner-"+uuid.NewString()+"@example.test", []byte("not-a-real-hash"),
	).Scan(&owner); err != nil {
		t.Fatalf("creating owner for %s: %v", name, err)
	}

	var id uuid.UUID
	if err := f.DB.QueryRow(ctx, `
		INSERT INTO complexes (owner_id, name, slug, address, city, province, phone, is_active)
		VALUES ($1, $2, $3, 'Av. Siempreviva 742', $4, 'Buenos Aires', '+5491100000001', $5)
		RETURNING id`,
		owner, name, "hub-"+uuid.NewString(), city, active,
	).Scan(&id); err != nil {
		t.Fatalf("creating complex %s: %v", name, err)
	}
	return id
}

// insertHubCourt inserts a court of the given sport under a complex and returns
// its id.
func insertHubCourt(ctx context.Context, t *testing.T, f *datatest.Fixture, complexID uuid.UUID, name, sport string, active bool) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	if err := f.DB.QueryRow(ctx, `
		INSERT INTO courts (complex_id, name, sport, is_active)
		VALUES ($1, $2, $3::sport_type, $4)
		RETURNING id`,
		complexID, name, sport, active,
	).Scan(&id); err != nil {
		t.Fatalf("creating court %s: %v", name, err)
	}
	return id
}
