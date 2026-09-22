import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { cashApi } from '../api/cash.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useIdempotentMutation, type WithAttemptKey } from '@/shared/lib/idempotency';
import type { VoidCashMovementRequest } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * Voids one movement of `sessionId` (always the currently open session in
 * this UI — see `MovementList`: voiding a movement from an already-closed
 * prior shift is a real API capability the CashPage screen does not expose).
 */
export function useVoidCashMovement(complexId: string, sessionId: string) {
  const queryClient = useQueryClient();

  return useIdempotentMutation({
    mutationFn: ({
      movementId,
      attemptKey,
      ...data
    }: WithAttemptKey<VoidCashMovementRequest & { movementId: string }>) =>
      cashApi.voidMovement(complexId, sessionId, movementId, data, attemptKey),
    onSuccess: () => {
      toast.success(t.cash.voidSuccess);
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.detail(complexId, sessionId) });
    },
    onError: (error: unknown) => {
      // 409: no session open, the original is already voided, or the
      // original is itself a void — any of these means the list shown is
      // stale, so refetch alongside the toast. Also invalidate `current`
      // (same as the create-movement and close hooks' 409 handling): "no
      // session open" is one of the three causes, and without this the page
      // would keep showing the now-stale open-session view instead of
      // falling out of it.
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.detail(complexId, sessionId) });
      toast.error(getHttpErrorMessage(error, t.cash.voidError));
    },
  });
}
