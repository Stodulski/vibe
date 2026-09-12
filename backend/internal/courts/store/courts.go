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

	"github.com/stodulski/vibe-server/internal/data"
	slotguard "github.com/stodulski/vibe-server/internal/data/slotguard"
	"github.com/stodulski/vibe-server/internal/db"
)

// Court represents a bookable playing surface within a complex.
type Court struct {
	ID        uuid.UUID `json:"id"`
	ComplexID uuid.UUID `json:"complex_id"`
	Name      string    `json:"name"`
	Sport     string    `json:"sport"`
	CourtType string    `json:"court_type"`
	IsActive  bool      `json:"is_active"`
	// What this court is like — floor, walls, lighting. Free text the owner
	// writes for a player choosing between two courts that the other columns
	// describe identically. Nil when never written, matching Complex.
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// Version is the row's optimistic-concurrency counter, bumped by a trigger
	// on every UPDATE (db/migrations/003_optimistic_concurrency.sql). It is
	// also the version of this court's price set, because those rows are
	// replaced wholesale and a version on one of them does not survive the
	// replace — see BumpCourtVersion in db/queries/courts.sql.
	Version int `json:"version"`
}

// CourtPrice defines the price charged for a court during a day-type and
// time-of-day range.
type CourtPrice struct {
	ID       uuid.UUID `json:"id"`
	CourtID  uuid.UUID `json:"court_id"`
	Price    int       `json:"price"`
	DayType  string    `json:"day_type"`
	TimeFrom string    `json:"time_from"`
	TimeTo   string    `json:"time_to"`
	// FromMin and ToMin are the band as minutes from its weekday's own
	// midnight, read off the generated span_min column (court_prices in db/migrations/001_init.sql).
	// ToMin exceeds 1440 for a band running past midnight — Thursday
	// 22:00-01:30 is [1320, 1530) — which is the whole reason they exist:
	// comparing the two clock strings puts such a band's end before its start,
	// and every match written that way covers nothing at all.
	//
	// They are read, never written. time_from and time_to remain the fields an
	// owner edits and the database derives these from them.
	//
	// Serialized so the storefront's price preview matches what is charged
	// without re-deriving the wrap rule in TypeScript. A second definition of
	// "a band ending before it starts runs into the next day" is a second thing
	// to get wrong, and the preview disagreeing with the charge is the failure
	// it produces.
	FromMin int `json:"from_min"`
	ToMin   int `json:"to_min"`
	// Version is this band's own optimistic-concurrency counter, bumped by the
	// trigger in db/migrations/003_optimistic_concurrency.sql. It is exposed
	// for completeness and for UpdatePrice, the single-band write; the PUT that
	// replaces a court's whole price table is guarded by the COURT's version
	// instead, because these rows are deleted and reinserted and a band's own
	// counter does not survive that.
	Version int `json:"version"`
}

// Covers reports whether this band prices the minute `min`, counted from its
// weekday's own midnight. Half-open, matching the range the exclusion
// constraint enforces: a band ending where the next begins does not share it.
func (p *CourtPrice) Covers(min int) bool {
	return min >= p.FromMin && min < p.ToMin
}

// BlockedSlot represents a court time range manually taken out of availability.
type BlockedSlot struct {
	ID        uuid.UUID  `json:"id"`
	CourtID   uuid.UUID  `json:"court_id"`
	Date      time.Time  `json:"date"`
	StartTime string     `json:"start_time"`
	EndTime   string     `json:"end_time"`
	Reason    *string    `json:"reason,omitempty"`
	CreatedBy *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	CourtName string     `json:"court_name,omitempty"`
}

// CursorKey orders blocked slots for pagination by (date, start time, id) —
// the same order GetBlockedSlotsByComplex's own ORDER BY uses (H-09; see the
// comment there and on ListBlockedSlots in internal/courts/blocked.go), so a
// caller paging through this list with the cursor this produces sees every
// row exactly once regardless of how many slots share one date.
func (s *BlockedSlot) CursorKey() (time.Time, uuid.UUID) {
	return slotguard.LocalInstant(s.Date, s.StartTime), s.ID
}

// Store implements CourtStore against PostgreSQL.
type Store struct {
	DB *data.DB
	Q  *db.Queries
	// PaymentExpiry is how long an unpaid public booking holds its slot, taken
	// from configuration by stores.New. InsertBlockedSlot needs it to ask the
	// same "is this hour taken" question the storefront and the booking guard
	// ask; a block refused by a booking the storefront already shows as free
	// would be a refusal the owner cannot act on. See Config.PaymentExpiry.
	PaymentExpiry time.Duration
}

