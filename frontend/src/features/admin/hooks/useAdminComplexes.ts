import { useInfiniteQuery } from '@tanstack/react-query';
import { queryKeys } from '@/shared/lib/queryKeys';
import { adminApi } from '../api/admin.api';
import { ADMIN_PAGE_SIZE } from '@/shared/lib/constants';

export function useAdminComplexes(search: string) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.admin.complexes({ search })],
    queryFn: ({ pageParam, signal }) =>
      adminApi.getComplexes(
        {
          search: search || undefined,
          cursor: pageParam || undefined,
          limit: ADMIN_PAGE_SIZE,
        },
        signal,
      ),
    initialPageParam: '',
    getNextPageParam: (lastPage) => (lastPage.metadata.has_more ? lastPage.metadata.next_cursor : undefined),
    staleTime: 2 * 60 * 1000,
  });
}
