package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/crypto"
	"github.com/stodulski/vibe-server/internal/data"
	slotguard "github.com/stodulski/vibe-server/internal/data/slotguard"
	"github.com/stodulski/vibe-server/internal/db"
	"github.com/stodulski/vibe-server/internal/mpcred"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// The two vocabularies bookings.payment_status was split into. They
// are text + CHECK in the database rather than a native enum — see the
// bookings section of db/migrations/001_init.sql for why — so nothing but these
// the column. Use them.
// They are spelled *Status* rather than the shorter CollectionUnpaid /
// RefundNone because RefundResult (internal/payments/store/refunds.go) already owns the
// short names, and its "none" answers a different question — "what did this
// cancellation do about the money" rather than "where does this booking's
// give-back stand".
const (
	// CollectionStatusUnpaid: nothing has been collected for this booking yet.
	CollectionStatusUnpaid = "unpaid"
	// CollectionStatusDepositPaid: part of the price is in, the balance is not.
	CollectionStatusDepositPaid = "deposit_paid"
	// CollectionStatusFullyPaid: the whole price has been collected.
	CollectionStatusFullyPaid = "fully_paid"

	// RefundStatusNone: no money is owed back.
	RefundStatusNone = "none"
	// RefundStatusPending: a refund claim is in flight and has not landed.
	RefundStatusPending = "pending"
	// RefundStatusPartial: the provider-backed rows are back and a cash or
	// transfer balance is still owed by hand. Only RecordManualRefund moves a
	// booking off this value — see the payment_status enum in db/migrations/001_init.sql for the state it names.
	RefundStatusPartial = "partial"
	// RefundStatusFull: everything that was collected has been given back.
	RefundStatusFull = "full"
)

// Booking represents a reserved court slot for a client.
type Booking struct {
	ID        uuid.UUID `json:"id"`
	ComplexID uuid.UUID `json:"complex_id"`
	CourtID   uuid.UUID `json:"court_id"`
	ClientID  uuid.UUID `json:"client_id"`
	// StartsAt and EndsAt are the booking's span, read straight off the
	// bookings.span column rather than rebuilt from the two fields below.
	// They are what a client should read: a time of day cannot say which day it
	// belongs to once a booking may run past midnight, and Date/StartTime
	// remain only until every reader has moved.
	//
	// There is no EndTime beside them. bookings.end_time was dropped by
	// the schema: it stored what the clock would read, so a 23:00
	// booking of two hours carried '01:00' with nothing saying which 01:00.
	// EndsAt is that same end with its date attached.
	StartsAt        time.Time `json:"starts_at"`
	EndsAt          time.Time `json:"ends_at"`
	Date            time.Time `json:"date"`
	StartTime       string    `json:"start_time"`
	DurationMinutes int       `json:"duration_minutes"`
	Price           int       `json:"price"`
	DepositAmount   int       `json:"deposit_amount"`
	Status          string    `json:"status"`
	// CollectionStatus and RefundStatus are the two axes payment_status was split
	// out of the single payment_status enum: how much of the booking's price
	// has been collected, and where the give-back stands. They are
	// independent — a booking that took only a deposit and is being refunded
	// reads (deposit_paid, pending), which the one-column spelling could not
	// express at all.
	CollectionStatus string     `json:"collection_status"`
	RefundStatus     string     `json:"refund_status"`
	ReminderSent2h   bool       `json:"reminder_sent_2h"`
	Notes            *string    `json:"notes,omitempty"`
	CreatedBy        *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	// RefundIntentAt marks a cancellation that owes a refund, written by the
	// same UPDATE that cancels the booking and cleared inside ClaimRefund's own
	// committed transaction. It is internal bookkeeping for the reconciliation
	// sweep (refund-intent-durability spec), not a client-facing field.
	RefundIntentAt *time.Time `json:"-"`
	// LinkToken is the plaintext access token InsertSafe (or, for the
	// webhook-confirmation path, internal/payments/process.go) just minted for
	// this booking. In-memory only, set exactly once by the mint that produced
	// it — the booking_link_tokens table stores only its hash, so this is the
	// one place in the process the plaintext exists after the mint call
	// returns. Never persisted on the bookings row itself.
	LinkToken string `json:"-"`

	// Enrichment fields populated by JOINs in GetByComplex/GetByID.
	CourtName   string `json:"court_name"`
	ClientName  string `json:"client_name"`
	ClientPhone string `json:"client_phone"`
}

// CursorKey returns the (date, ID) pair used to build a pagination cursor for this booking.
func (b *Booking) CursorKey() (time.Time, uuid.UUID) { return b.Date, b.ID }

// BookedSpan is one stretch of time a court is already spoken for.
//
// Two instants rather than a date and two times of day. A booking may run into
// the following day, and once it can, a time of day no longer says which day it
// belongs to: "01:00" reads as earlier than "23:00" under exactly the
// comparison the overlap checks were built on, which is how a 23:00 booking
// came to be invisible to the guard meant to see it (the retired CHECK (start_time < end_time)).
//
// It replaced a TimeSlot struct that also carried CourtName, Price and an
// Available flag. The one query that produced it filled none of them — they
// were left from when this was serialized straight to the storefront, which it
// has not been for some time — and four fields that are always zero are an
// invitation to start reading them.
type BookedSpan struct {
	CourtID  uuid.UUID
	StartsAt time.Time
	EndsAt   time.Time
}

// MaxBookingHorizonDays bounds how far into the future a booking or a court's
// blocked slot may be dated.
//
// H-08: internal/courts.BlockSlot and internal/bookings.PublicBook — the two
// write paths an anonymous visitor can reach with no account — validated a
// date's lower bound (not in the past) but never its upper one. A booking
// dated "9999-12-31" was accepted with a real MercadoPago preference: it
// holds a slot forever and is never reaped, since cron's completeBookings
// only completes a booking whose end time has already passed. A block that
// far out is merely inert, but the booking half is a standing way for an
// anonymous caller to leave permanent rows behind, one request at a time.
//
// A year is generous for a real booking. It lives here, next to
// defaultPaymentExpiry below, rather than as a literal duplicated in two
// handlers in two different packages — so the two validators agree by
// construction, and raising it later is a one-line change rather than an
// audit of every date check in the codebase.
const MaxBookingHorizonDays = 365

// isSlotAlreadySold reports whether err is the database refusing to sell the
// same court hours twice.
//
// It is a function rather than the condition written at each insert because
// there are two inserts — the plain one and the locked one — and the pair
// answering differently is how a booking came to be accepted by one path and
// refused by another before.
func isSlotAlreadySold(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == data.SQLStateUniqueViolation || pgErr.Code == data.SQLStateExclusionViolation
}

