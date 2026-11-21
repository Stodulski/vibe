import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { tournamentsApi } from '../api/tournaments.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';

const t = ES_AR.tournaments.pairs;

/** The owner taking one pair out of a category: its spot frees up and what it paid comes back. */
export function useWithdrawPair(complexId: string, tournamentId: string, categoryId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (registrationId: string) =>
      tournamentsApi.withdrawPair(complexId, tournamentId, categoryId, registrationId),
    onSuccess: ({ refund }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tournaments.base(complexId) });
      toast.success(t.withdrawSuccess(refund.status === 'issued' || refund.status === 'queued'));
    },
    onError: (error: unknown) => {
      toast.error(t.withdrawError, { description: getHttpErrorMessage(error, '') || undefined });
    },
  });
}
