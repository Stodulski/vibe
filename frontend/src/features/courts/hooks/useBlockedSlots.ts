import { useQuery } from '@tanstack/react-query';
import { courtsApi } from '../api/courts.api';
import { queryKeys } from '@/shared/lib/queryKeys';

export function useBlockedSlots(complexId: string, dateFrom: string, dateTo: string) {
  return useQuery({
    queryKey: queryKeys.courts.blockedSlots(complexId, dateFrom, dateTo),
    queryFn: ({ signal }) => courtsApi.listBlockedSlots(complexId, dateFrom, dateTo, signal),
    enabled: !!complexId && !!dateFrom && !!dateTo,
    // Same minute as `useBooking`: a blocked slot is edited from this very
    // screen, so the calendar must not keep showing a window someone just
    // freed — but it changes far less often than availability does.
    staleTime: 60 * 1000,
    select: (data) => data.blocked_slots,
  });
}
