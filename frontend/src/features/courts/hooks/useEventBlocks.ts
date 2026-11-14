import { useQuery } from '@tanstack/react-query';
import { courtsApi } from '../api/courts.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/**
 * The hours open-play events hold in the range ("canchas libres"). Same request
 * and cache entry as `useBlockedSlots` and `useFixedBlocks`: the server sends
 * the three lists in one response.
 */
export function useEventBlocks(complexId: string, dateFrom: string, dateTo: string) {
  return useQuery({
    queryKey: queryKeys.courts.blockedSlots(complexId, dateFrom, dateTo),
    queryFn: ({ signal }) => courtsApi.listBlockedSlots(complexId, dateFrom, dateTo, signal),
    enabled: !!complexId && !!dateFrom && !!dateTo,
    staleTime: 60 * 1000,
    select: (data) => data.event_blocks,
  });
}