// BookingColumns is the bookings-table half of every hand-written booking
// SELECT in this file. There were six, and the list was typed out in all six.
//
// It does not make adding a column free — the scan lists that consume it are
// still per-query and still positional — but it makes the SELECT side one edit
// instead of six, and it makes the six agree by construction rather than by
// six separate people remembering. The joined enrichment columns stay per
// query, because the six join different tables.
//
// starts_at and ends_at are read from the span rather than recomputed in Go
// from date + start_time + duration. The span's definition already lives in
// db/migrations/001_init.sql; a Go copy of that arithmetic would be a second definition
// that has to agree with it silently, which is the shape of defect this whole
// migration is unwinding.
const BookingColumns = `b.id, b.complex_id, b.court_id, b.client_id,
		       b.span,
		       b.date, b.start_time,
		       b.duration_minutes, b.price, b.deposit_amount, b.status,
		       b.collection_status, b.refund_status,
		       b.reminder_sent_2h, b.notes,
		       b.created_by, b.created_at, b.updated_at`

// Store implements BookingStore against PostgreSQL.
type Store struct {
	DB *data.DB
	Q  *db.Queries
	// PaymentExpiry is how long an unpaid public booking holds its slot, taken
	// from configuration by stores.New. See Config.PaymentExpiry.
	PaymentExpiry time.Duration
	// Keys opens the mp_access_token/mp_refresh_token columns enriched onto
	// CronBooking by scanCronBookings. See Config.Keys and crypto.Keyring —
	// a nil Keys errors rather than passing values through as plaintext.
	Keys *crypto.Keyring
	// LinkTokenBuffer is added to a booking's end time to compute the booking
	// link token InsertSafe mints for it. See Config.LinkTokenBuffer.
	LinkTokenBuffer time.Duration
}

// Insert creates a new booking inside a transaction, checking for slot overlap before committing.
//
// Test-only: it mints no booking link token. Every production insert path
// goes through InsertSafe, which does. Do not route a production path through
// Insert without adding a mint call — a booking committed here has no usable
// link on any of the three public routes.
func (m *Store) Insert(ctx context.Context, b *Booking) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbBooking, err := m.Q.InsertBooking(ctx, db.InsertBookingParams{
		ComplexID: data.UUIDToPg(b.ComplexID),
		CourtID:   data.UUIDToPg(b.CourtID),
		ClientID:  data.UUIDToPg(b.ClientID),
		Date:      data.DateToPg(b.Date),
		StartTime: data.TimeStrToPg(b.StartTime),
		//nolint:gosec // G115: DurationMinutes = slotDuration * SlotCount, and SlotCount is validated to 1..3 at
		// the handler (bookings_create.go); court.DurationMinutes is itself an int32 DB column. Far below int32 range.
		DurationMinutes: int32(b.DurationMinutes),
		//nolint:gosec // G115: Price is the sum of at most 3 court price rows (SlotCount validated 1..3) capped by
		// the courts price table (int32 DB column); realistic totals are far below int32 range.
		Price: int32(b.Price),
		//nolint:gosec // G115: DepositAmount is validated at the handler to be <= Price (bookings_create.go) or
		// derived from DepositPercentage validated <= 100; bounded by Price above, far below int32 range.
		DepositAmount:    int32(b.DepositAmount),
		Status:           db.BookingStatus(b.Status),
		CollectionStatus: b.CollectionStatus,
		RefundStatus:     b.RefundStatus,
		Notes:            data.TextToPg(b.Notes),
		CreatedBy:        data.UUIDPtrToPg(b.CreatedBy),
	})
	if err != nil {
		if isSlotAlreadySold(err) {
			return ErrDuplicateBooking
		}
		return err
	}

	b.ID = data.PgToUUID(dbBooking.ID)
	b.ReminderSent2h = dbBooking.ReminderSent2h
	b.CreatedAt = data.PgToTime(dbBooking.CreatedAt)
	b.UpdatedAt = data.PgToTime(dbBooking.UpdatedAt)
	// The span is generated by the database (bookings.span), so it is only
	// known after the insert. InsertSafe reads it back; this variant did not,
	// which left EndsAt at its zero value and made a link token minted from it
	// (EndsAt plus the buffer) expire in the year one.
	b.StartsAt = dbBooking.Span.Lower.Time
	b.EndsAt = dbBooking.Span.Upper.Time
	return nil
}

// InsertSafe inserts a booking inside a transaction with an advisory lock to
// prevent race conditions. It checks for collisions with existing bookings
// AND blocked slots atomically before inserting.
// (begin tx, lock/check overlap, insert, commit); matches this codebase's store method
// conventions (wrap sqlc queries, map to domain structs, translate PostgreSQL errors).
//
//nolint:funlen // single cohesive SQL-building-and-execution flow for one store operation
func (m *Store) InsertSafe(ctx context.Context, b *Booking) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	// RetryTx rather than WithTx: this transaction takes the court-day advisory
	// lock and then row locks (ReleaseStalePendingOverlaps, LockCourtLive), and
	// the confirmation path reaches the same two objects. The lock ORDER is what
	// keeps them from deadlocking and it stays the first line of defence — but
	// when PostgreSQL does break a cycle it aborts one side with 40P01, and
	// until now that side became a 500 for whoever was booking. Replaying the
	// whole transaction is what PostgreSQL's own answer to a deadlock assumes
	// the client will do. Nothing outside the transaction happens here, so a
	// second run duplicates nothing.
	return m.DB.RetryTx(ctx, data.DefaultTxAttempts, func(tx pgx.Tx, _ *db.Queries) error {
		return m.insertSafe(ctx, tx, b)
	})
}

