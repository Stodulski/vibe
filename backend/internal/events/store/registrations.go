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
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// ErrHoldLimit reports an address that already holds the most spots it may.
var ErrHoldLimit = errors.New("events: too many live spot holds from this address")

const tokenByteLength = 32

// NewAccessToken returns a fresh registration access token and the hash that is
// stored for it.
func NewAccessToken() (plaintext string, hash []byte, err error) {
	buf := make([]byte, tokenByteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generate registration token: %w", err)
	}
	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, HashAccessToken(plaintext), nil
}

// HashAccessToken is what is stored for an access token: SHA-256, raw, as for
// every other bearer token in this codebase.
func HashAccessToken(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}

// SignUpInput is a person asking for a spot.
type SignUpInput struct {
	EventID   uuid.UUID
	FirstName string
	LastName  string
	Phone     string
	Email     *string
	// ServiceFee is the flat fee the person pays on top of the event's price.
	ServiceFee int
	// IP is the rate-limit key of the address signing up, and MaxHoldsPerIP the
	// most live holds one address may have across every event.
	IP            string
	MaxHoldsPerIP int
}

// SignUp holds one spot of an event for a person and returns the registration
// with its access token. The spot counts against capacity until HeldUntil; an
// approved payment confirms it, and a hold nobody pays stops counting by
// itself.
//
// It takes the event's row lock first, so two people racing for the last spot
// are serialized: the first commits its hold, the second reads the full count
// and is refused with ErrEventFull. The price is the event's, read under that
// lock, never the caller's.
//
// ErrEventClosed is returned for an event that is cancelled or has begun,
// ErrAlreadyRegistered for a phone that already holds a live spot, and
// ErrHoldLimit for an address over its cap.
func (m *Store) SignUp(ctx context.Context, in SignUpInput) (*Registration, string, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	token, hash, err := NewAccessToken()
	if err != nil {
		return nil, "", err
	}
	var row db.OpenPlayRegistration
	err = m.DB.RetryTx(ctx, data.DefaultTxAttempts, func(tx pgx.Tx, q *db.Queries) error {
		event, err := q.LockOpenPlayEvent(ctx, data.UUIDToPg(in.EventID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("events: lock event: %w", err)
		}
		if err := data.AssertTenant(ctx, data.PgToUUID(event.ComplexID)); err != nil {
			return err
		}
		if event.Status != EventActive || !eventFromDB(event).StartsAt().After(time.Now()) {
			return ErrEventClosed
		}
		if err := m.checkHoldLimit(ctx, tx, in); err != nil {
			return err
		}
		if err := q.ExpireOpenPlayHoldsOfPhone(ctx, db.ExpireOpenPlayHoldsOfPhoneParams{
			EventID: event.ID, Phone: in.Phone,
		}); err != nil {
			return fmt.Errorf("events: expire stale holds: %w", err)
		}
		taken, err := q.CountOpenPlaySpotsTaken(ctx, db.CountOpenPlaySpotsTakenParams{EventID: event.ID})
		if err != nil {
			return fmt.Errorf("events: count spots: %w", err)
		}
		if taken >= event.Capacity {
			return ErrEventFull
		}
		row, err = q.InsertOpenPlayRegistration(ctx, db.InsertOpenPlayRegistrationParams{
			EventID: event.ID, ComplexID: event.ComplexID,
			FirstName: in.FirstName, LastName: in.LastName, Phone: in.Phone, Email: data.TextToPg(in.Email),
			Price: event.Price,
			//nolint:gosec // G115: the flat service fee on a price in centavos; far below int32 range.
			ServiceFee:      int32(in.ServiceFee),
			HeldUntil:       data.TimeToPg(time.Now().Add(m.PaymentExpiry)),
			AccessTokenHash: hash,
			HoldIp:          data.TextToPg(&in.IP),
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == data.SQLStateUniqueViolation {
				return ErrAlreadyRegistered
			}
			return fmt.Errorf("events: insert registration: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return registrationFromDB(row), token, nil
}

// checkHoldLimit refuses an address that already holds its cap of live spots,
// counted under a transaction-scoped advisory lock on the address so two
// requests from one address cannot both see room for one more.
func (m *Store) checkHoldLimit(ctx context.Context, tx pgx.Tx, in SignUpInput) error {
	if in.MaxHoldsPerIP <= 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "event-hold-ip:"+in.IP); err != nil {
		return fmt.Errorf("events: lock hold address: %w", err)
	}
	var live int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM open_play_registrations
		WHERE hold_ip = $1 AND status = 'held' AND held_until > NOW()`, in.IP).Scan(&live)
	if err != nil {
		return fmt.Errorf("events: count live holds: %w", err)
	}
	if live >= in.MaxHoldsPerIP {
		return ErrHoldLimit
	}
	return nil
}

// GetRegistration returns one registration, or ErrRecordNotFound. A scoped
// caller only ever sees its own tenant's.
func (m *Store) GetRegistration(ctx context.Context, id uuid.UUID) (*Registration, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetOpenPlayRegistration(ctx, db.GetOpenPlayRegistrationParams{
		ID: data.UUIDToPg(id), ComplexID: data.TenantParam(ctx),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, fmt.Errorf("events: get registration: %w", err)
	}
	return registrationFromDB(row), nil
}

// GetRegistrationByToken returns the registration an access token names, or
// ErrRecordNotFound. It finds the row whatever its status; the caller decides
// what the status allows.
func (m *Store) GetRegistrationByToken(ctx context.Context, plaintext string) (*Registration, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetOpenPlayRegistrationByTokenHash(ctx, HashAccessToken(plaintext))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, fmt.Errorf("events: get registration by token: %w", err)
	}
	return registrationFromDB(row), nil
}

// ListRegistrations returns every registration of an event with its payment,
// oldest first. liveOnly leaves out the holds that expired and the
// cancellations that never carried money.
func (m *Store) ListRegistrations(ctx context.Context, eventID uuid.UUID, liveOnly bool) ([]*RegistrationRow, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListOpenPlayRegistrations(ctx, db.ListOpenPlayRegistrationsParams{
		EventID: data.UUIDToPg(eventID), LiveOnly: liveOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("events: list registrations: %w", err)
	}
	out := make([]*RegistrationRow, len(rows))
	for i, r := range rows {
		row := &RegistrationRow{Registration: *registrationFromDB(db.OpenPlayRegistration{
			ID: r.ID, EventID: r.EventID, ComplexID: r.ComplexID, FirstName: r.FirstName, LastName: r.LastName,
			Phone: r.Phone, Email: r.Email, Status: r.Status, Price: r.Price, ServiceFee: r.ServiceFee,
			HeldUntil: r.HeldUntil, ConfirmedAt: r.ConfirmedAt, CancelledAt: r.CancelledAt,
			CancelledBy: r.CancelledBy, CreatedAt: r.CreatedAt,
		})}
		if r.PaymentID.Valid {
			row.Payment = &RegistrationPayment{
				ID:           data.PgToUUID(r.PaymentID),
				Status:       r.PaymentStatus,
				Amount:       int(r.PaymentAmount),
				ServiceFee:   int(r.PaymentServiceFee),
				RefundAmount: int(r.PaymentRefundAmount),
				MPPaymentID:  data.PgToTextPtr(r.PaymentMpPaymentID),
			}
		}
		out[i] = row
	}
	return out, nil
}

// CancelRegistration ends one registration: the person giving up their spot, or
// an owner action. It answers ErrNotCancellable for one that is already over.
func (m *Store) CancelRegistration(ctx context.Context, id uuid.UUID, by string) (*Registration, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.CancelOpenPlayRegistration(ctx, db.CancelOpenPlayRegistrationParams{
		ID: data.UUIDToPg(id), CancelledBy: data.TextToPg(&by),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotCancellable
		}
		return nil, fmt.Errorf("events: cancel registration: %w", err)
	}
	return registrationFromDB(row), nil
}

// ExpiredHold is a hold the sweep closed.
type ExpiredHold struct {
	ID        uuid.UUID
	ComplexID uuid.UUID
	EventID   uuid.UUID
}

// ExpireHolds marks expired the holds nobody paid in time and returns them. They
// stopped counting against capacity the moment HeldUntil passed; this only
// closes the rows, so the sweep running late changes nothing a visitor sees.
// Only the sweep calls it, under the tenant bypass.
func (m *Store) ExpireHolds(ctx context.Context) ([]ExpiredHold, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ExpireOpenPlayHolds(ctx)
	if err != nil {
		return nil, fmt.Errorf("events: expire holds: %w", err)
	}
	out := make([]ExpiredHold, len(rows))
	for i, r := range rows {
		out[i] = ExpiredHold{ID: data.PgToUUID(r.ID), ComplexID: data.PgToUUID(r.ComplexID), EventID: data.PgToUUID(r.EventID)}
	}
	return out, nil
}

// PendingRefund is a registration the owner's cancellation ended whose money has
// not been sent back.
type PendingRefund struct {
	RegistrationID uuid.UUID
	ComplexID      uuid.UUID
}

// OwnerCancelledStillPaid returns the registrations an event cancellation ended
// more than grace ago whose MercadoPago payment still reads collected: the
// refund the cancel request itself never finished. Only the sweep calls it,
// under the tenant bypass.
func (m *Store) OwnerCancelledStillPaid(ctx context.Context, grace time.Duration, limit int) ([]PendingRefund, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListOwnerCancelledRegistrationsStillPaid(ctx, db.ListOwnerCancelledRegistrationsStillPaidParams{
		OlderThan: data.TimeToPg(time.Now().Add(-grace)),
		//nolint:gosec // G115: a batch size chosen by the caller, far below int32 range.
		PageLimit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("events: list unrefunded cancellations: %w", err)
	}
	out := make([]PendingRefund, len(rows))
	for i, r := range rows {
		out[i] = PendingRefund{RegistrationID: data.PgToUUID(r.ID), ComplexID: data.PgToUUID(r.ComplexID)}
	}
	return out, nil
}

func registrationFromDB(r db.OpenPlayRegistration) *Registration {
	return &Registration{
		ID:          data.PgToUUID(r.ID),
		EventID:     data.PgToUUID(r.EventID),
		ComplexID:   data.PgToUUID(r.ComplexID),
		FirstName:   r.FirstName,
		LastName:    r.LastName,
		Phone:       r.Phone,
		Email:       data.PgToTextPtr(r.Email),
		Status:      r.Status,
		Price:       int(r.Price),
		ServiceFee:  int(r.ServiceFee),
		HeldUntil:   data.PgToTime(r.HeldUntil),
		ConfirmedAt: data.PgToTimePtr(r.ConfirmedAt),
		CancelledAt: data.PgToTimePtr(r.CancelledAt),
		CancelledBy: data.PgToTextPtr(r.CancelledBy),
		CreatedAt:   data.PgToTime(r.CreatedAt),
	}
}
