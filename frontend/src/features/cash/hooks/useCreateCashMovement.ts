import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { cashApi } from '../api/cash.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useIdempotentMutation, type WithAttemptKey } from '@/shared/lib/idempotency';
import type { CreateCashMovementRequest } from '@/shared/types/api.types';

const t = ES_AR;

export function useCreateCashMovement(complexId: string, sessionId: string) {
  const queryClient = useQueryClient();

  return useIdempotentMutation({
    mutationFn: ({ attemptKey, ...data }: WithAttemptKey<CreateCashMovementRequest>) =>
      cashApi.createMovement(complexId, sessionId, data, attemptKey),
    onSuccess: (_data, variables) => {
      toast.success(variables.kind === 'income' ? t.cash.incomeSuccess : t.cash.expenseSuccess);
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.detail(complexId, sessionId) });
    },
    onError: (error: unknown) => {
      // 409 ("session is not open" — closed by a concurrent request while the
      // dialog was open): refetch so the page falls out of the open-session
      // view instead of letting another submit hit the same 409.
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
      toast.error(getHttpErrorMessage(error, t.cash.movementError));
    },
  });
}
