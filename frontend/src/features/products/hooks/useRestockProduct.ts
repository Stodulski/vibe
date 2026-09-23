import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { productsApi } from '../api/products.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useIdempotentMutation, type WithAttemptKey } from '@/shared/lib/idempotency';
import type { RestockProductRequest } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * Records a stock delivery. Needs an open cash session — the API answers
 * 409 ("no cash session is open") otherwise, already mapped to Spanish by
 * `serverErrors.ts`, so `getHttpErrorMessage` renders it without special
 * casing here (same for "this product is not active"/"does not track
 * stock", the restock's other two 409 causes).
 */
export function useRestockProduct(complexId: string, productId: string) {
  const queryClient = useQueryClient();

  return useIdempotentMutation({
    mutationFn: ({ attemptKey, ...data }: WithAttemptKey<RestockProductRequest>) =>
      productsApi.restock(complexId, productId, data, attemptKey),
    onSuccess: () => {
      toast.success(t.products.restockSuccess);
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.byComplexAll(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.detail(complexId, productId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.stockMovements(complexId, productId) });
      // The restock's own cash expense changes the open session's expected
      // cash and movement ledger.
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.detailBase(complexId) });
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.products.restockError));
      // A race (the till closed between opening the dialog and submitting)
      // needs the current-session cache to fall out of the open view.
      void queryClient.invalidateQueries({ queryKey: queryKeys.cash.current(complexId) });
    },
  });
}
