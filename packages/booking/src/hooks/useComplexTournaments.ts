import { useQuery } from '@tanstack/react-query';
import { usePublicBookingApi } from '../api/context';
import { queryKeys } from '../lib/queryKeys';

/**
 * The upcoming tournaments of the complex, for its page. A failure renders
 * nothing: the list is an extra, and the booking flow beside it must not lose
 * its page over it.
 */
export function useComplexTournaments(slug: string) {
  const api = usePublicBookingApi();
  return useQuery({
    queryKey: queryKeys.tournaments.bySlug(slug),
    queryFn: ({ signal }) => api.listTournaments(slug, signal),
    select: (data) => data.tournaments,
    enabled: !!slug,
    throwOnError: false,
    retry: false,
    staleTime: 60_000,
  });
}
