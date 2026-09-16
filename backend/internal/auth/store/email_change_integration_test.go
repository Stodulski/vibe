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

// Put upserts on user_id: a second request for a different address replaces
// the first rather than leaving two live tokens racing to decide the
// account's next address (see EmailChanges.Put's own comment).
func TestIntegration_EmailChangesPutReplacesThePendingRequest(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	firstHash := []byte("first-hash-00000000000000000000")
	if err := f.Stores.EmailChange.Put(ctx, f.UserID, "first@example.com", firstHash); err != nil {
		t.Fatalf("first Put: %v", err)
	}

	secondHash := []byte("second-hash-0000000000000000000")
	if err := f.Stores.EmailChange.Put(ctx, f.UserID, "second@example.com", secondHash); err != nil {
		t.Fatalf("second Put: %v", err)
	}

	pending, err := f.Stores.EmailChange.GetPendingByUser(ctx, f.UserID)
	if err != nil {
		t.Fatalf("GetPendingByUser: %v", err)
	}
	if pending.NewEmail != "second@example.com" {
		t.Errorf("want the second request to win; got %q", pending.NewEmail)
	}

	if _, err := f.Stores.EmailChange.Peek(ctx, firstHash); err == nil {
		t.Error("the first request's token must not still resolve once a second one replaced it")
	}
}

// Peek reads without spending the token, and Consume is what makes it
// single-use — the split auth.Service.ConfirmEmailChange relies on so
// validation and the email write both run before the token is spent.
func TestIntegration_EmailChangesPeekThenConsumeIsSingleUse(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	hash := []byte("peek-consume-hash-000000000000000")
	if err := f.Stores.EmailChange.Put(ctx, f.UserID, "new@example.com", hash); err != nil {
		t.Fatalf("Put: %v", err)
	}

	first, err := f.Stores.EmailChange.Peek(ctx, hash)
	if err != nil {
		t.Fatalf("first Peek: %v", err)
	}
	if first.NewEmail != "new@example.com" {
		t.Errorf("want new@example.com; got %q", first.NewEmail)
	}

	// A second Peek before Consume must still find it — Peek alone must never
	// spend the token.
	if _, err := f.Stores.EmailChange.Peek(ctx, hash); err != nil {
		t.Fatalf("a second Peek before Consume must still find the request: %v", err)
	}

	if err := f.Stores.EmailChange.Consume(ctx, hash); err != nil {
		t.Fatalf("Consume: %v", err)
	}

	if _, err := f.Stores.EmailChange.Peek(ctx, hash); err == nil {
		t.Error("Peek must not find the request once Consume has removed it")
	}

	// A second Consume of the same hash is the race ConfirmEmailChange treats
	// as benign, reported as data.ErrRecordNotFound rather than success.
	err = f.Stores.EmailChange.Consume(ctx, hash)
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("want data.ErrRecordNotFound consuming an already-consumed token; got %v", err)
	}
}

// DeleteExpired only removes rows past their expiry — the same filtering
// PasswordResets.DeleteExpired proves for its own table, and what
// cronCleanExpiredTokens now relies on for this table too.
func TestIntegration_EmailChangesDeleteExpiredOnlyRemovesExpiredRows(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	liveHash := []byte("live-hash-0000000000000000000000000")
	if err := f.Stores.EmailChange.Put(ctx, f.UserID, "live@example.com", liveHash); err != nil {
		t.Fatalf("Put (live): %v", err)
	}

	// A second account of this test's own, so the expired row does not have
	// to share the live row's user_id — Put's ON CONFLICT (user_id) allows
	// only one pending request per account.
	var otherUserID uuid.UUID
	err := f.DB.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, first_name, last_name, phone, role, email_verified)
		VALUES ($1, 'x', 'Other', 'Owner', '+5491100009998', 'owner', true)
		RETURNING id`,
		"email-change-expiry-"+uuid.NewString()+"@test.com",
	).Scan(&otherUserID)
	if err != nil {
		t.Fatalf("creating the second account: %v", err)
	}

	expiredHash := []byte("expired-hash-000000000000000000000")
	if _, err := f.DB.Exec(ctx, `
		INSERT INTO email_change_requests (user_id, new_email, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`,
		otherUserID, "expired@example.com", expiredHash, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("seeding an expired row: %v", err)
	}

	if err := f.Stores.EmailChange.DeleteExpired(ctx); err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}

	if _, err := f.Stores.EmailChange.Peek(ctx, liveHash); err != nil {
		t.Errorf("the live request must survive DeleteExpired: %v", err)
	}

	var count int
	if err := f.DB.QueryRow(ctx, `SELECT count(*) FROM email_change_requests WHERE token_hash = $1`, expiredHash).Scan(&count); err != nil {
		t.Fatalf("checking the expired row: %v", err)
	}
	if count != 0 {
		t.Error("the expired request must be gone")
	}
}
