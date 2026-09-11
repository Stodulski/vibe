import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { complexApi } from '../api/complex.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import type { CreateComplexRequest } from '@/shared/types/api.types';

export function useCreateComplex() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateComplexRequest) => complexApi.create(data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.all });
      toast.success(ES_AR.complex.complexCreated);
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, ES_AR.complex.createError));
    },
  });
}
