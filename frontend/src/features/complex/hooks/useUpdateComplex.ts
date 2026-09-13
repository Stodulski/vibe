import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { complexApi } from '../api/complex.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { isVersionConflict } from '@/shared/lib/ApiError';
import type { Complex, UpdateComplexRequest } from '@/shared/types/api.types';

export function useUpdateComplex(complexId: string) {
  const t = ES_AR;
  const queryClient = useQueryClient();

  return useMutation({
    // Sends the row's own `version` back, read from the cache this form was
    // populated from — the PUT is refused with 409 if the row moved since.
    // `data` never sets `version` itself, so the cached one always wins.
    mutationFn: (data: UpdateComplexRequest) => {
      const cached = queryClient.getQueryData<{ complex: Complex }>(queryKeys.complexes.detail(complexId));
      return complexApi.update(complexId, { version: cached?.complex.version, ...data });
    },
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.all });
      queryClient.setQueryData(queryKeys.complexes.detail(complexId), data);
      toast.success(t.complex.complexUpdated);
    },
    onError: (error: unknown) => {
      // The row moved under this edit — the stale copy in cache is exactly
      // what caused the refusal, so it's invalidated instead of retried:
      // a retry would just send the same wrong version again.
      if (isVersionConflict(error)) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.detail(complexId) });
        toast.error(t.common.versionConflict);
        return;
      }
      toast.error(getHttpErrorMessage(error, t.complex.updateError));
    },
  });
}
