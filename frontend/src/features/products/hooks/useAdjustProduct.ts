import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { productsApi } from '../api/products.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useIdempotentMutation, type WithAttemptKey } from '@/shared/lib/idempotency';
import type { AdjustProductRequest } from '@/shared/types/api.types';

const t = ES_AR;

/** Corrects a product's stock by hand. No money, no cash session involved. */
export function useAdjustProduct(complexId: string, productId: string) {
  const queryClient = useQueryClient();

  return useIdempotentMutation({
    mutationFn: ({ attemptKey, ...data }: WithAttemptKey<AdjustProductRequest>) =>
      productsApi.adjust(complexId, productId, data, attemptKey),
    onSuccess: () => {
      toast.success(t.products.adjustSuccess);
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.byComplexAll(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.detail(complexId, productId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.stockMovements(complexId, productId) });
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.products.adjustError));
    },
  });
}