// insertSafe is InsertSafe's body, inside the transaction.
//
//nolint:funlen // single cohesive SQL-building-and-execution flow for one store operation
func (m *Store) insertSafe(ctx context.Context, tx pgx.Tx, b *Booking) error {
	// Acquire the advisory lock for every local day these hours touch, to
	// serialize concurrent writers on the court. The payment confirmation path
	// and the blocked-slot write take the same lock (see slot_guard.go), so an
	// insert, a confirmation and a maintenance block competing for these hours
	// cannot both succeed.
	//
	// It is a range rather than b.Date because a booking may outlive its own
	// date: 23:00 plus two hours occupies the day after as well, and a block
	// filed on that day locks only that day. Locking one date left the two
	// serialized against nothing — see lockCourtDays.
	startAt, endAt := slotguard.LocalRange(b.Date, b.StartTime, time.Duration(b.DurationMinutes)*time.Minute)
	if err := slotguard.LockCourtDays(ctx, tx, b.CourtID, startAt, endAt); err != nil {
		return err
	}

	// H-22: read the court under a row lock, and read it AFTER the advisory
	// locks so the acquisition order here is always advisory-then-row.
	//
	// The court-day lock serializes this against the other writers that take
	// it — the confirmation path and the blocked-slot write — and courtstore.Store.
	// SoftDelete is not one of them. It has no day to key a lock on, so the
	// only object the two writers can both hold is the court row itself.
	// Without this read they contended on nothing: under READ COMMITTED each
	// transaction's own check was correct against its own snapshot, the delete
	// did not see this booking and this booking did not see the delete, and
	// both committed. The client kept hours on a court the owner had already
	// removed.
	//
	// FOR SHARE rather than FOR UPDATE because this is a reader of the court,
	// not a writer of it: two bookings on different days of the same court
	// must not queue behind each other, and FOR SHARE lets them both hold it
	// while still blocking the delete's FOR UPDATE.
	if err := slotguard.LockCourtLive(ctx, tx, b.CourtID); err != nil {
		return err
	}

	// Release the stale pending bookings this one would land on. GetBookedSlots
	// and SlotTaken both treat a public, unpaid booking older than the payment
	// expiry as no longer holding its slot, but bookings_no_overlapping_span
	// (bookings_no_overlapping_span) cannot ask the clock and still counts it, so the insert
	// below would die on the constraint — "duplicate booking" for a slot the
	// storefront had just shown as free — until the release cron got to it, up
	// to five minutes later. Cancelling them here, under the same court-day
	// lock, in the same transaction, is what the cron would have done; the
	// abandoned checkout is what the carve-out exists to override.
	if err := ReleaseStalePendingOverlaps(ctx, tx, b, m.PaymentExpiry); err != nil {
		return err
	}

	// Check collision with existing non-cancelled bookings (time range overlap).
	// Stale pending bookings (public, unpaid, older than the configured payment
	// expiry) are excluded — same logic as GetBookedSlots so the user never sees
	// a slot as "available" that InsertSafe would then reject. Nothing is excluded
	// from the insert itself, so uuid.Nil is passed as the row to ignore.
	hasCollision, err := SlotTaken(ctx, tx, b, m.PaymentExpiry, uuid.Nil)
	if err != nil {
		return err
	}
	if hasCollision {
		return ErrSlotUnavailable
	}

	// Check collision with blocked slots.
	//
	// The overlap is asked of `span`, the generated column blocked_slots gained
	// blocked_slots, against a candidate range built exactly the way SlotTaken
	// builds its own: the booking's start instant plus its duration. Both sides
	// are absolute instants, so neither can wrap and neither has to agree about
	// which day a time of day belongs to.
	//
	// It used to filter `date = $2` and compare times of day against the
	// booking's stored end_time. Both were wrong once a booking could cross
	// midnight (which a booking may now do): slots.Add wrapped at midnight, so a 23:00
	// booking of 120 minutes carried '01:00', `start_time < end_time` was
	// false against almost any block, and the `date` filter excluded the
	// following day's blocks before
	// the comparison even ran. A block the booking covered completely was
	// invisible to the only check that runs inside the transaction. The
	// reproduction is in the blocked_slots section of db/migrations/001_init.sql.
	//
	// Seeing the blocks that exist is not the same as there being no window in
	// which one appears. Under READ COMMITTED this SELECT sees only what was
	// committed before it ran, and no constraint refuses a booking and a block
	// that overlap — EXCLUDE is single-table. What closes that window is the
	// lock above and InsertBlockedSlot taking it too, asking the mirror
	// question on its own side.
	err = tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM blocked_slots
			WHERE court_id = $1
			  AND span && tstzrange(
			        booking_starts_at($2, $3),
			        booking_starts_at($2, $3) + make_interval(mins => $4),
			        '[)')
		)`,
		data.UUIDToPg(b.CourtID), data.DateToPg(b.Date),
		data.TimeStrToPg(b.StartTime), b.DurationMinutes,
	).Scan(&hasCollision)
	if err != nil {
		return fmt.Errorf("check blocked slots: %w", err)
	}
	if hasCollision {
		return ErrSlotUnavailable
	}

	// Insert the booking within the same transaction.
	var id pgtype.UUID
	var reminderSent2h bool
	var createdAt, updatedAt pgtype.Timestamptz
	err = tx.QueryRow(ctx, `
		INSERT INTO bookings (
			complex_id, court_id, client_id, date,
			start_time, duration_minutes,
			price, deposit_amount, status, collection_status, refund_status,
			notes, created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, reminder_sent_2h, created_at, updated_at, lower(span), upper(span)`,
		data.UUIDToPg(b.ComplexID), data.UUIDToPg(b.CourtID), data.UUIDToPg(b.ClientID),
		data.DateToPg(b.Date), data.TimeStrToPg(b.StartTime),
		//nolint:gosec // G115: same bound as Store.Insert above — DurationMinutes/Price/DepositAmount are
		// validated at the bookings_create.go handler (SlotCount 1..3, DepositAmount <= Price); far below int32 range.
		int32(b.DurationMinutes), int32(b.Price), int32(b.DepositAmount),
		db.BookingStatus(b.Status), b.CollectionStatus, b.RefundStatus,
		data.TextToPg(b.Notes), data.UUIDPtrToPg(b.CreatedBy),
	).Scan(&id, &reminderSent2h, &createdAt, &updatedAt, &b.StartsAt, &b.EndsAt)
	if err != nil {
		if isSlotAlreadySold(err) {
			return ErrDuplicateBooking
		}
		return err
	}

	b.ID = data.PgToUUID(id)
	// The two instants come back on whatever clock pgx decoded them into. Put
	// them on the product's, so a booking scanned here and one read back
	// through BookingFromDB serialize to the same offset — the API's
	// starts_at/ends_at are the only thing left saying which day the hours end
	// on, and "-03:00" is what makes that legible rather than merely correct.
	b.StartsAt = b.StartsAt.In(timezone.Argentina)
	b.EndsAt = b.EndsAt.In(timezone.Argentina)
	b.ReminderSent2h = reminderSent2h
	b.CreatedAt = data.PgToTime(createdAt)
	b.UpdatedAt = data.PgToTime(updatedAt)

	// Mint inside the same transaction the booking insert just committed to
	// (not yet — Commit is below): a crash between the two would strand a
	// booking with no usable link on any of its three public routes. A
	// failing mint rolls back the booking insert via the deferred
	// tx.Rollback above — no booking row commits without a token.
	linkToken, err := MintLinkToken(ctx, tx, b.ID, b.EndsAt.Add(m.LinkTokenBuffer))
	if err != nil {
		return fmt.Errorf("mint booking link token: %w", err)
	}
	b.LinkToken = linkToken
	return nil
}

// GetByID returns the booking with the given ID, or ErrRecordNotFound if none exists.
func (m *Store) GetByID(ctx context.Context, id uuid.UUID) (*Booking, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var b db.Booking
	var courtName, clientName, clientPhone string
	err := m.DB.QueryRow(ctx, `
		SELECT `+BookingColumns+`,
		       COALESCE(co.name, '') AS court_name,
		       COALESCE(cl.first_name || ' ' || cl.last_name, '') AS client_name,
		       COALESCE(cl.phone, '') AS client_phone
		FROM bookings b
		LEFT JOIN courts co ON co.id = b.court_id
		LEFT JOIN clients cl ON cl.id = b.client_id
		WHERE b.id = $1
		LIMIT 1`, data.UUIDToPg(id)).Scan(
		&b.ID, &b.ComplexID, &b.CourtID, &b.ClientID,
		&b.Span,
		&b.Date, &b.StartTime, &b.DurationMinutes,
		&b.Price, &b.DepositAmount, &b.Status,
		&b.CollectionStatus, &b.RefundStatus,
		&b.ReminderSent2h,
		&b.Notes, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt,
		&courtName, &clientName, &clientPhone,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	booking := BookingFromDB(b)
	booking.CourtName = courtName
	booking.ClientName = clientName
	booking.ClientPhone = clientPhone
	return booking, nil
}

// GetByComplex returns paginated bookings for a complex within a date range.
// Uses raw SQL because the SQLC-generated params have incorrect types for
// the cursor-based (date, id) row comparison.
// (build filtered/paginated query, execute, scan rows, build metadata); matches this codebase's
// store method conventions (wrap sqlc queries, map to domain structs).
//
//nolint:funlen // single cohesive SQL-building-and-execution flow for one store operation
func (m *Store) GetByComplex(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time, filters data.Filters) ([]*Booking, data.Metadata, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	cursorDate, cursorID, err := filters.ParseCursor()
	if err != nil {
		return nil, data.Metadata{}, err
	}

	if cursorDate.IsZero() {
		cursorDate = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 20
	}

	rows, err := m.DB.Query(ctx, `
		SELECT `+BookingColumns+`,
		       COALESCE(co.name, '') AS court_name,
		       COALESCE(cl.first_name || ' ' || cl.last_name, '') AS client_name,
		       COALESCE(cl.phone, '') AS client_phone
		FROM bookings b
		LEFT JOIN courts co ON co.id = b.court_id
		LEFT JOIN clients cl ON cl.id = b.client_id
		WHERE b.complex_id = $1
		  -- Overlapping the days asked for, not starting on them: a booking
		  -- that runs 23:00 to 01:00 occupies both, and the grid for the second
		  -- one has to draw the hours it is still holding. See
		  -- local_day() in db/migrations/001_init.sql for why "occupies day X" beat "started on day X".
		  AND b.span && tstzrange(lower(local_day($2)), upper(local_day($3)), '[)')
		  AND (b.date, b.id) > ($4, $5)
		ORDER BY b.date ASC, b.id ASC
		LIMIT $6`,
		data.UUIDToPg(complexID),
		data.DateToPg(dateFrom),
		data.DateToPg(dateTo),
		data.DateToPg(cursorDate),
		data.UUIDToPg(cursorID),
		int32(limit+1),
	)
	if err != nil {
		return nil, data.Metadata{}, err
	}
	defer rows.Close()

	bookings := make([]*Booking, 0, limit+1)
	for rows.Next() {
		var b db.Booking
		var courtName, clientName, clientPhone string
		err := rows.Scan(
			&b.ID, &b.ComplexID, &b.CourtID, &b.ClientID,
			&b.Span,
			&b.Date, &b.StartTime, &b.DurationMinutes,
			&b.Price, &b.DepositAmount, &b.Status,
			&b.CollectionStatus, &b.RefundStatus,
			&b.ReminderSent2h,
			&b.Notes, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt,
			&courtName, &clientName, &clientPhone,
		)
		if err != nil {
			return nil, data.Metadata{}, err
		}
		booking := BookingFromDB(b)
		booking.CourtName = courtName
		booking.ClientName = clientName
		booking.ClientPhone = clientPhone
		bookings = append(bookings, booking)
	}
	if err := rows.Err(); err != nil {
		return nil, data.Metadata{}, err
	}

	bookings, meta := data.TrimPage(bookings, limit, data.BuildNextCursor)

	return bookings, meta, nil
}

// Update persists changes to an existing booking.
//
// It carries no optimistic-concurrency predicate. It used to match on a
// `version` counter that three of the four writers of this table bumped without
// checking, or did not touch at all, so a stale version still matched and the
// check never fired on the one race it was supposed to catch; the counter's removal
// removed the column and states the measurement. What refuses an illegitimate
// concurrent write is the bookings_forbid_status_reversal trigger from
// the schema, which every writer goes through — including the bulk sweeps
// and psql — and which returns SQLSTATE 23514 with the rule's name.
//
// Zero rows now means one thing only: no booking with that id. It comes back as
// ErrRecordNotFound rather than the removed ErrEditConflict, which is the same
// answer complexstore.Store.Update, courtstore.Store.Update and clientstore.Store.Update give for
// a row that vanished under an edit.
func (m *Store) Update(ctx context.Context, b *Booking) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbBooking, err := m.Q.UpdateBooking(ctx, db.UpdateBookingParams{
		ID:               data.UUIDToPg(b.ID),
		Status:           db.BookingStatus(b.Status),
		CollectionStatus: b.CollectionStatus,
		RefundStatus:     b.RefundStatus,
		Notes:            data.TextToPg(b.Notes),
		//nolint:gosec // G115: DepositAmount is accumulated from validated per-payment amounts (bookings_actions.go)
		// bounded to the same currency-amount range as Price; far below int32 range.
		DepositAmount:  int32(b.DepositAmount),
		RefundIntentAt: data.TimePtrToPg(b.RefundIntentAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return err
	}

	b.UpdatedAt = data.PgToTime(dbBooking.UpdatedAt)
	return nil
}

// GetBookedSlotsByCourtIDs returns the time slots already booked across multiple courts on the given date.
//
// "Booked" here is the canonical predicate — see db/queries/bookings.sql. It
// used to have a sibling, GetAvailableSlots, which ran the same query for a
// single court, returned booked slots under an inverted name, and had no
// caller. It is gone.
func (m *Store) GetBookedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]BookedSpan, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if len(courtIDs) == 0 {
		return nil, nil
	}
	dbSlots, err := m.Q.GetBookedSlots(ctx, db.GetBookedSlotsParams{
		Column1: data.UUIDSliceToPg(courtIDs),
		Day:     data.DateToPg(date),
		// Same configured hold SlotTaken uses (see slot_guard.go), so the
		// storefront and the insert-time collision guard agree on when a
		// stale pending booking stops blocking a slot.
		PaymentExpirySeconds: m.PaymentExpiry.Seconds(),
	})
	if err != nil {
		return nil, err
	}

	result := make([]BookedSpan, len(dbSlots))
	for i, s := range dbSlots {
		result[i] = BookedSpan{
			CourtID:  data.PgToUUID(s.CourtID),
			StartsAt: s.StartsAt.Time,
			EndsAt:   s.EndsAt.Time,
		}
	}
	return result, nil
}

// GetForReminder2h returns confirmed bookings that are due a 2-hour reminder and have not received one yet.
//
// now is the caller's clock: see GetForReminder2hEnriched, whose window this
// one must agree with.
func (m *Store) GetForReminder2h(ctx context.Context, now time.Time) ([]*Booking, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbBookings, err := m.Q.GetBookingsForReminder2h(ctx, data.TimeToPg(now))
	if err != nil {
		return nil, err
	}
	return bookingsFromDB(dbBookings), nil
}

// MarkReminderSent2h flags the booking's 2-hour reminder as already sent.
func (m *Store) MarkReminderSent2h(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return m.Q.MarkReminderSent2h(ctx, data.UUIDToPg(id))
}

// BookingFromDB maps one bookings row onto the domain Booking, including the
// start and end instants the generated span carries.
func BookingFromDB(b db.Booking) *Booking {
	return &Booking{
		ID:               data.PgToUUID(b.ID),
		ComplexID:        data.PgToUUID(b.ComplexID),
		CourtID:          data.PgToUUID(b.CourtID),
		ClientID:         data.PgToUUID(b.ClientID),
		StartsAt:         b.Span.Lower.Time.In(timezone.Argentina),
		EndsAt:           b.Span.Upper.Time.In(timezone.Argentina),
		Date:             data.PgToDate(b.Date),
		StartTime:        data.PgToTimeStr(b.StartTime),
		DurationMinutes:  int(b.DurationMinutes),
		Price:            int(b.Price),
		DepositAmount:    int(b.DepositAmount),
		Status:           string(b.Status),
		CollectionStatus: b.CollectionStatus,
		RefundStatus:     b.RefundStatus,
		ReminderSent2h:   b.ReminderSent2h,
		Notes:            data.PgToTextPtr(b.Notes),
		CreatedBy:        data.PgToUUIDPtr(b.CreatedBy),
		CreatedAt:        data.PgToTime(b.CreatedAt),
		UpdatedAt:        data.PgToTime(b.UpdatedAt),
		RefundIntentAt:   data.PgToTimePtr(b.RefundIntentAt),
	}
}

func bookingsFromDB(dbBookings []db.Booking) []*Booking {
	result := make([]*Booking, len(dbBookings))
	for i, b := range dbBookings {
		result[i] = BookingFromDB(b)
	}
	return result
}

// ─── Dashboard / Stats ───

// DashboardStats holds the aggregate booking and revenue counters shown on the operator dashboard.
type DashboardStats struct {
	TodayBookings      int `json:"today_bookings"`
	TodayBookedMinutes int `json:"today_booked_minutes"`
	TodayRevenue       int `json:"today_revenue"`
	YesterdayBookings  int `json:"yesterday_bookings"`
	YesterdayRevenue   int `json:"yesterday_revenue"`
	WeeklyRevenue      int `json:"weekly_revenue"`
	MonthlyRevenue     int `json:"monthly_revenue"`
	PendingBookings    int `json:"pending_bookings"`
}

// RevenueDataPoint holds the total revenue collected on a single day.
type RevenueDataPoint struct {
	Date   string `json:"date"`
	Amount int    `json:"amount"`
}

// OccupancyDataPoint holds the booking count for one day-of-week/hour bucket, used to render an occupancy heatmap.
type OccupancyDataPoint struct {
	DayOfWeek    int `json:"day_of_week"`
	Hour         int `json:"hour"`
	BookingCount int `json:"booking_count"`
}

// GetDashboardStats computes today's and yesterday's booking/revenue counters, plus weekly, monthly and pending totals, for the complex.
//
// The three booking counters ask releasedBookingStatuses, because they count
// court time and a resold no_show is one hour, not two. TodayBookedMinutes is
// the numerator the handler turns into occupancy_rate, so the over-count landed
// straight in a percentage. The revenue counters are unaffected either way:
// every figure here comes from payments.amount filtered on payments.status, and
// no revenue in this query is derived from a booking's status.
func (m *Store) GetDashboardStats(ctx context.Context, complexID uuid.UUID, today time.Time) (*DashboardStats, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var s DashboardStats
	err := m.DB.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM bookings WHERE complex_id = $1 AND date = $2 AND status NOT IN `+slotguard.ReleasedBookingStatuses+`),
			(SELECT COALESCE(SUM(duration_minutes), 0) FROM bookings WHERE complex_id = $1 AND date = $2 AND status NOT IN `+slotguard.ReleasedBookingStatuses+`),
			(SELECT COALESCE(SUM(p.amount), 0) FROM payments p WHERE p.complex_id = $1 AND p.status != 'refunded'
			    AND (p.created_at AT TIME ZONE 'America/Argentina/Buenos_Aires')::date = $2),
			(SELECT COUNT(*) FROM bookings WHERE complex_id = $1 AND date = $2 - INTERVAL '1 day' AND status NOT IN `+slotguard.ReleasedBookingStatuses+`),
			(SELECT COALESCE(SUM(p.amount), 0) FROM payments p WHERE p.complex_id = $1 AND p.status != 'refunded'
			    AND (p.created_at AT TIME ZONE 'America/Argentina/Buenos_Aires')::date = $2 - INTERVAL '1 day'),
			(SELECT COALESCE(SUM(p.amount), 0) FROM payments p WHERE p.complex_id = $1 AND p.status != 'refunded'
			    AND (p.created_at AT TIME ZONE 'America/Argentina/Buenos_Aires')::date BETWEEN $2 - INTERVAL '6 days' AND $2),
			(SELECT COALESCE(SUM(p.amount), 0) FROM payments p WHERE p.complex_id = $1 AND p.status != 'refunded'
			    AND (p.created_at AT TIME ZONE 'America/Argentina/Buenos_Aires')::date BETWEEN $2 - INTERVAL '29 days' AND $2),
			(SELECT COUNT(*) FROM bookings WHERE complex_id = $1 AND date BETWEEN $2 AND $2 + INTERVAL '30 days' AND status = 'pending')
	`, data.UUIDToPg(complexID), data.DateToPg(today)).Scan(
		&s.TodayBookings, &s.TodayBookedMinutes, &s.TodayRevenue,
		&s.YesterdayBookings, &s.YesterdayRevenue,
		&s.WeeklyRevenue, &s.MonthlyRevenue,
		&s.PendingBookings,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetUpcomingToday returns the next confirmed bookings that will use a court
// today, starting at or after nowTime.
//
// It deliberately asks for 'confirmed' only, not releasedBookingStatuses
// (cancelled/no_show excluded, everything else in): a 'pending' booking is
// still waiting on the client to pay online, not on the owner, so it has
// nothing for the owner to act on here — it resolves itself within the
// payment-expiry window (payment confirms it, or it releases the slot).
// Showing it in the "next up" list only crowded out bookings that are
// actually happening.
func (m *Store) GetUpcomingToday(ctx context.Context, complexID uuid.UUID, today time.Time, nowTime string, limit int) ([]*Booking, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT `+BookingColumns+`,
		       COALESCE(co.name, '') AS court_name,
		       COALESCE(cl.first_name || ' ' || cl.last_name, '') AS client_name,
		       COALESCE(cl.phone, '') AS client_phone
		FROM bookings b
		LEFT JOIN courts co ON co.id = b.court_id
		LEFT JOIN clients cl ON cl.id = b.client_id
		WHERE b.complex_id = $1
		  AND b.date = $2
		  AND b.start_time >= $3
		  AND b.status = 'confirmed'
		ORDER BY b.start_time ASC
		LIMIT $4`,
		//nolint:gosec // G115: limit is a hardcoded literal (10) at the only call site (stats.go), never user input.
		data.UUIDToPg(complexID), data.DateToPg(today), data.TimeStrToPg(nowTime), int32(limit),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBookingsWithJoins(rows)
}

// GetRevenueByDay returns one RevenueDataPoint per day in [from, to], including zero-revenue days.
func (m *Store) GetRevenueByDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]RevenueDataPoint, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT (p.created_at AT TIME ZONE 'America/Argentina/Buenos_Aires')::date AS pay_date, COALESCE(SUM(p.amount), 0)::bigint AS amount
		FROM payments p
		WHERE p.complex_id = $1 AND p.status != 'refunded'
		  AND (p.created_at AT TIME ZONE 'America/Argentina/Buenos_Aires')::date BETWEEN $2 AND $3
		GROUP BY pay_date
		ORDER BY pay_date ASC`,
		data.UUIDToPg(complexID), data.DateToPg(from), data.DateToPg(to),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dataMap := make(map[string]int)
	for rows.Next() {
		var d time.Time
		var amount int
		if err := rows.Scan(&d, &amount); err != nil {
			return nil, err
		}
		dataMap[d.Format("2006-01-02")] = amount
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fill in zero-days.
	var result []RevenueDataPoint
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		result = append(result, RevenueDataPoint{Date: key, Amount: dataMap[key]})
	}
	return result, nil
}

// GetOccupancyByHourDay returns booking counts grouped by ISO day-of-week and hour for the given date range.
//
// The count is court-hours, so it uses releasedBookingStatuses: the schema
// lets a no_show and its resale share one (court, date, start_time), and
// counting both made a cell read twice its true occupancy. The handler divides
// by courts x weeks and clamps at 100, which turned the over-count into a
// plausible hot cell instead of an obviously impossible number.
func (m *Store) GetOccupancyByHourDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]OccupancyDataPoint, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT EXTRACT(ISODOW FROM date)::int AS dow,
		       (EXTRACT(HOUR FROM start_time))::int AS hr,
		       COUNT(*)::int AS cnt
		FROM bookings
		WHERE complex_id = $1
		  AND date BETWEEN $2 AND $3
		  AND status NOT IN `+slotguard.ReleasedBookingStatuses+`
		GROUP BY dow, hr
		ORDER BY dow, hr`,
		data.UUIDToPg(complexID), data.DateToPg(from), data.DateToPg(to),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]OccupancyDataPoint, 0, 7*16)
	for rows.Next() {
		var dp OccupancyDataPoint
		if err := rows.Scan(&dp.DayOfWeek, &dp.Hour, &dp.BookingCount); err != nil {
			return nil, err
		}
		result = append(result, dp)
	}
	return result, rows.Err()
}

// GetByClient returns the client's most recent bookings in the complex, within a rolling window of the past year and next 30 days.
func (m *Store) GetByClient(ctx context.Context, complexID, clientID uuid.UUID, limit int) ([]*Booking, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT `+BookingColumns+`,
		       COALESCE(co.name, '') AS court_name,
		       '' AS client_name,
		       '' AS client_phone
		FROM bookings b
		LEFT JOIN courts co ON co.id = b.court_id
		WHERE b.complex_id = $1
		  AND b.client_id = $2
		  AND b.date BETWEEN CURRENT_DATE - INTERVAL '365 days' AND CURRENT_DATE + INTERVAL '30 days'
		ORDER BY b.date DESC, b.start_time DESC
		LIMIT $3`,
		//nolint:gosec // G115: limit is a hardcoded literal (20) at the only call site (clients.go), never user input.
		data.UUIDToPg(complexID), data.UUIDToPg(clientID), int32(limit),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBookingsWithJoins(rows)
}

