import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { makeBooking } from '@/test/factories';
import type { CreateBookingRequest } from '@/shared/types/api.types';

vi.mock('../api/bookings.api', () => ({
  bookingsApi: { create: vi.fn() },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

/**
 * One retry, not the app's zero: the key's whole job is to be the same across
 * the attempts of one submit, and a client that never retries cannot show that.
 */
function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: 1, retryDelay: 0 } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

const BOOKING: CreateBookingRequest = {
  court_id: 'ct1',
  date: '2026-03-18',
  start_time: '10:00',
  duration_minutes: 90,
  client_phone: '1155550000',
  client_first_name: 'Juan',
  client_last_name: 'Perez',
};

/** The `Idempotency-Key` argument of each `bookingsApi.create` call, in order. */
function sentKeys(create: { mock: { calls: unknown[][] } }): string[] {
  return create.mock.calls.map((call) => call[2] as string);
}

describe('useCreateBooking — Idempotency-Key', () => {
  afterEach(async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.create).mockReset();
  });

  it('sends a key with the request', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.create).mockResolvedValue({ booking: makeBooking() });

    const { useCreateBooking } = await import('./useCreateBooking');
    const { result } = renderHook(() => useCreateBooking('c1'), { wrapper: createWrapper() });

    result.current.mutate(BOOKING);
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(bookingsApi.create).toHaveBeenCalledWith('c1', BOOKING, expect.any(String));
    expect(sentKeys(vi.mocked(bookingsApi.create))[0]).toMatch(/^[0-9a-f-]{36}$/);
  });

  it('reuses the same key when the attempt is retried, so the server replays instead of double-booking', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.create)
      .mockRejectedValueOnce(new Error('connection dropped'))
      .mockResolvedValueOnce({ booking: makeBooking() });

    const { useCreateBooking } = await import('./useCreateBooking');
    const { result } = renderHook(() => useCreateBooking('c1'), { wrapper: createWrapper() });

    result.current.mutate(BOOKING);
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const keys = sentKeys(vi.mocked(bookingsApi.create));
    expect(keys).toHaveLength(2);
    expect(keys[1]).toBe(keys[0]);
  });

  it('mints a new key for a genuinely new submit, so a second booking is not refused as a replay', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.create).mockResolvedValue({ booking: makeBooking() });

    const { useCreateBooking } = await import('./useCreateBooking');
    const { result } = renderHook(() => useCreateBooking('c1'), { wrapper: createWrapper() });

    result.current.mutate(BOOKING);
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    result.current.mutate({ ...BOOKING, start_time: '11:00' });
    await waitFor(() => {
      expect(vi.mocked(bookingsApi.create)).toHaveBeenCalledTimes(2);
    });

    const keys = sentKeys(vi.mocked(bookingsApi.create));
    expect(keys[1]).not.toBe(keys[0]);
  });
});
