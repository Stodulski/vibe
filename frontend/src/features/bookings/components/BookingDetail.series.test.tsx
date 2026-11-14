import { describe, it, expect, vi, beforeEach } from 'vitest';
import { toast } from 'sonner';
import { renderWithProviders, screen, userEvent, waitFor } from '@/test/test-utils';
import { makeBooking } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { BookingDetail } from './BookingDetail';
import type { Booking, BookingSeries } from '@/shared/types/api.types';

vi.mock('../api/bookings.api', () => ({ bookingsApi: { getById: vi.fn() } }));
vi.mock('../api/bookingSeries.api', () => ({ bookingSeriesApi: { get: vi.fn(), end: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.bookings;

// A Tuesday, far ahead so "from this date" is never in the past.
const DATE = '2099-01-06';

const series: BookingSeries = {
  id: 'ser1',
  complex_id: 'c1',
  court_id: 'ct1',
  client_id: 'cl1',
  weekday: 2,
  start_time: '21:00:00',
  duration_minutes: 90,
  price: null,
  notes: null,
  starts_on: DATE,
  ends_on: null,
  status: 'active',
  created_at: '2026-11-13T00:00:00Z',
};

async function renderDetail(booking: Booking) {
  const { bookingsApi } = await import('../api/bookings.api');
  const { bookingSeriesApi } = await import('../api/bookingSeries.api');
  vi.mocked(bookingsApi.getById).mockResolvedValue({ booking, payments: [] });
  vi.mocked(bookingSeriesApi.get).mockResolvedValue({ series, upcoming: [] });
  const onClose = vi.fn();
  renderWithProviders(
    <BookingDetail
      open
      onClose={onClose}
      booking={booking}
      complexId="c1"
      onCancel={vi.fn()}
      onConfirmPayment={vi.fn()}
      onNoShow={vi.fn()}
      onManualRefund={vi.fn()}
    />,
  );
  return { bookingSeriesApi, bookingsApi, onClose };
}

describe('BookingDetail — fixed weekly bookings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('names the weekday and hour of the series for one of its occurrences', async () => {
    const { bookingSeriesApi } = await renderDetail(makeBooking({ date: DATE, series_id: 'ser1' }));

    expect(await screen.findByText(t.series.label('martes', '21:00'))).toBeInTheDocument();
    expect(bookingSeriesApi.get).toHaveBeenCalledWith('c1', 'ser1', expect.anything());
  });

  it('shows nothing about a series, and never fetches one, for an ordinary booking', async () => {
    const { bookingSeriesApi, bookingsApi } = await renderDetail(makeBooking({ date: DATE }));

    await waitFor(() => {
      expect(bookingsApi.getById).toHaveBeenCalled();
    });
    expect(screen.queryByText(/Turno fijo/)).not.toBeInTheDocument();
    expect(bookingSeriesApi.get).not.toHaveBeenCalled();
    await userEvent.setup().click(screen.getByRole('button', { name: ES_AR.common.rowActionsLabel }));
    expect(screen.getByRole('menuitem', { name: t.cancel })).toBeInTheDocument();
    expect(screen.queryByRole('menuitem', { name: t.series.endFromHere })).not.toBeInTheDocument();
  });

  it('labels the cancel action as covering only this date', async () => {
    const user = userEvent.setup();
    await renderDetail(makeBooking({ date: DATE, series_id: 'ser1' }));

    await user.click(await screen.findByRole('button', { name: ES_AR.common.rowActionsLabel }));

    expect(screen.getByRole('menuitem', { name: t.series.cancelOne })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: t.series.endFromHere })).toBeInTheDocument();
  });

  it('ends the series from this booking date after confirming, and reports what was cancelled and kept', async () => {
    const user = userEvent.setup();
    const { bookingSeriesApi, onClose } = await renderDetail(makeBooking({ date: DATE, series_id: 'ser1' }));
    vi.mocked(bookingSeriesApi.end).mockResolvedValue({
      series: { ...series, status: 'ended' },
      cancelled: [makeBooking(), makeBooking(), makeBooking()],
      kept: [makeBooking()],
    });

    await user.click(await screen.findByRole('button', { name: ES_AR.common.rowActionsLabel }));
    await user.click(screen.getByRole('menuitem', { name: t.series.endFromHere }));
    expect(screen.getByText(t.series.endConfirmDescription)).toBeInTheDocument();
    expect(bookingSeriesApi.end).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: t.series.endConfirmLabel }));

    await waitFor(() => {
      expect(bookingSeriesApi.end).toHaveBeenCalledWith('c1', 'ser1', DATE);
    });
    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('Turno fijo cancelado: 3 reservas canceladas · 1 paga se mantiene');
    });
    expect(onClose).toHaveBeenCalled();
  });
});
