package store

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// SeriesOccurrence is one date a fixed weekly series is holding: it matches the
// series' rule, falls inside its life, and has no booking and no exception yet.
// It occupies its court and hours exactly as a booking would.
type SeriesOccurrence struct {
	SeriesID  uuid.UUID
	ComplexID uuid.UUID
	CourtID   uuid.UUID
	// Date is the local day the occurrence starts on.
	Date     time.Time
	StartsAt time.Time
	EndsAt   time.Time
	// ClientName is who the series is for, as the owner's calendar labels it.
	ClientName string
}

// SeriesOccurrences returns the dates of active fixed weekly series that hold
// their court between from and to, inclusive, and are not yet settled.
//
// This is the one place the rule is expanded for a read in Go; the expansion
// itself is the SQL function booking_series_occurrences (db/migrations/027),
// which every write guard also calls. Every question that must see a fixed
// booking — the storefront's availability, the owner's calendar, the owner's
// booking and blocked-slot guards, the series pre-check, the daily job — goes
// through one or the other, so none can disagree about which dates a series
// covers.
//
// complexID and courtIDs narrow the series; nil and empty mean no narrowing,
// which only the job (under the tenant bypass) uses. An occurrence that runs
// past midnight is returned under its start date, so a caller interested in
// day D passes from = D-1 (GetBookedSlotsByCourtIDs does).
func (m *Store) SeriesOccurrences(
	ctx context.Context, complexID *uuid.UUID, courtIDs []uuid.UUID, from, to time.Time,
) ([]SeriesOccurrence, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var courts any
	if len(courtIDs) > 0 {
		courts = data.UUIDSliceToPg(courtIDs)
	}
	rows, err := m.DB.Query(ctx, `
		SELECT o.series_id, o.complex_id, o.court_id, o.occurs_on,
		       lower(o.span)::timestamptz, upper(o.span)::timestamptz,
		       cl.first_name, cl.last_name
		FROM booking_series_occurrences($1::uuid, $2::uuid[], $3::date, $4::date) o
		JOIN booking_series s ON s.id = o.series_id
		JOIN clients cl ON cl.id = s.client_id
		ORDER BY o.occurs_on, o.court_id, lower(o.span)`,
		data.UUIDPtrToPg(complexID), courts, data.DateToPg(from), data.DateToPg(to))
	if err != nil {
		return nil, fmt.Errorf("bookings: series occurrences: %w", err)
	}
	defer rows.Close()

	var out []SeriesOccurrence
	for rows.Next() {
		var (
			seriesID, complexUUID, courtID pgtype.UUID
			day                            pgtype.Date
			startsAt, endsAt               pgtype.Timestamptz
			first, last                    string
		)
		if err := rows.Scan(&seriesID, &complexUUID, &courtID, &day, &startsAt, &endsAt, &first, &last); err != nil {
			return nil, fmt.Errorf("bookings: scan series occurrence: %w", err)
		}
		out = append(out, SeriesOccurrence{
			SeriesID:   data.PgToUUID(seriesID),
			ComplexID:  data.PgToUUID(complexUUID),
			CourtID:    data.PgToUUID(courtID),
			Date:       data.PgToDate(day),
			StartsAt:   startsAt.Time.In(timezone.Argentina),
			EndsAt:     endsAt.Time.In(timezone.Argentina),
			ClientName: first + " " + last,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bookings: series occurrences: %w", err)
	}
	return out, nil
}

// fixedSpans turns occurrences into booked spans flagged as fixed.
func fixedSpans(occs []SeriesOccurrence, windowStart, windowEnd time.Time) []BookedSpan {
	var out []BookedSpan
	for _, o := range occs {
		if !o.StartsAt.Before(windowEnd) || !o.EndsAt.After(windowStart) {
			continue
		}
		out = append(out, BookedSpan{CourtID: o.CourtID, StartsAt: o.StartsAt, EndsAt: o.EndsAt, Fixed: true})
	}
	return out
}

// sortSpans orders spans by court then start, the order GetBookedSlots returns
// real bookings in, so merging the fixed ones in changes nothing for a caller.
func sortSpans(spans []BookedSpan) {
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].CourtID != spans[j].CourtID {
			return spans[i].CourtID.String() < spans[j].CourtID.String()
		}
		return spans[i].StartsAt.Before(spans[j].StartsAt)
	})
}
