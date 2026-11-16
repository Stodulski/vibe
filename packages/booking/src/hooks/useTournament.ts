import { useQuery } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { usePublicBookingApi } from '../api/context';
import { queryKeys } from '../lib/queryKeys';

/** One tournament for its public page. A 404 is final; anything else gets a retry. */
export function useTournament(slug: string, tournamentId: string) {
  const api = usePublicBookingApi();
  return useQuery({
    queryKey: queryKeys.tournaments.byId(slug, tournamentId),
    queryFn: ({ signal }) => api.getTournament(slug, tournamentId, signal),
    throwOnError: false,
    staleTime: 30_000,
    retry: (failureCount, error) => {
      if (error instanceof HTTPError && error.response.status === 404) return false;
      return failureCount < 2;
    },
  });
}
