import { useQuery } from '@tanstack/react-query';
import { dashboardApi } from '../api/dashboard.api';
import { queryKeys } from '@/shared/lib/queryKeys';

export function useMonthlyReport(complexId: string | null, month: number, year: number) {
  // `enabled` gates the query on `complexId` being non-null, so `queryFn`
  // only ever runs once it's a real string.
  const id = complexId ?? '';
  return useQuery({
    queryKey: [...queryKeys.dashboard.stats(id), 'monthly-report', month, year],
    queryFn: ({ signal }) => dashboardApi.getMonthlyReport(id, month, year, signal),
    enabled: !!complexId,
    select: (data) => data.report,
    staleTime: 5 * 60 * 1000,
  });
}
