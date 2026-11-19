//go:build integration

package store_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/stores"
)

// addComplex inserts one more complex, under its own owner, in the fixture's
// transaction.
func addComplex(t *testing.T, f *datatest.Fixture) uuid.UUID {
	t.Helper()
	owner := f.InsertOwner(t)
	var id uuid.UUID
	err := f.DB.QueryRow(context.Background(), `
		INSERT INTO complexes (owner_id, name, slug, address, city, province, phone)
		VALUES ($1, 'Other', $2, 'Calle 1', 'Rosario', 'Santa Fe', '+5491100000002')
		RETURNING id`, owner, "ig-"+uuid.NewString()).Scan(&id)
	if err != nil {
		t.Fatalf("creating a complex: %v", err)
	}
	return id
}

func saveIG(t *testing.T, ctx context.Context, s stores.InstagramConnectionStore, complexID uuid.UUID, token string, expires time.Time) {
	t.Helper()
	now := time.Now()
	err := s.SaveInstagramConnection(ctx, &complexstore.InstagramConnection{
		ComplexID: complexID, InstagramUserID: "17841400000000000", Username: "club",
		AccountType: "BUSINESS", AccessToken: token, ExpiresAt: expires, ConnectedAt: now,
	})
	if err != nil {
		t.Fatalf("SaveInstagramConnection: %v", err)
	}
}

// The token reaches the row sealed, opens back only for the complex it was
// written for, and a sealed value moved to another complex's row is refused.
func TestIntegration_InstagramTokenIsStoredSealedAndBoundToItsComplex(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := data.ContextWithTenantBypass(context.Background())
	s := f.Stores.Instagram
	const token = "IGAAlongLivedSecretToken123"
	now := time.Now()

	saveIG(t, ctx, s, f.ComplexID, token, now.Add(5*24*time.Hour))

	var raw string
	if err := f.DB.QueryRow(ctx, `SELECT access_token FROM instagram_connections WHERE complex_id = $1`, f.ComplexID).Scan(&raw); err != nil {
		t.Fatalf("reading the raw row: %v", err)
	}
	if strings.Contains(raw, token) {
		t.Fatalf("the stored access_token contains the plain token: %q", raw)
	}

	// Backdate so the refresh listing (which opens the token) returns it.
	if _, err := f.DB.Exec(ctx, `UPDATE instagram_connections SET refreshed_at = now() - interval '2 days' WHERE complex_id = $1`, f.ComplexID); err != nil {
		t.Fatalf("backdating: %v", err)
	}
	other := addComplex(t, f)
	saveIG(t, ctx, s, other, "other-token", now.Add(5*24*time.Hour))
	// Copy this complex's sealed token onto the other complex's row.
	if _, err := f.DB.Exec(ctx, `UPDATE instagram_connections SET access_token = $1, refreshed_at = now() - interval '2 days' WHERE complex_id = $2`, raw, other); err != nil {
		t.Fatalf("copying the sealed token: %v", err)
	}

	due, skipped, err := s.ListInstagramTokensDueForRefresh(ctx, now, now.Add(15*24*time.Hour), now.Add(-24*time.Hour), 100)
	if err != nil {
		t.Fatalf("ListInstagramTokensDueForRefresh: %v", err)
	}
	var opened string
	for _, d := range due {
		if d.ComplexID == other {
			t.Fatalf("a sealed token copied to another complex opened as %q", d.AccessToken)
		}
		if d.ComplexID == f.ComplexID {
			opened = d.AccessToken
		}
	}
	if opened != token {
		t.Fatalf("own token opened as %q, want %q", opened, token)
	}
	if len(skipped) != 1 || skipped[0] != other {
		t.Fatalf("skipped = %v, want only the complex holding the copied value (%s)", skipped, other)
	}
}