// scanBookingsWithJoins scans rows that include court_name, client_name, client_phone JOINs.
func scanBookingsWithJoins(rows pgx.Rows) ([]*Booking, error) {
	bookings := make([]*Booking, 0, 20)
	for rows.Next() {
		var b db.Booking
		var courtName, clientName, clientPhone string
		err := rows.Scan(
			&b.ID, &b.ComplexID, &b.CourtID, &b.ClientID,
			&b.Span,
			&b.Date, &b.StartTime, &b.DurationMinutes,
			&b.Price, &b.DepositAmount, &b.Status,
			&b.CollectionStatus, &b.RefundStatus,
			&b.ReminderSent2h,
			&b.Notes, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt,
			&courtName, &clientName, &clientPhone,
		)
		if err != nil {
			return nil, err
		}
		booking := BookingFromDB(b)
		booking.CourtName = courtName
		booking.ClientName = clientName
		booking.ClientPhone = clientPhone
		bookings = append(bookings, booking)
	}
	return bookings, rows.Err()
}

// CronBooking holds a Booking enriched with related client, complex, court and
// MercadoPago data needed by cron jobs, avoiding N+1 queries per booking.
type CronBooking struct {
	Booking
	ClientPhone string
	ClientEmail string
	ComplexName string
	CourtName   string
	// ComplexSlug, ComplexAddress, ComplexCity, ComplexLatitude and
	// ComplexLongitude are what the cron's own notifications need and cannot
	// look up: the reminder tells a client where to go and offers a map, and
	// the expiry sweep's cancellation offers the venue's booking page. Every
	// one of those is a per-booking query the enriched read exists to avoid.
	ComplexSlug       string
	ComplexAddress    string
	ComplexCity       string
	ComplexLatitude   *float64
	ComplexLongitude  *float64
	mpAccessToken     *string
	mpAccessTokenErr  error
	mpRefreshToken    *string
	mpRefreshTokenErr error
	CancellationHours int
}

