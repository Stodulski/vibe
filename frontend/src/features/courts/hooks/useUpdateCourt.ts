import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { courtsApi } from '../api/courts.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { isVersionConflict } from '@/shared/lib/ApiError';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UpdateCourtRequest, CourtWithPrices } from '@/shared/types/api.types';

export function useUpdateCourt(complexId: string) {
  const queryClient = useQueryClient();
  const queryKey = queryKeys.courts.byComplex(complexId);

  return useMutation({
    // Sends the court's own `version` back, read from the same cache the
    // optimistic update below reads from — the PUT is refused with 409 if
    // the row moved since. `data` never sets `version` itself, so the
    // cached one always wins.
    mutationFn: ({ courtId, data }: { courtId: string; data: UpdateCourtRequest }) => {
      const cached = queryClient.getQueryData<{ courts: CourtWithPrices[] }>(queryKey);
      const version = cached?.courts.find((c) => c.id === courtId)?.version;
      return courtsApi.update(complexId, courtId, { version, ...data });
    },
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
      // `onSettled` below already invalidates this query on every outcome,
      // fetching the row the other edit left behind — only the copy differs
      // here, so the person knows to look again rather than just retry.
      toast.error(
        isVersionConflict(error) ? ES_AR.common.versionConflict : getHttpErrorMessage(error, ES_AR.courts.updateError),
      );
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey });
    },
  });
}
