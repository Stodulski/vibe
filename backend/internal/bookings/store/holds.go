package store

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
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// A slot hold is the pending storefront booking, taken when the visitor picks
// a court rather than when they pay (db/migrations/025_booking_slot_holds.sql).
// Until POST /book converts it, it names nobody: no client and no snapshot,
// only the hash of the token the visitor was handed and the address that took
// it. Everything that already applies to an unpaid public pending booking
// applies to it unchanged: it blocks the grid through the exclusion
// constraint, the owner's reads leave it out, and it stops holding its slot
// once the payment expiry has passed since created_at.

// ErrHoldNotFound reports a hold token that names no live, untouched hold: it
// never existed, was released, was converted, or ran out.
var ErrHoldNotFound = errors.New("bookings: no live slot hold for that token")

// ErrHoldLimit reports an address that already holds the most slots it may.
var ErrHoldLimit = errors.New("bookings: too many live slot holds from this address")

// untouchedHold is the SQL predicate, over the bookings row aliased b, for a
// hold nobody has claimed yet: it still carries its token, a visitor made it,
// nothing was collected and it names nobody. It is what keeps every delete
// below off a converted or paid booking, whatever token it is handed:
// conversion clears the token and writes the snapshot, and payment attaches a
// client and moves the collection status.
const untouchedHold = `b.hold_token_hash IS NOT NULL
	  AND b.created_by IS NULL
	  AND b.collection_status = 'unpaid'
	  AND b.client_id IS NULL
	  AND b.client_phone IS NULL
	  AND b.status IN ('pending', 'cancelled')`

// holdTag is what marks an inserted booking as a slot hold: the hash of its
// token and the address that took it. The zero value marks nothing.
type holdTag struct {
	tokenHash []byte
	ip        *string
}

// holdTokenByteLength is the entropy of a hold token, the same 32 random bytes
// a booking link token draws (see linkTokenByteLength).
const holdTokenByteLength = 32

// NewHoldToken returns a fresh hold token and the hash that is stored for it.
func NewHoldToken() (plaintext string, hash []byte, err error) {
	buf := make([]byte, holdTokenByteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generate hold token: %w", err)
	}
	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, HashHoldToken(plaintext), nil
}

// HashHoldToken is what is stored for a hold token: SHA-256, raw, as for every
// other bearer token in this codebase (see HashLinkToken).
func HashHoldToken(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}

