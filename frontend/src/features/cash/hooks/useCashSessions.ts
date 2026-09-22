import { useInfiniteQuery } from '@tanstack/react-query';
import { cashApi } from '../api/cash.api';
import { queryKeys } from '@/shared/lib/queryKeys';

const PAGE_SIZE = 20;

/** The complex's cash session history — newest (most recently opened) first, paginated. */
export function useCashSessions(complexId: string | null) {
  const id = complexId ?? '';
  return useInfiniteQuery({
    queryKey: queryKeys.cash.sessions(id),
    queryFn: ({ pageParam, signal }) => cashApi.list(id, { cursor: pageParam || undefined, limit: PAGE_SIZE }, signal),
    initialPageParam: '',
    getNextPageParam: (lastPage) => (lastPage.metadata.has_more ? lastPage.metadata.next_cursor : undefined),
    enabled: !!complexId,
    staleTime: 60 * 1000,
  });
}
