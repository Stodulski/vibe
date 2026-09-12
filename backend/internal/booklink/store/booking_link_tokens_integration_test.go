//go:build integration

package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// These tests exercise the booking-link-token store layer
// (specs/booking-link-credential) against a real PostgreSQL. Tasks 6.1-6.3
// are labeled [Unit] in tasks.md, but InsertSafe and ResolveBooking both
// require a real transaction and a real SELECT — see
// TestBookingModel_RequiresDB in bookings_test.go, which documents that every
// bookingstore.Store method needs *pgxpool.Pool. They live here rather than in a
// mock-based unit test for the same reason every other InsertSafe/atomicity
// test in this package does (see refund_intents_integration_test.go).

// TestInsertSafeMintsATokenInTheSameTransaction is task 6.1: a booking never
// commits without a usable token. It also covers 6.2 (LinkToken shape) as a
// byproduct of the same successful insert, since splitting them would insert
// the same booking twice for no added coverage.
//
// Mutation, run and recorded: move the mint call (MintLinkToken) outside
// InsertSafe's transaction — i.e. after tx.Commit(ctx) — re-run, and the
// booking row must persist despite the mint failing, because a mint that no
// longer participates in the transaction cannot roll it back.
func TestInsertSafeMintsATokenInTheSameTransaction(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	b := f.NewBooking(datatest.BookingOptions{Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid, Public: true})
	if err := f.Stores.Bookings.InsertSafe(ctx, b); err != nil {
		t.Fatalf("InsertSafe: %v", err)
	}

	if len(b.LinkToken) != 43 {
		t.Errorf("LinkToken length = %d, want 43 (32 random bytes, base64.RawURLEncoding); got %q", len(b.LinkToken), b.LinkToken)
	}
	if b.LinkToken == b.ID.String() {
		t.Error("LinkToken must never equal the booking id — that is the exact confusion this change ends")
	}

	// The token actually resolves, proving it was committed, not merely held
	// in memory on b.
	resolved, expiresAt, err := f.Stores.BookingLinkTokens.ResolveBooking(ctx, b.LinkToken)
	if err != nil {
		t.Fatalf("ResolveBooking(minted token): %v", err)
	}
	if resolved.ID != b.ID {
		t.Errorf("ResolveBooking resolved to %s, want %s", resolved.ID, b.ID)
	}
	if !expiresAt.After(time.Now()) {
		t.Errorf("expires_at = %v, want a time in the future for a freshly minted token", expiresAt)
	}
}

// TestTwoMintsForTheSameBookingProduceDifferentTokensBothResolve is task 6.3.
//
// Mutation, run and recorded: reuse a per-booking constant (e.g. derive the
// plaintext from bookingID.String() instead of fresh crypto/rand bytes)
// instead of fresh random bytes in MintLinkToken — re-run, and either the
// second Mint fails (token_hash UNIQUE collision) or, if the constant is
// varied by call count instead, the two plaintexts stop being independently
// unguessable, which this test's non-equality assertion alone would not
// catch as sharply as the UNIQUE-collision failure does. Either failure mode
// breaks this test.
func TestTwoMintsForTheSameBookingProduceDifferentTokensBothResolve(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	b := f.CreateBooking(t, datatest.BookingOptions{})

	expiresAt := b.EndsAt.Add(24 * time.Hour)
	first, err := f.Stores.BookingLinkTokens.Mint(ctx, b.ID, expiresAt)
	if err != nil {
		t.Fatalf("first Mint: %v", err)
	}
	second, err := f.Stores.BookingLinkTokens.Mint(ctx, b.ID, expiresAt)
	if err != nil {
		t.Fatalf("second Mint: %v", err)
	}

	if first == second {
		t.Fatalf("two mints for the same booking produced the same plaintext: %q", first)
	}

	for _, plaintext := range []string{first, second} {
		resolved, _, err := f.Stores.BookingLinkTokens.ResolveBooking(ctx, plaintext)
		if err != nil {
			t.Fatalf("ResolveBooking(%q): %v", plaintext, err)
		}
		if resolved.ID != b.ID {
			t.Errorf("ResolveBooking(%q) resolved to %s, want %s", plaintext, resolved.ID, b.ID)
		}
	}
}

