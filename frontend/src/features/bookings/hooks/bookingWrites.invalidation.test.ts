import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider, type QueryKey } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { makeBooking } from '@/test/factories';
import { queryKeys } from '@/shared/lib/queryKeys';
import type { CancelBookingResponse, ConfirmPaymentResponse, ManualRefundResponse } from '@/shared/types/api.types';

vi.mock('../api/bookings.api', () => ({
  bookingsApi: { create: vi.fn(), cancel: vi.fn(), confirmPayment: vi.fn(), markManualRefund: vi.fn() },
}));
vi.mock('../api/bookingSeries.api', () => ({ bookingSeriesApi: { create: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn(), warning: vi.fn() } }));

const C = 'c1';

/**
 * Every screen a booking write can leave stale, each seeded as it is cached
 * when that screen is open, plus one unrelated feature that must stay put.
 */
const caches = {
  cashCurrent: queryKeys.cash.current(C),
  bookingsDue: queryKeys.cash.bookingsDue(C),
  clientsList: [...queryKeys.clients.byComplex(C), ''] as QueryKey,
  clientDetail: queryKeys.clients.detail(C, 'cl1'),
  series: queryKeys.bookings.series(C, 's1'),
  calendarDay: queryKeys.bookings.byDate(C, '2026-03-18'),
  products: queryKeys.products.byComplexAll(C),
};

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  for (const key of Object.values(caches)) queryClient.setQueryData(key, { seeded: true });
  queryClient.setQueryData(caches.calendarDay, { bookings: [makeBooking({ id: 'b1' })] });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  const invalidated = (name: keyof typeof caches) => queryClient.getQueryState(caches[name])?.isInvalidated;
  return { wrapper, invalidated };
}

async function api() {
  const { bookingsApi } = await import('../api/bookings.api');
  const { bookingSeriesApi } = await import('../api/bookingSeries.api');
  return { bookingsApi: vi.mocked(bookingsApi), bookingSeriesApi: vi.mocked(bookingSeriesApi) };
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('booking writes refresh Caja, Clientes and the series panel', () => {
  it('cobrar: confirming a payment', async () => {
    const { bookingsApi } = await api();
    bookingsApi.confirmPayment.mockResolvedValueOnce({ booking: makeBooking() } as ConfirmPaymentResponse);
    const { useConfirmPayment } = await import('./useConfirmPayment');
    const { wrapper, invalidated } = setup();
    const { result } = renderHook(() => useConfirmPayment(C, '2026-03-18'), { wrapper });

    result.current.mutate({ bookingId: 'b1', data: { amount: 1000, method: 'cash' } });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    for (const name of ['cashCurrent', 'bookingsDue', 'clientsList', 'clientDetail', 'series'] as const) {
      expect(invalidated(name), name).toBe(true);
    }
    expect(invalidated('products')).toBe(false);
  });

  it('cancelar: cancelling a booking', async () => {
    const { bookingsApi } = await api();
    bookingsApi.cancel.mockResolvedValueOnce({
      booking: makeBooking({ status: 'cancelled' }),
      refund: { status: 'none', message: '' },
    } as unknown as CancelBookingResponse);
    const { useCancelBooking } = await import('./useCancelBooking');
    const { wrapper, invalidated } = setup();
    const { result } = renderHook(() => useCancelBooking(C, '2026-03-18'), { wrapper });

    result.current.mutate({ bookingId: 'b1' });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    for (const name of ['cashCurrent', 'bookingsDue', 'clientsList', 'clientDetail', 'series'] as const) {
      expect(invalidated(name), name).toBe(true);
    }
    expect(invalidated('products')).toBe(false);
  });

  it('devolución manual: marking a refund as returned by hand', async () => {
    const { bookingsApi } = await api();
    bookingsApi.markManualRefund.mockResolvedValueOnce({ booking: makeBooking() } as unknown as ManualRefundResponse);
    const { useMarkManualRefund } = await import('./useMarkManualRefund');
    const { wrapper, invalidated } = setup();
    const { result } = renderHook(() => useMarkManualRefund(C, '2026-03-18'), { wrapper });

    result.current.mutate({ bookingId: 'b1' });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    for (const name of ['cashCurrent', 'clientsList', 'clientDetail', 'series'] as const) {
      expect(invalidated(name), name).toBe(true);
    }
    expect(invalidated('products')).toBe(false);
  });
});

describe('booking creations refresh Caja and Clientes', () => {
  it('crear: a booking, which may record a counter payment and a new client', async () => {
    const { bookingsApi } = await api();
    bookingsApi.create.mockResolvedValueOnce({ booking: makeBooking() });
    const { useCreateBooking } = await import('./useCreateBooking');
    const { wrapper, invalidated } = setup();
    const { result } = renderHook(() => useCreateBooking(C), { wrapper });

    result.current.mutate({
      court_id: 'ct1',
      date: '2026-03-18',
      start_time: '10:00',
      duration_minutes: 90,
      client_phone: '1155550000',
      client_first_name: 'Juan',
      client_last_name: 'Perez',
    });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    for (const name of ['cashCurrent', 'bookingsDue', 'clientsList', 'clientDetail'] as const) {
      expect(invalidated(name), name).toBe(true);
    }
    expect(invalidated('products')).toBe(false);
    // Its first import pulls `features/courts`, which is slow to transform cold.
  }, 30_000);

  it('crear turno fijo: a series creates bookings for its client', async () => {
    const { bookingSeriesApi } = await api();
    bookingSeriesApi.create.mockResolvedValueOnce({
      series: { weekday: 3, start_time: '10:00:00' },
      skipped: [],
    } as unknown as Awaited<ReturnType<typeof bookingSeriesApi.create>>);
    const { useCreateBookingSeries } = await import('./useCreateBookingSeries');
    const { wrapper, invalidated } = setup();
    const { result } = renderHook(() => useCreateBookingSeries(C), { wrapper });

    result.current.mutate({} as Parameters<typeof result.current.mutate>[0]);
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    for (const name of ['bookingsDue', 'clientsList', 'clientDetail'] as const) {
      expect(invalidated(name), name).toBe(true);
    }
    expect(invalidated('products')).toBe(false);
  });
});

describe('booking writes refresh the calendar day of a timestamped date', () => {
  // `Booking.date` can arrive as a full RFC 3339 timestamp (see the OpenAPI
  // note); the client drawer passes it straight through as the day.
  it('cancelling with a timestamp date refreshes that day of the calendar', async () => {
    const { bookingsApi } = await api();
    bookingsApi.cancel.mockResolvedValueOnce({
      booking: makeBooking({ status: 'cancelled' }),
      refund: { status: 'none', message: '' },
    } as unknown as CancelBookingResponse);
    const { useCancelBooking } = await import('./useCancelBooking');
    const { wrapper, invalidated } = setup();
    const { result } = renderHook(() => useCancelBooking(C, '2026-03-18T00:00:00Z'), { wrapper });

    result.current.mutate({ bookingId: 'b1' });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidated('calendarDay')).toBe(true);
  });
});
