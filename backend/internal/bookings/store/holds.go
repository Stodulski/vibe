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
	"github.com/stodulski/vibe-server/internal/data/slotguard"
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
// A nil tokenHash inserts a storefront booking that is not a hold — POST /book
// without a hold token, whose b carries its ClientSnapshot — with the address
// and under the same cap, which that path otherwise escaped entirely.
//
// The cap counts every storefront booking from the address that still holds
// its slot: an untouched hold, and one converted (or booked without a hold)
// whose payment has not arrived within its window. Counting only untouched
// holds let one address convert each hold with made-up details and take the
// next. Payment attaches the client and clears the address
// (AttachBookingClient), and the expiry window ends the rest.
//
// The per-address cap is counted and enforced in the same transaction, under
// a transaction-scoped advisory lock on the address, so two requests from one
// address cannot both see room for one more. That lock is always taken before
// the court-day locks the insert takes, and nothing takes them the other way
// round, so the two cannot deadlock. A maxPerIP of 0 enforces no cap.
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
			  AND b.created_by IS NULL
			  AND b.collection_status = 'unpaid'
			  AND b.client_id IS NULL
			  AND b.created_at > NOW() - make_interval(secs => $2)`,
			ip, m.PaymentExpiry.Seconds()).Scan(&live)
		if err != nil {
			return fmt.Errorf("count live holds: %w", err)
		}
		if maxPerIP > 0 && live >= maxPerIP {
			return ErrHoldLimit
		}

		// The cooldown (S2) applies to holds: the storefront always holds before
		// it books, and a booking without a hold names a visitor already.
		var (
			started time.Time
			resumes bool
		)
		if tokenHash != nil {
			var running, found bool
			var until time.Time
			started, running, found, until, err = ownHoldClock(ctx, tx, b, ip, m.PaymentExpiry)
			if err != nil {
				return err
			}
			if found && !running {
				return &HoldCooldownError{Until: until}
			}
			resumes = found
		}

		if err := m.insertSafe(ctx, tx, b, holdTag{tokenHash: tokenHash, ip: &ip}); err != nil {
			return err
		}
		if resumes {
			if _, err := tx.Exec(ctx, `UPDATE bookings SET created_at = $2 WHERE id = $1`,
				data.UUIDToPg(b.ID), data.TimeToPg(started)); err != nil {
				return fmt.Errorf("continue the address's earlier hold: %w", err)
			}
			b.CreatedAt = started
		}
		return nil
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
// to insert: it writes b.ClientSnapshot and b.Notes, clears the token, and
// mints the booking's link token into b.LinkToken, in one transaction.
//
// The address is kept: until it is paid the converted booking still counts
// against the address that took it (see InsertHold), and the payment write
// clears it (AttachBookingClient).
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

	// S7: the visitor gets at least MinPaymentWindow to pay from the moment
	// they submit. The window is moved by moving created_at, the one instant
	// every reader counts it from (SlotTaken, GetBookedSlots, the sweeps, the
	// per-address cap, the checkout's own expiry), so none of them can
	// disagree about when it ends. It never moves past now: a window shorter
	// than the minimum simply starts over.
	shift := max(m.PaymentExpiry-MinPaymentWindow, 0)

	return m.DB.WithTx(ctx, func(tx pgx.Tx, _ *db.Queries) error {
		var endsAt, createdAt pgtype.Timestamptz
		err := tx.QueryRow(ctx, `
			UPDATE bookings b
			SET client_first_name = $3, client_last_name = $4, client_phone = $5, client_email = $6,
			    notes = COALESCE($7, b.notes),
			    hold_token_hash = NULL,
			    created_at = GREATEST(b.created_at, NOW() - make_interval(secs => $9))
			WHERE b.id = $1
			  AND b.hold_token_hash = $2
			  AND b.status = 'pending'
			  AND b.created_at > NOW() - make_interval(secs => $8)
			  AND `+untouchedHold+`
			RETURNING upper(b.span), b.created_at`,
			data.UUIDToPg(b.ID), tokenHash,
			data.TextToPg(first), data.TextToPg(last), data.TextToPg(phone), data.TextToPg(email),
			data.TextToPg(b.Notes), m.PaymentExpiry.Seconds(), shift.Seconds(),
		).Scan(&endsAt, &createdAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrHoldNotFound
			}
			return fmt.Errorf("bookings: convert slot hold: %w", err)
		}
		b.CreatedAt = data.PgToTime(createdAt)

		linkToken, err := MintLinkToken(ctx, tx, b.ID, endsAt.Time.Add(m.LinkTokenBuffer))
		if err != nil {
			return fmt.Errorf("mint booking link token: %w", err)
		}
		b.LinkToken = linkToken
		return nil
	})
}

// ReleaseHold ends the live untouched hold a token names and reports the slot
// it freed, or released false when there is none (unknown, already released,
// converted, paid, run out). It is safe to call any number of times.
//
// The hold is cancelled, not deleted, and stays until its cooldown is over
// (DeleteExpiredHolds removes it then): it is the record that this address
// already spent part of a window on these hours, which a hold taken on them
// again continues rather than restarting (ownHoldClock). An untouched hold
// names nobody, took no money and was never shown to the owner
// (slotguard.UnpaidPublicBooking), and no audit entry is written for one.
func (m *Store) ReleaseHold(ctx context.Context, tokenHash []byte) (ReleasedHold, bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var (
		complexID pgtype.UUID
		day       pgtype.Date
		out       ReleasedHold
	)
	err := m.DB.QueryRow(ctx, `
		UPDATE bookings b
		SET status = 'cancelled'
		FROM complexes c
		WHERE c.id = b.complex_id
		  AND b.hold_token_hash = $1
		  AND b.status = 'pending'
		  AND `+untouchedHold+`
		RETURNING b.complex_id, c.name, c.slug, b.date`, tokenHash).Scan(&complexID, &out.ComplexName, &out.ComplexSlug, &day)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ReleasedHold{}, false, nil
		}
		return ReleasedHold{}, false, fmt.Errorf("bookings: release slot hold: %w", err)
	}
	out.ComplexID = data.PgToUUID(complexID)
	out.Day = data.PgToDate(day)
	return out, true, nil
}

// ReleasedHold is the slot a deleted hold freed: enough for the caller to tell
// the venue's grid and hand the day to the waitlist (waitlist.Freed).
type ReleasedHold struct {
	ComplexID   uuid.UUID
	ComplexName string
	ComplexSlug string
	Day         time.Time
}

// DeleteExpiredHolds ends every untouched hold older than expiry, the payment
// window it held its slot for, and returns the slot each one freed so the
// caller can tell the grids and the waitlist.
//
// A hold that ran out is cancelled first and kept through HoldCooldown, as the
// record ownHoldClock reads; an untouched hold older than expiry plus the
// cooldown is deleted, whatever its status. One that was still pending when it
// is deleted (the sweep was not running) is reported as freed too.
func (m *Store) DeleteExpiredHolds(ctx context.Context, expiry time.Duration) ([]ReleasedHold, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		WITH purged AS (
			DELETE FROM bookings b
			WHERE b.created_at < NOW() - make_interval(secs => $1::float8 + $2::float8)
			  AND `+untouchedHold+`
			RETURNING b.complex_id, b.date, b.status
		), lapsed AS (
			UPDATE bookings b
			SET status = 'cancelled'
			WHERE b.status = 'pending'
			  AND b.created_at < NOW() - make_interval(secs => $1)
			  AND b.created_at >= NOW() - make_interval(secs => $1::float8 + $2::float8)
			  AND `+untouchedHold+`
			RETURNING b.complex_id, b.date
		), freed AS (
			SELECT complex_id, date FROM purged WHERE status = 'pending'
			UNION ALL
			SELECT complex_id, date FROM lapsed
		)
		SELECT f.complex_id, c.name, c.slug, f.date
		FROM freed f
		JOIN complexes c ON c.id = f.complex_id`, expiry.Seconds(), HoldCooldown.Seconds())
	if err != nil {
		return nil, fmt.Errorf("bookings: delete expired slot holds: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ReleasedHold, error) {
		var (
			complexID pgtype.UUID
			day       pgtype.Date
			h         ReleasedHold
		)
		if err := row.Scan(&complexID, &h.ComplexName, &h.ComplexSlug, &day); err != nil {
			return h, fmt.Errorf("scan freed hold: %w", err)
		}
		h.ComplexID = data.PgToUUID(complexID)
		h.Day = data.PgToDate(day)
		return h, nil
	})
	if err != nil {
		return nil, fmt.Errorf("bookings: delete expired slot holds: %w", err)
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

// HoldCooldown is how long an address waits before it may hold court hours
// again after its own hold on them ran out unpaid. Without it a hold needs
// nothing but an address, and one address could keep a slot off sale for ever
// by taking it again each time its window ended.
const HoldCooldown = 10 * time.Minute

// MinPaymentWindow is the least time a visitor has to pay once they submit
// their details: converting a hold with less than this left extends it to this
// (ConvertHold), so nobody is sent to a MercadoPago checkout about to close.
const MinPaymentWindow = 10 * time.Minute

// HoldCooldownError refuses a hold on hours the same address held, unpaid,
// for a whole window that ended less than HoldCooldown ago. Until is when the
// address may hold them again.
type HoldCooldownError struct{ Until time.Time }

func (e *HoldCooldownError) Error() string {
	return "bookings: these hours were just held from this address, try again at " + e.Until.UTC().Format(time.RFC3339)
}

// HoldBlockedError refuses an owner's booking over hours a visitor is booking
// online right now. FreesAt is when the visitor's window ends if they do not
// pay. It is ErrSlotUnavailable to every caller that asks with errors.Is.
type HoldBlockedError struct{ FreesAt time.Time }

func (e *HoldBlockedError) Error() string {
	return "bookings: the slot is being booked online until " + e.FreesAt.UTC().Format(time.RFC3339)
}

func (e *HoldBlockedError) Unwrap() error { return ErrSlotUnavailable }

// liveOnlineHold is the SQL predicate, over the bookings row aliased b, for a
// storefront booking still inside its payment window (slotguard.LiveOnlineHold,
// shared with every other owner write a live hold refuses).
const liveOnlineHold = slotguard.LiveOnlineHold

// LiveHold is one stretch of a court a visitor is booking online: what the
// owner's grid draws, and nothing that names the visitor (no address, no
// token, no contact).
type LiveHold struct {
	CourtID         uuid.UUID
	Date            time.Time
	StartTime       string
	DurationMinutes int
	StartsAt        time.Time
	EndsAt          time.Time
	// FreesAt is when the hold ends if it is not paid.
	FreesAt time.Time
}

// LiveHolds lists the online holds on a complex's courts that occupy the local
// day, earliest first: the hours the owner's grid shows as being booked online.
func (m *Store) LiveHolds(ctx context.Context, complexID uuid.UUID, day time.Time) ([]LiveHold, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT b.court_id, b.date, to_char(b.start_time, 'HH24:MI'), b.duration_minutes,
		       lower(b.span), upper(b.span), b.created_at + make_interval(secs => $3)
		FROM bookings b
		WHERE b.complex_id = $1
		  AND b.span && local_day($2::date)
		  AND `+fmt.Sprintf(liveOnlineHold, 3)+`
		ORDER BY lower(b.span), b.court_id`,
		data.UUIDToPg(complexID), data.DateToPg(day), m.PaymentExpiry.Seconds())
	if err != nil {
		return nil, fmt.Errorf("bookings: list live holds: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (LiveHold, error) {
		var (
			courtID  pgtype.UUID
			date     pgtype.Date
			h        LiveHold
			duration int32
		)
		if err := row.Scan(&courtID, &date, &h.StartTime, &duration, &h.StartsAt, &h.EndsAt, &h.FreesAt); err != nil {
			return h, fmt.Errorf("scan live hold: %w", err)
		}
		h.CourtID = data.PgToUUID(courtID)
		h.Date = data.PgToDate(date)
		h.DurationMinutes = int(duration)
		return h, nil
	})
	if err != nil {
		return nil, fmt.Errorf("bookings: list live holds: %w", err)
	}
	return out, nil
}

// liveHoldFreesAt reports when the last of the live online holds overlapping
// the hours b wants on its court frees up, and whether there is one. The
// caller holds the court-day lock, so the answer cannot change under it.
func liveHoldFreesAt(ctx context.Context, tx pgx.Tx, b *Booking, window time.Duration) (time.Time, bool, error) {
	return slotguard.LiveHoldFreesAt(ctx, tx, []uuid.UUID{b.CourtID}, b.Date, b.StartTime, b.DurationMinutes, window)
}

// ownHoldClock answers, for a new hold on b's hours from ip, the window the
// same address already spent on them: the earliest start among its own unpaid
// storefront bookings on that court overlapping those hours whose window ended
// (ran out, or was let go) less than HoldCooldown ago.
//
// found is false when there is none, and the new hold starts a window of its
// own. Otherwise, while that earliest window has not run out (live), the new
// hold continues it — letting a slot go and picking it again does not restart
// the clock — and once it has, the address waits until cooldownUntil.
//
// A booking that was paid is never counted: payment clears its address.
func ownHoldClock(ctx context.Context, tx pgx.Tx, b *Booking, ip string, window time.Duration) (started time.Time, live, found bool, cooldownUntil time.Time, err error) {
	var startedAt, until pgtype.Timestamptz
	err = tx.QueryRow(ctx, `
		SELECT MIN(x.created_at), COALESCE(MIN(x.created_at) + make_interval(secs => $6) > NOW(), false),
		       MAX(x.ended_at) + make_interval(secs => $7)
		FROM (
			SELECT b.created_at,
			       CASE WHEN b.status = 'cancelled'
			            THEN LEAST(b.updated_at, b.created_at + make_interval(secs => $6))
			            ELSE b.created_at + make_interval(secs => $6)
			       END AS ended_at
			FROM bookings b
			WHERE b.hold_ip = $1
			  AND b.court_id = $2
			  AND b.span && tstzrange(
			        booking_starts_at($3, $4),
			        booking_starts_at($3, $4) + make_interval(mins => $5),
			        '[)')
			  AND b.created_by IS NULL
			  AND b.collection_status = 'unpaid'
			  AND b.client_id IS NULL
			  AND (b.status = 'cancelled'
			       OR (b.status = 'pending' AND b.created_at + make_interval(secs => $6) <= NOW()))
		) x
		WHERE x.ended_at > NOW() - make_interval(secs => $7)`,
		ip, data.UUIDToPg(b.CourtID), data.DateToPg(b.Date), data.TimeStrToPg(b.StartTime),
		b.DurationMinutes, window.Seconds(), HoldCooldown.Seconds(),
	).Scan(&startedAt, &live, &until)
	if err != nil {
		return time.Time{}, false, false, time.Time{}, fmt.Errorf("read the address's earlier holds: %w", err)
	}
	return startedAt.Time, live, startedAt.Valid, until.Time, nil
}
