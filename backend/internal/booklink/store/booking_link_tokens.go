package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// Store implements BookingLinkTokenStore against PostgreSQL.
//
// Hand-written rather than sqlc, precedent internal/bookings/store/refund_intents.go:
// ResolveBooking needs GetByID's enrichment JOIN, which sqlc's
// :one/:many/:exec generators have no shape for, so all three statements here
// stay hand-written rather than mixing generated and hand-written access to
// the same table.
type Store struct {
	DB *data.DB
}

// Mint creates and commits a new access token for bookingID in its own short
// transaction, for a caller that mints standalone rather than inside a
// booking's own insert — internal/payments/process.go's webhook-confirmation
// path, which runs in a separate request from the booking's own insert and so
// cannot reuse InsertSafe's transaction. See design.md's "why 1:N rather than
// one token per booking": the plaintext is unrecoverable once hashed, so
// every process that emits a link mints its own row rather than reading one
// back — this deliberately does not revoke or affect any token minted
// elsewhere for the same booking.
func (m *Store) Mint(ctx context.Context, bookingID uuid.UUID, expiresAt time.Time) (string, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	var plaintext string
	err := m.DB.WithTx(ctx, func(tx pgx.Tx, _ *db.Queries) error {
		var err error
		plaintext, err = bookingstore.MintLinkToken(ctx, tx, bookingID, expiresAt)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("mint booking link token: %w", err)
	}
	return plaintext, nil
}

// ResolveBooking hashes plaintext and returns the booking it resolves to,
// enriched the same way GetByID enriches its own result, plus the token's
// stored expiry.
//
// It carries no expiry predicate: design.md Decision 2 rejects baking
// `AND expires_at > NOW()` into this lookup (the shape email_verification.sql
// uses) because it would make a distinguishable 410 unrepresentable — the row
// would simply vanish from the result set, and "expired" would collapse into
// "unknown" the way auth/handlers.go's generic "invalid or expired" already
// has to. ErrRecordNotFound therefore means no row carries this hash, never
// that it expired; the caller (pricing.LinkLive, added in slice 2) is what
// decides expiry.
func (m *Store) ResolveBooking(ctx context.Context, plaintext string) (*bookingstore.Booking, time.Time, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	hash := bookingstore.HashLinkToken(plaintext)

	var b db.Booking
	var courtName, clientName, clientPhone string
	var expiresAt time.Time
	// BookingColumns rather than the names this query used to spell out. The
	// list it spelled out was the same one minus b.span, and
	// BookingFromDB reads StartsAt and EndsAt off the span — so every booking
	// resolved through a link token came back starting and ending in the year
	// one. Sharing the constant with the other hand-written booking SELECTs is
	// what stops the nineteenth column from being forgotten again.
	err := m.DB.QueryRow(ctx, `
		SELECT `+bookingstore.BookingColumns+`,
		       COALESCE(co.name, '') AS court_name,
		       COALESCE(cl.first_name || ' ' || cl.last_name, '') AS client_name,
		       COALESCE(cl.phone, '') AS client_phone,
		       t.expires_at
		FROM booking_link_tokens t
		JOIN bookings b ON b.id = t.booking_id
		LEFT JOIN courts co ON co.id = b.court_id
		LEFT JOIN clients cl ON cl.id = b.client_id
		WHERE t.token_hash = $1
		LIMIT 1`, hash).Scan(
		&b.ID, &b.ComplexID, &b.CourtID, &b.ClientID,
		&b.Span,
		&b.Date, &b.StartTime, &b.DurationMinutes,
		&b.Price, &b.DepositAmount, &b.Status,
		&b.CollectionStatus, &b.RefundStatus,
		&b.ReminderSent2h,
		&b.Notes, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt,
		&courtName, &clientName, &clientPhone,
		&expiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, time.Time{}, data.ErrRecordNotFound
		}
		return nil, time.Time{}, fmt.Errorf("booklink: resolve booking: %w", err)
	}
	booking := bookingstore.BookingFromDB(b)
	booking.CourtName = courtName
	booking.ClientName = clientName
	booking.ClientPhone = clientPhone
	return booking, expiresAt, nil
}

// DeleteExpiredTerminal deletes booking link tokens whose booking has reached
// a terminal state (cancelled, completed, no_show) and whose stored expiry is
// older than retention. The terminal-status predicate is load-bearing: it is
// what keeps this sweep from ever turning a live link into a 404 — a pending
// or confirmed booking's token is never touched here, however old its expiry.
func (m *Store) DeleteExpiredTerminal(ctx context.Context, retention time.Duration) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx, `
		DELETE FROM booking_link_tokens t
		USING bookings b
		WHERE t.booking_id = b.id
		  AND b.status IN ('cancelled', 'completed', 'no_show')
		  AND t.expires_at < NOW() - $1::interval`,
		retention.String(),
	)
	if err != nil {
		return fmt.Errorf("delete expired terminal booking link tokens: %w", err)
	}
	return nil
}
