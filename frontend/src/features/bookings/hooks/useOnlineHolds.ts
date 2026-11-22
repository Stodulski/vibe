import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { bookingsApi } from '../api/bookings.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { parseOnlineHolds, type OnlineHold } from '../schemas/onlineHold.schema';

const NO_HOLDS: OnlineHold[] = [];

/**
 * The hours visitors are booking online on the day. They ride on the same
 * response as the day's bookings, so this is the same query as `useBookings`
 * with another `select`: one request, one cache entry, refreshed together
 * (realtime invalidation included).
 *
 * A hold ends by itself at its `frees_at` when nobody pays, and nothing tells
 * the grid it did. So a hold past its `frees_at` is left out, and the day is
 * asked for again when the next one frees up: by then it was paid (a booking),
 * kept for a payment in process (a later `frees_at`) or let go (gone).
 */
export function useOnlineHolds(complexId: string | null, date: string) {
  const query = useQuery({
    queryKey: queryKeys.bookings.byDate(complexId ?? '', date),
    queryFn: ({ signal }) => {
      if (!complexId) return Promise.reject(new Error('complexId is required'));
      return bookingsApi.list(complexId, date, undefined, undefined, signal);
    },
    select: (data) => parseOnlineHolds((data as { online_holds?: unknown }).online_holds),
    enabled: !!complexId && !!date,
    staleTime: 5 * 60 * 1000,
  });

  const [now, setNow] = useState(() => Date.now());
  const holds = query.data ?? NO_HOLDS;
  const live = holds.filter((h) => Date.parse(h.frees_at) > now);
  const nextFree = live.length > 0 ? Math.min(...live.map((h) => Date.parse(h.frees_at))) : undefined;

  const { refetch } = query;
  useEffect(() => {
    if (nextFree === undefined) return;
    const timer = setTimeout(
      () => {
        setNow(Date.now());
        void refetch();
      },
      Math.max(0, nextFree - Date.now()),
    );
    return () => {
      clearTimeout(timer);
    };
  }, [nextFree, refetch]);

  return { data: live };
}