// Constraint names from db/migrations/001_init.sql that this store translates into domain
// errors. They are matched by name rather than by SQLSTATE alone: courts and
// court_prices each carry more than one constraint that raises the same code, and
// a translation keyed on the code would report the wrong reason for the refusal.
const (
	courtsActiveNameUnique       = "courts_active_name_unique"
	courtPricesNoOverlappingRule = "court_prices_no_overlapping_rule"
)

// translateCourtWrite maps the constraint refusals a court write can produce onto
// the package's sentinel errors, so a handler answers with a validation error
// rather than a 500. Anything else is returned unchanged — an unrecognised
// constraint failure is a genuine fault and must not be dressed up as one the
// client can fix.
func translateCourtWrite(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.ConstraintName {
	case courtsActiveNameUnique:
		return ErrDuplicateCourtName
	case courtPricesNoOverlappingRule:
		return ErrOverlappingPriceRule
	default:
		return err
	}
}

// Insert creates a new court and populates c with its generated ID and defaults.
func (m *Store) Insert(ctx context.Context, c *Court) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	// The store's own authorization check: see data.AssertTenant. Every caller
	// today reaches here through the HTTP chain, which has already decided
	// this; the point is that a caller which does not is refused rather than
	// trusted.
	if err := data.AssertTenant(ctx, c.ComplexID); err != nil {
		return err
	}

	dbCourt, err := m.Q.InsertCourt(ctx, db.InsertCourtParams{
		ComplexID:   data.UUIDToPg(c.ComplexID),
		Name:        c.Name,
		Sport:       db.SportType(c.Sport),
		CourtType:   db.CourtType(c.CourtType),
		Description: data.TextToPg(c.Description),
	})
	if err != nil {
		return translateCourtWrite(err)
	}

	c.ID = data.PgToUUID(dbCourt.ID)
	c.IsActive = dbCourt.IsActive
	c.CreatedAt = data.PgToTime(dbCourt.CreatedAt)
	c.UpdatedAt = data.PgToTime(dbCourt.UpdatedAt)
	return nil
}

