import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { courtsApi } from '../api/courts.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { getFieldErrors } from '@/shared/lib/serverErrors';
import { isVersionConflict } from '@/shared/lib/ApiError';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CourtWithPrices, UpdatePricesRequest } from '@/shared/types/api.types';

export function useUpdatePrices(complexId: string) {
  const t = ES_AR;
  const queryClient = useQueryClient();
  const queryKey = queryKeys.courts.byComplex(complexId);

  return useMutation({
    // The price table is replaced wholesale on every save, so it carries no
    // `version` of its own — the PUT is guarded by the COURT's version
    // instead, read from the same cache the court list renders from.
    mutationFn: ({ courtId, data }: { courtId: string; data: UpdatePricesRequest }) => {
      const cached = queryClient.getQueryData<{ courts: CourtWithPrices[] }>(queryKey);
      const version = cached?.courts.find((c) => c.id === courtId)?.version;
      return courtsApi.updatePrices(complexId, courtId, { version, ...data });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey });
      toast.success(t.courts.pricesUpdated);
    },
    onError: (error: unknown) => {
      // The court moved under this edit — the stale row in cache is exactly
      // what caused the refusal, so it's invalidated instead of retried.
      if (isVersionConflict(error)) {
        void queryClient.invalidateQueries({ queryKey });
        toast.error(t.common.versionConflict);
        return;
      }
      // A 422 with per-field errors is handled by usePriceConfigForm, which
      // maps each one onto its row; toasting the same failure here as well
      // would show it twice. Anything else (network failure, 500, a
      // whole-array error with no field to land on) still gets the toast.
      if (Object.keys(getFieldErrors(error)).length > 0) return;
      toast.error(getHttpErrorMessage(error, t.courts.pricesUpdateError));
    },
  });
}