// TestResolveBookingIgnoresExpiryAndReturnsIt is task 6.5: ResolveBooking
// carries no expiry predicate — design.md Decision 2 rejects baking
// `AND expires_at > NOW()` into the lookup because it would make a
// distinguishable 410 unrepresentable.
//
// Mutation, run and recorded: restore `AND expires_at > NOW()` in
// ResolveBooking's SELECT — re-run, and this test must fail with
// ErrRecordNotFound instead of returning the row and its past expires_at.
func TestResolveBookingIgnoresExpiryAndReturnsIt(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	b := f.CreateBooking(t, datatest.BookingOptions{})
	past := time.Now().Add(-48 * time.Hour)
	plaintext, err := f.Stores.BookingLinkTokens.Mint(ctx, b.ID, past)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	resolved, expiresAt, err := f.Stores.BookingLinkTokens.ResolveBooking(ctx, plaintext)
	if err != nil {
		t.Fatalf("ResolveBooking on an expired token must still return the row, not %v", err)
	}
	if resolved.ID != b.ID {
		t.Errorf("resolved booking id = %s, want %s", resolved.ID, b.ID)
	}
	if !expiresAt.Before(time.Now()) {
		t.Errorf("expires_at = %v, want the past value Mint stored", expiresAt)
	}
}

// TestResolveBookingUnknownTokenIsNotFound proves the negative counterpart:
// a value never minted resolves to ErrRecordNotFound, not any other error.
func TestResolveBookingUnknownTokenIsNotFound(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	_, _, err := f.Stores.BookingLinkTokens.ResolveBooking(ctx, "never-minted-value")
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("ResolveBooking(unknown) = %v, want ErrRecordNotFound", err)
	}
}

// TestDeleteExpiredTerminalNeverTouchesALiveBooking is task 6.6.
//
// Mutation, run and recorded: drop the terminal-status predicate from
// DeleteExpiredTerminal's DELETE (i.e. delete on expires_at alone) — re-run,
// and the confirmed booking's token, despite its past expiry, must survive
// this test's assertion but will not once the predicate is gone.
func TestDeleteExpiredTerminalNeverTouchesALiveBooking(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	live := f.CreateBooking(t, datatest.BookingOptions{Status: "confirmed"})
	terminal := f.CreateBooking(t, datatest.BookingOptions{Status: "cancelled", StartTime: "08:00", EndTime: "09:30"})

	past := time.Now().Add(-48 * time.Hour)
	liveToken, err := f.Stores.BookingLinkTokens.Mint(ctx, live.ID, past)
	if err != nil {
		t.Fatalf("Mint(live): %v", err)
	}
	terminalToken, err := f.Stores.BookingLinkTokens.Mint(ctx, terminal.ID, past)
	if err != nil {
		t.Fatalf("Mint(terminal): %v", err)
	}

	if err := f.Stores.BookingLinkTokens.DeleteExpiredTerminal(ctx, time.Hour); err != nil {
		t.Fatalf("DeleteExpiredTerminal: %v", err)
	}

	if _, _, err := f.Stores.BookingLinkTokens.ResolveBooking(ctx, liveToken); err != nil {
		t.Errorf("a live (confirmed) booking's token must survive the sweep however old its expiry; got %v", err)
	}
	if _, _, err := f.Stores.BookingLinkTokens.ResolveBooking(ctx, terminalToken); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a terminal booking's token past retention must be swept; ResolveBooking = %v, want ErrRecordNotFound", err)
	}
}

// TestDeleteExpiredTerminalRespectsRetention proves the sweep does not delete
// a terminal booking's token before its own retention window has passed,
// even though its status already qualifies — the predicate is a conjunction,
// not an either/or.
func TestDeleteExpiredTerminalRespectsRetention(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	terminal := f.CreateBooking(t, datatest.BookingOptions{Status: "completed"})
	recentPast := time.Now().Add(-10 * time.Minute)
	token, err := f.Stores.BookingLinkTokens.Mint(ctx, terminal.ID, recentPast)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	if err := f.Stores.BookingLinkTokens.DeleteExpiredTerminal(ctx, 24*time.Hour); err != nil {
		t.Fatalf("DeleteExpiredTerminal: %v", err)
	}

	if _, _, err := f.Stores.BookingLinkTokens.ResolveBooking(ctx, token); err != nil {
		t.Errorf("a terminal booking's token still inside its retention window must survive; got %v", err)
	}
}

