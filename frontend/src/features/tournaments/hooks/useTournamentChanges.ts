import { useMutation, useQueryClient } from '@tanstack/react-query';
import { tournamentsApi } from '../api/tournaments.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import type { ChargeCourtFeeRequest, MatchResultRequest, TournamentDetailResponse } from '@/shared/types/api.types';

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
    mutationFn: () => tournamentsApi.generateDraw(complexId, tournamentId, categoryId, {}),
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

export function useSetCourtFee(complexId: string, tournamentId: string, categoryId: string) {
  const store = useStoreDetail(complexId);
  return useMutation({
    mutationFn: (courtFee: number) =>
      tournamentsApi.setCourtFee(complexId, tournamentId, categoryId, { court_fee: courtFee }),
    onSuccess: store,
  });
}

/** A court fee charge or its undo is money in the open cash session, so the cash screens refetch too. */
function useStoreFeeChange(complexId: string) {
  const store = useStoreDetail(complexId);
  const queryClient = useQueryClient();
  return (detail: TournamentDetailResponse) => {
    store(detail);
    void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
    void queryClient.invalidateQueries({ queryKey: queryKeys.cash.detailBase(complexId) });
  };
}

export interface CourtFeeTarget {
  matchId: string;
  side: 1 | 2;
}

export function useChargeCourtFee(complexId: string, tournamentId: string) {
  const store = useStoreFeeChange(complexId);
  return useMutation({
    mutationFn: ({ matchId, side, ...data }: CourtFeeTarget & ChargeCourtFeeRequest) =>
      tournamentsApi.chargeCourtFee(complexId, tournamentId, matchId, side, data),
    onSuccess: store,
  });
}

export function useUndoCourtFee(complexId: string, tournamentId: string) {
  const store = useStoreFeeChange(complexId);
  return useMutation({
    mutationFn: ({ matchId, side }: CourtFeeTarget) =>
      tournamentsApi.undoCourtFee(complexId, tournamentId, matchId, side),
    onSuccess: store,
  });
}
