import { useInfiniteQuery } from '@tanstack/react-query';
import { clientsApi } from '../api/clients.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { CLIENTS_PAGE_SIZE } from '@/shared/lib/constants';

const PAGE_SIZE = CLIENTS_PAGE_SIZE;

export function useClients(complexId: string | null, search: string) {
  // `enabled` gates the query on `complexId` being non-null, so `queryFn`
  // only ever runs once it's a real string.
  const id = complexId ?? '';
  return useInfiniteQuery({
    queryKey: [...queryKeys.clients.byComplex(id), search],
    queryFn: ({ pageParam, signal }) =>
      clientsApi.list(
        id,
        {
          search: search || undefined,
          cursor: pageParam || undefined,
          limit: PAGE_SIZE,
        },
        signal,
      ),
    initialPageParam: '',
    getNextPageParam: (lastPage) => (lastPage.metadata.has_more ? lastPage.metadata.next_cursor : undefined),
    enabled: !!complexId,
    staleTime: 5 * 60 * 1000,
  });
}
