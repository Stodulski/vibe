import { useQuery } from '@tanstack/react-query';
import { courtsApi } from '../api/courts.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/**
 * The dates fixed weekly bookings hold in the range that have no booking yet.
 * Same request and cache entry as `useBlockedSlots`, which is where the server
 * sends them: the two lists are read off one response.
 */
export function useFixedBlocks(complexId: string, dateFrom: string, dateTo: string) {
  return useQuery({
    queryKey: queryKeys.courts.blockedSlots(complexId, dateFrom, dateTo),
    queryFn: ({ signal }) => courtsApi.listBlockedSlots(complexId, dateFrom, dateTo, signal),
    enabled: !!complexId && !!dateFrom && !!dateTo,
    staleTime: 60 * 1000,
    select: (data) => data.fixed_blocks,
  });
}
