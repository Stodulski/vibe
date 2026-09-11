import { useQuery } from '@tanstack/react-query';
import { queryKeys } from '@/shared/lib/queryKeys';
import { adminApi } from '../api/admin.api';

export function useAdminUserDetail(userId: string) {
  return useQuery({
    queryKey: queryKeys.admin.userDetail(userId),
    queryFn: ({ signal }) => adminApi.getUserDetail(userId, signal),
    enabled: !!userId,
    staleTime: 2 * 60 * 1000,
  });
}
