import { describe, it, expect, vi, beforeEach } from 'vitest';
import { toast } from 'sonner';
import { renderWithProviders, screen, userEvent, waitFor } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { FixedBlockDetail } from './FixedBlockDetail';
import type { FixedBlock } from '@/shared/types/api.types';

vi.mock('../api/bookingSeries.api', () => ({ bookingSeriesApi: { skip: vi.fn(), end: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.bookings.series;

// A Tuesday far ahead, so the date is never in the past.
const DATE = '2099-01-06';

const block: FixedBlock = {
  series_id: 'ser1',
  court_id: 'ct1',
  date: DATE,
  start_time: '21:00',
  end_time: '22:30',
  duration_minutes: 90,
  client_name: 'Ana Gomez',
};

async function renderDetail(overrides: Partial<FixedBlock> = {}) {
  const { bookingSeriesApi } = await import('../api/bookingSeries.api');
  const onClose = vi.fn();
  renderWithProviders(
    <FixedBlockDetail open onClose={onClose} block={{ ...block, ...overrides }} courtName="Cancha 2" complexId="c1" />,
  );
  return { bookingSeriesApi, onClose };
}

describe('FixedBlockDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('names the weekly rule, the client, the court and the date', async () => {
    await renderDetail();

    expect(await screen.findByText(t.label('martes', '21:00'))).toBeInTheDocument();
    expect(screen.getByText('Ana Gomez')).toBeInTheDocument();
    expect(screen.getByText('Cancha 2')).toBeInTheDocument();
  });

  it('"Cancelar solo esta fecha" frees that date and says which', async () => {
    const user = userEvent.setup();
    const { bookingSeriesApi, onClose } = await renderDetail();
    vi.mocked(bookingSeriesApi.skip).mockResolvedValue({ skipped: { date: DATE, reason: 'cancelled_by_owner' } });

    await user.click(await screen.findByRole('button', { name: t.cancelOne }));

    await waitFor(() => {
      expect(bookingSeriesApi.skip).toHaveBeenCalledWith('c1', 'ser1', DATE);
    });
    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('Fecha liberada: 06/01');
    });
    expect(onClose).toHaveBeenCalled();
    expect(bookingSeriesApi.end).not.toHaveBeenCalled();
  });
});

describe('FixedBlockDetail — cancelling the whole fixed booking', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('"Cancelar turno fijo" ends the series from that date after confirming', async () => {
    const user = userEvent.setup();
    const { bookingSeriesApi, onClose } = await renderDetail();
    vi.mocked(bookingSeriesApi.end).mockResolvedValue({
      series: {
        id: 'ser1',
        complex_id: 'c1',
        court_id: 'ct1',
        client_id: 'cl1',
        weekday: 2,
        start_time: '21:00',
        duration_minutes: 90,
        starts_on: DATE,
        status: 'ended',
        created_at: '2026-11-13T00:00:00Z',
      },
      cancelled: [],
      kept: [],
    });

    await user.click(await screen.findByRole('button', { name: t.endFromHere }));
    expect(bookingSeriesApi.end).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: t.endConfirmLabel }));

    await waitFor(() => {
      expect(bookingSeriesApi.end).toHaveBeenCalledWith('c1', 'ser1', DATE);
    });
    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('Turno fijo cancelado');
    });
    expect(onClose).toHaveBeenCalled();
    expect(bookingSeriesApi.skip).not.toHaveBeenCalled();
  });

  it('offers no cancellation for a date already in the past', async () => {
    await renderDetail({ date: '2020-01-07' });

    await screen.findByText(t.label('martes', '21:00'));
    expect(screen.queryByRole('button', { name: t.cancelOne })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.endFromHere })).not.toBeInTheDocument();
  });
});
