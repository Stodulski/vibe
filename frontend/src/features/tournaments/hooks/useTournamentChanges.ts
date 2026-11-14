import { useMutation, useQueryClient } from '@tanstack/react-query';
import { tournamentsApi } from '../api/tournaments.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import type { MatchResultRequest, TournamentDetailResponse } from '@/shared/types/api.types';

/**
 * What every draw and result change answers: the whole tournament, since one
 * change moves other matches. It replaces the cached detail at once and the
 * lists (counts, status) and the grid's match blocks refetch.
 */
function useStoreDetail(complexId: string) {
  const queryClient = useQueryClient();
  return (detail: TournamentDetailResponse) => {
    queryClient.setQueryData(queryKeys.tournaments.detail(complexId, detail.tournament.id), detail);
    void queryClient.invalidateQueries({ queryKey: [...queryKeys.tournaments.base(complexId), 'list'] });
  };
}

export function useGenerateDraw(complexId: string, tournamentId: string, categoryId: string) {
  const store = useStoreDetail(complexId);
  return useMutation({
    mutationFn: (seedOrder: string[]) =>
      tournamentsApi.generateDraw(
        complexId,
        tournamentId,
        categoryId,
        seedOrder.length > 0 ? { seed_order: seedOrder } : {},
      ),
    onSuccess: store,
  });
}

export function useMovePair(complexId: string, tournamentId: string, categoryId: string) {
  const store = useStoreDetail(complexId);
  return useMutation({
    mutationFn: (move: { pairId: string; withPairId: string }) =>
      tournamentsApi.moveDraw(complexId, tournamentId, categoryId, {
        pair_id: move.pairId,
        with_pair_id: move.withPairId,
      }),
    onSuccess: store,
  });
}

export function useSetResult(complexId: string, tournamentId: string) {
  const store = useStoreDetail(complexId);
  return useMutation({
    mutationFn: ({ matchId, result }: { matchId: string; result: MatchResultRequest }) =>
      tournamentsApi.setResult(complexId, tournamentId, matchId, result),
    onSuccess: store,
  });
}
