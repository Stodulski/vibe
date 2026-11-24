import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { complexApi } from '../api/complex.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { isVersionConflict } from '@/shared/lib/ApiError';
import type { UpdateComplexRequest } from '@/shared/types/api.types';

export function useUpdateComplex(complexId: string) {
  const t = ES_AR;
  const queryClient = useQueryClient();

  return useMutation({
    // The caller sends the `version` of the copy its form was filled from
    // (`useComplexForm`): only it knows which copy is on screen. Settings and
    // onboarding fill it from the list, which the detail entry never mirrored,
    // so reading the version from the detail cache sent none at all.
    mutationFn: (data: UpdateComplexRequest) => complexApi.update(complexId, data),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.all });
      queryClient.setQueryData(queryKeys.complexes.detail(complexId), data);
      toast.success(t.complex.complexUpdated);
    },
    onError: (error: unknown) => {
      // The row moved under this edit — the stale copy in cache is exactly
      // what caused the refusal, so it's refetched instead of retried: a
      // retry would just send the same wrong version again. The list is the
      // copy the forms read; the detail is refreshed too for its readers.
      if (isVersionConflict(error)) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.all, exact: true });
        void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.detail(complexId), exact: true });
        toast.error(t.common.versionConflict);
        return;
      }
      toast.error(getHttpErrorMessage(error, t.complex.updateError));
    },
  });
}
