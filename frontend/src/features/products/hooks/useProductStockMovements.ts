import { useInfiniteQuery } from '@tanstack/react-query';
import { productsApi } from '../api/products.api';
import { queryKeys } from '@/shared/lib/queryKeys';

const PAGE_SIZE = 20;

/** A product's stock-movement history — newest first, paginated. */
export function useProductStockMovements(complexId: string | null, productId: string | null) {
  const id = complexId ?? '';
  const pid = productId ?? '';
  return useInfiniteQuery({
    queryKey: queryKeys.products.stockMovements(id, pid),
    queryFn: ({ pageParam, signal }) =>
      productsApi.listStockMovements(id, pid, { cursor: pageParam || undefined, limit: PAGE_SIZE }, signal),
    initialPageParam: '',
    getNextPageParam: (lastPage) => (lastPage.metadata.has_more ? lastPage.metadata.next_cursor : undefined),
    enabled: !!complexId && !!productId,
    staleTime: 30 * 1000,
  });
}
