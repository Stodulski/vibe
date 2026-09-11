import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { courtsApi } from '../api/courts.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CreateCourtRequest } from '@/shared/types/api.types';

export function useCreateCourt(complexId: string) {
  const t = ES_AR;
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateCourtRequest) => courtsApi.create(complexId, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.courts.byComplex(complexId),
      });
      toast.success(t.courts.createdSuccess);
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.courts.createError));
    },
  });
}
