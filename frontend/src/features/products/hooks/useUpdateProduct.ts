import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { productsApi } from '../api/products.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { isVersionConflict } from '@/shared/lib/ApiError';
import { useIdempotentMutation, type WithAttemptKey } from '@/shared/lib/idempotency';
import type { UpdateProductRequest } from '@/shared/types/api.types';

const t = ES_AR;

interface UpdateProductVars {
  productId: string;
  data: UpdateProductRequest;
  /**
   * Set only by `DeactivateProductDialog`, to pick the deactivate/reactivate
   * toast explicitly instead of inferring it from `data.active`'s mere
   * presence — an ordinary content edit that happens to include `active` in
   * its payload (e.g. round-tripping the current product) must still get the
   * generic "producto actualizado" toast.
   */
  toggledActive?: boolean;
}

/**
 * Edits a product. `data.version` (read by the caller from the cached
 * product) guards optimistic concurrency the same way `useUpdateCourt` sends
 * a court's `version` — a stale one is refused with the `stale-version` kind
 * (`isVersionConflict`), distinct from the generic 409 the API answers with
 * for "turn off stock tracking while stock is not zero"
 * (`ErrProductHasStock`, `backend/internal/products/service.go`): that one's
 * English detail is already mapped to Spanish by `serverErrors.ts`, so
 * `getHttpErrorMessage` alone renders it correctly.
 */
export function useUpdateProduct(complexId: string) {
  const queryClient = useQueryClient();

  return useIdempotentMutation({
    mutationFn: ({ attemptKey, productId, data }: WithAttemptKey<UpdateProductVars>) =>
      productsApi.update(complexId, productId, data, attemptKey),
    onSuccess: (_data, variables) => {
      // `DeactivateProductDialog` sets `toggledActive` explicitly — its own
      // Spanish copy instead of the generic "producto actualizado", the same
      // way `useCreateCashMovement` picks Ingreso/Egreso off its variables.
      if (variables.toggledActive !== undefined) {
        toast.success(variables.toggledActive ? t.products.reactivateSuccess : t.products.deactivateSuccess);
        return;
      }
      toast.success(t.products.updateSuccess);
    },
    onError: (error: unknown, variables) => {
      if (isVersionConflict(error)) {
        toast.error(t.products.updateConflict);
        return;
      }
      const fallback =
        variables.toggledActive === undefined
          ? t.products.updateError
          : variables.toggledActive
            ? t.products.reactivateError
            : t.products.deactivateError;
      toast.error(getHttpErrorMessage(error, fallback));
    },
    onSettled: (_data, _error, variables) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.byComplexAll(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.detail(complexId, variables.productId) });
    },
  });
}
