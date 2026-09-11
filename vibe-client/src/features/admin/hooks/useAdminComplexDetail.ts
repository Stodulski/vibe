import { useQuery } from '@tanstack/react-query';
import { queryKeys } from '@/shared/lib/queryKeys';
import { adminApi } from '../api/admin.api';

export function useAdminComplexDetail(complexId: string) {
  return useQuery({
    queryKey: queryKeys.admin.complexDetail(complexId),
    queryFn: ({ signal }) => adminApi.getComplexDetail(complexId, signal),
    enabled: !!complexId,
    staleTime: 2 * 60 * 1000,
  });
}