// GetForReminder2hEnriched returns confirmed bookings starting within the next 2 hours that have
// not been sent a reminder yet, enriched with client, complex and court data for the notification.
//
// The window is (now, now + 2h] over booking_starts_at (db/migrations/001_init.sql) — the
// instant the game begins — and not over start_time, which is a time of day and
// therefore wraps. The predicate this replaced asked
// `start_time <= (now::time + INTERVAL '2 hours') AND start_time > now::time`,
// which at 23:00 reads `<= 01:00 AND > 23:00`: no row can satisfy it, so from
// 22:00 local onwards nobody was reminded at all. Its `date = today` companion
// dropped tomorrow's 00:30 game for the same reason from the other side. Both
// disappear once the comparison is between two instants, which is why this
// query and GetBookingsForReminder2h in db/queries/bookings.sql now share one
// definition of when a booking starts instead of each rebuilding it.
//
// now is a parameter so the clock has one owner. It also has to be, to be
// testable: a reminder window that only misbehaves between 22:00 and midnight
// cannot be pinned by a suite that reads the database's own NOW().
//
// The created_at floor is compared against the same instant, in absolute time.
// It used to read `b.created_at < NOW() AT TIME ZONE '<zone>' - INTERVAL '2 hours'`,
// and that AT TIME ZONE strips the offset off a timestamptz to produce a naked
// local timestamp, which the comparison against timestamptz created_at then
// re-reads in the session's TimeZone — UTC on this server. The clause therefore
// meant `created_at < NOW() - 5 hours`, not two, and silently withheld the
// reminder from anyone who booked between two and five hours before their game.
func (m *Store) GetForReminder2hEnriched(ctx context.Context, now time.Time) ([]*CronBooking, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT `+BookingColumns+`,
		       cl.phone, COALESCE(cl.email, ''), cx.name, co.name,
		       cx.slug, cx.address, cx.city, cx.latitude, cx.longitude,
		       cx.mp_access_token, cx.mp_refresh_token, cx.cancellation_hours
		FROM bookings b
		JOIN clients cl ON cl.id = b.client_id
		JOIN complexes cx ON cx.id = b.complex_id
		JOIN courts co ON co.id = b.court_id
		WHERE booking_starts_at(b.date, b.start_time) > $1::timestamptz
		  AND booking_starts_at(b.date, b.start_time) <= $1::timestamptz + INTERVAL '2 hours'
		  AND b.status = 'confirmed'
		  AND b.reminder_sent_2h = false
		  AND b.created_at < $1::timestamptz - INTERVAL '2 hours'`, data.TimeToPg(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCronBookings(rows, m.Keys)
}

// GetExpiredPendingEnriched returns pending bookings older than expiry, enriched with client,
// complex and court data needed to notify the client and release the slot.
func (m *Store) GetExpiredPendingEnriched(ctx context.Context, expiry time.Duration) ([]*CronBooking, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT `+BookingColumns+`,
		       cl.phone, COALESCE(cl.email, ''), cx.name, co.name,
		       cx.slug, cx.address, cx.city, cx.latitude, cx.longitude,
		       cx.mp_access_token, cx.mp_refresh_token, cx.cancellation_hours
		FROM bookings b
		JOIN clients cl ON cl.id = b.client_id
		JOIN complexes cx ON cx.id = b.complex_id
		JOIN courts co ON co.id = b.court_id
		WHERE b.status = 'pending'
		  AND b.collection_status = 'unpaid'
		  AND b.created_by IS NULL
		  AND b.created_at < NOW() - $1::interval`, fmt.Sprintf("%d seconds", int(expiry.Seconds())))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCronBookings(rows, m.Keys)
}

