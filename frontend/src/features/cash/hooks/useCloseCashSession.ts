import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { cashApi } from '../api/cash.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useIdempotentMutation, type WithAttemptKey } from '@/shared/lib/idempotency';
import type { CloseCashSessionRequest } from '@/shared/types/api.types';

const t = ES_AR;

export function useCloseCashSession(complexId: string, sessionId: string) {
  const queryClient = useQueryClient();

  return useIdempotentMutation({
    mutationFn: ({ attemptKey, ...data }: WithAttemptKey<CloseCashSessionRequest>) =>
      cashApi.close(complexId, sessionId, data, attemptKey),
    onSuccess: () => {
      toast.success(t.cash.closeSuccess);
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.sessions(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.detail(complexId, sessionId) });
    },
    onError: (error: unknown) => {
      // 409 ("not open" — already closed, or closed by a concurrent request):
      // the parent refetches `current` regardless (via onSettled-less
      // invalidate here) so the UI drops out of the close dialog into
      // whatever the server now says is true, instead of re-showing a stale
      // "open" screen the next click would just 409 again.
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
      toast.error(getHttpErrorMessage(error, t.cash.closeError));
    },
  });
}
