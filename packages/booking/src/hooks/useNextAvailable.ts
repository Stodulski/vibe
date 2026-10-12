import { useQuery } from '@tanstack/react-query';
import { usePublicBookingApi } from '../api/context';
import { queryKeys } from '../lib/queryKeys';
import type { DurationMinutes } from '../api/types';

/**
 * The first day after `from` with a free slot, as `YYYY-MM-DD`, or `null` when
 * the window has none. Asked only while a day is fully booked, so the query
 * exists for that one state.
 */
export function useNextAvailable(slug: string, from: string, duration: DurationMinutes | null) {
  const api = usePublicBookingApi();
  return useQuery({
    queryKey: queryKeys.nextAvailable.bySlugFromAndDuration(slug, from, duration),
    queryFn: ({ signal }) => api.getNextAvailable(slug, from, duration, signal),
    select: (data) => data.date,
    enabled: !!slug && !!from,
    throwOnError: false,
    staleTime: 30 * 1000,
  });
}
