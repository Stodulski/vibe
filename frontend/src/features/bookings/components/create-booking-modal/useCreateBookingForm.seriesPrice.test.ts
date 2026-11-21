import { describe, it, expect, vi } from 'vitest';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { makeCourt, makePrice } from '@/test/factories';
import { useCreateBookingForm } from './useCreateBookingForm';
import type { CourtWithPrices, Schedule } from '@/shared/types/api.types';

vi.mock('../../api/bookingSeries.api', () => ({ bookingSeriesApi: { create: vi.fn() } }));
vi.mock('../../api/bookings.api', () => ({ bookingsApi: { create: vi.fn(), list: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn(), warning: vi.fn() } }));

const days = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'] as const;
const SCHEDULES: Schedule[] = days.map((day, i) => ({
  id: `s${String(i)}`,
  complex_id: 'c1',
  day,
  open_time: '08:00',
  close_time: '23:00',
  is_closed: false,
}));

// 2099-01-02 is a Friday, far enough ahead to clear the "not in the past" rule.
const FRIDAY = '2099-01-02';

const court: CourtWithPrices = {
  ...makeCourt({ id: 'ct1' }),
  prices: [makePrice({ day_type: 'friday', time_from: '08:00', time_to: '23:00', price: 100000 })],
};

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return createElement(QueryClientProvider, { client: queryClient }, children);
}

// A manual price typed while an earlier span had no rate stays in the form
// once the owner picks an hour that has one: the field is hidden, not cleared.
// A series with a price is priced at it every week, so the stale value must
// not travel.
describe('useCreateBookingForm, series with a stale manual price', () => {
  it('sends no price when the span has a computed estimate', async () => {
    const { bookingSeriesApi } = await import('../../api/bookingSeries.api');
    vi.mocked(bookingSeriesApi.create).mockResolvedValueOnce({
      series: { weekday: 5, start_time: '10:00:00' },
      bookings: [],
      skipped: [],
    } as never);
    const { result } = renderHook(
      () =>
        useCreateBookingForm({
          open: false,
          complexId: 'c1',
          courts: [court],
          depositPercentage: 0,
          schedules: SCHEDULES,
        }),
      { wrapper },
    );
    act(() => {
      const form = result.current;
      form.setValue('court_id', 'ct1');
      form.setValue('date', FRIDAY);
      form.setValue('start_time', '10:00');
      form.setValue('duration_minutes', 60);
      form.setValue('client_phone', '+541155550000');
      form.setValue('client_first_name', 'Juan');
      form.setValue('client_last_name', 'Perez');
      form.setValue('price', 5000);
      form.series.setRepeat(true);
    });
    await waitFor(() => {
      expect(result.current.estimatedPrice).toBe(100000);
    });

    const onClose = vi.fn();
    await act(async () => {
      await result.current.handleSubmit((data) => {
        result.current.onSubmit(data, onClose);
      })();
    });

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
    expect(vi.mocked(bookingSeriesApi.create).mock.calls[0]?.[1]).not.toHaveProperty('price');
  });
});
