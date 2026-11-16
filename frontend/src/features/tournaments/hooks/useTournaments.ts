import { useQuery } from '@tanstack/react-query';
import { tournamentsApi } from '../api/tournaments.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/** The complex's tournaments whose days overlap the range, cancelled ones included, soonest first. */
export function useTournaments(complexId: string, dateFrom: string, dateTo: string) {
  return useQuery({
    queryKey: queryKeys.tournaments.list(complexId, dateFrom, dateTo),
    queryFn: ({ signal }) => tournamentsApi.list(complexId, dateFrom, dateTo, signal),
    enabled: !!complexId,
    select: (data) => data.tournaments,
  });
}