// TestCheckoutTokenSurvivesTheConfirmationMint is task 14.1: a public booking
// that completes checkout via the MercadoPago webhook ends up with two live
// rows in booking_link_tokens for the same booking_id — the one InsertSafe
// minted for the checkout back_url, and the one internal/payments/process.go's
// confirmation path mints (via this same Mint method) for the confirmation
// email — both resolving to the same booking, and the checkout token
// deliberately not revoked when the second mint runs: MercadoPago redirects
// the browser to the success back_url at roughly the moment the webhook
// fires, so revoking it would break that redirect (design.md's "two live
// tokens" consequence).
//
// Mutation, run and recorded: add a delete of every other token for the same
// booking_id to Mint itself —
//
//	_, _ = tx.Exec(ctx, `DELETE FROM booking_link_tokens WHERE booking_id = $1`, UUIDToPg(bookingID))
//
// — right before the INSERT in MintLinkToken, standing in for a revoke-on-
// confirm the production confirmation path (internal/payments/process.go)
// does not and must not have. Re-run: the checkout token's ResolveBooking
// call must then fail, breaking the "checkout token still resolves"
// assertion below.
func TestCheckoutTokenSurvivesTheConfirmationMint(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	b := f.NewBooking(datatest.BookingOptions{Status: "pending", CollectionStatus: bookingstore.CollectionStatusUnpaid, Public: true})
	if err := f.Stores.Bookings.InsertSafe(ctx, b); err != nil {
		t.Fatalf("InsertSafe: %v", err)
	}
	checkoutToken := b.LinkToken

	// Stands in for internal/payments/process.go's confirmation-path mint,
	// which runs in a separate request/transaction from InsertSafe's.
	confirmationToken, err := f.Stores.BookingLinkTokens.Mint(ctx, b.ID, b.EndsAt.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("confirmation Mint: %v", err)
	}
	if confirmationToken == checkoutToken {
		t.Fatal("the confirmation path must mint its own token, not reuse the checkout one")
	}

	if _, _, err := f.Stores.BookingLinkTokens.ResolveBooking(ctx, checkoutToken); err != nil {
		t.Errorf("the checkout token must still resolve after the confirmation mint — it is deliberately not revoked: %v", err)
	}
	if _, _, err := f.Stores.BookingLinkTokens.ResolveBooking(ctx, confirmationToken); err != nil {
		t.Errorf("the confirmation token must resolve: %v", err)
	}
}

// TestMintRejectsAnUnknownBooking proves the foreign key does its job: a
// bookingID that names no row refuses rather than inserting an orphan.
func TestMintRejectsAnUnknownBooking(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	_, err := f.Stores.BookingLinkTokens.Mint(ctx, uuid.New(), time.Now().Add(24*time.Hour))
	if err == nil {
		t.Fatal("Mint for an unknown booking id must fail on the foreign key, not silently insert an orphan")
	}
	if !strings.Contains(err.Error(), "insert booking link token") {
		t.Errorf("Mint error = %v, want it wrapped as an insert failure", err)
	}
}

// TestResolveBookingCarriesTheSpansInstants pins the one enrichment
// ResolveBooking's SELECT used to leave out.
//
// BookingFromDB reads StartsAt and EndsAt off `span` (bookings.go), so a
// SELECT that does not list b.span hands back a zero pgtype.Range and the
// booking resolves with both instants at 0001-01-01T00:00:00Z. The comment
// above that assignment already records one bug of exactly this shape — a link
// token minted from a zero EndsAt "expires in the year one" — which is why the
// column list is now BookingColumns rather than eighteen names typed out
// again: the six other hand-written booking SELECTs cannot drift from it.
//
// The span is read back from the row itself rather than compared against
// InsertSafe's copy alone, so the assertion is against what the database
// stored and not against another Go value that could be zero for the same
// reason.
func TestResolveBookingCarriesTheSpansInstants(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	b := f.NewBooking(datatest.BookingOptions{StartTime: "18:00", EndTime: "19:30", Public: true})
	if err := f.Stores.Bookings.InsertSafe(ctx, b); err != nil {
		t.Fatalf("InsertSafe: %v", err)
	}

	var wantStart, wantEnd time.Time
	err := f.Pool.QueryRow(ctx,
		`SELECT lower(span), upper(span) FROM bookings WHERE id = $1`, b.ID,
	).Scan(&wantStart, &wantEnd)
	if err != nil {
		t.Fatalf("reading the stored span: %v", err)
	}

	resolved, _, err := f.Stores.BookingLinkTokens.ResolveBooking(ctx, b.LinkToken)
	if err != nil {
		t.Fatalf("ResolveBooking: %v", err)
	}

	if resolved.StartsAt.IsZero() {
		t.Fatalf("StartsAt is the zero instant (%s): the SELECT does not carry b.span, so every "+
			"booking resolved through a link token starts in the year one",
			resolved.StartsAt.Format(time.RFC3339))
	}
	if !resolved.StartsAt.Equal(wantStart) {
		t.Errorf("StartsAt = %s, want the span's lower bound %s",
			resolved.StartsAt.Format(time.RFC3339), wantStart.Format(time.RFC3339))
	}
	if !resolved.EndsAt.Equal(wantEnd) {
		t.Errorf("EndsAt = %s, want the span's upper bound %s",
			resolved.EndsAt.Format(time.RFC3339), wantEnd.Format(time.RFC3339))
	}
}
