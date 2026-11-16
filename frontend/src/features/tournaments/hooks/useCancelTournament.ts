import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { tournamentsApi } from '../api/tournaments.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import type { CancelTournamentCategoryResponse } from '@/shared/types/api.types';

const t = ES_AR.tournaments.cancel;

function owed({ refunds }: CancelTournamentCategoryResponse): number {
  return refunds.filter(({ refund }) => refund.status === 'issued' || refund.status === 'queued').length;
}

function useRefreshAfterCancel(complexId: string) {
  const queryClient = useQueryClient();
  return () => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.tournaments.base(complexId) });
    // The matches' courts are free again, so the grid's match blocks go away.
    void queryClient.invalidateQueries({ queryKey: queryKeys.courts.blockedSlotsBase(complexId) });
  };
}

export function useCancelTournament(complexId: string) {
  const refresh = useRefreshAfterCancel(complexId);
  return useMutation({
    mutationFn: (tournamentId: string) => tournamentsApi.cancel(complexId, tournamentId),
    onSuccess: (response) => {
      refresh();
      toast.success(t.success(owed(response)));
    },
    onError: (error: unknown) => {
      toast.error(t.error, { description: getHttpErrorMessage(error, '') || undefined });
    },
  });
}

export function useCancelCategory(complexId: string, tournamentId: string) {
  const refresh = useRefreshAfterCancel(complexId);
  return useMutation({
    mutationFn: (categoryId: string) => tournamentsApi.cancelCategory(complexId, tournamentId, categoryId),
    onSuccess: (response) => {
      refresh();
      toast.success(t.categorySuccess(owed(response)));
    },
    onError: (error: unknown) => {
      toast.error(t.categoryError, { description: getHttpErrorMessage(error, '') || undefined });
    },
  });
}
