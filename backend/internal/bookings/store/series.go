package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// ErrSeriesDateGenerated is returned when a series occurrence is inserted for a
// date the series has already been generated for, whether that date was booked
// (and perhaps cancelled since) or skipped. A date is generated at most once
// ever; the insert that finds it taken rolls back whole.
var ErrSeriesDateGenerated = errors.New("series date already generated")

// Series statuses.
const (
	SeriesStatusActive = "active"
	SeriesStatusEnded  = "ended"
)

// Outcomes recorded for a generated date.
const (
	SeriesDateBooked  = "booked"
	SeriesDateSkipped = "skipped"
)

// Series is a fixed weekly booking: the standing order the occurrences are
// generated from. Weekday is 0 = Sunday .. 6 = Saturday.
type Series struct {
	ID              uuid.UUID
	ComplexID       uuid.UUID
	CourtID         uuid.UUID
	ClientID        uuid.UUID
	Weekday         int
	StartTime       string
	DurationMinutes int
	// Price is the manual price of every occurrence; nil means each one is
	// priced when it is generated.
	Price     *int
	Notes     *string
	StartsOn  time.Time
	EndsOn    *time.Time
	Status    string
	CreatedBy uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SeriesDate is one date a series has been generated for.
type SeriesDate struct {
	Date    time.Time
	Outcome string
	Reason  *string
}

func seriesFromDB(s db.BookingSeries) *Series {
	out := &Series{
		ID:              data.PgToUUID(s.ID),
		ComplexID:       data.PgToUUID(s.ComplexID),
		CourtID:         data.PgToUUID(s.CourtID),
		ClientID:        data.PgToUUID(s.ClientID),
		Weekday:         int(s.Weekday),
		StartTime:       data.PgToTimeStr(s.StartTime),
		DurationMinutes: int(s.DurationMinutes),
		Price:           data.PgToInt4Ptr(s.Price),
		Notes:           data.PgToTextPtr(s.Notes),
		StartsOn:        data.PgToDate(s.StartsOn),
		Status:          s.Status,
		CreatedBy:       data.PgToUUID(s.CreatedBy),
		CreatedAt:       data.PgToTime(s.CreatedAt),
		UpdatedAt:       data.PgToTime(s.UpdatedAt),
	}
	if s.EndsOn.Valid {
		end := data.PgToDate(s.EndsOn)
		out.EndsOn = &end
	}
	return out
}

// optDateToPg is a nullable DATE: nil becomes NULL.
func optDateToPg(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return data.DateToPg(*t)
}

// InsertSeries writes a new series.
func (m *Store) InsertSeries(ctx context.Context, s *Series) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, s.ComplexID); err != nil {
		return err
	}
	params := db.InsertBookingSeriesParams{
		ComplexID: data.UUIDToPg(s.ComplexID),
		CourtID:   data.UUIDToPg(s.CourtID),
		ClientID:  data.UUIDToPg(s.ClientID),
		//nolint:gosec // G115: weekday is 0..6, validated by the caller and the schema.
		Weekday:   int16(s.Weekday),
		StartTime: data.TimeStrToPg(s.StartTime),
		//nolint:gosec // G115: one of the permitted durations.
		DurationMinutes: int32(s.DurationMinutes),
		Price:           data.Int4PtrToPg(s.Price),
		Notes:           data.TextToPg(s.Notes),
		StartsOn:        data.DateToPg(s.StartsOn),
		EndsOn:          optDateToPg(s.EndsOn),
		CreatedBy:       data.UUIDToPg(s.CreatedBy),
	}
	row, err := m.Q.InsertBookingSeries(ctx, params)
	if err != nil {
		return fmt.Errorf("bookings: insert series: %w", err)
	}
	*s = *seriesFromDB(row)
	return nil
}