// scanCronBookings is the single decode point for the mp_access_token /
// mp_refresh_token columns enriched onto CronBooking — the second,
// independent surface these credentials are read into memory from, besides
// complexFromDB. keys may be nil — Keyring.Open on a nil *Keyring errors
// rather than passing the raw column through.
func scanCronBookings(rows pgx.Rows, keys *crypto.Keyring) ([]*CronBooking, error) {
	result := make([]*CronBooking, 0, 32)
	for rows.Next() {
		var b db.Booking
		var clientPhone, clientEmail, complexName, courtName string
		var complexSlug, complexAddress, complexCity string
		var latitude, longitude pgtype.Float8
		var mpAccessToken, mpRefreshToken pgtype.Text
		var cancellationHours int32
		err := rows.Scan(
			&b.ID, &b.ComplexID, &b.CourtID, &b.ClientID,
			&b.Span,
			&b.Date, &b.StartTime, &b.DurationMinutes,
			&b.Price, &b.DepositAmount, &b.Status,
			&b.CollectionStatus, &b.RefundStatus,
			&b.ReminderSent2h,
			&b.Notes, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt,
			&clientPhone, &clientEmail, &complexName, &courtName,
			&complexSlug, &complexAddress, &complexCity, &latitude, &longitude,
			&mpAccessToken, &mpRefreshToken, &cancellationHours,
		)
		if err != nil {
			return nil, err
		}
		booking := BookingFromDB(b)
		cb := &CronBooking{
			Booking:           *booking,
			ClientPhone:       clientPhone,
			ClientEmail:       clientEmail,
			ComplexName:       complexName,
			CourtName:         courtName,
			ComplexSlug:       complexSlug,
			ComplexAddress:    complexAddress,
			ComplexCity:       complexCity,
			ComplexLatitude:   data.PgToFloat8Ptr(latitude),
			ComplexLongitude:  data.PgToFloat8Ptr(longitude),
			CancellationHours: int(cancellationHours),
		}
		cb.mpAccessToken, cb.mpAccessTokenErr = mpcred.Open(keys, booking.ComplexID, mpcred.AccessTokenColumn, data.PgToTextPtr(mpAccessToken))
		cb.mpRefreshToken, cb.mpRefreshTokenErr = mpcred.Open(keys, booking.ComplexID, mpcred.RefreshTokenColumn, data.PgToTextPtr(mpRefreshToken))
		result = append(result, cb)
	}
	return result, rows.Err()
}

