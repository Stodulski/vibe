import { useQuery } from '@tanstack/react-query';
import { courtsApi } from '../api/courts.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/**
 * The hours scheduled tournament matches hold in the range. Same request and
 * cache entry as `useBlockedSlots`, `useFixedBlocks` and `useEventBlocks`: the
 * server sends the four lists in one response.
 */
export function useMatchBlocks(complexId: string, dateFrom: string, dateTo: string) {
  return useQuery({
    queryKey: queryKeys.courts.blockedSlots(complexId, dateFrom, dateTo),
    queryFn: ({ signal }) => courtsApi.listBlockedSlots(complexId, dateFrom, dateTo, signal),
    enabled: !!complexId && !!dateFrom && !!dateTo,
    staleTime: 60 * 1000,
    select: (data) => data.match_blocks,
  });
}
