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
	"github.com/jackc/pgx/v5/pgtype"

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
	// Gender and Level are what the person declared, stored on the registration.
	Gender *string
	Level  *int
	// ServiceFee is the flat fee the person pays on top of the event's price.
	ServiceFee int
	// IP is the rate-limit key of the address signing up, and MaxHoldsPerIP the
	// most live holds one address may have across every event.
	IP            string
	MaxHoldsPerIP int
	// Hooks is set by the tournaments domain for a pair's sign-up: what only it
	// knows how to check and write, run inside the same transaction and under the
	// same event lock. Nil for an open-play sign-up.
	Hooks *SignUpHooks
}

// SignUpHooks are the steps a tournament adds to a sign-up. They run inside the
// transaction that holds the event's row lock, so what they read cannot change
// under them.
type SignUpHooks struct {
	// Admit runs after the event is locked and found live, before anything is
	// held. It refuses with ErrEventClosed (the draw exists) or
	// ErrAlreadyRegistered (a player is already in the category).
	Admit func(ctx context.Context, q *db.Queries, event db.OpenPlayEvent) error
	// Attach runs after the registration is inserted, to write what belongs to it.
	Attach func(ctx context.Context, q *db.Queries, reg db.OpenPlayRegistration) error
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
		var err error
		row, err = m.signUpInTx(ctx, tx, q, in, hash)
		return err
	})
	if err != nil {
		return nil, "", err
	}
	return registrationFromDB(row), token, nil
}

