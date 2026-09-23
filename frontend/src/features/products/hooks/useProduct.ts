import { useQuery } from '@tanstack/react-query';
import { productsApi } from '../api/products.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/** One product's detail — `/cash/products/:productId`'s own header. */
export function useProduct(complexId: string | null, productId: string | null) {
  const id = complexId ?? '';
  const pid = productId ?? '';
  return useQuery({
    queryKey: queryKeys.products.detail(id, pid),
    queryFn: ({ signal }) => productsApi.getById(id, pid, signal),
    enabled: !!complexId && !!productId,
    staleTime: 30 * 1000,
  });
}