// InsertHold inserts b as a hold, carrying its token hash and the address that
// took it, through the same guarded insert InsertSafe runs.
//
// The per-address cap is counted and enforced in the same transaction, under
// a transaction-scoped advisory lock on the address, so two requests from one
// address cannot both see room for one more. That lock is always taken before
// the court-day locks the insert takes, and nothing takes them the other way
// round, so the two cannot deadlock.
func (m *Store) InsertHold(ctx context.Context, b *Booking, tokenHash []byte, ip string, maxPerIP int) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, b.ComplexID); err != nil {
		return err
	}

	return m.DB.RetryTx(ctx, data.DefaultTxAttempts, func(tx pgx.Tx, _ *db.Queries) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "booking-hold-ip:"+ip); err != nil {
			return fmt.Errorf("lock hold address: %w", err)
		}
		var live int
		err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM bookings b
			WHERE b.hold_ip = $1
			  AND b.status = 'pending'
			  AND b.created_at > NOW() - make_interval(secs => $2)
			  AND `+untouchedHold, ip, m.PaymentExpiry.Seconds()).Scan(&live)
		if err != nil {
			return fmt.Errorf("count live holds: %w", err)
		}
		if live >= maxPerIP {
			return ErrHoldLimit
		}

		return m.insertSafe(ctx, tx, b, holdTag{tokenHash: tokenHash, ip: &ip})
	})
}

// GetHold returns the booking a hold token still names, or
// data.ErrRecordNotFound. It finds the row whatever its age or status; the
// caller decides whether the hold is still live, and ConvertHold decides it
// again atomically.
func (m *Store) GetHold(ctx context.Context, tokenHash []byte) (*Booking, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var b db.Booking
	err := m.DB.QueryRow(ctx, `
		SELECT `+BookingColumns+`
		FROM bookings b
		WHERE b.hold_token_hash = $1`, tokenHash).Scan(
		&b.ID, &b.ComplexID, &b.CourtID, &b.ClientID,
		&b.Span,
		&b.Date, &b.StartTime, &b.DurationMinutes,
		&b.Price, &b.DepositAmount, &b.Status,
		&b.CollectionStatus, &b.RefundStatus,
		&b.ReminderSent2h,
		&b.Notes, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, fmt.Errorf("bookings: read slot hold: %w", err)
	}
	return BookingFromDB(b), nil
}

// ConvertHold turns the hold b.ID into the storefront booking POST /book used
// to insert: it writes b.ClientSnapshot and b.Notes, clears the token and the
// address, and mints the booking's link token into b.LinkToken, in one
// transaction.
//
// The UPDATE re-checks, under the row lock, that the row is still the live,
// untouched hold this token names. A second conversion, a release or the
// expiry sweep that got there first leaves nothing for it to match, and that
// is ErrHoldNotFound — so exactly one caller ever converts a hold, and only
// that caller goes on to create a checkout for it.
func (m *Store) ConvertHold(ctx context.Context, b *Booking, tokenHash []byte) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, b.ComplexID); err != nil {
		return err
	}
	if b.ClientSnapshot == nil {
		return errors.New("bookings: converting a hold needs who it is for")
	}
	first, last, phone, email := snapshotParams(b)

	return m.DB.WithTx(ctx, func(tx pgx.Tx, _ *db.Queries) error {
		var endsAt pgtype.Timestamptz
		err := tx.QueryRow(ctx, `
			UPDATE bookings b
			SET client_first_name = $3, client_last_name = $4, client_phone = $5, client_email = $6,
			    notes = COALESCE($7, b.notes),
			    hold_token_hash = NULL, hold_ip = NULL
			WHERE b.id = $1
			  AND b.hold_token_hash = $2
			  AND b.status = 'pending'
			  AND b.created_at > NOW() - make_interval(secs => $8)
			  AND `+untouchedHold+`
			RETURNING upper(b.span)`,
			data.UUIDToPg(b.ID), tokenHash,
			data.TextToPg(first), data.TextToPg(last), data.TextToPg(phone), data.TextToPg(email),
			data.TextToPg(b.Notes), m.PaymentExpiry.Seconds(),
		).Scan(&endsAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrHoldNotFound
			}
			return fmt.Errorf("bookings: convert slot hold: %w", err)
		}

		linkToken, err := MintLinkToken(ctx, tx, b.ID, endsAt.Time.Add(m.LinkTokenBuffer))
		if err != nil {
			return fmt.Errorf("mint booking link token: %w", err)
		}
		b.LinkToken = linkToken
		return nil
	})
}

// ReleaseHold deletes the untouched hold a token names and reports its complex,
// or released false when there is none (unknown, already released, converted,
// paid). It is safe to call any number of times.
//
// Deleted rather than cancelled: an untouched hold names nobody, took no money,
// was never shown to the owner (slotguard.UnpaidPublicBooking) and has no
// notice to send, and every court a visitor taps creates one. Kept as a
// cancelled row it would be noise in the bookings table and in nothing else:
// no audit entry is written for a hold (the trail starts at the conversion,
// as public_book), and its only dependent row, the link token InsertSafe
// minted with it, goes with it by ON DELETE CASCADE.
func (m *Store) ReleaseHold(ctx context.Context, tokenHash []byte) (uuid.UUID, bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var complexID pgtype.UUID
	err := m.DB.QueryRow(ctx, `
		DELETE FROM bookings b
		WHERE b.hold_token_hash = $1
		  AND `+untouchedHold+`
		RETURNING b.complex_id`, tokenHash).Scan(&complexID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, fmt.Errorf("bookings: release slot hold: %w", err)
	}
	return data.PgToUUID(complexID), true, nil
}

// DeleteExpiredHolds deletes every untouched hold older than expiry, the
// payment window it held its slot for, and returns the complex of each one so
// the caller can tell their grids. See ReleaseHold for why it deletes.
func (m *Store) DeleteExpiredHolds(ctx context.Context, expiry time.Duration) ([]uuid.UUID, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		DELETE FROM bookings b
		WHERE b.created_at < NOW() - make_interval(secs => $1)
		  AND `+untouchedHold+`
		RETURNING b.complex_id`, expiry.Seconds())
	if err != nil {
		return nil, fmt.Errorf("bookings: delete expired slot holds: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[pgtype.UUID])
	if err != nil {
		return nil, fmt.Errorf("bookings: delete expired slot holds: %w", err)
	}
	out := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		out[i] = data.PgToUUID(id)
	}
	return out, nil
}

// DeleteComplexHolds deletes one complex's untouched holds, whatever their age.
// Disconnecting MercadoPago runs it: a hold there could never be paid.
func (m *Store) DeleteComplexHolds(ctx context.Context, complexID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if _, err := m.DB.Exec(ctx, `
		DELETE FROM bookings b
		WHERE b.complex_id = $1
		  AND `+untouchedHold, data.UUIDToPg(complexID)); err != nil {
		return fmt.Errorf("bookings: delete complex slot holds: %w", err)
	}
	return nil
}
