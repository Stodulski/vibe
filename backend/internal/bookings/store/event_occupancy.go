package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// EventOccupancy is one court's hours held by an active open-play event. It
// occupies its court exactly as a booking would, for as long as the event is
// not cancelled.
type EventOccupancy struct {
	EventID   uuid.UUID
	ComplexID uuid.UUID
	CourtID   uuid.UUID
	// Date is the local day the event starts on.
	Date     time.Time
	StartsAt time.Time
	EndsAt   time.Time
	// Name is the event's name, as the owner's calendar labels it.
	Name string
}

// EventOccupancies returns the hours active open-play events hold between from
// and to, inclusive, one row per event and court.
//
// It is the Go read of the SQL function open_play_event_occupancy
// (db/migrations/028), which every write guard calls too, so the storefront's
// availability, the owner's calendar and the guards cannot disagree about which
// hours an event covers. complexID and courtIDs narrow the rows; nil and empty
// mean no narrowing. An event that runs past midnight is returned under its
// start date, so a caller interested in day D passes from = D-1
// (BookedSpansBetween does).
func (m *Store) EventOccupancies(
	ctx context.Context, complexID *uuid.UUID, courtIDs []uuid.UUID, from, to time.Time,
) ([]EventOccupancy, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var courts any
	if len(courtIDs) > 0 {
		courts = data.UUIDSliceToPg(courtIDs)
	}
	rows, err := m.DB.Query(ctx, `
		SELECT o.event_id, o.complex_id, o.court_id, o.occurs_on,
		       lower(o.span)::timestamptz, upper(o.span)::timestamptz, e.name
		FROM open_play_event_occupancy($1::uuid, $2::uuid[], $3::date, $4::date) o
		JOIN open_play_events e ON e.id = o.event_id
		ORDER BY o.occurs_on, o.court_id, lower(o.span)`,
		data.UUIDPtrToPg(complexID), courts, data.DateToPg(from), data.DateToPg(to))
	if err != nil {
		return nil, fmt.Errorf("bookings: event occupancies: %w", err)
	}
	defer rows.Close()

	var out []EventOccupancy
	for rows.Next() {
		var (
			eventID, complexUUID, courtID pgtype.UUID
			day                           pgtype.Date
			startsAt, endsAt              pgtype.Timestamptz
			name                          string
		)
		if err := rows.Scan(&eventID, &complexUUID, &courtID, &day, &startsAt, &endsAt, &name); err != nil {
			return nil, fmt.Errorf("bookings: scan event occupancy: %w", err)
		}
		out = append(out, EventOccupancy{
			EventID:   data.PgToUUID(eventID),
			ComplexID: data.PgToUUID(complexUUID),
			CourtID:   data.PgToUUID(courtID),
			Date:      data.PgToDate(day),
			StartsAt:  startsAt.Time.In(timezone.Argentina),
			EndsAt:    endsAt.Time.In(timezone.Argentina),
			Name:      name,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bookings: event occupancies: %w", err)
	}
	return out, nil
}

// eventSpans turns occupancies into booked spans flagged as events, keeping
// the ones that touch the window.
func eventSpans(occs []EventOccupancy, windowStart, windowEnd time.Time) []BookedSpan {
	var out []BookedSpan
	for _, o := range occs {
		if !o.StartsAt.Before(windowEnd) || !o.EndsAt.After(windowStart) {
			continue
		}
		out = append(out, BookedSpan{CourtID: o.CourtID, StartsAt: o.StartsAt, EndsAt: o.EndsAt, Event: true})
	}
	return out
}
