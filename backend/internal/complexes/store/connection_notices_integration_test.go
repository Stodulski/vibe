//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// The expired list holds only the connections that ran out and are not flagged
// yet, each with the expiry the flag will match on; flagging one removes it.
func TestIntegration_ListUnflaggedExpiredInstagramConnections(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := data.ContextWithTenantBypass(context.Background())
	ig, complexes := f.Stores.Instagram, f.Stores.Complexes
	now := time.Now()

	expired, live, flagged := f.ComplexID, addComplex(t, f), addComplex(t, f)
	saveIG(t, ctx, ig, expired, "tok-expired", now.Add(-time.Hour))
	saveIG(t, ctx, ig, live, "tok-live", now.Add(10*24*time.Hour))
	saveIG(t, ctx, ig, flagged, "tok-flagged", now.Add(-2*time.Hour))
	if _, err := f.DB.Exec(ctx, `UPDATE instagram_connections SET needs_reconnect = true WHERE complex_id = $1`, flagged); err != nil {
		t.Fatal(err)
	}

	list := func() map[uuid.UUID]time.Time {
		t.Helper()
		rows, err := complexes.ListUnflaggedExpiredInstagramConnections(ctx, now, 100)
		if err != nil {
			t.Fatalf("ListUnflaggedExpiredInstagramConnections: %v", err)
		}
		out := map[uuid.UUID]time.Time{}
		for _, r := range rows {
			out[r.ComplexID] = r.ExpiresAt
		}
		return out
	}

	got := list()
	if _, ok := got[live]; ok {
		t.Error("a live connection is listed as expired")
	}
	if _, ok := got[flagged]; ok {
		t.Error("an already flagged connection is listed again")
	}
	at, ok := got[expired]
	if !ok {
		t.Fatal("the expired connection is not listed")
	}

	won, err := ig.MarkInstagramNeedsReconnect(ctx, expired, at)
	if err != nil || !won {
		t.Fatalf("flagging with the listed expiry: won=%v err=%v", won, err)
	}
	if _, ok := list()[expired]; ok {
		t.Error("a connection flagged a moment ago is listed again")
	}
}

func TestIntegration_ComplexOwnerAndName(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := data.ContextWithTenantBypass(context.Background())

	owner, name, err := f.Stores.Complexes.ComplexOwnerAndName(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("ComplexOwnerAndName: %v", err)
	}
	var wantOwner uuid.UUID
	var wantName string
	if err := f.DB.QueryRow(ctx, `SELECT owner_id, name FROM complexes WHERE id = $1`, f.ComplexID).Scan(&wantOwner, &wantName); err != nil {
		t.Fatal(err)
	}
	if owner != wantOwner || name != wantName {
		t.Errorf("got owner %s name %q; want %s %q", owner, name, wantOwner, wantName)
	}

	if _, _, err := f.Stores.Complexes.ComplexOwnerAndName(ctx, uuid.New()); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("an unknown complex = %v, want ErrRecordNotFound", err)
	}
}
