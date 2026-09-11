import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { clientsApi } from '../api/clients.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import type { UpdateClientRequest } from '@/shared/types/api.types';

const t = ES_AR;

export function useUpdateClient(complexId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ clientId, data }: { clientId: string; data: UpdateClientRequest }) =>
      clientsApi.update(complexId, clientId, data),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.clients.byComplex(complexId),
      });
      void queryClient.invalidateQueries({
        queryKey: queryKeys.clients.detail(complexId, variables.clientId),
      });
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.clients.updateError));
    },
  });
}