// The refresh picks only live tokens inside the window and old enough for Meta
// to refresh; expired ones are marked for a reconnect instead; and a refresh
// that lost a race to a reconnect writes nothing.
func TestIntegration_InstagramRefreshSelectsOnlyTheWindow(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := data.ContextWithTenantBypass(context.Background())
	s := f.Stores.Instagram
	now := time.Now()

	type row struct {
		name      string
		expiresIn time.Duration
		refreshed time.Duration // ago
		needs     bool
		wantDue   bool
	}
	rows := []row{
		{"due", 10 * 24 * time.Hour, 48 * time.Hour, false, true},
		{"far from expiry", 30 * 24 * time.Hour, 48 * time.Hour, false, false},
		{"refreshed an hour ago", 10 * 24 * time.Hour, time.Hour, false, false},
		{"expired", -time.Hour, 48 * time.Hour, false, false},
		{"already needs reconnect", 10 * 24 * time.Hour, 48 * time.Hour, true, false},
	}
	ids := map[uuid.UUID]row{}
	for i, r := range rows {
		id := f.ComplexID
		if i > 0 {
			id = addComplex(t, f)
		}
		saveIG(t, ctx, s, id, "tok-"+r.name, now.Add(r.expiresIn))
		if _, err := f.DB.Exec(ctx, `UPDATE instagram_connections SET refreshed_at = $1, needs_reconnect = $2 WHERE complex_id = $3`,
			now.Add(-r.refreshed), r.needs, id); err != nil {
			t.Fatalf("shaping %s: %v", r.name, err)
		}
		ids[id] = r
	}

	due, _, err := s.ListInstagramTokensDueForRefresh(ctx, now, now.Add(15*24*time.Hour), now.Add(-24*time.Hour), 100)
	if err != nil {
		t.Fatalf("ListInstagramTokensDueForRefresh: %v", err)
	}
	got := map[uuid.UUID]bool{}
	for _, d := range due {
		if _, ours := ids[d.ComplexID]; ours {
			got[d.ComplexID] = true
		}
	}
	for id, r := range ids {
		if got[id] != r.wantDue {
			t.Errorf("%s: due = %v, want %v", r.name, got[id], r.wantDue)
		}
	}

	if _, err := s.MarkExpiredInstagramConnections(ctx, now); err != nil {
		t.Fatalf("MarkExpiredInstagramConnections: %v", err)
	}
	for id, r := range ids {
		conn, err := s.GetInstagramConnection(ctx, id)
		if err != nil {
			t.Fatalf("GetInstagramConnection(%s): %v", r.name, err)
		}
		want := r.needs || r.name == "expired"
		if conn.NeedsReconnect != want {
			t.Errorf("%s: needs_reconnect = %v after marking, want %v", r.name, conn.NeedsReconnect, want)
		}
	}

	// A refresh computed against an expiry the row no longer has is dropped.
	var dueID uuid.UUID
	for id, r := range ids {
		if r.wantDue {
			dueID = id
		}
	}
	before, _ := s.GetInstagramConnection(ctx, dueID)
	ok, err := s.UpdateInstagramToken(ctx, dueID, before.ExpiresAt.Add(-time.Minute), "stale", now.Add(60*24*time.Hour), now)
	if err != nil || ok {
		t.Fatalf("UpdateInstagramToken with a stale expiry = (%v, %v), want (false, nil)", ok, err)
	}
	ok, err = s.UpdateInstagramToken(ctx, dueID, before.ExpiresAt, "fresh", now.Add(60*24*time.Hour), now)
	if err != nil || !ok {
		t.Fatalf("UpdateInstagramToken with the current expiry = (%v, %v), want (true, nil)", ok, err)
	}
	after, _ := s.GetInstagramConnection(ctx, dueID)
	if !after.ExpiresAt.After(before.ExpiresAt.Add(40 * 24 * time.Hour)) {
		t.Fatalf("expires_at = %v after the refresh, want about 60 days out", after.ExpiresAt)
	}
}

func TestIntegration_InstagramDisconnectDeletesTheRow(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := f.Scoped(context.Background())
	s := f.Stores.Instagram

	saveIG(t, ctx, s, f.ComplexID, "tok", time.Now().Add(time.Hour))
	deleted, err := s.DeleteInstagramConnection(ctx, f.ComplexID)
	if err != nil || !deleted {
		t.Fatalf("DeleteInstagramConnection = (%v, %v), want (true, nil)", deleted, err)
	}
	var n int
	if err := f.DB.QueryRow(ctx, `SELECT count(*) FROM instagram_connections WHERE complex_id = $1`, f.ComplexID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows left after disconnect = %d (%v), want 0", n, err)
	}
	if again, err := s.DeleteInstagramConnection(ctx, f.ComplexID); err != nil || again {
		t.Fatalf("second DeleteInstagramConnection = (%v, %v), want (false, nil)", again, err)
	}
}

