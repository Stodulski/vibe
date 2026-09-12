import { describe, it, expect, vi } from 'vitest';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { makeBooking } from '@/test/factories';
import { useBookingModals } from './useBookingModals';

vi.mock('../api/bookings.api', () => ({
  bookingsApi: { getById: vi.fn() },
}));

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return createElement(QueryClientProvider, { client: queryClient }, children);
}

describe('useBookingModals — selection is an id resolved from the query cache, not a stored object (03-owner-pages-dashboard.md M8)', () => {
  it('replaces the selected booking with the fresher one the cache resolves to, once a complexId is given', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    const fresh = makeBooking({ id: 'b1', status: 'cancelled' });
    vi.mocked(bookingsApi.getById).mockResolvedValueOnce({ booking: fresh, payments: [] });

    const { result } = renderHook(() => useBookingModals('2026-03-18', 'c1'), { wrapper });

    // Selected from a list row that only ever had the stale "confirmed" copy.
    const staleFromList = makeBooking({ id: 'b1', status: 'confirmed' });
    act(() => {
      result.current.handleSelectBooking(staleFromList);
    });

    expect(result.current.selectedBookingId).toBe('b1');
    // Immediately after selecting, before the cache read resolves, the seed
    // is what's shown — this is the one render the fallback exists for.
    expect(result.current.selectedBooking?.status).toBe('confirmed');

    await waitFor(() => {
      expect(result.current.selectedBooking?.status).toBe('cancelled');
    });
  });

  it('falls back to the selected object when no complexId is available (back-compat for a caller that has not passed one yet)', () => {
    const { result } = renderHook(() => useBookingModals('2026-03-18'), { wrapper });

    const booking = makeBooking({ id: 'b1', status: 'confirmed' });
    act(() => {
      result.current.handleSelectBooking(booking);
    });

    expect(result.current.selectedBooking).toEqual(booking);
  });

  it('clears the selection through setSelectedBooking(null), same as before', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.getById).mockResolvedValueOnce({
      booking: makeBooking({ id: 'b1' }),
      payments: [],
    });

    const { result } = renderHook(() => useBookingModals('2026-03-18', 'c1'), { wrapper });

    act(() => {
      result.current.handleSelectBooking(makeBooking({ id: 'b1' }));
    });
    expect(result.current.selectedBookingId).toBe('b1');

    act(() => {
      result.current.setSelectedBooking(null);
    });
    expect(result.current.selectedBookingId).toBeNull();
    expect(result.current.selectedBooking).toBeNull();
  });
});
