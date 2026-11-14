package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// Why an approved payment cannot confirm the registration it was made for. Both
// mean the money arrived and has to go back.
var (
	// ErrRegistrationUnavailable is returned by ConfirmRegistrationPayment when
	// the registration was cancelled, its event was cancelled, or its hold ran
	// out and the spot was taken by someone else meanwhile.
	ErrRegistrationUnavailable = errors.New("registration can no longer be confirmed")
	// ErrRegistrationAlreadyPaid is returned when the registration is already
	// confirmed by another payment: this one is a duplicate.
	ErrRegistrationAlreadyPaid = errors.New("registration already paid")
)

// ListByRegistrationID returns every payment row of an event registration,
// oldest first. Like ListByBookingID an empty ledger is a nil slice, not an
// error.
func (m *Payments) ListByRegistrationID(ctx context.Context, registrationID uuid.UUID) ([]*Payment, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListPaymentsByRegistrationID(ctx, data.UUIDToPg(registrationID))
	if err != nil {
		return nil, fmt.Errorf("list registration payments: %w", err)
	}
	var out []*Payment
	for _, row := range rows {
		out = append(out, paymentFromDB(row))
	}
	return out, nil
}

// ConfirmRegistrationPayment settles an approved payment against the event
// registration it paid for: the payment row is written and the registration
// confirmed in one transaction.
//
// It takes the event's row lock first, the lock every sign-up takes, so the
// capacity decision is made against the same count the sign-ups see. A
// registration still holding its spot is confirmed. One whose hold ran out is
// confirmed only if the event still has room for it (the count never includes
// the registration itself). When it cannot be confirmed nothing is written and
// ErrRegistrationUnavailable is returned, for the caller to record the payment
// and send the money back; ErrRegistrationAlreadyPaid says the same for a
// second payment on a registration already confirmed.
//
// p is the checkout row sign-up wrote when it has an ID, and is updated;
// otherwise it is a new row and is inserted.
func (m *Payments) ConfirmRegistrationPayment(ctx context.Context, p *Payment, registrationID uuid.UUID) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, p.ComplexID); err != nil {
		return err
	}
	return m.DB.WithTx(ctx, func(tx pgx.Tx, qtx *db.Queries) error {
		reg, err := qtx.GetOpenPlayRegistration(ctx, db.GetOpenPlayRegistrationParams{
			ID: data.UUIDToPg(registrationID), ComplexID: data.TenantParam(ctx),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("read registration: %w", err)
		}
		event, err := qtx.LockOpenPlayEvent(ctx, reg.EventID)
		if err != nil {
			return fmt.Errorf("lock event: %w", err)
		}
		// Read again under the lock: a cancellation committed while this one waited.
		if reg, err = qtx.GetOpenPlayRegistration(ctx, db.GetOpenPlayRegistrationParams{
			ID: reg.ID, ComplexID: data.TenantParam(ctx),
		}); err != nil {
			return fmt.Errorf("re-read registration: %w", err)
		}
		if err := m.registrationConfirmable(ctx, qtx, event, reg); err != nil {
			return err
		}
		if _, err := qtx.ConfirmOpenPlayRegistration(ctx, reg.ID); err != nil {
			return fmt.Errorf("confirm registration: %w", err)
		}
		return writeRegistrationPayment(ctx, qtx, p)
	})
}

// registrationConfirmable decides, under the event lock, whether a payment may
// confirm the registration.
func (m *Payments) registrationConfirmable(ctx context.Context, qtx *db.Queries, event db.OpenPlayEvent, reg db.OpenPlayRegistration) error {
	switch {
	case reg.Status == "confirmed":
		return ErrRegistrationAlreadyPaid
	case reg.Status == "cancelled", event.Status != "active":
		return ErrRegistrationUnavailable
	}
	taken, err := qtx.CountOpenPlaySpotsTaken(ctx, db.CountOpenPlaySpotsTakenParams{
		EventID: event.ID, Except: reg.ID,
	})
	if err != nil {
		return fmt.Errorf("count spots: %w", err)
	}
	if taken >= event.Capacity {
		return ErrRegistrationUnavailable
	}
	return nil
}

// writeRegistrationPayment updates the checkout row sign-up wrote, or inserts
// the row when there is none, and fills p in.
func writeRegistrationPayment(ctx context.Context, qtx *db.Queries, p *Payment) error {
	if p.ID == uuid.Nil {
		row, err := qtx.InsertPayment(ctx, insertPaymentParams(p))
		if err != nil {
			return fmt.Errorf("insert payment: %w", err)
		}
		p.fillInserted(row)
		return nil
	}
	row, err := qtx.UpdatePayment(ctx, db.UpdatePaymentParams{
		Status:         db.PaymentStatus(p.Status),
		MpPaymentID:    data.TextToPg(p.MPPaymentID),
		MpPreferenceID: data.TextToPg(p.MPPreferenceID),
		//nolint:gosec // G115: currency amount (cents), bounded by the original payment amount; far below int32 range.
		RefundAmount: int32(p.RefundAmount),
		StatusDetail: data.TextToPg(p.StatusDetail),
		ID:           data.UUIDToPg(p.ID),
	})
	if err != nil {
		return fmt.Errorf("update payment: %w", err)
	}
	p.UpdatedAt = data.PgToTime(row.UpdatedAt)
	return nil
}