// The tenant policies, as the application role: a session scoped to one
// complex can neither read, delete nor write another complex's connection.
// Like internal/data's row-level-security tests, it needs DATABASE_APP_URL
// (vibe_app); as a superuser it would prove nothing.
func TestIntegration_InstagramConnectionsAreTenantIsolated(t *testing.T) {
	appDSN := os.Getenv("DATABASE_APP_URL")
	adminDSN := os.Getenv("DATABASE_ADMIN_URL")
	if adminDSN == "" {
		adminDSN = os.Getenv("DATABASE_URL")
	}
	if appDSN == "" || adminDSN == "" {
		t.Skip("DATABASE_APP_URL and DATABASE_ADMIN_URL/DATABASE_URL are needed for the row-level-security test")
	}
	ctx := context.Background()
	admin := openPool(t, adminDSN, nil)
	app := openPool(t, appDSN, data.StampTenantScope)

	var super, bypass bool
	if err := app.QueryRow(ctx, `SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&super, &bypass); err != nil {
		t.Fatalf("reading the app role: %v", err)
	}
	if super || bypass {
		t.Fatalf("DATABASE_APP_URL bypasses row-level security; this test would pass on nothing")
	}

	a, b := seedIGTenant(t, admin, "a"), seedIGTenant(t, admin, "b")
	s := stores.New(app, stores.Config{Keys: datatest.CredentialKeyring(t)}).Instagram
	bypassCtx := data.ContextWithTenantBypass(ctx)
	saveIG(t, bypassCtx, s, a, "token-a", time.Now().Add(time.Hour))
	saveIG(t, bypassCtx, s, b, "token-b", time.Now().Add(time.Hour))

	asA := data.ContextWithTenant(ctx, a)
	if _, err := s.GetInstagramConnection(asA, a); err != nil {
		t.Fatalf("tenant A reading its own connection: %v", err)
	}
	if _, err := s.GetInstagramConnection(asA, b); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("tenant A reading tenant B's connection = %v, want ErrRecordNotFound", err)
	}
	if deleted, err := s.DeleteInstagramConnection(asA, b); err != nil || deleted {
		t.Fatalf("tenant A deleting tenant B's connection = (%v, %v), want (false, nil)", deleted, err)
	}
	if _, err := s.GetInstagramConnection(bypassCtx, b); err != nil {
		t.Fatalf("tenant B's connection is gone after tenant A's delete: %v", err)
	}
	err := s.SaveInstagramConnection(asA, &complexstore.InstagramConnection{
		ComplexID: b, InstagramUserID: "1", Username: "hijack", AccessToken: "x",
		ExpiresAt: time.Now().Add(time.Hour), ConnectedAt: time.Now(),
	})
	if err == nil {
		t.Fatalf("tenant A overwrote tenant B's connection")
	}
	if conn, _ := s.GetInstagramConnection(bypassCtx, b); conn == nil || conn.Username != "club" {
		t.Fatalf("tenant B's connection changed after tenant A's refused write: %+v", conn)
	}
	if _, err := s.GetInstagramConnection(ctx, a); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("an unscoped session read a connection (err = %v), want ErrRecordNotFound", err)
	}
}

func openPool(t *testing.T, dsn string, prepare func(context.Context, *pgx.Conn) (bool, error)) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parsing DSN: %v", err)
	}
	cfg.MaxConns = 2
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	if prepare != nil {
		cfg.PrepareConn = prepare
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedIGTenant creates an owner and a complex with the admin pool and removes
// them, connection included, when the test ends.
func seedIGTenant(t *testing.T, admin *pgxpool.Pool, label string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()
	var owner, complexID uuid.UUID
	if err := admin.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, first_name, last_name, phone, role, email_verified)
		VALUES ($1, $2, 'Owner', 'Ig', '+5491100000000', 'owner', true) RETURNING id`,
		"ig-"+label+"-"+suffix+"@example.test", []byte("not-a-real-hash")).Scan(&owner); err != nil {
		t.Fatalf("seeding owner: %v", err)
	}
	if err := admin.QueryRow(ctx, `
		INSERT INTO complexes (owner_id, name, slug, address, city, province, phone)
		VALUES ($1, 'IG', $2, 'Calle 1', 'Rosario', 'Santa Fe', '+5491100000001') RETURNING id`,
		owner, "ig-"+label+"-"+suffix).Scan(&complexID); err != nil {
		t.Fatalf("seeding complex: %v", err)
	}
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM instagram_connections WHERE complex_id = $1`,
			`DELETE FROM audit_log WHERE complex_id = $1`,
			`DELETE FROM complexes WHERE id = $1`,
		} {
			if _, err := admin.Exec(context.Background(), q, complexID); err != nil {
				t.Errorf("cleanup %q: %v", q, err)
			}
		}
		if _, err := admin.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, owner); err != nil {
			t.Errorf("cleanup owner: %v", err)
		}
	})
	return complexID
}
