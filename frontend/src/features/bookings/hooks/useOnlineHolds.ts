import { useQuery } from '@tanstack/react-query';
import { bookingsApi } from '../api/bookings.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { parseOnlineHolds } from '../schemas/onlineHold.schema';

/**
 * The hours visitors are booking online on the day. They ride on the same
 * response as the day's bookings, so this is the same query as `useBookings`
 * with another `select`: one request, one cache entry, refreshed together
 * (realtime invalidation included).
 */
export function useOnlineHolds(complexId: string | null, date: string) {
  return useQuery({
    queryKey: queryKeys.bookings.byDate(complexId ?? '', date),
    queryFn: ({ signal }) => {
      if (!complexId) return Promise.reject(new Error('complexId is required'));
      return bookingsApi.list(complexId, date, undefined, undefined, signal);
    },
    select: (data) => parseOnlineHolds((data as { online_holds?: unknown }).online_holds),
    enabled: !!complexId && !!date,
    staleTime: 5 * 60 * 1000,
  });
}