// signUpInTx is SignUp's transaction: lock the event, refuse what the event or
// the person cannot take, count, and insert the hold.
func (m *Store) signUpInTx(ctx context.Context, tx pgx.Tx, q *db.Queries, in SignUpInput, hash []byte) (db.OpenPlayRegistration, error) {
	event, err := q.LockOpenPlayEvent(ctx, data.UUIDToPg(in.EventID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.OpenPlayRegistration{}, data.ErrRecordNotFound
		}
		return db.OpenPlayRegistration{}, fmt.Errorf("events: lock event: %w", err)
	}
	if err := data.AssertTenant(ctx, data.PgToUUID(event.ComplexID)); err != nil {
		return db.OpenPlayRegistration{}, err
	}
	if err := admitSignUp(ctx, q, event, in); err != nil {
		return db.OpenPlayRegistration{}, err
	}
	if err := m.checkHoldLimit(ctx, tx, in); err != nil {
		return db.OpenPlayRegistration{}, err
	}
	if err := q.ExpireOpenPlayHoldsOfPhone(ctx, db.ExpireOpenPlayHoldsOfPhoneParams{
		EventID: event.ID, Phone: in.Phone,
	}); err != nil {
		return db.OpenPlayRegistration{}, fmt.Errorf("events: expire stale holds: %w", err)
	}
	if err := m.checkNotRegistered(ctx, tx, event.ID, in.Phone); err != nil {
		return db.OpenPlayRegistration{}, err
	}
	taken, err := q.CountOpenPlaySpotsTaken(ctx, db.CountOpenPlaySpotsTakenParams{EventID: event.ID})
	if err != nil {
		return db.OpenPlayRegistration{}, fmt.Errorf("events: count spots: %w", err)
	}
	if taken >= event.Capacity {
		return db.OpenPlayRegistration{}, ErrEventFull
	}
	row, err := q.InsertOpenPlayRegistration(ctx, db.InsertOpenPlayRegistrationParams{
		EventID: event.ID, ComplexID: event.ComplexID,
		FirstName: in.FirstName, LastName: in.LastName, Phone: in.Phone, Email: data.TextToPg(in.Email),
		Price: event.Price,
		//nolint:gosec // G115: the flat service fee on a price in centavos; far below int32 range.
		ServiceFee:      int32(in.ServiceFee),
		HoldSeconds:     m.PaymentExpiry.Seconds(),
		AccessTokenHash: hash,
		HoldIp:          data.TextToPg(&in.IP),
		PlayerGender:    data.TextToPg(in.Gender),
		PlayerLevel:     data.Int4PtrToPg(in.Level),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == data.SQLStateUniqueViolation {
			return db.OpenPlayRegistration{}, ErrAlreadyRegistered
		}
		return db.OpenPlayRegistration{}, fmt.Errorf("events: insert registration: %w", err)
	}
	if in.Hooks != nil && in.Hooks.Attach != nil {
		if err := in.Hooks.Attach(ctx, q, row); err != nil {
			return db.OpenPlayRegistration{}, err
		}
	}
	return row, nil
}

// admitSignUp refuses a sign-up for an event that is cancelled or closed. A
// tournament category closes with its draw (the hook's Admit), not at a start
// time.
func admitSignUp(ctx context.Context, q *db.Queries, event db.OpenPlayEvent, in SignUpInput) error {
	if event.Status != EventActive || (!event.TournamentID.Valid && !eventFromDB(event).StartsAt().After(time.Now())) {
		return ErrEventClosed
	}
	if in.Hooks != nil && in.Hooks.Admit != nil {
		return in.Hooks.Admit(ctx, q, event)
	}
	return nil
}

// checkNotRegistered refuses a phone that already holds a live spot of the
// event, ahead of the capacity count: a person who is already in is told so even
// when the event has since filled.
func (m *Store) checkNotRegistered(ctx context.Context, tx pgx.Tx, eventID pgtype.UUID, phone string) error {
	var registered bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM open_play_registrations
			WHERE event_id = $1 AND phone = $2 AND status IN ('held', 'confirmed'))`, eventID, phone).Scan(&registered)
	if err != nil {
		return fmt.Errorf("events: check registered: %w", err)
	}
	if registered {
		return ErrAlreadyRegistered
	}
	return nil
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
			CancelledBy: r.CancelledBy, CreatedAt: r.CreatedAt, PlayerGender: r.PlayerGender, PlayerLevel: r.PlayerLevel,
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
// No refund is marked as owed: the caller of a cancellation that owes one uses
// CancelByParticipant.
func (m *Store) CancelRegistration(ctx context.Context, id uuid.UUID, by string) (*Registration, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return cancelRegistration(ctx, m.Q, id, by, false)
}

// ParticipantCancel is a spot its person cancelled, and whether the tournament
// category it belongs to had its draw when the cancellation committed.
type ParticipantCancel struct {
	Registration *Registration
	// Drawn is true for a tournament pair whose category's draw exists: leaving
	// is a walkover, the payment is kept and no refund was marked owed.
	Drawn bool
}

// CancelByParticipant ends a person's spot from their link. refundOwed (the
// cancellation came inside the complex's refund window) is written with the
// cancellation itself, so a refund the request never finishes is found by the
// refund sweep (OwnerCancelledStillPaid) rather than lost.
//
// It takes the event's row lock and, for a tournament pair, the category's after
// it: the locks payment confirmation and the draw take. A draw in progress is
// waited for and then reported (Drawn), and no refund is marked owed for a pair
// that is in the bracket; a draw that starts after this commits leaves the pair
// out.
func (m *Store) CancelByParticipant(ctx context.Context, id uuid.UUID, refundOwed bool) (*ParticipantCancel, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	var out *ParticipantCancel
	err := m.DB.RetryTx(ctx, data.DefaultTxAttempts, func(_ pgx.Tx, q *db.Queries) error {
		reg, err := q.GetOpenPlayRegistration(ctx, db.GetOpenPlayRegistrationParams{
			ID: data.UUIDToPg(id), ComplexID: data.TenantParam(ctx),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("events: read registration: %w", err)
		}
		event, err := q.LockOpenPlayEvent(ctx, reg.EventID)
		if err != nil {
			return fmt.Errorf("events: lock event: %w", err)
		}
		drawn := false
		if event.TournamentID.Valid {
			if drawn, err = q.LockEventCategoryDraw(ctx, event.ID); err != nil {
				return fmt.Errorf("events: lock the category: %w", err)
			}
		}
		cancelled, err := cancelRegistration(ctx, q, id, CancelledByParticipant, refundOwed && !drawn)
		if err != nil {
			return err
		}
		out = &ParticipantCancel{Registration: cancelled, Drawn: drawn}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func cancelRegistration(ctx context.Context, q *db.Queries, id uuid.UUID, by string, refundOwed bool) (*Registration, error) {
	row, err := q.CancelOpenPlayRegistration(ctx, db.CancelOpenPlayRegistrationParams{
		ID: data.UUIDToPg(id), CancelledBy: data.TextToPg(&by), RefundOwed: refundOwed,
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

// PendingRefund is a cancelled registration whose money is owed back and has not
// been sent: the owner's cancellation, or a participant's inside the window.
type PendingRefund struct {
	RegistrationID uuid.UUID
	ComplexID      uuid.UUID
}

// OwnerCancelledStillPaid returns the registrations cancelled more than grace
// ago that owe their refund — by the owner, or by the participant inside the
// window (CancelByParticipant) — whose MercadoPago payment still reads
// collected: the refund the cancel request itself never finished. Only the sweep calls it,
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

// DueReminder is a paid spot whose day-before reminder is due. StartsAt is when
// the event, or the tournament category, starts; HasTime is false for a
// tournament category the owner gave no hour, whose reminder names the day
// alone. A tournament pair's partner comes with it, nil when there is none.
type DueReminder struct {
	RegistrationID   uuid.UUID
	StartsAt         time.Time
	HasTime          bool
	PartnerFirstName string
	PartnerEmail     *string
}

// DueReminders returns the paid spots whose event starts within a day of now
// and that were not reminded yet. now is the caller's clock, as the booking
// reminder's is. Only the sweep calls it, under the tenant bypass.
func (m *Store) DueReminders(ctx context.Context, now time.Time) ([]DueReminder, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListOpenPlayRegistrationsForReminder(ctx, data.TimeToPg(now))
	if err != nil {
		return nil, fmt.Errorf("events: list due reminders: %w", err)
	}
	out := make([]DueReminder, len(rows))
	for i, r := range rows {
		out[i] = DueReminder{
			RegistrationID: data.PgToUUID(r.RegistrationID), StartsAt: data.PgToTime(r.StartsAt), HasTime: r.HasTime,
			PartnerFirstName: r.PartnerFirstName.String, PartnerEmail: data.PgToTextPtr(r.PartnerEmail),
		}
	}
	return out, nil
}

// MarkReminded records that a registration's reminder was queued, so the next
// sweep does not select it again.
func (m *Store) MarkReminded(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if err := m.Q.MarkOpenPlayRegistrationReminded(ctx, data.UUIDToPg(id)); err != nil {
		return fmt.Errorf("events: mark reminded: %w", err)
	}
	return nil
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
		Gender:      data.PgToTextPtr(r.PlayerGender),
		Level:       data.PgToInt4Ptr(r.PlayerLevel),
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
