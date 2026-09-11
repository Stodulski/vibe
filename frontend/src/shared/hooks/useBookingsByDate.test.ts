import { renderHook, waitFor } from '@testing-library/react';
import { createWrapper } from '@/test/test-utils';
import { makeBooking } from '@/test/factories';
import { ApiResponseError } from '@/shared/lib/apiParse';

const { mockGet } = vi.hoisted(() => ({
  mockGet: vi.fn(),
}));

vi.mock('@/shared/lib/ky', () => ({
  default: { get: mockGet },
}));

vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

function mockJsonOnce(data: unknown) {
  mockGet.mockReturnValueOnce({ json: vi.fn().mockResolvedValue(data) });
}

describe('useBookingsByDate', () => {
  beforeEach(() => {
    mockGet.mockClear();
  });

  it('calls GET complexes/:id/bookings and returns the parsed bookings array', async () => {
    const booking = makeBooking({ id: 'b1' });
    mockJsonOnce({
      bookings: [booking],
      metadata: { has_more: false },
    });

    const { useBookingsByDate } = await import('./useBookingsByDate');
    const { result } = renderHook(() => useBookingsByDate('c1', '2026-03-18'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual([booking]);
    expect(mockGet).toHaveBeenCalledWith(
      'complexes/c1/bookings',
      expect.objectContaining({
        searchParams: { date: '2026-03-18', limit: '200' },
      }),
    );
  });

  // Proves the schema wired into the query (H-schemas.md's wiring table)
  // actually rejects a malformed response instead of handing the caller
  // `undefined`/partial bookings.
  it('rejects with ApiResponseError when the response body does not match the schema', async () => {
    mockJsonOnce({ bookings: [{ id: 'b1' }], metadata: { has_more: false } });

    const { useBookingsByDate } = await import('./useBookingsByDate');
    const { result } = renderHook(() => useBookingsByDate('c1', '2026-03-18'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeInstanceOf(ApiResponseError);
  });
});