// GetByID returns the court with the given ID, or ErrRecordNotFound if none exists.
func (m *Store) GetByID(ctx context.Context, id uuid.UUID) (*Court, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbCourt, err := m.Q.GetCourtByID(ctx, db.GetCourtByIDParams{
		ID:        data.UUIDToPg(id),
		ComplexID: data.TenantParam(ctx),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return courtFromDB(courtRowFromActive(dbCourt)), nil
}

// GetByComplex returns every court belonging to the complex.
func (m *Store) GetByComplex(ctx context.Context, complexID uuid.UUID) ([]*Court, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbCourts, err := m.Q.GetCourtsByComplex(ctx, data.UUIDToPg(complexID))
	if err != nil {
		return nil, err
	}

	result := make([]*Court, len(dbCourts))
	for i, c := range dbCourts {
		result[i] = courtFromDB(courtRowFromActive(c))
	}
	return result, nil
}

// Update persists changes to an existing court, returning ErrRecordNotFound if it no longer exists.
//
// expectedVersion is the caller's optimistic-concurrency precondition: the
// version it read before it filled in the form. Nil means it sent none, and the
// write is the last-write-wins it always was (API-08).
func (m *Store) Update(ctx context.Context, c *Court, expectedVersion *int) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, c.ComplexID); err != nil {
		return err
	}

	dbCourt, err := m.Q.UpdateCourt(ctx, db.UpdateCourtParams{
		ExpectedVersion: data.Int4PtrToPg(expectedVersion),
		Name:            c.Name,
		Sport:           db.SportType(c.Sport),
		CourtType:       db.CourtType(c.CourtType),
		IsActive:        c.IsActive,
		Description:     data.TextToPg(c.Description),
		ID:              data.UUIDToPg(c.ID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return translateCourtWrite(err)
	}

	c.UpdatedAt = data.PgToTime(dbCourt.UpdatedAt)
	c.Version = int(dbCourt.Version)
	return nil
}

// SoftDelete marks the court as deleted, refusing with
// ErrCourtHasActiveBookings when the court still owes someone their hours.
//
// H-02: this used to be a plain unconditional delete, with the "does it have
// active bookings" question asked by the handler beforehand
// (HasActiveBookingsByCourt) and nothing serializing the two calls. A booking
// that committed in the gap between them survived on a court the owner had
// just watched disappear from their own dashboard — the same check-then-act
// shape H-01 closed for clients.GetOrCreate, one level up. The existence test
// and the delete are now one statement: the UPDATE's own WHERE clause is the
// check, so there is no gap between asking and acting for a booking to land
// in.
//
// The predicate mirrors the bookings store's HasActiveBookingsByCourt exactly
// (upper(span) > NOW() AND status NOT IN slotguard.ReleasedBookingStatuses), so
// a court this deletes is exactly a court that check would have answered "no"
// for, and both spell the predicate with that one shared constant rather than
// duplicating it.
//
// H-22: one statement was not enough either, and the reason is worth writing
// down because it is invisible from the SQL. Folding the booking question into
// the UPDATE's own WHERE only closes the case where the booking has ALREADY
// COMMITTED. A booking still in flight was seen by neither side: it took an
// advisory lock on (court, day) that this delete has no day to key and so
// could never take, and under READ COMMITTED each transaction's own check was
// correct against its own snapshot. Both committed, and a client kept hours on
// a court the owner had just watched disappear.
//
// So the two writers are made to contend on the court row, which is the only
// object both of them can hold. This takes it FOR UPDATE; the booking path
// takes it FOR SHARE (see slotguard.LockCourtLive).
//
// The booking question then has to be asked by a SEPARATE statement, after the
// lock is held, and that is the part a row lock alone does not give you. When
// an UPDATE blocks on a row another transaction has locked, PostgreSQL
// re-evaluates its qual against the updated row once that transaction commits
// — but the subquery in that qual still runs against the command's ORIGINAL
// snapshot, so a NOT EXISTS over `bookings` does not see the booking that just
// committed. READ COMMITTED gives each STATEMENT a fresh snapshot; EvalPlanQual
// does not. Asking after the lock, in a statement of its own, is what makes the
// answer current.
//
// There are three independent ways for this to refuse — an id nothing owns, a
// court already soft-deleted, and a court with live bookings — and a first
// version of this function mapped all three to ErrCourtHasActiveBookings. That
// collapsed a retried or concurrent DELETE of an already-deleted court, which
// used to succeed silently, into a 409 claiming bookings that do not exist,
// and cost the delete its idempotence on retry. The lock read below tells the
// first two apart before the booking question is even asked.
func (m *Store) SoftDelete(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	return m.DB.WithTx(ctx, func(tx pgx.Tx, _ *db.Queries) error {
		return softDeleteCourt(ctx, tx, id)
	})
}

// softDeleteCourt is SoftDelete's body, inside the transaction.
func softDeleteCourt(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	// Take the court row first. Any booking transaction holding it under
	// FOR SHARE has to commit or roll back before this returns, so the
	// question below is asked of a settled world rather than a racing one.
	var deletedAt pgtype.Timestamptz
	err := tx.QueryRow(ctx,
		`SELECT deleted_at FROM courts WHERE id = $1 FOR UPDATE`, data.UUIDToPg(id)).Scan(&deletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return fmt.Errorf("lock court: %w", err)
	}
	if deletedAt.Valid {
		// Already gone. A retried or concurrent delete of the same court is not
		// a conflict to report — it is the caller asking for a state that is
		// already true — so this is idempotent success, matching what the
		// unconditional delete this replaced used to do.
		return nil
	}

	// A statement of its own, and that is the whole point: it runs on a
	// snapshot taken after the lock was granted, so a booking that committed
	// while this transaction waited is visible here. The predicate mirrors
	// bookingstore.Store.HasActiveBookingsByCourt's exactly.
	var hasBookings bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM bookings
		  WHERE court_id = $1
		    AND upper(span) > NOW()
		    AND status NOT IN `+slotguard.ReleasedBookingStatuses+`
		)`, data.UUIDToPg(id)).Scan(&hasBookings)
	if err != nil {
		return fmt.Errorf("check active bookings: %w", err)
	}
	if hasBookings {
		return ErrCourtHasActiveBookings
	}

	if _, err := tx.Exec(ctx,
		`UPDATE courts SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`,
		data.UUIDToPg(id)); err != nil {
		return fmt.Errorf("soft-delete court: %w", err)
	}
	return nil
}

// InsertPrice creates a new price rule for a court and populates p with its generated ID.
func (m *Store) InsertPrice(ctx context.Context, p *CourtPrice) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbPrice, err := m.Q.InsertCourtPrice(ctx, db.InsertCourtPriceParams{
		CourtID: data.UUIDToPg(p.CourtID),
		//nolint:gosec // G115: Price is validated > 0 at the courts.go handler; it is a currency amount realistically
		// far below int32 range, matching the same bound rationale as booking Price.
		Price:    int32(p.Price),
		DayType:  db.DayOfWeek(p.DayType),
		TimeFrom: data.TimeStrToPg(p.TimeFrom),
		TimeTo:   data.TimeStrToPg(p.TimeTo),
	})
	if err != nil {
		return translateCourtWrite(err)
	}

	p.ID = data.PgToUUID(dbPrice.ID)
	return nil
}

// GetPrices returns every price rule defined for the court.
func (m *Store) GetPrices(ctx context.Context, courtID uuid.UUID) ([]*CourtPrice, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbPrices, err := m.Q.GetCourtPrices(ctx, data.UUIDToPg(courtID))
	if err != nil {
		return nil, err
	}

	result := make([]*CourtPrice, len(dbPrices))
	for i, p := range dbPrices {
		result[i] = &CourtPrice{
			ID:       data.PgToUUID(p.ID),
			CourtID:  data.PgToUUID(p.CourtID),
			Price:    int(p.Price),
			DayType:  string(p.DayType),
			TimeFrom: data.PgToTimeStr(p.TimeFrom),
			TimeTo:   data.PgToTimeStr(p.TimeTo),
			FromMin:  int(p.SpanMin.Lower.Int32),
			ToMin:    int(p.SpanMin.Upper.Int32),
			Version:  int(p.Version),
		}
	}
	return result, nil
}

// UpdatePrice persists changes to an existing price rule, returning ErrRecordNotFound if it no longer exists.
func (m *Store) UpdatePrice(ctx context.Context, p *CourtPrice) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbPrice, err := m.Q.UpdateCourtPrice(ctx, db.UpdateCourtPriceParams{
		//nolint:gosec // G115: Price is validated > 0 at the courts.go handler; it is a currency amount realistically
		// far below int32 range, matching the same bound rationale as booking Price.
		Price:    int32(p.Price),
		DayType:  db.DayOfWeek(p.DayType),
		TimeFrom: data.TimeStrToPg(p.TimeFrom),
		TimeTo:   data.TimeStrToPg(p.TimeTo),
		ID:       data.UUIDToPg(p.ID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return translateCourtWrite(err)
	}

	p.CourtID = data.PgToUUID(dbPrice.CourtID)
	return nil
}

// DeletePrice removes a single price rule by ID.
func (m *Store) DeletePrice(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return m.Q.DeleteCourtPrice(ctx, data.UUIDToPg(id))
}

// DeletePricesByCourtID removes every price rule belonging to the court.
//
// Deliberately left as its own unconditional statement, rather than folded
// into ReplacePrices below: CourtPricingManager (internal/stores) is
// also the interface cmd/api's own store wiring is built against, so this
// method's shape stays exactly what it always was for whatever else depends
// on it existing on its own.
func (m *Store) DeletePricesByCourtID(ctx context.Context, courtID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx, `DELETE FROM court_prices WHERE court_id = $1`, data.UUIDToPg(courtID))
	return err
}

// ReplacePrices atomically replaces every price rule for a court: the whole
// existing table is deleted and the new bands are inserted, both inside one
// transaction.
//
// H-07: UpdatePrices used to run these as two independent statements — the
// delete committed on its own, and any insert failure after it (the column's
// INTEGER range, the court_prices_no_overlapping_rule exclusion constraint,
// or a concurrent writer — everything the in-memory validation cannot catch)
// left the court with zero price bands, and CRT-11 confirmed a court with no
// prices is not bookable. One malformed price update silently took the court
// off sale. Wrapping the delete and every insert in one transaction is what
// makes a refused write cost nothing: nothing commits unless every band does.
//
// The pattern follows InsertBlockedSlot above (m.DB.Begin / defer
// tx.Rollback / tx.Commit) rather than inventing another one for this file.
//
// On a failure caused by one specific price, failedIndex names which element
// of prices could not be inserted, so the caller can build the same
// field-scoped validation error ("prices[i].time_from: ...") the old
// per-row loop built; it is -1 when the failure is not attributable to one
// row (the delete itself, or the transaction machinery).
//
// expectedVersion is the caller's precondition, and it is the COURT's version
// rather than any band's: the bands are deleted and reinserted here, so a
// counter on one of them cannot be held across this call. Bumping the court in
// the same transaction is what gives the new price set a version at all — see
// BumpCourtVersion in db/queries/courts.sql. Nil means no precondition, which
// is the last-write-wins this endpoint had before versions existed (API-08).
func (m *Store) ReplacePrices(ctx context.Context, courtID uuid.UUID, prices []*CourtPrice,
	expectedVersion *int) (failedIndex int, err error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	failedIndex = -1
	err = m.DB.WithTx(ctx, func(tx pgx.Tx, qtx *db.Queries) error {
		// First, because it is the precondition: if the version has moved,
		// nothing below should run at all, and inside one transaction the
		// delete would otherwise be rolled back rather than never attempted.
		if _, err := qtx.BumpCourtVersion(ctx, db.BumpCourtVersionParams{
			ID:              data.UUIDToPg(courtID),
			ExpectedVersion: data.Int4PtrToPg(expectedVersion),
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return err
		}

		if _, err := tx.Exec(ctx, `DELETE FROM court_prices WHERE court_id = $1`, data.UUIDToPg(courtID)); err != nil {
			return err
		}

		// qtx reuses InsertCourtPrice's own sqlc-generated encoding for the
		// day_of_week enum rather than hand-rolling a raw INSERT for it — the
		// same reason InsertPrice below calls it directly.
		for i, p := range prices {
			dbPrice, err := qtx.InsertCourtPrice(ctx, db.InsertCourtPriceParams{
				CourtID: data.UUIDToPg(p.CourtID),
				//nolint:gosec // G115: see InsertPrice's note above.
				Price:    int32(p.Price),
				DayType:  db.DayOfWeek(p.DayType),
				TimeFrom: data.TimeStrToPg(p.TimeFrom),
				TimeTo:   data.TimeStrToPg(p.TimeTo),
			})
			if err != nil {
				// The index of the band that was refused is part of the
				// answer, not only the error: the handler names it back to the
				// client. It is set here rather than returned because the
				// transaction wrapper carries only an error.
				failedIndex = i
				return translateCourtWrite(err)
			}
			p.ID = data.PgToUUID(dbPrice.ID)
		}
		return nil
	})
	return failedIndex, err
}

// InsertBlockedSlot creates a new blocked slot inside a transaction, returning
// ErrSlotAlreadyBlocked if it overlaps an existing blocked slot on the same
// court, and ErrSlotHasBooking if a live booking already holds those hours.
//
// Both refusals are decided inside the transaction, under the same court-day
// advisory lock bookingstore.Store.InsertSafe takes. Only the first of them could
// have been left to the database: blocked_slots_no_overlapping_span
// refuses an overlapping block with no lock at all, but EXCLUDE is
// single-table and cannot see bookings, so the booking half is a query — and a
// query is only race-free while the writers that could invalidate it are
// serialized against it.
//
// What that lock closes is the window F01 opens with. Without it the only
// booking-versus-block check on this side was the handler pre-check, which runs
// outside any transaction: an owner filing a block at the moment a customer
// checked out the same hours got both rows committed, because the booking's own
// check ran before the block existed and this one ran before the booking did.
// Neither is wrong on its own; taking the lock is what stops them from being
// wrong together.
//
// What it does NOT close, and is left where it was: the confirmation path
// (paymentstore.Payments.guardSlotStillFree) still asks only about bookings, so a stale
// pending booking — one this check ignores because it no longer holds its slot
// — can be confirmed onto hours blocked in the meantime, caught only by the
// handler pre-check in internal/bookings/actions.go.
//
// It is raw SQL rather than a sqlc query, and db/queries/blocked_slots.sql no
// longer carries an InsertBlockedSlot for it to use. A generated one-shot
// INSERT cannot run inside a transaction that already holds the advisory locks
// and cannot read `span` back to ask the booking question against the same
// value the exclusion constraint uses. The generated one that used to sit there
// had no caller at all.
func (m *Store) InsertBlockedSlot(ctx context.Context, s *BlockedSlot) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	var id pgtype.UUID
	var createdAt pgtype.Timestamptz
	if err := m.DB.WithTx(ctx, func(tx pgx.Tx, _ *db.Queries) error {
		return m.insertBlockedSlot(ctx, tx, s, &id, &createdAt)
	}); err != nil {
		return err
	}

	s.ID = data.PgToUUID(id)
	s.CreatedAt = data.PgToTime(createdAt)
	return nil
}

// insertBlockedSlot is InsertBlockedSlot's body, inside the transaction. The
// generated id and timestamp are written back through the pointers rather than
// returned, because the row must not reach s until the transaction has actually
// committed.
func (m *Store) insertBlockedSlot(
	ctx context.Context, tx pgx.Tx, s *BlockedSlot, id *pgtype.UUID, createdAt *pgtype.Timestamptz,
) error {
	// Every local day these hours touch, ascending, exactly as InsertSafe locks
	// a booking's. blocked_slots_check keeps a block inside one day today, so
	// this is one lock in practice; asking for the range anyway is what keeps
	// the two sides speaking one language if that check is ever lifted.
	blockStart := slotguard.LocalInstant(s.Date, s.StartTime)
	blockEnd := slotguard.LocalInstant(s.Date, s.EndTime)
	if err := slotguard.LockCourtDays(ctx, tx, s.CourtID, blockStart, blockEnd); err != nil {
		return err
	}

	var span pgtype.Range[pgtype.Timestamptz]
	err := tx.QueryRow(ctx, `
		INSERT INTO blocked_slots (court_id, date, start_time, end_time, reason, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, span`,
		data.UUIDToPg(s.CourtID), data.DateToPg(s.Date),
		data.TimeStrToPg(s.StartTime), data.TimeStrToPg(s.EndTime),
		data.TextToPg(s.Reason), data.UUIDPtrToPg(s.CreatedBy),
	).Scan(id, createdAt, &span)
	if err != nil {
		// The block-versus-block overlap is refused by
		// blocked_slots_no_overlapping_span rather than by a
		// SELECT above this INSERT. The SELECT was a read-then-decide check
		// under READ COMMITTED: two concurrent requests both saw no overlap
		// and both inserted, and the owner was left with duplicate maintenance
		// blocks the availability grid subtracted twice. An EXCLUDE constraint
		// needs no external lock to be race-free, which is the same argument
		// bookings_no_overlapping_span made when it replaced the overlap half of the booking
		// guard.
		if isOverlapRefusal(err) {
			return ErrSlotAlreadyBlocked
		}
		return err
	}

	// The booking half, asked of the span the row above just generated rather
	// than of a range rebuilt here — a second copy of that arithmetic is the
	// defect blocked_slots.span exists to unwind. The insert happens first so the
	// comparison is against the value the database stored; nothing commits
	// unless the answer is no.
	booked, err := slotguard.SpanTaken(ctx, tx, s.CourtID, span, m.PaymentExpiry)
	if err != nil {
		return err
	}
	if booked {
		return ErrSlotHasBooking
	}
	return nil
}

// isOverlapRefusal reports whether err is an EXCLUDE constraint refusing a row
// because it overlaps one already stored.
//
// It is deliberately narrower than isSlotAlreadySold in bookings.go, which
// also accepts 23505: there is no unique index on blocked_slots to produce
// one, and widening the match would turn an unrelated duplicate-key bug into
// a friendly "already blocked" the caller cannot distinguish from the real
// thing.
func isOverlapRefusal(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == data.SQLStateExclusionViolation
}

// GetBlockedSlots returns the blocked slots for a court on the given date.
func (m *Store) GetBlockedSlots(ctx context.Context, courtID uuid.UUID, date time.Time) ([]*BlockedSlot, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	pgDate := data.DateToPg(date)
	dbSlots, err := m.Q.GetBlockedSlots(ctx, db.GetBlockedSlotsParams{
		CourtID: data.UUIDToPg(courtID),
		Date:    pgDate,
		Date_2:  pgDate,
	})
	if err != nil {
		return nil, err
	}

	result := make([]*BlockedSlot, len(dbSlots))
	for i, s := range dbSlots {
		result[i] = &BlockedSlot{
			ID:        data.PgToUUID(s.ID),
			CourtID:   data.PgToUUID(s.CourtID),
			Date:      data.PgToDate(s.Date),
			StartTime: data.PgToTimeStr(s.StartTime),
			EndTime:   data.PgToTimeStr(s.EndTime),
			Reason:    data.PgToTextPtr(s.Reason),
			CreatedBy: data.PgToUUIDPtr(s.CreatedBy),
			CreatedAt: data.PgToTime(s.CreatedAt),
		}
	}
	return result, nil
}

// GetPricesByCourtIDs returns the price rules for multiple courts in one query, ordered by court, day type and start time.
func (m *Store) GetPricesByCourtIDs(ctx context.Context, courtIDs []uuid.UUID) ([]*CourtPrice, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx,
		`SELECT id, court_id, price, day_type, time_from, time_to, span_min
		 FROM court_prices
		 WHERE court_id = ANY($1)
		 ORDER BY court_id, day_type, time_from`, data.UUIDSliceToPg(courtIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*CourtPrice
	for rows.Next() {
		var p CourtPrice
		var tf, tt time.Time
		// span_min comes along too. It is what every price lookup matches on,
		// and a band read without it covers no minute at all — which is
		// invisible to a test with a stubbed store and shows up as a storefront
		// publishing nothing.
		var span pgtype.Range[pgtype.Int4]
		if err := rows.Scan(&p.ID, &p.CourtID, &p.Price, &p.DayType, &tf, &tt, &span); err != nil {
			return nil, err
		}
		p.TimeFrom = tf.Format("15:04")
		p.TimeTo = tt.Format("15:04")
		p.FromMin = int(span.Lower.Int32)
		p.ToMin = int(span.Upper.Int32)
		result = append(result, &p)
	}
	return result, rows.Err()
}

// GetBlockedSlotsByCourtIDs returns the blocked slots across multiple courts on the given date.
func (m *Store) GetBlockedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]*BlockedSlot, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx,
		`SELECT id, court_id, date, start_time, end_time, reason, created_by, created_at
		 FROM blocked_slots
		 WHERE court_id = ANY($1) AND date = $2
		 ORDER BY court_id, start_time`, data.UUIDSliceToPg(courtIDs), date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*BlockedSlot
	for rows.Next() {
		var s BlockedSlot
		var st, et time.Time
		var createdBy pgtype.UUID
		var createdAt pgtype.Timestamptz
		if err := rows.Scan(&s.ID, &s.CourtID, &s.Date, &st, &et, &s.Reason, &createdBy, &createdAt); err != nil {
			return nil, err
		}
		s.StartTime = st.Format("15:04")
		s.EndTime = et.Format("15:04")
		s.CreatedBy = data.PgToUUIDPtr(createdBy)
		s.CreatedAt = data.PgToTime(createdAt)
		result = append(result, &s)
	}
	return result, rows.Err()
}

// GetBlockedSlotByID returns the blocked slot with the given ID, or ErrRecordNotFound if none exists.
func (m *Store) GetBlockedSlotByID(ctx context.Context, id uuid.UUID) (*BlockedSlot, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbSlot, err := m.Q.GetBlockedSlotByID(ctx, data.UUIDToPg(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return &BlockedSlot{
		ID:        data.PgToUUID(dbSlot.ID),
		CourtID:   data.PgToUUID(dbSlot.CourtID),
		Date:      data.PgToDate(dbSlot.Date),
		StartTime: data.PgToTimeStr(dbSlot.StartTime),
		EndTime:   data.PgToTimeStr(dbSlot.EndTime),
		Reason:    data.PgToTextPtr(dbSlot.Reason),
		CreatedBy: data.PgToUUIDPtr(dbSlot.CreatedBy),
		CreatedAt: data.PgToTime(dbSlot.CreatedAt),
	}, nil
}

// GetBlockedSlotsByComplex returns the blocked slots for every court in the
// complex within the given date range, ordered by date, start time and
// finally id — the id is a tie-break added for H-09 (see ListBlockedSlots in
// internal/courts/blocked.go, which pages this result by cursor and needs a
// total order to page over correctly; two blocks sharing one date and start
// time on different courts previously had no defined relative order at all).
//
// This still returns the whole date range in one query — H-09's own fix
// paginates the response, not this query, because the paginated shape
// internal/courts.BookingReader would need is a signature change to
// CourtBlockedSlotManager (models.go), and that interface is also what
// cmd/api's own store wiring is built against. See the comment on
// ListBlockedSlots for the rest of that trade-off.
func (m *Store) GetBlockedSlotsByComplex(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time) ([]*BlockedSlot, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx,
		`SELECT bs.id, bs.court_id, bs.date, bs.start_time, bs.end_time, bs.reason, bs.created_by, bs.created_at, c.name
		 FROM blocked_slots bs
		 JOIN active_courts c ON c.id = bs.court_id
		 WHERE c.complex_id = $1 AND bs.date BETWEEN $2 AND $3
		 ORDER BY bs.date, bs.start_time, bs.id`, complexID, dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*BlockedSlot
	for rows.Next() {
		var s BlockedSlot
		var st, et time.Time
		var createdBy pgtype.UUID
		var createdAt pgtype.Timestamptz
		if err := rows.Scan(&s.ID, &s.CourtID, &s.Date, &st, &et, &s.Reason, &createdBy, &createdAt, &s.CourtName); err != nil {
			return nil, err
		}
		s.StartTime = st.Format("15:04")
		s.EndTime = et.Format("15:04")
		s.CreatedBy = data.PgToUUIDPtr(createdBy)
		s.CreatedAt = data.PgToTime(createdAt)
		result = append(result, &s)
	}
	return result, rows.Err()
}

// DeleteBlockedSlot removes a single blocked slot by ID, answering
// ErrRecordNotFound when no row was actually removed.
//
// H-10: the sqlc-generated :exec query this used to call discards the
// command tag, so a delete that removed nothing still answered success —
// two concurrent deletes of the same slot both got [200, 200] on most runs
// (CRT-18), because neither caller could tell a real deletion from a no-op.
// Nothing was corrupted by that — the slot did end up deleted either way —
// but the caller who deleted nothing was told they deleted something, which
// is its own small defect: a client cannot answer "did my delete do
// anything" from that response. Checking the affected row count is what
// makes that answerable, matching how a plain fetch-then-delete already
// answers 404 when the row is simply missing outright.
func (m *Store) DeleteBlockedSlot(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	tag, err := m.DB.Exec(ctx, `DELETE FROM blocked_slots WHERE id = $1`, data.UUIDToPg(id))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return data.ErrRecordNotFound
	}
	return nil
}

// courtRowFromActive re-labels a row of the active_courts view as a row of the
// courts table. Same reason and same guarantee as complexRowFromActive in
// complexes.go: sqlc gives a view its own struct, one mapper is better than two
// that can drift, and the conversion compiles only while the view still carries
// every column of the table in the same order.
func courtRowFromActive(c db.ActiveCourt) db.Court {
	return db.Court(c)
}

func courtFromDB(c db.Court) *Court {
	return &Court{
		ID:          data.PgToUUID(c.ID),
		ComplexID:   data.PgToUUID(c.ComplexID),
		Name:        c.Name,
		Sport:       string(c.Sport),
		CourtType:   string(c.CourtType),
		IsActive:    c.IsActive,
		Description: data.PgToTextPtr(c.Description),
		CreatedAt:   data.PgToTime(c.CreatedAt),
		UpdatedAt:   data.PgToTime(c.UpdatedAt),
		Version:     int(c.Version),
	}
}

// NewCourtPriceForTest builds a price band with its minute span filled in, the
// way a row read from the database arrives.
//
// It exists because span_min is a generated column: the database computes it
// and CourtPrice only carries it, so a band built as a composite literal in a
// test has FromMin and ToMin at zero and matches no minute at all. Tests would
// otherwise silently assert against a band that prices nothing.
//
// The arithmetic here mirrors span_min in db/migrations/001_init.sql, and that is the one place it
// is duplicated. It is confined to test construction on purpose — no production
// path derives the span in Go, because the database already did.
func NewCourtPriceForTest(courtID uuid.UUID, dayType, timeFrom, timeTo string, price int) *CourtPrice {
	fromMin := minutesOfDay(timeFrom)
	toMin := minutesOfDay(timeTo)
	if toMin <= fromMin {
		toMin += minutesPerDay
	}
	return &CourtPrice{
		// v7, matching what the growth tables' column defaults now mint
		// (005_tenant_columns.sql). This is the only place in the repository
		// where Go mints a row id at all — every other id comes from the
		// column's own DEFAULT — so it is the only place that could disagree.
		// uuid.Must: NewV7 fails only if the kernel refuses randomness, which
		// nothing here survives anyway.
		ID: uuid.Must(uuid.NewV7()), CourtID: courtID, DayType: dayType,
		TimeFrom: timeFrom, TimeTo: timeTo, Price: price,
		FromMin: fromMin, ToMin: toMin,
	}
}

const minutesPerDay = 24 * 60

func minutesOfDay(hhmm string) int {
	var h, m int
	_, _ = fmt.Sscanf(hhmm, "%d:%d", &h, &m) //nolint:errcheck // a schedule value the DB stored through a validated path; a malformed one yields 0
	return h*60 + m
}
