import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { complexApi } from '../api/complex.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import type { UpdateComplexRequest } from '@/shared/types/api.types';

export function useUpdateComplex(complexId: string) {
  const t = ES_AR;
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: UpdateComplexRequest) => complexApi.update(complexId, data),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.all });
      queryClient.setQueryData(queryKeys.complexes.detail(complexId), data);
      toast.success(t.complex.complexUpdated);
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.complex.updateError));
    },
  });
}
