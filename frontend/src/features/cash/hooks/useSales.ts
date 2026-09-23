import { useInfiniteQuery } from '@tanstack/react-query';
import { salesApi } from '../api/sales.api';
import { queryKeys } from '@/shared/lib/queryKeys';

const PAGE_SIZE = 20;

/** This session's own sales, newest first, paginated — same shape as `useCashSessions`. */
export function useSales(complexId: string | null, sessionId: string | null) {
  const id = complexId ?? '';
  const sid = sessionId ?? '';
  return useInfiniteQuery({
    queryKey: queryKeys.sales.bySession(id, sid),
    queryFn: ({ pageParam, signal }) =>
      salesApi.list(id, { sessionId: sid, cursor: pageParam || undefined, limit: PAGE_SIZE }, signal),
    initialPageParam: '',
    getNextPageParam: (lastPage) => (lastPage.metadata.has_more ? lastPage.metadata.next_cursor : undefined),
    enabled: !!complexId && !!sessionId,
    staleTime: 30 * 1000,
  });
}
