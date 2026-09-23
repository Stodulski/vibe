import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { salesApi } from '../api/sales.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useIdempotentMutation, type WithAttemptKey } from '@/shared/lib/idempotency';
import type { CreateSaleRequest } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * Sells a cart. `sessionId` is the currently open session (or `null` while it
 * hasn't loaded yet) — used only to target the right `sales.bySession`
 * invalidation; the sale itself always lands in whichever session is open on
 * the server at the moment the request is processed.
 *
 * No `onSuccess` toast/clear-cart here — the caller (`useSellPage`) needs the
 * server response itself (`total`, `stock_warnings`) to drive the
 * confirmation dialog, so that side effect lives at the call site's own
 * `onSuccess`, the same split `VoidMovementDialog` makes with `onClose`. The
 * generic 409/422 toast below still fires from the hook, same as every other
 * mutation in this feature.
 */
export function useCreateSale(complexId: string, sessionId: string | null) {
  const queryClient = useQueryClient();

  return useIdempotentMutation({
    mutationFn: ({ attemptKey, ...data }: WithAttemptKey<CreateSaleRequest>) =>
      salesApi.create(complexId, data, attemptKey),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.byComplexAll(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.detailBase(complexId) });
      if (sessionId) void queryClient.invalidateQueries({ queryKey: queryKeys.sales.bySession(complexId, sessionId) });
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.cash.cartChargeError));
      // A race (the till closed between loading the screen and charging)
      // needs the current-session cache to fall out of the open view — same
      // reasoning as `useRestockProduct`'s own 409 handling.
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
    },
  });
}
