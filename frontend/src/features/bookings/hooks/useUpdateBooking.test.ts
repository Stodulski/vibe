import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { toast } from 'sonner';
import { makeBooking } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { queryKeys } from '@/shared/lib/queryKeys';

vi.mock('../api/bookings.api', () => ({
  bookingsApi: {
    update: vi.fn(),
  },
}));

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

describe('useUpdateBooking', () => {
  afterEach(async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.update).mockReset();
    vi.clearAllMocks();
  });

  it("invalidates the updated booking's detail query, not just the byComplex list", async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.update).mockResolvedValueOnce({
      booking: makeBooking({ id: 'b1', status: 'no_show' }),
    });

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(queryKeys.bookings.detail('b1'), {
      booking: makeBooking({ id: 'b1' }),
    });
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { useUpdateBooking } = await import('./useUpdateBooking');
    const { result } = renderHook(() => useUpdateBooking('c1'), { wrapper });

    result.current.mutate({ bookingId: 'b1', data: { status: 'no_show' } });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(queryClient.getQueryState(queryKeys.bookings.detail('b1'))?.isInvalidated).toBe(true);
    expect(toast.success).toHaveBeenCalledWith(ES_AR.bookings.updateSuccess);
  });

  it('shows an error toast when the update fails', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.update).mockRejectedValueOnce(new Error('network'));

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { useUpdateBooking } = await import('./useUpdateBooking');
    const { result } = renderHook(() => useUpdateBooking('c1'), { wrapper });

    result.current.mutate({ bookingId: 'b1', data: { status: 'no_show' } });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalled();
  });
});
