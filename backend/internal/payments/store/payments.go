package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	slotguard "github.com/stodulski/vibe-server/internal/data/slotguard"
	"github.com/stodulski/vibe-server/internal/db"
)

// Payment represents a payment or deposit transaction associated with a booking.
type Payment struct {
	ID uuid.UUID `json:"id"`
	// BookingID is the booking this payment is for; uuid.Nil on a registration's
	// payment, which names RegistrationID instead. A payment has exactly one of
	// the two (payments_exactly_one_subject).
	BookingID uuid.UUID `json:"booking_id"`
	// RegistrationID is the open-play event sign-up this payment is for; nil on
	// every booking payment.
	RegistrationID *uuid.UUID `json:"registration_id,omitempty"`
	ComplexID      uuid.UUID  `json:"complex_id"`
	Amount         int        `json:"amount"`
	ServiceFee     int        `json:"service_fee"`
	Method         string     `json:"method"`
	Status         string     `json:"status"`
	MPPaymentID    *string    `json:"mp_payment_id,omitempty"`
	MPPreferenceID *string    `json:"mp_preference_id,omitempty"`
	RefundAmount   int        `json:"refund_amount"`
	// StatusDetail is MercadoPago's own reason for the current status (e.g.
	// "accredited", "cc_rejected_insufficient_amount", "partially_refunded").
	// Nil when the payment was never confirmed/rejected/refunded through a
	// path that carries one.
	StatusDetail *string   `json:"status_detail,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Payments implements PaymentStore against PostgreSQL.
type Payments struct {
	DB *data.DB
	Q  *db.Queries
	// PaymentExpiry is how long an unpaid public booking holds its slot, taken
	// from configuration by stores.New. The confirmation-time slot guard needs it
	// to ask the same question InsertSafe asks. See Config.PaymentExpiry.
	PaymentExpiry time.Duration
}

// guardBookingConfirmable re-reads the booking's status inside tx and refuses
// with ErrBookingNotConfirmable when the caller is asking to move it to
// 'confirmed' and it is no longer one InsertAndConfirmBooking may confirm
// (cancelled, completed, or a no-show).
//
// It only fires when b.Status == "confirmed" — the same test
// guardSlotStillFree uses to decide whether it has anything to check. A first
// version of this guard ran unconditionally and rejected every cancelled
// booking, including the auto-refund path that calls InsertAndConfirmBooking
// with b.Status already set to 'cancelled' precisely so the payment captured
// for it can be recorded and then sent back (see the paragraph above and
// refundBookingWhoseSlotIsGone in internal/payments/process.go). That guard
// refused that exact write: the payment row the refund is issued against was
// never inserted, and the money stayed captured with nothing to claim it
// back against. Scoping the check to the confirm transition is what lets it
// catch the race it exists for — a cancellation committing between
// ConfirmPayment's read and this transaction — without also catching every
// other reason InsertAndConfirmBooking gets called with a non-confirmed
// target status.
//
// FOR UPDATE is what makes "re-read" mean something under a race rather than
// just moving the same stale read a few statements later: it blocks until any
// concurrent writer holding this row (Cancel's own Update, in particular) has
// committed, so the SELECT that follows sees the real, current status instead
// of a value that could still change out from under it. See H-15's comment on
// InsertAndConfirmBooking's call site.
//
// The one cancelled booking it lets through is a storefront booking cancelled
// only because its payment window ran out (a booking_hold_expiries row, see
// 044_booking_hold_expiries.sql), being confirmed by an online payment: the
// payment it was waiting for arrived late, and like the same payment a minute
// earlier it confirms the booking when its hours are still free
// (guardSlotStillFree, which runs after this, decides that). reopen reports
// it, and the caller deletes the expiry row in the same transaction. Only
// while the booking is reopenable at all (bookingstore.ReopenableByLatePaymentSQL,
// the reversal trigger's own conditions plus "not yet started"): anything else
// is ErrBookingCancelled, which the webhook refunds, rather than the trigger's
// raw 23514, which it retried forever. A counter payment never reopens one:
// the owner's paths refuse a cancelled booking on their own.
func (m *Payments) guardBookingConfirmable(ctx context.Context, tx pgx.Tx, p *Payment, b *bookingstore.Booking) (reopen bool, err error) {
	if b.Status != "confirmed" {
		return false, nil
	}

	var status string
	var reopenable bool
	err = tx.QueryRow(ctx, `
		SELECT b.status, `+bookingstore.ReopenableByLatePaymentSQL("b")+`
		FROM bookings b
		WHERE b.id = $1
		FOR UPDATE`, data.UUIDToPg(b.ID)).Scan(&status, &reopenable)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, data.ErrRecordNotFound
		}
		return false, fmt.Errorf("payments: guard booking confirmable: %w", err)
	}
	// H-23: cancelled is answered with its own sentinel. It is the only one of
	// the three where the client's money must be sent back, and collapsing it
	// into ErrBookingNotConfirmable is what routed a paid-but-cancelled booking
	// past the webhook's refund branch. See ErrBookingCancelled.
	if status == "cancelled" {
		if reopenable && p.Method == "mercadopago" {
			return true, nil
		}
		return false, bookingstore.ErrBookingCancelled
	}
	if status == "completed" || status == "no_show" {
		return false, bookingstore.ErrBookingNotConfirmable
	}
	return false, nil
}

// afterBookingWrite is what every payment write does once the booking row is
// written, in the same transaction. A reopened booking's expiry row is
// deleted: from here it is a live booking, and a later cancellation of it is
// final like any other. Then the booking gets its client (attachClient).
func afterBookingWrite(ctx context.Context, tx pgx.Tx, qtx *db.Queries, b *bookingstore.Booking, reopen bool) error {
	if reopen {
		if err := qtx.DeleteBookingHoldExpiry(ctx, data.UUIDToPg(b.ID)); err != nil {
			return fmt.Errorf("payments: close the hold expiry of reopened booking %s: %w", b.ID, err)
		}
	}
	return attachClient(ctx, tx, qtx, b)
}

// prepareReopen readies a booking a late payment is reopening, before its row
// is written and under the court-day lock both callers hold. Another
// visitor's checkout on the same hours that is itself past its payment window
// does not block them (SlotTaken's stale carve-out) but still counts for the
// exclusion constraint, which the reopening write would hit as a raw 23P01
// and the event requeue forever; it is released first, exactly as an insert
// releases it (ReleaseStalePendingOverlaps, which also records it for the
// sweep). And the expiry note goes: the booking is live again.
func (m *Payments) prepareReopen(ctx context.Context, tx pgx.Tx, b *bookingstore.Booking) error {
	if err := bookingstore.ReleaseStalePendingOverlaps(ctx, tx, b, m.PaymentExpiry); err != nil {
		return fmt.Errorf("payments: release stale checkouts under reopened booking %s: %w", b.ID, err)
	}
	b.Notes = bookingstore.WithoutHoldExpiryNote(b.Notes)
	return nil
}

// guardSlotAndReopen is guardSlotStillFree followed, for a booking being
// reopened, by prepareReopen: both confirmation writes run the two together.
func (m *Payments) guardSlotAndReopen(ctx context.Context, tx pgx.Tx, qtx *db.Queries, b *bookingstore.Booking, reopen bool) error {
	if err := m.guardSlotStillFree(ctx, tx, qtx, b); err != nil {
		return err
	}
	if !reopen {
		return nil
	}
	return m.prepareReopen(ctx, tx, b)
}

// guardWebhookConfirmation runs confirmWebhookPayment's guards in
// ConfirmInTx's lock order: the court day first, then the booking row
// (guardBookingConfirmable), then the court row and the slot check
// (guardSlotStillFree, whose court-day lock is re-entrant). The status guard
// is what lets a booking cancelled only by hold expiry be reopened by its late
// payment, and what answers any other cancellation with ErrBookingCancelled
// (refunded by the caller) rather than the reversal trigger's raw error.
func (m *Payments) guardWebhookConfirmation(ctx context.Context, tx pgx.Tx, qtx *db.Queries, p *Payment, b *bookingstore.Booking) (reopen bool, err error) {
	if b.Status == "confirmed" {
		if err := slotguard.LockCourtDay(ctx, tx, b.CourtID, b.Date); err != nil {
			return false, err
		}
	}
	reopen, err = m.guardBookingConfirmable(ctx, tx, p, b)
	if err != nil {
		return false, err
	}
	// Before anything is written: the slot this booking is about to claim has to
	// still be free.
	if err := m.guardSlotAndReopen(ctx, tx, qtx, b, reopen); err != nil {
		return false, err
	}
	return reopen, nil
}

// guardAmountWithinBalance refuses a payment that would collect more than the booking
// still owes, judged on the row as it stands under the lock. ConfirmPayment checks the
// amount against the booking it read before any lock, so two counter collections can
// both pass that check; the second one is refused here, after the first has committed.
//
// It applies to the transitions that take money on a live booking: a confirmation
// (status 'confirmed', the same test guardBookingConfirmable uses) and a collection on
// a completed booking. A cancelled booking is left alone. Its payment is recorded so
// the money can be refunded, and the amount is what MercadoPago sent, which need not
// match what the booking still owes (see recordPaymentOwedARefund in internal/payments).
//
// The price, deposit and collection status are read from the locked row, never from b:
// the caller has already moved b's in-memory collection status to its post-payment value.
// The balance itself is BalanceDue on the locked row, so this check cannot drift from
// the definition the service and the sales checkout use.
func (m *Payments) guardAmountWithinBalance(ctx context.Context, qtx *db.Queries, p *Payment, b *bookingstore.Booking) error {
	if b.Status != "confirmed" && b.Status != "completed" {
		return nil
	}

	row, err := qtx.GetBookingByIDForUpdate(ctx, db.GetBookingByIDForUpdateParams{
		ID:        data.UUIDToPg(b.ID),
		ComplexID: data.UUIDToPg(b.ComplexID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return fmt.Errorf("payments: lock booking %s for its balance: %w", b.ID, err)
	}

	if balance := bookingstore.BookingFromDB(row).BalanceDue(); p.Amount > balance {
		return &AmountExceedsBalanceError{Balance: balance}
	}
	return nil
}

// guardSlotStillFree refuses to confirm a booking whose slot was taken while its
// payment was in flight. InsertAndConfirmBooking takes the court-day lock
// before calling it (see the comment there), so the lock covers the row read
// that guardBookingConfirmable performs as well as everything after it.
//
// This is the other half of the stale-pending carve-out. A public booking that
// goes unpaid past the payment expiry stops holding its slot, which is what lets
// the next client book those hours — but nothing used to stop the first booking
// from being confirmed afterwards by a late webhook, and two confirmed bookings
// on one court is exactly the failure the advisory lock exists to prevent.
//
// It deliberately gates on the slot rather than on the booking's age. A client
// whose payment merely arrived late, into hours nobody else took, still gets
// their booking; only a slot genuinely sold to somebody else is refused, with
// ErrSlotUnavailable.
//
// Confirmations only: a write that leaves the booking cancelled, completed or
// no_show does not claim the slot, so it is none of this function's business —
// which is what keeps the auto-refund path (a cancelled booking whose payment is
// being recorded so it can be sent back) working.
//
// The owner's blocked slots count as taken too. A public booking made before the
// owner blocked its hours is still pending once the payment expiry has passed, and
// a late payment must not sell those hours after the owner withdrew them.
func (m *Payments) guardSlotStillFree(ctx context.Context, tx pgx.Tx, qtx *db.Queries, b *bookingstore.Booking) error {
	if b.Status != "confirmed" {
		return nil
	}

	if err := slotguard.LockCourtDay(ctx, tx, b.CourtID, b.Date); err != nil {
		return err
	}

	// The court-day lock serializes this against booking inserts and blocked
	// slots, but creating an event, a fixed weekly series or a tournament match
	// takes no court-day lock: it takes the court's row FOR UPDATE and checks
	// the hours under it. Without the court row here the two contended on
	// nothing, and a late webhook could confirm over an event committed at the
	// same moment, each check blind to the other's uncommitted row. FOR SHARE,
	// after the advisory lock, as every booking insert takes it
	// (slotguard.LockCourtLive): it waits for such a creation to commit, and
	// the check below then sees what it wrote; one that starts later waits for
	// this confirmation and finds the booking. A court deleted meanwhile is
	// ErrRecordNotFound, as on the insert path.
	if err := slotguard.LockCourtLive(ctx, tx, b.CourtID); err != nil {
		return err
	}

	// The booking being confirmed overlaps itself, so it is the one row the
	// overlap query has to ignore.
	taken, err := bookingstore.SlotTaken(ctx, tx, b, m.PaymentExpiry, b.ID)
	if err != nil {
		return err
	}
	if taken {
		return bookingstore.ErrSlotUnavailable
	}

	blocked, err := qtx.BlockedSlotOverlapsBooking(ctx, db.BlockedSlotOverlapsBookingParams{
		CourtID:     data.UUIDToPg(b.CourtID),
		BookingDate: data.DateToPg(b.Date),
		StartTime:   data.TimeStrToPg(b.StartTime),
		//nolint:gosec // G115: a booking's length is bounded to one day in minutes; far below int32 range.
		DurationMinutes: int32(b.DurationMinutes),
	})
	if err != nil {
		return fmt.Errorf("check blocked slots: %w", err)
	}
	if blocked {
		return bookingstore.ErrSlotUnavailable
	}
	return nil
}

// Insert creates a new payment and populates p with its generated ID and timestamps.
func (m *Payments) Insert(ctx context.Context, p *Payment) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	// The store's own authorization check: see data.AssertTenant. Every caller
	// today reaches here through the HTTP chain, which has already decided
	// this; the point is that a caller which does not is refused rather than
	// trusted.
	if err := data.AssertTenant(ctx, p.ComplexID); err != nil {
		return err
	}

	dbPayment, err := m.Q.InsertPayment(ctx, insertPaymentParams(p))
	if err != nil {
		return err
	}
	p.fillInserted(dbPayment)
	return nil
}

// insertPaymentParams maps a new payment onto the insert query. Insert and
// InsertAndClaimRefund share it, so a new column reaches both.
func insertPaymentParams(p *Payment) db.InsertPaymentParams {
	return db.InsertPaymentParams{
		BookingID:      bookingIDToPg(p.BookingID),
		RegistrationID: data.UUIDPtrToPg(p.RegistrationID),
		ComplexID:      data.UUIDToPg(p.ComplexID),
		//nolint:gosec // G115: currency amount (cents) derived from booking.DepositAmount (validated <= Price) or
		// MercadoPago's own payment amount; realistically far below int32 range.
		Amount: int32(p.Amount),
		Method: db.PaymentMethod(p.Method),
		Status: db.PaymentStatus(p.Status),
		//nolint:gosec // G115: currency amount (cents) derived from a bounded calculation; far below int32 range.
		ServiceFee:     int32(p.ServiceFee),
		MpPaymentID:    data.TextToPg(p.MPPaymentID),
		MpPreferenceID: data.TextToPg(p.MPPreferenceID),
		StatusDetail:   data.TextToPg(p.StatusDetail),
	}
}

// bookingIDToPg is the booking column of a payment: NULL when the payment is a
// registration's, which carries no booking.
func bookingIDToPg(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{}
	}
	return data.UUIDToPg(id)
}

// fillInserted writes back the columns the database generated for a new row.
func (p *Payment) fillInserted(row db.Payment) {
	p.ID = data.PgToUUID(row.ID)
	p.CreatedAt = data.PgToTime(row.CreatedAt)
	p.UpdatedAt = data.PgToTime(row.UpdatedAt)
}

// GetByBookingID returns the payment for the given booking, or ErrRecordNotFound if none exists.
func (m *Payments) GetByBookingID(ctx context.Context, bookingID uuid.UUID) (*Payment, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbPayment, err := m.Q.GetPaymentByBookingID(ctx, data.UUIDToPg(bookingID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return paymentFromDB(dbPayment), nil
}

// ListByBookingID returns every payment row of the given booking, oldest first
// (the deposit before the balance). Unlike GetByBookingID this is not an
// arbitrary-one-row lookup: a booking can legitimately carry more than one
// payment row (idx_payments_booking is not unique), and callers that need the
// whole ledger — summing money for a refund, or showing every payment on the
// booking detail — must see all of them, not the MercadoPago-preferred single
// row GetByBookingID returns. An empty ledger is not an error: it returns a
// nil slice and a nil error, never ErrRecordNotFound.
func (m *Payments) ListByBookingID(ctx context.Context, bookingID uuid.UUID) ([]*Payment, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbPayments, err := m.Q.ListPaymentsByBookingID(ctx, data.UUIDToPg(bookingID))
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}

	if len(dbPayments) == 0 {
		return nil, nil
	}

	payments := make([]*Payment, len(dbPayments))
	for i, dbPayment := range dbPayments {
		payments[i] = paymentFromDB(dbPayment)
	}
	return payments, nil
}

// GetByMPPaymentID returns the payment matching the given MercadoPago payment ID, or ErrRecordNotFound if none exists.
func (m *Payments) GetByMPPaymentID(ctx context.Context, mpPaymentID string) (*Payment, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbPayment, err := m.Q.GetPaymentByMPID(ctx, pgtype.Text{String: mpPaymentID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return paymentFromDB(dbPayment), nil
}

// Update persists changes to an existing payment, returning ErrRecordNotFound if it no longer exists.
func (m *Payments) Update(ctx context.Context, p *Payment) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, p.ComplexID); err != nil {
		return err
	}

	dbPayment, err := m.Q.UpdatePayment(ctx, db.UpdatePaymentParams{
		Status:         db.PaymentStatus(p.Status),
		MpPaymentID:    data.TextToPg(p.MPPaymentID),
		MpPreferenceID: data.TextToPg(p.MPPreferenceID),
		//nolint:gosec // G115: currency amount (cents), bounded by the original payment amount; far below int32 range.
		// refund_amount is NOT NULL, so sqlc generates plain int32 rather than pgtype.Int4.
		RefundAmount: int32(p.RefundAmount),
		StatusDetail: data.TextToPg(p.StatusDetail),
		ID:           data.UUIDToPg(p.ID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return err
	}

	p.UpdatedAt = data.PgToTime(dbPayment.UpdatedAt)
	return nil
}

// RecordRejection writes a rejected attempt's MercadoPago id (p.MPPaymentID)
// and decline reason (p.StatusDetail) onto p.ID and reports whether it wrote.
// It writes only while the row is still unpaid and never touches its status:
// a row an approved payment settled after the caller read it keeps that
// payment's id, so its refund still goes to the money that was captured.
func (m *Payments) RecordRejection(ctx context.Context, p *Payment) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, p.ComplexID); err != nil {
		return false, err
	}

	n, err := m.Q.RecordPaymentRejection(ctx, db.RecordPaymentRejectionParams{
		ID:           data.UUIDToPg(p.ID),
		MpPaymentID:  data.TextToPg(p.MPPaymentID),
		StatusDetail: data.TextToPg(p.StatusDetail),
	})
	if err != nil {
		return false, fmt.Errorf("payments: record rejection: %w", err)
	}
	return n == 1, nil
}

// UpdateUnlessRefundPending is Update for the webhook's record of a refund
// MercadoPago reports. It writes only while the row is not 'refund_pending' and
// reports whether it wrote: the webhook read the row before it called
// MercadoPago, and a ClaimRefund that committed meanwhile owns the refund, so
// overwriting its status would strand the claim.
func (m *Payments) UpdateUnlessRefundPending(ctx context.Context, p *Payment) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, p.ComplexID); err != nil {
		return false, err
	}

	dbPayment, err := m.Q.UpdatePaymentUnlessRefundPending(ctx, updateUnlessRefundPendingParams(p))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	p.UpdatedAt = data.PgToTime(dbPayment.UpdatedAt)
	return true, nil
}

func updateUnlessRefundPendingParams(p *Payment) db.UpdatePaymentUnlessRefundPendingParams {
	return db.UpdatePaymentUnlessRefundPendingParams{
		Status:         db.PaymentStatus(p.Status),
		MpPaymentID:    data.TextToPg(p.MPPaymentID),
		MpPreferenceID: data.TextToPg(p.MPPreferenceID),
		//nolint:gosec // G115: currency amount (cents), bounded by the original payment amount; far below int32 range.
		RefundAmount: int32(p.RefundAmount),
		StatusDetail: data.TextToPg(p.StatusDetail),
		ID:           data.UUIDToPg(p.ID),
	}
}

// InsertAndConfirmBooking inserts a payment and updates the booking status atomically in a transaction.
//
// When the booking is being confirmed, the transaction opens with
// guardSlotStillFree: it takes the court/day advisory lock and refuses with
// ErrSlotUnavailable if another live booking now covers these hours.
func (m *Payments) InsertAndConfirmBooking(ctx context.Context, p *Payment, b *bookingstore.Booking) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, p.ComplexID); err != nil {
		return err
	}
	if err := data.AssertTenant(ctx, b.ComplexID); err != nil {
		return err
	}

	// Retried whole on a deadlock: the court-day lock and the court row lock
	// taken here meet series creation's court row and court-day locks in the
	// opposite order (bookingstore.Store.InsertSeriesWithBookings). The body does nothing
	// outside the transaction, and p and b are put back as they were before
	// every attempt, so an aborted attempt leaves no trace a replay could trip
	// on (attachClient, for one, skips a booking that already names a client).
	pIn, bIn := *p, *b
	return m.DB.RetryTx(ctx, data.DefaultTxAttempts, func(tx pgx.Tx, qtx *db.Queries) error {
		*p, *b = pIn, bIn
		return m.ConfirmInTx(ctx, tx, qtx, p, b)
	})
}

// ConfirmInTx is InsertAndConfirmBooking's transaction body, for a caller that
// already holds the transaction and must write the payment in the same one (the
// sales checkout records a sale and its booking balances atomically). It takes the
// court-day lock, the booking row lock and the slot guard exactly as
// InsertAndConfirmBooking does, so the lock order it documents still holds: a
// caller must lock the court day BEFORE it locks the booking row.
func (m *Payments) ConfirmInTx(ctx context.Context, tx pgx.Tx, qtx *db.Queries, p *Payment, b *bookingstore.Booking) error {
	// H-15: booking.Status was read by ConfirmPayment well before this
	// transaction opened — it is what that handler's own "cannot confirm a
	// cancelled booking" check is based on — so a client cancellation
	// committing in the gap used to reach the UPDATE below anyway, with
	// bookings_forbid_status_reversal refusing the cancelled -> confirmed
	// move and the plain database error falling through to a 500 after the
	// owner had already taken the client's cash. Re-reading the status here,
	// inside the same transaction the payment is about to be inserted in, is
	// what closes that gap: the check and the act can no longer be separated
	// by a race.
	// The court-day lock is taken FIRST, ahead of the row lock below, and the
	// order is the whole point.
	//
	// Two writers reach the same pair of locks from opposite directions.
	// InsertSafe (internal/bookings/store/bookings.go) takes lockCourtDays and then
	// UPDATEs the stale pending bookings that overlap the hours it wants
	// (ReleaseStalePendingOverlaps), which locks those rows. This transaction
	// used to take the booking row first — guardBookingConfirmable's
	// SELECT ... FOR UPDATE — and then ask guardSlotStillFree for the court-day
	// lock. Advisory lock then row, against row then advisory lock: a deadlock,
	// and a reachable one rather than a theoretical one. The row InsertSafe
	// wants to cancel is public, pending, unpaid and past its payment expiry —
	// exactly the booking an owner is confirming when they take cash at the
	// counter for a checkout that timed out. PostgreSQL breaks the tie by
	// aborting one side with 40P01, which is neither ErrSlotUnavailable nor
	// ErrDuplicateBooking and so falls through to the generic 500 that H-15
	// exists to remove.
	//
	// Taking the court-day lock first makes both paths agree, so they queue
	// instead of deadlocking. It is guarded by the same status test the two
	// guards use, so the refund path — which calls this with a cancelled
	// booking and must not be serialized against the court's day at all —
	// still takes no lock.
	if b.Status == "confirmed" {
		if err := slotguard.LockCourtDay(ctx, tx, b.CourtID, b.Date); err != nil {
			return err
		}
	}

	reopen, err := m.guardBookingConfirmable(ctx, tx, p, b)
	if err != nil {
		return err
	}

	// Before anything is written: the payment must still fit what the booking owes.
	// Like the status guard above, it reads the row under its lock. For a confirmation
	// that lock comes after the court-day lock taken at the top, the order the comment
	// at the top of this function documents; a completed booking takes no court-day lock
	// at all, so there is no later court-day lock for it to invert.
	if err := m.guardAmountWithinBalance(ctx, qtx, p, b); err != nil {
		return err
	}

	// Before anything is written: the slot this booking is about to claim has to
	// still be free. The court-day lock it asks for is already held from above;
	// pg_advisory_xact_lock is re-entrant within a transaction, so asking twice
	// is free rather than a second wait.
	if err := m.guardSlotAndReopen(ctx, tx, qtx, b, reopen); err != nil {
		return err
	}

	// INSERT payment.
	dbPayment, err := qtx.InsertPayment(ctx, db.InsertPaymentParams{
		BookingID:      bookingIDToPg(p.BookingID),
		RegistrationID: data.UUIDPtrToPg(p.RegistrationID),
		ComplexID:      data.UUIDToPg(p.ComplexID),
		//nolint:gosec // G115: currency amount (cents) derived from booking.DepositAmount (validated <= Price) or
		// MercadoPago's own payment amount; realistically far below int32 range.
		Amount: int32(p.Amount),
		Method: db.PaymentMethod(p.Method),
		Status: db.PaymentStatus(p.Status),
		//nolint:gosec // G115: currency amount (cents) derived from a bounded calculation; far below int32 range.
		ServiceFee:     int32(p.ServiceFee),
		MpPaymentID:    data.TextToPg(p.MPPaymentID),
		MpPreferenceID: data.TextToPg(p.MPPreferenceID),
		StatusDetail:   data.TextToPg(p.StatusDetail),
	})
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}

	p.ID = data.PgToUUID(dbPayment.ID)
	p.CreatedAt = data.PgToTime(dbPayment.CreatedAt)
	p.UpdatedAt = data.PgToTime(dbPayment.UpdatedAt)

	// UPDATE booking status.
	dbBooking, err := qtx.UpdateBooking(ctx, db.UpdateBookingParams{
		Status:           db.BookingStatus(b.Status),
		CollectionStatus: string(b.CollectionStatus),
		RefundStatus:     string(b.RefundStatus),
		Notes:            data.TextToPg(b.Notes),
		//nolint:gosec // G115: DepositAmount bounded to Price (bookings_create.go validation); far below int32 range.
		DepositAmount: int32(b.DepositAmount),
		ID:            data.UUIDToPg(b.ID),
		// Neither InsertAndConfirmBooking nor ConfirmWebhookPayment sets the
		// marker; both carry through whatever the in-memory Booking already
		// holds (nil, for every booking reaching these two confirm paths).
		RefundIntentAt: data.TimePtrToPg(b.RefundIntentAt),
	})
	if err != nil {
		return fmt.Errorf("update booking: %w", err)
	}

	if err := afterBookingWrite(ctx, tx, qtx, b, reopen); err != nil {
		return err
	}

	b.UpdatedAt = data.PgToTime(dbBooking.UpdatedAt)
	return nil
}

// attachClient gives a storefront booking its client in the transaction that
// records its payment (owner rule 2026-11-10: a client exists only once a
// booking succeeded). Every payment write runs it — the webhook, the counter,
// the sales checkout and a late payment on a cancelled booking — so a booking
// that holds money always names somebody; one that already does is untouched.
// See bookingstore.AttachClientFromSnapshot.
func attachClient(ctx context.Context, tx pgx.Tx, qtx *db.Queries, b *bookingstore.Booking) error {
	if b.HasClient() {
		return nil
	}
	return bookingstore.AttachClientFromSnapshot(ctx, tx, qtx, b)
}

// ConfirmWebhookPayment updates an existing payment and confirms the booking atomically in a transaction.
// Used by the webhook handler to update the payment created during public booking.
// The payment is written only while the row still takes it (TakePaymentRow);
// otherwise nothing is written and ErrPaymentRowTaken is returned.
//
// Like InsertAndConfirmBooking, a confirmation runs under guardSlotStillFree and
// is refused with ErrSlotUnavailable when the slot was taken meanwhile.
func (m *Payments) ConfirmWebhookPayment(ctx context.Context, p *Payment, b *bookingstore.Booking) error {
	_, err := m.confirmWebhookPayment(ctx, p, b, false)
	return err
}

// RecordWebhookRefund is ConfirmWebhookPayment for the webhook's record of a
// refund MercadoPago reports: the payment is written only while it is not
// 'refund_pending', and when it is not written the booking is not written either.
// It reports whether it wrote. See UpdateUnlessRefundPending for why.
func (m *Payments) RecordWebhookRefund(ctx context.Context, p *Payment, b *bookingstore.Booking) (bool, error) {
	return m.confirmWebhookPayment(ctx, p, b, true)
}

// confirmWebhookPayment is the transaction behind ConfirmWebhookPayment and
// RecordWebhookRefund. unlessRefundPending makes the payment write conditional;
// a refused write rolls nothing back because nothing was written yet, and the
// booking is left alone.
func (m *Payments) confirmWebhookPayment(ctx context.Context, p *Payment, b *bookingstore.Booking, unlessRefundPending bool) (bool, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, p.ComplexID); err != nil {
		return false, err
	}
	if err := data.AssertTenant(ctx, b.ComplexID); err != nil {
		return false, err
	}

	// Retried whole on a deadlock, for the reason InsertAndConfirmBooking
	// gives, with p and b put back before every attempt.
	written := false
	pIn, bIn := *p, *b
	err := m.DB.RetryTx(ctx, data.DefaultTxAttempts, func(tx pgx.Tx, qtx *db.Queries) error {
		*p, *b = pIn, bIn
		written = false
		// Before anything is written: the booking may still be confirmed, and
		// the slot it is about to claim is still free.
		reopen, err := m.guardWebhookConfirmation(ctx, tx, qtx, p, b)
		if err != nil {
			return err
		}

		// UPDATE existing payment.
		updatedAt, ok, err := updateWebhookPayment(ctx, qtx, p, unlessRefundPending)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		p.UpdatedAt = data.PgToTime(updatedAt)

		// UPDATE booking status.
		dbBooking, err := qtx.UpdateBooking(ctx, db.UpdateBookingParams{
			Status:           db.BookingStatus(b.Status),
			CollectionStatus: string(b.CollectionStatus),
			RefundStatus:     string(b.RefundStatus),
			Notes:            data.TextToPg(b.Notes),
			//nolint:gosec // G115: DepositAmount bounded to Price (bookings_create.go validation); far below int32 range.
			DepositAmount: int32(b.DepositAmount),
			ID:            data.UUIDToPg(b.ID),
			// Neither InsertAndConfirmBooking nor ConfirmWebhookPayment sets the
			// marker; both carry through whatever the in-memory Booking already
			// holds (nil, for every booking reaching these two confirm paths).
			RefundIntentAt: data.TimePtrToPg(b.RefundIntentAt),
		})
		if err != nil {
			return fmt.Errorf("update booking: %w", err)
		}

		if err := afterBookingWrite(ctx, tx, qtx, b, reopen); err != nil {
			return err
		}

		b.UpdatedAt = data.PgToTime(dbBooking.UpdatedAt)
		written = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return written, nil
}

// updateWebhookPayment is the payment write inside confirmWebhookPayment. It
// reports false, with no error, when unlessRefundPending refused the write
// because the payment is 'refund_pending'.
func updateWebhookPayment(ctx context.Context, qtx *db.Queries, p *Payment, unlessRefundPending bool) (pgtype.Timestamptz, bool, error) {
	if unlessRefundPending {
		dbPayment, err := qtx.UpdatePaymentUnlessRefundPending(ctx, updateUnlessRefundPendingParams(p))
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.Timestamptz{}, false, nil
		}
		if err != nil {
			return pgtype.Timestamptz{}, false, fmt.Errorf("update payment: %w", err)
		}
		return dbPayment.UpdatedAt, true, nil
	}
	// Conditional (TakePaymentRow): the caller decided the row takes this
	// payment from a read taken outside this transaction, and another approved
	// payment may have settled the row since. The UPDATE re-checks the row
	// after any lock it waited on, so only one of two such payments lands.
	dbPayment, err := qtx.TakePaymentRow(ctx, db.TakePaymentRowParams{
		Status:         db.PaymentStatus(p.Status),
		MpPaymentID:    data.TextToPg(p.MPPaymentID),
		MpPreferenceID: data.TextToPg(p.MPPreferenceID),
		//nolint:gosec // G115: currency amount (cents), bounded by the original payment amount; far below int32 range.
		// refund_amount is NOT NULL, so sqlc generates plain int32 rather than pgtype.Int4.
		RefundAmount: int32(p.RefundAmount),
		StatusDetail: data.TextToPg(p.StatusDetail),
		ID:           data.UUIDToPg(p.ID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.Timestamptz{}, false, ErrPaymentRowTaken
	}
	if err != nil {
		return pgtype.Timestamptz{}, false, fmt.Errorf("update payment: %w", err)
	}
	return dbPayment.UpdatedAt, true, nil
}

func paymentFromDB(p db.Payment) *Payment {
	return &Payment{
		ID:             data.PgToUUID(p.ID),
		BookingID:      data.PgToUUID(p.BookingID),
		RegistrationID: data.PgToUUIDPtr(p.RegistrationID),
		ComplexID:      data.PgToUUID(p.ComplexID),
		Amount:         int(p.Amount),
		ServiceFee:     int(p.ServiceFee),
		Method:         string(p.Method),
		Status:         string(p.Status),
		MPPaymentID:    data.PgToTextPtr(p.MpPaymentID),
		MPPreferenceID: data.PgToTextPtr(p.MpPreferenceID),
		// refund_amount is NOT NULL: sqlc's plain int32 for it
		// reads correctly with a direct conversion; PgToInt's zero-for-invalid
		// branch no longer applies to this column.
		RefundAmount: int(p.RefundAmount),
		StatusDetail: data.PgToTextPtr(p.StatusDetail),
		CreatedAt:    data.PgToTime(p.CreatedAt),
		UpdatedAt:    data.PgToTime(p.UpdatedAt),
	}
}
