import { useQuery } from '@tanstack/react-query';
import { courtsApi } from '../api/courts.api';
import { queryKeys } from '@/shared/lib/queryKeys';

export function useBlockedSlots(complexId: string, dateFrom: string, dateTo: string) {
  return useQuery({
    queryKey: queryKeys.courts.blockedSlots(complexId, dateFrom, dateTo),
    queryFn: ({ signal }) => courtsApi.listBlockedSlots(complexId, dateFrom, dateTo, signal),
    enabled: !!complexId && !!dateFrom && !!dateTo,
    select: (data) => data.blocked_slots,
  });
}
