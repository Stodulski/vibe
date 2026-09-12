import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { makeConsumedHttpError } from '@/test/factories';
import { createQueryWrapper } from '@/test/test-utils';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { bookingStatusResponseSchema } from '@/shared/schemas/publicBooking.schema';

function makeSchemaRejectionError() {
  const result = bookingStatusResponseSchema.safeParse({
    booking: { status: 'not-a-real-status' },
  });
  if (result.success) throw new Error('expected the fixture to fail schema validation');
  return new ApiResponseError('publicBookingApi.getBookingStatus', result.error);
}

const getBookingStatus = vi.fn<(...args: unknown[]) => Promise<unknown>>();
vi.mock('../api/public-booking.api', () => ({
  publicBookingApi: {
    getBookingStatus: (...args: unknown[]) => getBookingStatus(...args),
  },
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe('useBookingStatus link status (resolveLink 404 vs 410)', () => {
  it('sets linkExpired (not linkNotFound) on a 410 response', async () => {
    getBookingStatus.mockRejectedValue(await makeConsumedHttpError(410, { error: 'link expired' }));

    const { useBookingStatus } = await import('./useBookingStatus');
    const { result } = renderHook(() => useBookingStatus('tok-1'), { wrapper: createQueryWrapper() });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.linkExpired).toBe(true);
    expect(result.current.linkNotFound).toBe(false);
  });

  it('sets linkNotFound (not linkExpired) on a 404 response', async () => {
    getBookingStatus.mockRejectedValue(await makeConsumedHttpError(404, { error: 'not found' }));

    const { useBookingStatus } = await import('./useBookingStatus');
    const { result } = renderHook(() => useBookingStatus('tok-2'), { wrapper: createQueryWrapper() });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.linkNotFound).toBe(true);
    expect(result.current.linkExpired).toBe(false);
  });

  // M11: 404/410 were covered, but a plain 500 or a dropped connection
  // (M2's actual bug) was not — this is what let it read as "still loading"
  // instead of a real error.
  it('sets isError true with both link flags false on a 500 response', async () => {
    getBookingStatus.mockRejectedValue(await makeConsumedHttpError(500, { error: 'internal error' }));

    const { useBookingStatus } = await import('./useBookingStatus');
    const { result } = renderHook(() => useBookingStatus('tok-500'), { wrapper: createQueryWrapper() });

    await waitFor(
      () => {
        expect(result.current.isError).toBe(true);
      },
      { timeout: 5000 },
    );

    expect(result.current.data).toBeUndefined();
    expect(result.current.linkExpired).toBe(false);
    expect(result.current.linkNotFound).toBe(false);
  }, 8000);

  it('sets isError true with both link flags false on a network error (no response at all)', async () => {
    getBookingStatus.mockRejectedValue(new TypeError('Failed to fetch'));

    const { useBookingStatus } = await import('./useBookingStatus');
    const { result } = renderHook(() => useBookingStatus('tok-network'), {
      wrapper: createQueryWrapper(),
    });

    await waitFor(
      () => {
        expect(result.current.isError).toBe(true);
      },
      { timeout: 5000 },
    );

    expect(result.current.data).toBeUndefined();
    expect(result.current.linkExpired).toBe(false);
    expect(result.current.linkNotFound).toBe(false);
  }, 8000);
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call.
describe('useBookingStatus link status — success case', () => {
  it('leaves both flags false when the request succeeds', async () => {
    getBookingStatus.mockResolvedValue({
      booking: { status: 'confirmed', collection_status: 'deposit_paid', refund_status: 'none' },
    });

    const { useBookingStatus } = await import('./useBookingStatus');
    const { result } = renderHook(() => useBookingStatus('tok-3'), { wrapper: createQueryWrapper() });

    await waitFor(() => {
      expect(result.current.data?.status).toBe('confirmed');
    });

    expect(result.current.linkExpired).toBe(false);
    expect(result.current.linkNotFound).toBe(false);
  });
});

describe('useBookingStatus schema-rejected polling', () => {
  it('settles into the error state and stops polling instead of retrying forever', async () => {
    getBookingStatus.mockRejectedValue(makeSchemaRejectionError());

    const { useBookingStatus } = await import('./useBookingStatus');
    const { result } = renderHook(() => useBookingStatus('tok-4'), { wrapper: createQueryWrapper() });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
    expect(result.current.error).toBeInstanceOf(ApiResponseError);
    expect(getBookingStatus).toHaveBeenCalledTimes(1);

    // Before the fix, `refetchInterval` didn't recognize `ApiResponseError`
    // and kept scheduling a refetch every 2s forever — the caller never saw
    // anything but "still processing". Waiting past that interval and
    // finding no new call proves the query actually stopped instead of
    // silently retrying the same malformed response.
    await new Promise((resolve) => setTimeout(resolve, 2500));

    expect(getBookingStatus).toHaveBeenCalledTimes(1);
    expect(result.current.isError).toBe(true);
  }, 8000);
});
