import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { complexApi } from '../api/complex.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import type { UpdateSchedulesRequest } from '@/shared/types/api.types';

export function useUpdateSchedules(complexId: string) {
  const t = ES_AR;
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: UpdateSchedulesRequest) => complexApi.updateSchedules(complexId, data),
    onSuccess: () => {
      // Not `setQueryData`: the query at this key is keyed by `complexId` but
      // actually fetches `getPublicComplex(slug)` (see useSchedules) and
      // `select`s its `.schedules`. The mutation only has `{ schedules }` to
      // write, which is a different shape from the `PublicComplexResponse`
      // the query caches — writing it would corrupt the cache entry for any
      // other reader of the same key (usePriceConfigForm reads it too).
      // Invalidating instead lets the real query refetch and re-derive the
      // correct shape.
      void queryClient.invalidateQueries({
        queryKey: queryKeys.complexes.schedules(complexId),
      });
      toast.success(t.complex.schedulesUpdated);
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.complex.updateSchedulesError));
    },
  });
}
