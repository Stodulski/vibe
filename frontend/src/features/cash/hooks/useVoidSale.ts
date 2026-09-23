import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { salesApi } from '../api/sales.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useIdempotentMutation, type WithAttemptKey } from '@/shared/lib/idempotency';
import type { VoidSaleRequest } from '@/shared/types/api.types';

const t = ES_AR;

/** Voids one sale of the currently open session (the only sales this screen ever lists — see `useSales`). */
export function useVoidSale(complexId: string, sessionId: string) {
  const queryClient = useQueryClient();

  return useIdempotentMutation({
    mutationFn: ({ saleId, attemptKey, ...data }: WithAttemptKey<VoidSaleRequest & { saleId: string }>) =>
      salesApi.void(complexId, saleId, data, attemptKey),
    onSuccess: () => {
      toast.success(t.cash.saleVoidSuccess);
      void queryClient.invalidateQueries({ queryKey: queryKeys.sales.bySession(complexId, sessionId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.byComplexAll(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.detailBase(complexId) });
    },
    onError: (error: unknown) => {
      // 409: no session open, or this sale (or its income movement) was
      // already voided — any of these means the list shown is stale, same
      // "toast + refetch" shape as `useVoidCashMovement`'s own 409 handling.
      void queryClient.invalidateQueries({ queryKey: queryKeys.sales.bySession(complexId, sessionId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
      toast.error(getHttpErrorMessage(error, t.cash.saleVoidError));
    },
  });
}
