import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { HTTPError } from 'ky';
import { courtsApi } from '../api/courts.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { getFieldErrors } from '@/shared/lib/serverErrors';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UpdatePricesRequest } from '@/shared/types/api.types';

function errorBody(error: unknown): unknown {
  return error instanceof HTTPError ? error.data : null;
}

export function useUpdatePrices(complexId: string) {
  const t = ES_AR;
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ courtId, data }: { courtId: string; data: UpdatePricesRequest }) =>
      courtsApi.updatePrices(complexId, courtId, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.courts.byComplex(complexId),
      });
      toast.success(t.courts.pricesUpdated);
    },
    onError: (error: unknown) => {
      // A 422 with per-field errors is handled by usePriceConfigForm, which
      // maps each one onto its row; toasting the same failure here as well
      // would show it twice. Anything else (network failure, 500, a
      // whole-array error with no field to land on) still gets the toast.
      if (Object.keys(getFieldErrors(errorBody(error))).length > 0) return;
      toast.error(getHttpErrorMessage(error, t.courts.pricesUpdateError));
    },
  });
}
