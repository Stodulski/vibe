import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';

vi.mock('../api/bookings.api', () => ({
  bookingsApi: {
    cancel: vi.fn(),
    confirmPayment: vi.fn(),
    update: vi.fn(),
    markManualRefund: vi.fn(),
  },
}));

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn(), warning: vi.fn() },
}));

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return createElement(QueryClientProvider, { client: queryClient }, children);
}

// 03-owner-pages-dashboard.md M1: these mutations used to build on
// `selectedComplexId ?? ''` with only a comment vouching that `null` never
// reaches them in practice. This locks in the actual guard instead of the
// assumption: with no complex selected, every mutation here is a disabled
// no-op that can't reach the network.
describe("useBookingMutations — no complex selected is a disabled mutation, not `?? ''`", () => {
  it('does not call the API when cancelBooking.mutate is invoked with no complex selected', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    const { useBookingMutations } = await import('./useBookingMutations');

    const { result } = renderHook(() => useBookingMutations(null, '2026-03-18'), { wrapper });
    result.current.cancelBooking.mutate({ bookingId: 'b1' });

    expect(bookingsApi.cancel).not.toHaveBeenCalled();
  });

  it('does not call the API when confirmPayment.mutate is invoked with no complex selected', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    const { useBookingMutations } = await import('./useBookingMutations');

    const { result } = renderHook(() => useBookingMutations(null, '2026-03-18'), { wrapper });
    result.current.confirmPayment.mutate({
      bookingId: 'b1',
      data: { amount: 1000, method: 'cash' },
    });

    expect(bookingsApi.confirmPayment).not.toHaveBeenCalled();
  });

  it('does not call the API when updateBooking.mutate is invoked with no complex selected', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    const { useBookingMutations } = await import('./useBookingMutations');

    const { result } = renderHook(() => useBookingMutations(null, '2026-03-18'), { wrapper });
    result.current.updateBooking.mutate({ bookingId: 'b1', data: { status: 'no_show' } });

    expect(bookingsApi.update).not.toHaveBeenCalled();
  });

  it('does not call the API when markManualRefund.mutate is invoked with no complex selected', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    const { useBookingMutations } = await import('./useBookingMutations');

    const { result } = renderHook(() => useBookingMutations(null, '2026-03-18'), { wrapper });
    result.current.markManualRefund.mutate({ bookingId: 'b1' });

    expect(bookingsApi.markManualRefund).not.toHaveBeenCalled();
  });

  it('calls the API normally once a complex is selected', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.cancel).mockResolvedValueOnce({
      booking: { id: 'b1' } as never,
      refund: { status: 'not_applicable', message: '' } as never,
    });

    const { useBookingMutations } = await import('./useBookingMutations');
    const { result } = renderHook(() => useBookingMutations('c1', '2026-03-18'), { wrapper });
    result.current.cancelBooking.mutate({ bookingId: 'b1' });

    await waitFor(() => {
      expect(bookingsApi.cancel).toHaveBeenCalledWith('c1', 'b1', undefined);
    });
  });
});
