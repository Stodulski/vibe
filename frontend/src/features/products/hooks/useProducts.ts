import { useQuery } from '@tanstack/react-query';
import { productsApi } from '../api/products.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/**
 * The complex's product catalog for one Activos/Inactivos filter value.
 * Unpaginated — `productsList`'s own doc comment: a shop's catalog is
 * bounded. `active` undefined asks for both.
 */
export function useProducts(complexId: string | null, active?: boolean) {
  const id = complexId ?? '';
  return useQuery({
    queryKey: queryKeys.products.byComplex(id, active),
    queryFn: ({ signal }) => productsApi.list(id, { active }, signal),
    enabled: !!complexId,
    staleTime: 60 * 1000,
  });
}
