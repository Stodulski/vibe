import { useQuery } from '@tanstack/react-query';
import { tournamentsApi } from '../api/tournaments.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/** One tournament with its categories, pairs, groups with standings and matches. */
export function useTournament(complexId: string, tournamentId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.tournaments.detail(complexId, tournamentId ?? ''),
    queryFn: ({ signal }) => tournamentsApi.get(complexId, tournamentId ?? '', signal),
    enabled: !!complexId && !!tournamentId,
    // Pairs pay while the owner looks: a spot taken meanwhile shows up on refocus.
    staleTime: 30 * 1000,
  });
}
