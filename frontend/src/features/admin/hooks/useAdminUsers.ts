import { useInfiniteQuery, keepPreviousData } from '@tanstack/react-query';
import { queryKeys } from '@/shared/lib/queryKeys';
import { adminApi } from '../api/admin.api';
import { ADMIN_PAGE_SIZE } from '@/shared/lib/constants';

export function useAdminUsers(search: string, role: string) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.admin.users({ search, role })],
    queryFn: ({ pageParam, signal }) =>
      adminApi.getUsers(
        {
          search: search || undefined,
          role: role || undefined,
          cursor: pageParam || undefined,
          limit: ADMIN_PAGE_SIZE,
        },
        signal,
      ),
    initialPageParam: '',
    getNextPageParam: (lastPage) => (lastPage.metadata.has_more ? lastPage.metadata.next_cursor : undefined),
    staleTime: 2 * 60 * 1000,
    // The search text is part of the queryKey, so every keystroke is a brand
    // new cache entry — and without this the list emptied to a skeleton between
    // one letter and the next, which is the one thing a filter must never do:
    // the rows someone is narrowing down disappear while they narrow them. The
    // previous answer stays on screen, marked `isPlaceholderData`, until the
    // new one replaces it.
    placeholderData: keepPreviousData,
  });
}
