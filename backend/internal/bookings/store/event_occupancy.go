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
	// Name is what the owner's calendar labels it: the event's name, or
	// "tournament · category" for a match.
	Name string
	// Kind is OccupancyEvent or OccupancyMatch. For a match, EventID is the
	// match's id.
	Kind string
	// ParentID is the tournament of a match; nil for an event.
	ParentID *uuid.UUID
	// Pair1 and Pair2 label the two pairs of a match by surname, "Payer /
	// Partner", the way the owner's pair list reads. Nil for an event and for
	// a slot of a match no pair fills yet.
	Pair1, Pair2 *string
}

// What holds the hours of an EventOccupancy.
const (
	OccupancyEvent = "event"
	OccupancyMatch = "match"
)

// EventOccupancies returns the hours active open-play events hold between from
// and to, inclusive, one row per event and court.
//
// It is the Go read of the SQL function court_event_occupancy
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
		       lower(o.span)::timestamptz, upper(o.span)::timestamptz, o.label, o.kind, o.parent_id,
		       r1.last_name || ' / ' || p1.partner_last_name,
		       r2.last_name || ' / ' || p2.partner_last_name
		FROM court_event_occupancy($1::uuid, $2::uuid[], $3::date, $4::date) o
		LEFT JOIN tournament_matches m ON o.kind = 'match' AND m.id = o.event_id
		LEFT JOIN tournament_pairs p1 ON p1.id = m.pair1_id
		LEFT JOIN open_play_registrations r1 ON r1.id = p1.registration_id
		LEFT JOIN tournament_pairs p2 ON p2.id = m.pair2_id
		LEFT JOIN open_play_registrations r2 ON r2.id = p2.registration_id
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
			name, kind                    string
			parent                        pgtype.UUID
			pair1, pair2                  pgtype.Text
		)
		if err := rows.Scan(&eventID, &complexUUID, &courtID, &day, &startsAt, &endsAt, &name, &kind, &parent, &pair1, &pair2); err != nil {
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
			Kind:      kind,
			ParentID:  data.PgToUUIDPtr(parent),
			Pair1:     data.PgToTextPtr(pair1),
			Pair2:     data.PgToTextPtr(pair2),
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
		out = append(out, BookedSpan{CourtID: o.CourtID, StartsAt: o.StartsAt, EndsAt: o.EndsAt, Event: true, Match: o.Kind == OccupancyMatch})
	}
	return out
}
