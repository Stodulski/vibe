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
	"github.com/stodulski/vibe-server/internal/data/slotguard"
	"github.com/stodulski/vibe-server/internal/db"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// ErrSeriesDateGenerated is returned when a series occurrence is inserted for a
// date the series has already settled, whether that date was booked (and perhaps
// cancelled since) or excepted. A date is settled at most once ever; the insert
// that finds it taken rolls back whole.
var ErrSeriesDateGenerated = errors.New("series date already settled")

// Series statuses.
const (
	SeriesStatusActive = "active"
	SeriesStatusEnded  = "ended"
)

// Outcomes recorded for a settled date.
const (
	SeriesDateBooked    = "booked"
	SeriesDateException = "exception"
)

// Reasons a date of a series is an exception, as the ledger and the API
// spell them.
const (
	ReasonCancelledByOwner = "cancelled_by_owner"
	ReasonOverlap          = "overlap"
	ReasonBlocked          = "blocked"
	ReasonClosedDay        = "closed_day"
	ReasonNoPrice          = "no_price"
	ReasonClientBlocked    = "client_blocked"
	// ReasonMissed marks a date whose hours were over before the job reached it.
	ReasonMissed = "missed"
)

// Series is a fixed weekly booking: the weekly rule whose dates hold the court
// and, on their own day, become bookings. Weekday is 0 = Sunday .. 6 = Saturday.
type Series struct {
	ID              uuid.UUID
	ComplexID       uuid.UUID
	CourtID         uuid.UUID
	ClientID        uuid.UUID
	Weekday         int
	StartTime       string
	DurationMinutes int
	// Price is the manual price of every occurrence; nil means each one is
	// priced on the day it is booked.
	Price     *int
	Notes     *string
	StartsOn  time.Time
	EndsOn    *time.Time
	Status    string
	CreatedBy uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SeriesDate is one settled date of a series: booked, or an exception with its
// reason.
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

// SeriesException is a date a new series is created already settled: one the
// owner chose to skip because something else holds it.
type SeriesException struct {
	Date   time.Time
	Reason string
}

// InsertSeries writes a new series and its exceptions in one transaction.
//
// It takes the court's row lock for update first. Every booking insert holds
// that row for share (slotguard.LockCourtLive), so the two exclude each other:
// a storefront booking committed before this transaction is seen by the
// recheck below, and one that starts after it waits, then finds the series
// holding the hours (SlotTaken reads booking_series_occurrences). That closes
// the window between the service's read-only pre-check and this write for
// bookings. It does not for a blocked slot filed at the same instant, which
// takes only the court-day advisory locks; the owner is the only writer of
// both, so that residual window is accepted.
//
// ErrSlotUnavailable is returned, writing nothing, when a booking now covers a
// date the series holds.
func (m *Store) InsertSeries(ctx context.Context, s *Series, exceptions []SeriesException) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, s.ComplexID); err != nil {
		return err
	}
	var row db.BookingSeries
	err := m.DB.WithTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		if err := lockCourtForSeries(ctx, tx, s.CourtID); err != nil {
			return err
		}
		var err error
		if row, err = q.InsertBookingSeries(ctx, insertSeriesParams(s)); err != nil {
			return fmt.Errorf("bookings: insert series: %w", err)
		}
		created := seriesFromDB(row)
		for _, e := range exceptions {
			if _, err := q.RecordBookingSeriesException(ctx, db.RecordBookingSeriesExceptionParams{
				SeriesID: row.ID, ComplexID: row.ComplexID, Date: data.DateToPg(e.Date), Reason: data.TextToPg(&e.Reason),
			}); err != nil {
				return fmt.Errorf("bookings: record series exception: %w", err)
			}
		}
		return recheckSeriesAgainstBookings(ctx, tx, created, m.PaymentExpiry)
	})
	if err != nil {
		return err
	}
	*s = *seriesFromDB(row)
	return nil
}

func insertSeriesParams(s *Series) db.InsertBookingSeriesParams {
	return db.InsertBookingSeriesParams{
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
}

// lockCourtForSeries takes the court's row lock for update, refusing with
// ErrRecordNotFound when the court is gone.
func lockCourtForSeries(ctx context.Context, tx pgx.Tx, courtID uuid.UUID) error {
	var id pgtype.UUID
	err := tx.QueryRow(ctx,
		`SELECT id FROM courts WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		data.UUIDToPg(courtID)).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return fmt.Errorf("bookings: lock court: %w", err)
	}
	return nil
}

// recheckSeriesAgainstBookings asks, inside the transaction that holds the
// court, whether any booking covers a date the new series holds within the
// booking horizon. The predicate is the canonical one (see SlotTaken).
func recheckSeriesAgainstBookings(ctx context.Context, tx pgx.Tx, s *Series, hold time.Duration) error {
	horizon := timezone.Today().AddDate(0, 0, MaxBookingHorizonDays)
	var taken bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM booking_series_occurrences(NULL, ARRAY[$1]::uuid[], $2::date, $3::date) o
			JOIN bookings b ON b.court_id = o.court_id AND b.span && o.span
			WHERE o.series_id = $4
			  AND b.status NOT IN `+slotguard.ReleasedBookingStatuses+`
			  AND NOT (
			    b.status = 'pending'
			    AND b.collection_status = 'unpaid'
			    AND b.created_by IS NULL
			    AND b.created_at < NOW() - make_interval(secs => $5)
			  )
		)`,
		data.UUIDToPg(s.CourtID), data.DateToPg(s.StartsOn), data.DateToPg(horizon),
		data.UUIDToPg(s.ID), hold.Seconds(),
	).Scan(&taken)
	if err != nil {
		return fmt.Errorf("bookings: recheck series: %w", err)
	}
	if taken {
		return ErrSlotUnavailable
	}
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

// EndExpiredSeries marks ended the active series whose last day is before day.
// Only the materialization job calls it, under the tenant bypass.
func (m *Store) EndExpiredSeries(ctx context.Context, day time.Time) (int64, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	n, err := m.Q.EndExpiredBookingSeries(ctx, data.DateToPg(day))
	if err != nil {
		return 0, fmt.Errorf("bookings: end expired series: %w", err)
	}
	return n, nil
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

// SeriesDates returns the settled dates of a series between from and to,
// inclusive.
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

// RecordSeriesException settles a date of a series as an exception, so it no
// longer holds its hours and the job never books it. It reports whether this
// call settled it: a date already booked or excepted keeps its first answer.
func (m *Store) RecordSeriesException(ctx context.Context, s *Series, date time.Time, reason string) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	n, err := m.Q.RecordBookingSeriesException(ctx, db.RecordBookingSeriesExceptionParams{
		SeriesID:  data.UUIDToPg(s.ID),
		ComplexID: data.UUIDToPg(s.ComplexID),
		Date:      data.DateToPg(date),
		Reason:    data.TextToPg(&reason),
	})
	if err != nil {
		return false, fmt.Errorf("bookings: record series exception: %w", err)
	}
	return n > 0, nil
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
