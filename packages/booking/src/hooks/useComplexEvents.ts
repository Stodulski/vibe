import { useQuery } from '@tanstack/react-query';
import { usePublicBookingApi } from '../api/context';
import { queryKeys } from '../lib/queryKeys';

/**
 * The upcoming open-play events of the complex, for its page. A failure renders
 * nothing: the list is an extra, and the booking flow beside it must not lose
 * its page over it.
 */
export function useComplexEvents(slug: string) {
  const api = usePublicBookingApi();
  return useQuery({
    queryKey: queryKeys.events.bySlug(slug),
    queryFn: ({ signal }) => api.listEvents(slug, signal),
    select: (data) => data.events,
    enabled: !!slug,
    throwOnError: false,
    retry: false,
    staleTime: 60_000,
  });
}
