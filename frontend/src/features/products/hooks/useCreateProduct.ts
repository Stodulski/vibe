import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { productsApi } from '../api/products.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useIdempotentMutation, type WithAttemptKey } from '@/shared/lib/idempotency';
import type { CreateProductRequest } from '@/shared/types/api.types';

const t = ES_AR;

export function useCreateProduct(complexId: string) {
  const queryClient = useQueryClient();

  return useIdempotentMutation({
    mutationFn: ({ attemptKey, ...data }: WithAttemptKey<CreateProductRequest>) =>
      productsApi.create(complexId, data, attemptKey),
    onSuccess: () => {
      toast.success(t.products.createSuccess);
      void queryClient.invalidateQueries({ queryKey: queryKeys.products.byComplexAll(complexId) });
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, t.products.createError));
    },
  });
}
