package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/data/slotguard"
	"github.com/stodulski/vibe-server/internal/db"
)

// MaxListEvents bounds one page of an events list.
const MaxListEvents = 200

// CreateEvent writes an event and the courts it holds in one transaction,
// refusing with a *ConflictError when something already holds the event's hours
// on any of those courts: a live booking, an owner's blocked slot, a date of a
// fixed weekly series, or another event.
//
// It takes every court's row lock for update first, in id order. Every booking
// insert holds that row for share (slotguard.LockCourtLive), so the two exclude
// each other: a storefront booking committed before this transaction is seen by
// the checks below, and one that starts after it waits, then finds the event
// holding the hours (SlotTaken reads open_play_event_occupancy). It does not
// close the window against a blocked slot filed at the same instant, which
// takes only the court-day advisory locks; the owner is the only writer of
// both, so that residual window is accepted, as InsertSeries accepts it.
func (m *Store) CreateEvent(ctx context.Context, e *Event, courtIDs []uuid.UUID) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, e.ComplexID); err != nil {
		return err
	}
	ids := slices.Clone(courtIDs)
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })

	var created db.OpenPlayEvent
	err := m.DB.WithTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		for _, id := range ids {
			if err := lockCourt(ctx, tx, id); err != nil {
				return err
			}
		}
		if err := m.checkCourtsFree(ctx, tx, e, ids); err != nil {
			return err
		}
		var err error
		if created, err = q.InsertOpenPlayEvent(ctx, insertEventParams(e)); err != nil {
			return fmt.Errorf("events: insert event: %w", err)
		}
		for _, id := range ids {
			if err := q.InsertOpenPlayEventCourt(ctx, db.InsertOpenPlayEventCourtParams{
				EventID: created.ID, ComplexID: created.ComplexID, CourtID: data.UUIDToPg(id),
			}); err != nil {
				return fmt.Errorf("events: insert event court: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	*e = *eventFromDB(created)
	return nil
}

func insertEventParams(e *Event) db.InsertOpenPlayEventParams {
	return db.InsertOpenPlayEventParams{
		ComplexID: data.UUIDToPg(e.ComplexID),
		Name:      e.Name,
		Sport:     db.SportType(e.Sport),
		Date:      data.DateToPg(e.Date),
		StartTime: data.TimeStrToPg(e.StartTime),
		//nolint:gosec // G115: bounded by the schema's CHECK (30..720); the caller validates first.
		DurationMinutes: int32(e.DurationMinutes),
		//nolint:gosec // G115: bounded by the schema's CHECK (1..500); the caller validates first.
		Capacity: int32(e.Capacity),
		//nolint:gosec // G115: a price in centavos, validated by the caller; far below int32 range.
		Price:     int32(e.Price),
		CreatedBy: data.UUIDToPg(e.CreatedBy),
	}
}

// lockCourt takes the court's row lock for update, refusing with
// ErrRecordNotFound when the court is gone.
func lockCourt(ctx context.Context, tx pgx.Tx, courtID uuid.UUID) error {
	var id uuid.UUID
	err := tx.QueryRow(ctx,
		`SELECT id FROM courts WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		courtID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return fmt.Errorf("events: lock court: %w", err)
	}
	return nil
}

// checkCourtsFree asks, inside the transaction that holds the courts, whether
// anything holds the event's hours on any of them, and names the first thing it
// finds. The booking predicate is the canonical one (see bookingstore.SlotTaken):
// released bookings and storefront holds past their window do not count.
//
// It is one query so every court is judged on the same snapshot; the reasons
// are ordered so the most concrete answer wins when several things collide.
func (m *Store) checkCourtsFree(ctx context.Context, tx pgx.Tx, e *Event, courtIDs []uuid.UUID) error {
	var reason string
	var courtID uuid.UUID
	err := tx.QueryRow(ctx, `
		WITH candidate AS (
		  SELECT tstzrange(
		           booking_starts_at($2::date, $3::time),
		           booking_starts_at($2::date, $3::time) + make_interval(mins => $4),
		           '[)') AS span
		), held AS (
		  SELECT b.court_id, 1 AS rank, 'booking' AS reason
		  FROM bookings b, candidate c
		  WHERE b.court_id = ANY($1::uuid[]) AND b.span && c.span
		    AND b.status NOT IN `+slotguard.ReleasedBookingStatuses+`
		    AND NOT (
		      b.status = 'pending' AND b.collection_status = 'unpaid' AND b.created_by IS NULL
		      AND b.created_at < NOW() - make_interval(secs => $5)
		    )
		  UNION ALL
		  SELECT s.court_id, 2, 'blocked'
		  FROM blocked_slots s, candidate c
		  WHERE s.court_id = ANY($1::uuid[]) AND s.span && c.span
		  UNION ALL
		  SELECT o.court_id, 3, 'fixed'
		  FROM booking_series_occurrences(NULL, $1::uuid[], $2::date - 1, $2::date + 1) o, candidate c
		  WHERE o.span && c.span
		  UNION ALL
		  SELECT o.court_id, 4, 'event'
		  FROM open_play_event_occupancy(NULL, $1::uuid[], $2::date - 1, $2::date + 1) o, candidate c
		  WHERE o.span && c.span
		)
		SELECT reason, court_id FROM held ORDER BY rank, court_id LIMIT 1`,
		data.UUIDSliceToPg(courtIDs), data.DateToPg(e.Date), data.TimeStrToPg(e.StartTime),
		e.DurationMinutes, m.PaymentExpiry.Seconds(),
	).Scan(&reason, &courtID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("events: check courts free: %w", err)
	}
	return &ConflictError{Reason: reason, CourtID: courtID}
}

// GetEvent returns one event with its courts and spot counts, or
// ErrRecordNotFound. A scoped caller only ever sees its own tenant's.
func (m *Store) GetEvent(ctx context.Context, id uuid.UUID) (*Event, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetOpenPlayEvent(ctx, db.GetOpenPlayEventParams{
		ID: data.UUIDToPg(id), ComplexID: data.TenantParam(ctx),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, fmt.Errorf("events: get event: %w", err)
	}
	e := eventFromDB(row)
	if err := m.fillCourts(ctx, e); err != nil {
		return nil, err
	}
	if err := m.fillCounts(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (m *Store) fillCourts(ctx context.Context, e *Event) error {
	courts, err := m.Q.ListOpenPlayEventCourts(ctx, data.UUIDToPg(e.ID))
	if err != nil {
		return fmt.Errorf("events: list event courts: %w", err)
	}
	e.Courts = make([]Court, len(courts))
	for i, c := range courts {
		e.Courts[i] = Court{ID: data.PgToUUID(c.CourtID), Name: c.CourtName}
	}
	return nil
}

func (m *Store) fillCounts(ctx context.Context, e *Event) error {
	var confirmed, held int
	err := m.DB.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status = 'confirmed')::int,
		       count(*) FILTER (WHERE status = 'held' AND held_until > NOW())::int
		FROM open_play_registrations WHERE event_id = $1`, data.UUIDToPg(e.ID)).Scan(&confirmed, &held)
	if err != nil {
		return fmt.Errorf("events: count spots: %w", err)
	}
	e.ConfirmedCount, e.HeldCount = confirmed, held
	return nil
}

// ListEvents returns the events of one complex whose date is in [from, to],
// soonest first, with their courts and spot counts. includeCancelled is what
// the owner's list sets and the public one does not. At most MaxListEvents.
func (m *Store) ListEvents(ctx context.Context, complexID uuid.UUID, from, to time.Time, includeCancelled bool) ([]*Event, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListOpenPlayEvents(ctx, db.ListOpenPlayEventsParams{
		ComplexID:        data.UUIDToPg(complexID),
		DateFrom:         data.DateToPg(from),
		DateTo:           data.DateToPg(to),
		IncludeCancelled: includeCancelled,
		PageLimit:        MaxListEvents,
	})
	if err != nil {
		return nil, fmt.Errorf("events: list events: %w", err)
	}
	out := make([]*Event, len(rows))
	for i, r := range rows {
		e := eventFromDB(db.OpenPlayEvent{
			ID: r.ID, ComplexID: r.ComplexID, Name: r.Name, Sport: r.Sport, Date: r.Date,
			StartTime: r.StartTime, DurationMinutes: r.DurationMinutes, Capacity: r.Capacity,
			Price: r.Price, Status: r.Status, CancelReason: r.CancelReason, CancelledAt: r.CancelledAt,
			CancelledBy: r.CancelledBy, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
		e.ConfirmedCount, e.HeldCount = int(r.ConfirmedCount), int(r.HeldCount)
		if err := m.fillCourts(ctx, e); err != nil {
			return nil, err
		}
		out[i] = e
	}
	return out, nil
}

// CancelEvent cancels an active event and every registration of it in one
// transaction, so its courts and its spots are released together, and returns
// the cancelled event and the ids of the registrations that had been paid (the
// ones owed a refund). A second cancel answers ErrEventNotActive.
func (m *Store) CancelEvent(ctx context.Context, id, by uuid.UUID, reason *string) (*Event, []uuid.UUID, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	var cancelled db.OpenPlayEvent
	var paid []uuid.UUID
	err := m.DB.WithTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		locked, err := q.LockOpenPlayEvent(ctx, data.UUIDToPg(id))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("events: lock event: %w", err)
		}
		if err := data.AssertTenant(ctx, data.PgToUUID(locked.ComplexID)); err != nil {
			return err
		}
		if cancelled, err = q.CancelOpenPlayEvent(ctx, db.CancelOpenPlayEventParams{
			ID: locked.ID, CancelledBy: data.UUIDToPg(by), CancelReason: data.TextToPg(reason),
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrEventNotActive
			}
			return fmt.Errorf("events: cancel event: %w", err)
		}
		ended, err := q.CancelOpenPlayEventRegistrations(ctx, locked.ID)
		if err != nil {
			return fmt.Errorf("events: cancel registrations: %w", err)
		}
		for _, r := range ended {
			if r.PreviousStatus == RegistrationConfirmed {
				paid = append(paid, data.PgToUUID(r.ID))
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return eventFromDB(cancelled), paid, nil
}

func eventFromDB(e db.OpenPlayEvent) *Event {
	return &Event{
		ID:              data.PgToUUID(e.ID),
		ComplexID:       data.PgToUUID(e.ComplexID),
		Name:            e.Name,
		Sport:           string(e.Sport),
		Date:            data.PgToDate(e.Date),
		StartTime:       data.PgToTimeStr(e.StartTime),
		DurationMinutes: int(e.DurationMinutes),
		Capacity:        int(e.Capacity),
		Price:           int(e.Price),
		Status:          e.Status,
		CancelReason:    data.PgToTextPtr(e.CancelReason),
		CancelledAt:     data.PgToTimePtr(e.CancelledAt),
		CancelledBy:     data.PgToUUIDPtr(e.CancelledBy),
		CreatedBy:       data.PgToUUID(e.CreatedBy),
		CreatedAt:       data.PgToTime(e.CreatedAt),
	}
}
