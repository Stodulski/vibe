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
		if err := m.registrationConfirmable(ctx, tx, qtx, event, reg); err != nil {
			return err
		}
		// A hold of the same phone that ran out but was not swept yet still counts
		// for the one-live-spot-per-phone index: closed here, as sign-up does, so
		// the confirmation below cannot fail on it.
		if err := qtx.ExpireOpenPlayHoldsOfPhone(ctx, db.ExpireOpenPlayHoldsOfPhoneParams{EventID: event.ID, Phone: reg.Phone}); err != nil {
			return fmt.Errorf("expire stale holds of the phone: %w", err)
		}
		if _, err := qtx.ConfirmOpenPlayRegistration(ctx, reg.ID); err != nil {
			return fmt.Errorf("confirm registration: %w", err)
		}
		return writeRegistrationPayment(ctx, qtx, p)
	})
}

// registrationConfirmable decides, under the event lock, whether a payment may
// confirm the registration.
func (m *Payments) registrationConfirmable(ctx context.Context, tx pgx.Tx, qtx *db.Queries, event db.OpenPlayEvent, reg db.OpenPlayRegistration) error {
	switch {
	case reg.Status == "confirmed":
		return ErrRegistrationAlreadyPaid
	case reg.Status == "cancelled", event.Status != "active":
		return ErrRegistrationUnavailable
	}
	// A pair that pays after its category's draw was made is not in it: the spot
	// is gone and the payment goes back. The category row is read under its lock,
	// the one the draw holds (tournaments' lockedCategory), always taken after the
	// event's: a draw in progress is waited for and then seen, and a draw that
	// starts after this payment lists the pair it confirmed.
	if event.TournamentID.Valid {
		var drawn bool
		if err := tx.QueryRow(ctx, `SELECT draw_generated_at IS NOT NULL FROM tournament_categories WHERE event_id = $1 FOR UPDATE`, event.ID).Scan(&drawn); err != nil {
			return fmt.Errorf("read the category's draw: %w", err)
		}
		if drawn {
			return ErrRegistrationUnavailable
		}
	}
	if err := playersFree(ctx, qtx, event, reg); err != nil {
		return err
	}
	// A hold still inside its window already reserves this spot: it was
	// counted when it signed up, and every other sign-up since was counted
	// with it. Counting it again as if it were a new sign-up refunded an
	// on-time payer whenever the event sat at its edge with a lapsed hold kept
	// for its payment check (CountOpenPlaySpotsTaken). Only a payment after its
	// hold ran out competes for a spot. The database's clock decides, as it
	// does for the count.
	if reg.Status == "held" {
		var live bool
		if err := tx.QueryRow(ctx, `SELECT held_until > NOW() FROM open_play_registrations WHERE id = $1`, reg.ID).Scan(&live); err != nil {
			return fmt.Errorf("read the registration's hold: %w", err)
		}
		if live {
			return nil
		}
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

// playersFree refuses a payment whose people hold another live spot of the
// event: a hold that ran out let them sign up again, and confirming the old one
// would give them two (and, by phone, fail on the one-live-spot index, which
// retried the webhook forever instead of refunding). For a tournament pair both
// players count, as payer or as partner.
func playersFree(ctx context.Context, qtx *db.Queries, event db.OpenPlayEvent, reg db.OpenPlayRegistration) error {
	taken, err := qtx.PhoneHoldsAnotherLiveSpot(ctx, db.PhoneHoldsAnotherLiveSpotParams{
		EventID: event.ID, Phone: reg.Phone, ExceptID: reg.ID,
	})
	if err != nil {
		return fmt.Errorf("check the phone's other spots: %w", err)
	}
	if !taken && event.TournamentID.Valid {
		if taken, err = qtx.PairOfRegistrationPlayersTaken(ctx, reg.ID); err != nil {
			return fmt.Errorf("check the pair's players: %w", err)
		}
	}
	if taken {
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
