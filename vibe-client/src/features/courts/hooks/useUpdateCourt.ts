import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { courtsApi } from '../api/courts.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UpdateCourtRequest, CourtWithPrices } from '@/shared/types/api.types';

export function useUpdateCourt(complexId: string) {
  const queryClient = useQueryClient();
  const queryKey = queryKeys.courts.byComplex(complexId);

  return useMutation({
    mutationFn: ({ courtId, data }: { courtId: string; data: UpdateCourtRequest }) =>
      courtsApi.update(complexId, courtId, data),
    onMutate: async ({ courtId, data }) => {
      await queryClient.cancelQueries({ queryKey });

      const previous = queryClient.getQueryData(queryKey);

      queryClient.setQueryData(queryKey, (old: { courts: CourtWithPrices[] } | undefined) => {
        if (!old) return old;
        return {
          ...old,
          courts: old.courts.map((c) => (c.id === courtId ? { ...c, ...data } : c)),
        };
      });

      return { previous };
    },
    onSuccess: () => {
      toast.success(ES_AR.courts.updateSuccess);
    },
    onError: (error: unknown, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(queryKey, context.previous);
      }
      toast.error(getHttpErrorMessage(error, ES_AR.courts.updateError));
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey });
    },
  });
}
