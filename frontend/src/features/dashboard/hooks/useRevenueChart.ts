import { useQuery } from '@tanstack/react-query';
import { dashboardApi } from '../api/dashboard.api';
import { queryKeys } from '@/shared/lib/queryKeys';

export function useRevenueChart(complexId: string | null, period: 'week' | 'month') {
  // `enabled` gates the query on `complexId` being non-null, so `queryFn`
  // only ever runs once it's a real string.
  const id = complexId ?? '';
  return useQuery({
    queryKey: queryKeys.dashboard.revenue(id, period),
    queryFn: ({ signal }) => dashboardApi.getRevenue(id, period, signal),
    select: (data) => data.revenue,
    enabled: !!complexId,
    staleTime: 10 * 60 * 1000,
  });
}
