import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { createQueryWrapper } from '@/test/test-utils';
import { makeBooking } from '@/test/factories';
import { ApiResponseError } from '@/shared/lib/apiParse';

vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

describe('useBookingsByDate', () => {
  it('calls GET complexes/:id/bookings and returns the parsed bookings array', async () => {
    const booking = makeBooking({ id: 'b1' });
    let receivedDate: string | null = null;
    let receivedLimit: string | null = null;
    server.use(
      http.get('*/complexes/:complexId/bookings', ({ request }) => {
        const params = new URL(request.url).searchParams;
        receivedDate = params.get('date');
        receivedLimit = params.get('limit');
        return HttpResponse.json({ bookings: [booking], metadata: { has_more: false } });
      }),
    );

    const { useBookingsByDate } = await import('./useBookingsByDate');
    const { result } = renderHook(() => useBookingsByDate('c1', '2026-03-18'), {
      wrapper: createQueryWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual([booking]);
    expect(receivedDate).toBe('2026-03-18');
    expect(receivedLimit).toBe('200');
  });

  // Proves the schema wired into the query (H-schemas.md's wiring table)
  // actually rejects a malformed response instead of handing the caller
  // `undefined`/partial bookings.
  it('rejects with ApiResponseError when the response body does not match the schema', async () => {
    server.use(
      http.get('*/complexes/:complexId/bookings', () =>
        HttpResponse.json({ bookings: [{ id: 'b1' }], metadata: { has_more: false } }),
      ),
    );

    const { useBookingsByDate } = await import('./useBookingsByDate');
    const { result } = renderHook(() => useBookingsByDate('c1', '2026-03-18'), {
      wrapper: createQueryWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeInstanceOf(ApiResponseError);
  });
});
