import { useQuery } from '@tanstack/react-query';
import { clientsApi } from '../api/clients.api';
import { queryKeys } from '@/shared/lib/queryKeys';

export function useClient(complexId: string | null, clientId: string | null) {
  // `enabled` gates the query on both ids being non-null, so `queryFn` only
  // ever runs once they're real strings — same guard-ladder precedent as
  // `useCourts.ts`/`useBookingsPage.ts`.
  const id = complexId ?? '';
  const cId = clientId ?? '';
  return useQuery({
    queryKey: queryKeys.clients.detail(id, cId),
    queryFn: ({ signal }) => clientsApi.getById(id, cId, signal),
    enabled: !!complexId && !!clientId,
    staleTime: 5 * 60 * 1000,
  });
}
