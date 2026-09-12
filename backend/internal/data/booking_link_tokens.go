package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/db"
)

// BookingLinkTokenModel implements BookingLinkTokenStore against PostgreSQL.
//
// Hand-written rather than sqlc, precedent internal/data/refund_intents.go:
// ResolveBooking needs GetByID's enrichment JOIN, which sqlc's
// :one/:many/:exec generators have no shape for, so all three statements here
// stay hand-written rather than mixing generated and hand-written access to
// the same table.
type BookingLinkTokenModel struct {
	DB *DB
}

// linkTokenByteLength is how much entropy MintLinkToken draws for a plaintext
// token: 32 random bytes, base64.RawURLEncoding-encoded to 43 characters —
// the same shape generateRefreshToken/hashRefreshToken (internal/auth/tokens.go)
// use for a refresh token, duplicated here rather than imported —
// internal/data must not depend on internal/auth. Chosen over the house
// uuid.New() idiom deliberately: a UUID-shaped token would be
// indistinguishable from a booking id in any URL or log, which is the
// confusion this change exists to end (design.md Decision 1).
const linkTokenByteLength = 32

// hashLinkToken hashes a plaintext booking link token the same way
// hashRefreshToken hashes a refresh token: SHA-256, stored raw.
func hashLinkToken(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}

// MintLinkToken generates a fresh plaintext token and inserts its hash within
// tx, so a caller that already holds a transaction (BookingModel.InsertSafe)
// can mint atomically with the booking it belongs to — a crash between the
// two commits would otherwise strand a booking with no usable link on any of
// its public routes.
func MintLinkToken(ctx context.Context, tx pgx.Tx, bookingID uuid.UUID, expiresAt time.Time) (string, error) {
	buf := make([]byte, linkTokenByteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate booking link token: %w", err)
	}
	plaintext := base64.RawURLEncoding.EncodeToString(buf)

	_, err := tx.Exec(ctx, `
		INSERT INTO booking_link_tokens (booking_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`,
		UUIDToPg(bookingID), hashLinkToken(plaintext), TimeToPg(expiresAt),
	)
	if err != nil {
		return "", fmt.Errorf("insert booking link token: %w", err)
	}
	return plaintext, nil
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
func (m *BookingLinkTokenModel) Mint(ctx context.Context, bookingID uuid.UUID, expiresAt time.Time) (string, error) {
	ctx, cancel := TxContext(ctx)
	defer cancel()

	tx, err := m.DB.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	// Rollback is a no-op once Commit succeeds (pgx returns ErrTxClosed, which is expected).
	defer func() { _ = tx.Rollback(ctx) }()

	plaintext, err := MintLinkToken(ctx, tx, bookingID, expiresAt)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit booking link token: %w", err)
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
func (m *BookingLinkTokenModel) ResolveBooking(ctx context.Context, plaintext string) (*Booking, time.Time, error) {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	hash := hashLinkToken(plaintext)

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
		SELECT `+BookingColumns+`,
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
			return nil, time.Time{}, ErrRecordNotFound
		}
		return nil, time.Time{}, err
	}
	booking := BookingFromDB(b)
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
func (m *BookingLinkTokenModel) DeleteExpiredTerminal(ctx context.Context, retention time.Duration) error {
	ctx, cancel := QueryContext(ctx)
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
