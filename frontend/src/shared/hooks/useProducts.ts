import { useQuery } from '@tanstack/react-query';
import { listProducts } from '@/shared/api/products.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/**
 * The complex's product catalog for one Activos/Inactivos filter value.
 * Unpaginated — `productsList`'s own doc comment: a shop's catalog is
 * bounded. `active` undefined asks for both.
 *
 * Moved here from `features/products/hooks/` (pos-cashbox T5b):
 * `features/cash`'s "Vender" screen needs the same active-only list and
 * features never import from one another — same move as
 * `shared/hooks/useCashSession.ts`. `features/products`'s own `useProducts`
 * re-exports this one so there stays a single hook.
 */
export function useProducts(complexId: string | null, active?: boolean) {
  const id = complexId ?? '';
  return useQuery({
    queryKey: queryKeys.products.byComplex(id, active),
    queryFn: ({ signal }) => listProducts(id, { active }, signal),
    enabled: !!complexId,
    staleTime: 60 * 1000,
  });
}