// CompletePastBookings marks confirmed bookings that have finished as completed.
//
// Keyed on when the booking actually ends, not on the date it is filed under. A
// booking dated yesterday running 23:30 to 00:30 is still being played at ten
// past midnight; `date < today` was true the moment the clock rolled over, so
// the cron marked a game completed while it was in progress — and a completed
// booking no longer blocks deleting its court.
func (m *Store) CompletePastBookings(ctx context.Context) (int64, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	result, err := m.DB.Exec(ctx, `
		UPDATE bookings
		SET status = 'completed', updated_at = NOW()
		WHERE status = 'confirmed'
		  AND upper(span) <= NOW()`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// HasActiveBookingsByCourt reports whether the court still owes anyone their
// hours, which is what blocks deleting it.
//
// Asked as "has this booking finished yet", not "is its date today or later".
// A booking dated yesterday running 23:30 to 00:30 is being played right now at
// ten past midnight, and `date >= CURRENT_DATE` is false for it — so the court
// read as free to delete out from under a live game.
//
// It asks releasedBookingStatuses, so pending, confirmed and completed block
// and a no_show does not. A no_show blocking was an accident rather than
// protection: nobody is coming, the hours are already back on sale (a no_show
// releases them), and the 409 the handler returns tells the owner to "cancel them first" —
// a move internal/bookings/handlers.go refuses, since no_show maps to the empty
// set of transitions. The court simply became undeletable until midnight moved
// the row behind CURRENT_DATE.
func (m *Store) HasActiveBookingsByCourt(ctx context.Context, courtID uuid.UUID) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var exists bool
	err := m.DB.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM bookings
			WHERE court_id = $1
			  AND upper(span) > NOW()
			  AND status NOT IN `+slotguard.ReleasedBookingStatuses+`
		)`, data.UUIDToPg(courtID)).Scan(&exists)
	return exists, err
}

// HasActiveBookings reports whether the complex still owes anyone their hours
// from today onwards. Same predicate and same reasoning as
// HasActiveBookingsByCourt, one level up: it gates deleting the complex and
// disconnecting its MercadoPago account.
func (m *Store) HasActiveBookings(ctx context.Context, complexID uuid.UUID) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var exists bool
	err := m.DB.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM bookings
			WHERE complex_id = $1
			  AND upper(span) > NOW()
			  AND status NOT IN `+slotguard.ReleasedBookingStatuses+`
		)`, data.UUIDToPg(complexID)).Scan(&exists)
	return exists, err
}

