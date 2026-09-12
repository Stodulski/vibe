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
	ID             uuid.UUID `json:"id"`
	BookingID      uuid.UUID `json:"booking_id"`
	ComplexID      uuid.UUID `json:"complex_id"`
	Amount         int       `json:"amount"`
	ServiceFee     int       `json:"service_fee"`
	Method         string    `json:"method"`
	Status         string    `json:"status"`
	MPPaymentID    *string   `json:"mp_payment_id,omitempty"`
	MPPreferenceID *string   `json:"mp_preference_id,omitempty"`
	RefundAmount   int       `json:"refund_amount"`
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
	// from configuration by NewModels. The confirmation-time slot guard needs it
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
func (m *Payments) guardBookingConfirmable(ctx context.Context, tx pgx.Tx, b *bookingstore.Booking) error {
	if b.Status != "confirmed" {
		return nil
	}

	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM bookings WHERE id = $1 FOR UPDATE`, data.UUIDToPg(b.ID)).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return err
	}
	// H-23: cancelled is answered with its own sentinel. It is the only one of
	// the three where the client's money must be sent back, and collapsing it
	// into ErrBookingNotConfirmable is what routed a paid-but-cancelled booking
	// past the webhook's refund branch. See ErrBookingCancelled.
	if status == "cancelled" {
		return bookingstore.ErrBookingCancelled
	}
	if status == "completed" || status == "no_show" {
		return bookingstore.ErrBookingNotConfirmable
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
func (m *Payments) guardSlotStillFree(ctx context.Context, tx pgx.Tx, b *bookingstore.Booking) error {
	if b.Status != "confirmed" {
		return nil
	}

	if err := slotguard.LockCourtDay(ctx, tx, b.CourtID, b.Date); err != nil {
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
	return nil
}

// Insert creates a new payment and populates p with its generated ID and timestamps.
func (m *Payments) Insert(ctx context.Context, p *Payment) error {
	dbPayment, err := m.Q.InsertPayment(ctx, db.InsertPaymentParams{
		BookingID: data.UUIDToPg(p.BookingID),
		ComplexID: data.UUIDToPg(p.ComplexID),
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
		return err
	}

	p.ID = data.PgToUUID(dbPayment.ID)
	p.CreatedAt = data.PgToTime(dbPayment.CreatedAt)
	p.UpdatedAt = data.PgToTime(dbPayment.UpdatedAt)
	return nil
}

// GetByBookingID returns the payment for the given booking, or ErrRecordNotFound if none exists.
func (m *Payments) GetByBookingID(ctx context.Context, bookingID uuid.UUID) (*Payment, error) {
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

// InsertAndConfirmBooking inserts a payment and updates the booking status atomically in a transaction.
//
// When the booking is being confirmed, the transaction opens with
// guardSlotStillFree: it takes the court/day advisory lock and refuses with
// ErrSlotUnavailable if another live booking now covers these hours.
func (m *Payments) InsertAndConfirmBooking(ctx context.Context, p *Payment, b *bookingstore.Booking) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	tx, err := m.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// Rollback is a no-op once Commit succeeds (pgx returns ErrTxClosed, which is expected).
	defer func() { _ = tx.Rollback(ctx) }()

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
	// InsertSafe (internal/data/bookings.go) takes lockCourtDays and then
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

	if err := m.guardBookingConfirmable(ctx, tx, b); err != nil {
		return err
	}

	// Before anything is written: the slot this booking is about to claim has to
	// still be free. The court-day lock it asks for is already held from above;
	// pg_advisory_xact_lock is re-entrant within a transaction, so asking twice
	// is free rather than a second wait.
	if err := m.guardSlotStillFree(ctx, tx, b); err != nil {
		return err
	}

	qtx := m.Q.WithTx(tx)

	// INSERT payment.
	dbPayment, err := qtx.InsertPayment(ctx, db.InsertPaymentParams{
		BookingID: data.UUIDToPg(p.BookingID),
		ComplexID: data.UUIDToPg(p.ComplexID),
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
		CollectionStatus: b.CollectionStatus,
		RefundStatus:     b.RefundStatus,
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

	b.UpdatedAt = data.PgToTime(dbBooking.UpdatedAt)

	return tx.Commit(ctx)
}

// ConfirmWebhookPayment updates an existing payment and confirms the booking atomically in a transaction.
// Used by the webhook handler to update the payment created during public booking.
//
// Like InsertAndConfirmBooking, a confirmation runs under guardSlotStillFree and
// is refused with ErrSlotUnavailable when the slot was taken meanwhile.
func (m *Payments) ConfirmWebhookPayment(ctx context.Context, p *Payment, b *bookingstore.Booking) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	tx, err := m.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// Rollback is a no-op once Commit succeeds (pgx returns ErrTxClosed, which is expected).
	defer func() { _ = tx.Rollback(ctx) }()

	// Before anything is written: the slot this booking is about to claim has to
	// still be free.
	if err := m.guardSlotStillFree(ctx, tx, b); err != nil {
		return err
	}

	qtx := m.Q.WithTx(tx)

	// UPDATE existing payment.
	dbPayment, err := qtx.UpdatePayment(ctx, db.UpdatePaymentParams{
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
		return fmt.Errorf("update payment: %w", err)
	}

	p.UpdatedAt = data.PgToTime(dbPayment.UpdatedAt)

	// UPDATE booking status.
	dbBooking, err := qtx.UpdateBooking(ctx, db.UpdateBookingParams{
		Status:           db.BookingStatus(b.Status),
		CollectionStatus: b.CollectionStatus,
		RefundStatus:     b.RefundStatus,
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

	b.UpdatedAt = data.PgToTime(dbBooking.UpdatedAt)

	return tx.Commit(ctx)
}

func paymentFromDB(p db.Payment) *Payment {
	return &Payment{
		ID:             data.PgToUUID(p.ID),
		BookingID:      data.PgToUUID(p.BookingID),
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
