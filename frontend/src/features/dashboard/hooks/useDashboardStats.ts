import { useQuery } from '@tanstack/react-query';
import { dashboardApi } from '../api/dashboard.api';
import { queryKeys } from '@/shared/lib/queryKeys';

export function useDashboardStats(complexId: string | null) {
  // `enabled` gates the query on `complexId` being non-null, so `queryFn`
  // only ever runs once it's a real string.
  const id = complexId ?? '';
  return useQuery({
    queryKey: queryKeys.dashboard.stats(id),
    queryFn: ({ signal }) => dashboardApi.getStats(id, signal),
    select: (data) => data.stats,
    enabled: !!complexId,
    staleTime: 5 * 60 * 1000,
  });
}
