import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { cashApi } from '../api/cash.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useIdempotentMutation, type WithAttemptKey } from '@/shared/lib/idempotency';
import type { OpenCashSessionRequest } from '@/shared/types/api.types';

const t = ES_AR;

export function useOpenCashSession(complexId: string) {
  const queryClient = useQueryClient();

  return useIdempotentMutation({
    mutationFn: ({ attemptKey, ...data }: WithAttemptKey<OpenCashSessionRequest>) =>
      cashApi.open(complexId, data, attemptKey),
    onSuccess: () => {
      toast.success(t.cash.openSuccess);
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.sessions(complexId) });
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.cash.openError));
    },
  });
}
