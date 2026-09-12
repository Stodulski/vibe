import { noop, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/shared/lib/queryKeys';
import { dashboardApi } from '@/features/dashboard/api/dashboard.api';
import { courtsApi } from '@/features/courts/api/courts.api';
import { bookingsApi } from '@/features/bookings/api/bookings.api';
import { format } from 'date-fns/format';

/**
 * Prefetch data for a route on hover to eliminate loading states
 * when the user navigates. Only prefetches if data is stale.
 *
 * Errors are swallowed with noop: a hover that fails to warm the cache must
 * stay invisible, and the real useQuery on the destination will fetch again
 * and own the failure. query() rejects where the old prefetchQuery absorbed,
 * so the catch is what keeps a hover from raising an unhandled rejection.
 */
export function usePrefetch(complexId: string | null) {
  const queryClient = useQueryClient();

  return (route: string) => {
    if (!complexId) return;

    switch (route) {
      case '/dashboard':
        queryClient
          .query({
            queryKey: queryKeys.dashboard.stats(complexId),
            queryFn: ({ signal }) => dashboardApi.getStats(complexId, signal),
            staleTime: 5 * 60 * 1000,
          })
          .catch(noop);
        break;

      case '/bookings': {
        const today = format(new Date(), 'yyyy-MM-dd');
        queryClient
          .query({
            queryKey: queryKeys.bookings.byDate(complexId, today),
            queryFn: ({ signal }) => bookingsApi.list(complexId, today, undefined, undefined, signal),
            staleTime: 5 * 60 * 1000,
          })
          .catch(noop);
        break;
      }

      case '/courts':
        queryClient
          .query({
            queryKey: queryKeys.courts.byComplex(complexId),
            queryFn: ({ signal }) => courtsApi.list(complexId, signal),
            staleTime: 5 * 60 * 1000,
          })
          .catch(noop);
        break;
    }
  };
}
