import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { toast } from 'sonner';
import { makeBooking } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { queryKeys } from '@/shared/lib/queryKeys';
import { createQueryWrapper } from '@/test/test-utils';

vi.mock('../api/bookings.api', () => ({
  bookingsApi: {
    cancel: vi.fn(),
  },
}));

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn(), warning: vi.fn() },
}));

describe("useCancelBooking — tells the refund outcome from the complex's side", () => {
  it("describes an automatic refund as a success toast, in the owner's words", async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.cancel).mockResolvedValueOnce({
      booking: makeBooking({ status: 'cancelled' }),
      // The server's `message` is addressed to the person who booked.
      refund: { status: 'issued', message: 'Se reembolsaron $5.000 a tu tarjeta', amount: 500000 },
    });

    const { useCancelBooking } = await import('./useCancelBooking');
    const { result } = renderHook(() => useCancelBooking('c1', '2026-03-18'), {
      wrapper: createQueryWrapper(),
    });

    result.current.mutate({ bookingId: 'b1' });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(toast.success).toHaveBeenCalledWith(ES_AR.bookings.cancelSuccess, {
      description: ES_AR.bookings.refundOwner.issued,
    });
    expect(toast.warning).not.toHaveBeenCalled();
  });

  it('never echoes the server sentence written for the client', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    const clientFacing = 'El complejo tiene que devolverte este dinero a mano. Comunicate con ellos.';
    vi.mocked(bookingsApi.cancel).mockResolvedValueOnce({
      booking: makeBooking({ status: 'cancelled' }),
      refund: { status: 'manual', message: clientFacing },
    });

    const { useCancelBooking } = await import('./useCancelBooking');
    const { result } = renderHook(() => useCancelBooking('c1', '2026-03-18'), {
      wrapper: createQueryWrapper(),
    });

    result.current.mutate({ bookingId: 'b1' });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Telling the complex to "contact them" about their own money is the bug
    // this pins: the owner reads their own copy, keyed off `refund.status`.
    expect(toast.warning).toHaveBeenCalledWith(ES_AR.bookings.cancelSuccess, {
      description: ES_AR.bookings.refundOwner.manual,
    });
    expect(JSON.stringify(vi.mocked(toast.warning).mock.calls)).not.toContain(clientFacing);
    expect(toast.success).not.toHaveBeenCalled();
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call. `afterEach` below is file-scoped (declared
// outside both describes), so it still runs after every test in the file.
describe('useCancelBooking — cache invalidation', () => {
  it("invalidates the cancelled booking's detail query, not just the byDate list", async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.cancel).mockResolvedValueOnce({
      booking: makeBooking({ id: 'b1', status: 'cancelled' }),
      refund: { status: 'issued', message: 'Se reembolsaron $5.000 a tu tarjeta', amount: 500000 },
    });

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(queryKeys.bookings.detail('b1'), {
      booking: makeBooking({ id: 'b1' }),
    });
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { useCancelBooking } = await import('./useCancelBooking');
    const { result } = renderHook(() => useCancelBooking('c1', '2026-03-18'), { wrapper });

    result.current.mutate({ bookingId: 'b1' });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(queryClient.getQueryState(queryKeys.bookings.detail('b1'))?.isInvalidated).toBe(true);
  });
});

afterEach(async () => {
  const { bookingsApi } = await import('../api/bookings.api');
  vi.mocked(bookingsApi.cancel).mockReset();
});