// GetSeries returns one series, or ErrRecordNotFound. A scoped caller only
// ever sees its own tenant's.
func (m *Store) GetSeries(ctx context.Context, id uuid.UUID) (*Series, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetBookingSeries(ctx, db.GetBookingSeriesParams{
		ID:        data.UUIDToPg(id),
		ComplexID: data.TenantParam(ctx),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, fmt.Errorf("bookings: get series: %w", err)
	}
	return seriesFromDB(row), nil
}

// ListActiveSeries returns every active series across tenants. Only the
// scheduled extension job calls it, under the tenant bypass.
func (m *Store) ListActiveSeries(ctx context.Context) ([]*Series, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListActiveBookingSeries(ctx)
	if err != nil {
		return nil, fmt.Errorf("bookings: list active series: %w", err)
	}
	out := make([]*Series, len(rows))
	for i, r := range rows {
		out[i] = seriesFromDB(r)
	}
	return out, nil
}

// EndSeries sets a series' last day and status.
func (m *Store) EndSeries(ctx context.Context, id uuid.UUID, endsOn *time.Time, status string) (*Series, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.EndBookingSeries(ctx, db.EndBookingSeriesParams{
		ID: data.UUIDToPg(id), EndsOn: optDateToPg(endsOn), Status: status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, fmt.Errorf("bookings: end series: %w", err)
	}
	return seriesFromDB(row), nil
}

// SeriesDates returns the dates a series has been generated for between from
// and to, inclusive.
func (m *Store) SeriesDates(ctx context.Context, seriesID uuid.UUID, from, to time.Time) ([]SeriesDate, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListBookingSeriesDates(ctx, db.ListBookingSeriesDatesParams{
		SeriesID: data.UUIDToPg(seriesID),
		Date:     data.DateToPg(from),
		Date_2:   data.DateToPg(to),
	})
	if err != nil {
		return nil, fmt.Errorf("bookings: list series dates: %w", err)
	}
	out := make([]SeriesDate, len(rows))
	for i, r := range rows {
		out[i] = SeriesDate{Date: data.PgToDate(r.Date), Outcome: r.Outcome, Reason: data.PgToTextPtr(r.Reason)}
	}
	return out, nil
}

// RecordSeriesSkip records that a date could not be booked, so the job does not
// try it again. A date already recorded keeps its first answer.
func (m *Store) RecordSeriesSkip(ctx context.Context, s *Series, date time.Time, reason string) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	if _, err := m.Q.RecordBookingSeriesSkip(ctx, db.RecordBookingSeriesSkipParams{
		SeriesID:  data.UUIDToPg(s.ID),
		ComplexID: data.UUIDToPg(s.ComplexID),
		Date:      data.DateToPg(date),
		Reason:    data.TextToPg(&reason),
	}); err != nil {
		return fmt.Errorf("bookings: record series skip: %w", err)
	}
	return nil
}

// SeriesBookingsFrom returns a series' occurrences dated on or after from.
func (m *Store) SeriesBookingsFrom(ctx context.Context, seriesID uuid.UUID, from time.Time) ([]*Booking, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListBookingSeriesBookingsFrom(ctx, db.ListBookingSeriesBookingsFromParams{
		SeriesID: data.UUIDToPg(seriesID),
		Date:     data.DateToPg(from),
	})
	if err != nil {
		return nil, fmt.Errorf("bookings: list series bookings: %w", err)
	}
	return bookingsFromDB(rows), nil
}

// claimSeriesDate records the booking's date as generated for its series, in the
// booking's own transaction.
func claimSeriesDate(ctx context.Context, tx pgx.Tx, b *Booking) error {
	tag, err := tx.Exec(ctx, `
		INSERT INTO booking_series_dates (series_id, complex_id, date, outcome)
		VALUES ($1, $2, $3, 'booked')
		ON CONFLICT (series_id, date) DO NOTHING`,
		data.UUIDPtrToPg(b.SeriesID), data.UUIDToPg(b.ComplexID), data.DateToPg(b.Date))
	if err != nil {
		return fmt.Errorf("bookings: claim series date: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSeriesDateGenerated
	}
	return nil
}
