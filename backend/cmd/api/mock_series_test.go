package main

import (
	"context"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// mockBookingSeriesStore is an empty store of fixed weekly bookings: it holds
// none, so a lookup answers not found and the extension job has nothing to do.
type mockBookingSeriesStore struct {
	// GetSeriesFn answers GetSeries; unset, the series is not found.
	GetSeriesFn func(ctx context.Context, id uuid.UUID) (*bookingstore.Series, error)
	// EndSeriesFn answers EndSeries; unset, the series is not found.
	EndSeriesFn func(ctx context.Context, id uuid.UUID, endsOn *time.Time, status string) (*bookingstore.Series, error)
}

func (m *mockBookingSeriesStore) InsertSeries(context.Context, *bookingstore.Series, []bookingstore.SeriesException) error {
	return nil
}

func (m *mockBookingSeriesStore) GetSeries(ctx context.Context, id uuid.UUID) (*bookingstore.Series, error) {
	if m.GetSeriesFn != nil {
		return m.GetSeriesFn(ctx, id)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockBookingSeriesStore) EndExpiredSeries(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (m *mockBookingSeriesStore) EndSeries(ctx context.Context, id uuid.UUID, endsOn *time.Time, status string) (*bookingstore.Series, error) {
	if m.EndSeriesFn != nil {
		return m.EndSeriesFn(ctx, id, endsOn, status)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockBookingSeriesStore) SeriesDates(context.Context, uuid.UUID, time.Time, time.Time) ([]bookingstore.SeriesDate, error) {
	return nil, nil
}

func (m *mockBookingSeriesStore) RecordSeriesException(context.Context, *bookingstore.Series, time.Time, string) (bool, error) {
	return true, nil
}

func (m *mockBookingSeriesStore) SeriesBookingsFrom(context.Context, uuid.UUID, time.Time) ([]*bookingstore.Booking, error) {
	return nil, nil
}
