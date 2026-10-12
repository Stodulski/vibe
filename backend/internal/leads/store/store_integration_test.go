//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/db"
	"github.com/stodulski/vibe-server/internal/leads/store"
)

func newLeadStore(f *datatest.Fixture) *store.Store {
	return &store.Store{Q: db.New(f.DB)}
}

type leadRow struct {
	first, last, phone   string
	createdAt, updatedAt time.Time
}

func readLead(t *testing.T, f *datatest.Fixture, email string) leadRow {
	t.Helper()
	var r leadRow
	if err := f.DB.QueryRow(context.Background(), `
		SELECT first_name, last_name, phone, created_at, updated_at FROM registration_leads WHERE email = $1`,
		email).Scan(&r.first, &r.last, &r.phone, &r.createdAt, &r.updatedAt); err != nil {
		t.Fatalf("reading lead %s: %v", email, err)
	}
	return r
}

// A second capture of the same address refreshes what it has and keeps what it does
// not: an empty field from a later capture never erases an earlier, fuller one. The
// first capture time is kept, since the nudge clock starts there.
func TestUpsertKeepsTheNamesAnEarlierCaptureGave(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLeadStore(f)
	ctx := context.Background()

	if err := st.Upsert(ctx, "ana@example.com", "Ana", "Perez", "+5411"); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	before := readLead(t, f, "ana@example.com")

	if err := st.Upsert(ctx, "ana@example.com", "", "", "+5491155555555"); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	after := readLead(t, f, "ana@example.com")

	if after.first != "Ana" || after.last != "Perez" {
		t.Errorf("names = %q %q after an empty capture, want the earlier ones kept", after.first, after.last)
	}
	if after.phone != "+5491155555555" {
		t.Errorf("phone = %q, want the newer non-empty value", after.phone)
	}
	if !after.createdAt.Equal(before.createdAt) {
		t.Errorf("created_at moved on a repeat capture: %v -> %v", before.createdAt, after.createdAt)
	}
}

// The schema refuses an address that was not trimmed and lowercased first, so a
// handler that forgot to normalize fails loudly instead of creating a second lead.
func TestTheSchemaRefusesAnUnnormalizedAddress(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLeadStore(f)

	if err := st.Upsert(context.Background(), "Ana@Example.com", "Ana", "", ""); err == nil {
		t.Fatal("an uppercase address was stored; it would be a second lead for the same person")
	}
}

// Leads are kept for thirty days and no longer, which is the privacy promise on the
// register form. The purge removes older rows and nothing newer.
func TestTheLeadPurgeKeepsThirtyDaysAndNoLonger(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLeadStore(f)
	ctx := context.Background()

	for email, ago := range map[string]string{"old@example.com": "31 days", "recent@example.com": "29 days"} {
		if _, err := f.DB.Exec(ctx, `INSERT INTO registration_leads (email, created_at) VALUES ($1, NOW() - $2::interval)`,
			email, ago); err != nil {
			t.Fatalf("inserting %s: %v", email, err)
		}
	}

	removed, err := st.DeleteOlderThan30Days(ctx)
	if err != nil {
		t.Fatalf("DeleteOlderThan30Days: %v", err)
	}
	var oldLeft, recentLeft int
	if err := f.DB.QueryRow(ctx, `SELECT count(*) FROM registration_leads WHERE email = 'old@example.com'`).Scan(&oldLeft); err != nil {
		t.Fatalf("counting old: %v", err)
	}
	if err := f.DB.QueryRow(ctx, `SELECT count(*) FROM registration_leads WHERE email = 'recent@example.com'`).Scan(&recentLeft); err != nil {
		t.Fatalf("counting recent: %v", err)
	}
	if oldLeft != 0 || recentLeft != 1 {
		t.Fatalf("after purge: old=%d recent=%d (removed %d), want old=0 recent=1", oldLeft, recentLeft, removed)
	}
}
