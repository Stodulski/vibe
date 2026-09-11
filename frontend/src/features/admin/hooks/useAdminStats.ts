import { useQuery } from '@tanstack/react-query';
import { queryKeys } from '@/shared/lib/queryKeys';
import { adminApi } from '../api/admin.api';

export function useAdminStats() {
  return useQuery({
    queryKey: queryKeys.admin.stats,
    queryFn: ({ signal }) => adminApi.getStats(signal),
    select: (data) => data.stats,
    staleTime: 5 * 60 * 1000,
  });
}
