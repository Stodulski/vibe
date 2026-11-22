import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { createQueryWrapper } from '@/test/test-utils';

vi.mock('../api/bookings.api', () => ({
  bookingsApi: {
    list: vi.fn(),
  },
}));

const NOW = new Date('2026-11-21T21:00:00Z').getTime();

function hold(courtId: string, freesInMs: number) {
  return {
    court_id: courtId,
    date: '2026-11-21',
    start_time: '20:00',
    end_time: '21:30',
    duration_minutes: 90,
    frees_at: new Date(NOW + freesInMs).toISOString(),
  };
}

describe('useOnlineHolds — the grid never shows a hold past its end', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true, now: NOW });
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('leaves out a hold whose frees_at has already passed', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.list).mockResolvedValue({
      bookings: [],
      online_holds: [hold('lapsed', -1_000), hold('live', 5 * 60_000)],
    } as never);

    const { useOnlineHolds } = await import('./useOnlineHolds');
    const { result } = renderHook(() => useOnlineHolds('c1', '2026-11-21'), { wrapper: createQueryWrapper() });

    await waitFor(() => {
      expect(result.current.data.map((h) => h.court_id)).toEqual(['live']);
    });
  });

  it('hides a hold when it frees up and asks the server again then', async () => {
    const { bookingsApi } = await import('../api/bookings.api');
    vi.mocked(bookingsApi.list).mockResolvedValueOnce({ bookings: [], online_holds: [hold('live', 60_000)] } as never);
    // At its end the server has nothing more: the visitor did not pay.
    vi.mocked(bookingsApi.list).mockResolvedValue({ bookings: [], online_holds: [] } as never);

    const { useOnlineHolds } = await import('./useOnlineHolds');
    const { result } = renderHook(() => useOnlineHolds('c1', '2026-11-21'), { wrapper: createQueryWrapper() });
    await waitFor(() => {
      expect(result.current.data).toHaveLength(1);
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(61_000);
    });

    expect(result.current.data).toHaveLength(0);
    await waitFor(() => {
      expect(bookingsApi.list).toHaveBeenCalledTimes(2);
    });
  });
});