// CancelFutureByComplex cancels every future, non-terminal booking in the complex, used when a complex is deactivated.
func (m *Store) CancelFutureByComplex(ctx context.Context, complexID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx,
		`UPDATE bookings SET status = 'cancelled', updated_at = NOW()
		 WHERE complex_id = $1 AND date >= CURRENT_DATE AND status NOT IN ('cancelled', 'completed', 'no_show')`, complexID)
	return err
}

// ─── Dashboard: Live Courts & Payment Summary ───────────────────────────

// PaymentStatusBreakdown holds the booking count and total amount collected for
// one collection status.
type PaymentStatusBreakdown struct {
	Count int `json:"count"`
	Total int `json:"total"`
}

// PaymentSummary breaks down today's takings by collection status and by
// payment method.
//
// ByStatus is keyed by bookings.collection_status — unpaid, deposit_paid,
// fully_paid — and not by the refund axis. The payment_status split separated the two, and
// this card was always the collection one: it sums payments rows that are NOT
// refunded, over bookings that are NOT cancelled, so a bucket labelled
// 'refunded' here could only ever have held the amount a partially refunded
// booking still had in hand, filed under a refund label. That was the
// conflation the payment_status split removed; the number did not change, only the name it is filed
// under.
type PaymentSummary struct {
	ByStatus map[string]PaymentStatusBreakdown `json:"by_status"`
	ByMethod map[string]int                    `json:"by_method"`
}

// GetPaymentSummary returns today's takings for the complex grouped by payment
// status and by method.
//
// This one deliberately does NOT use releasedBookingStatuses, and the exception
// is the point rather than an oversight. Every other counter above asks about
// court time, where a resold no_show is one hour sold twice and must be counted
// once. This asks about money, where the same pair is two separate amounts the
// venue actually holds: the absent client's forfeited deposit and the resale's
// payment. A no_show is recorded as a no_show precisely because the client owes
// for it, so dropping it here would erase a real forfeit from the owner's cash
// view. Hours cannot be sold twice; pesos can be collected twice.
func (m *Store) GetPaymentSummary(ctx context.Context, complexID uuid.UUID, today time.Time) (*PaymentSummary, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	summary := &PaymentSummary{
		ByStatus: make(map[string]PaymentStatusBreakdown),
		ByMethod: make(map[string]int),
	}

	// Grouped by the booking's CURRENT collection_status, but summed from the same
	// payments actually collected today (by created_at, like the by-method
	// query below) rather than from bookings scheduled for today — a booking's
	// calendar date and when it got paid are unrelated, so filtering this by
	// booking date instead of payment date made this total diverge from the
	// by-method one below, which is real cash collected today. p.amount
	// excludes service_fee (a separate column) by construction, so this never
	// counts the platform's cut as takings.
	rows, err := m.DB.Query(ctx, `
		SELECT b.collection_status, COUNT(DISTINCT b.id)::int,
		       COALESCE(SUM(p.amount), 0)::bigint
		FROM bookings b
		JOIN payments p ON p.booking_id = b.id
		WHERE b.complex_id = $1 AND b.status != 'cancelled' AND p.status != 'refunded'
		  AND (p.created_at AT TIME ZONE 'America/Argentina/Buenos_Aires')::date = $2
		GROUP BY b.collection_status`,
		data.UUIDToPg(complexID), data.DateToPg(today),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ps string
		var count, total int
		if err := rows.Scan(&ps, &count, &total); err != nil {
			return nil, err
		}
		summary.ByStatus[ps] = PaymentStatusBreakdown{Count: count, Total: total}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows2, err := m.DB.Query(ctx, `
		SELECT p.method::text, COALESCE(SUM(p.amount), 0)::bigint
		FROM payments p
		WHERE p.complex_id = $1 AND p.status != 'refunded'
		  AND (p.created_at AT TIME ZONE 'America/Argentina/Buenos_Aires')::date = $2
		GROUP BY p.method`,
		data.UUIDToPg(complexID), data.DateToPg(today),
	)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var method string
		var amount int
		if err := rows2.Scan(&method, &amount); err != nil {
			return nil, err
		}
		summary.ByMethod[method] = amount
	}
	return summary, rows2.Err()
}
