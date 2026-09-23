import api, { withSignal } from '@/shared/lib/ky';
import type { ProductsListResponse } from '@/shared/types/api.types';
import { productsListResponseSchema } from '@/shared/schemas';
import { parseWith } from '@/shared/lib/apiParse';

/**
 * The complex's active-or-inactive product catalog — unpaginated
 * (`productsList`'s own doc comment: a shop's catalog is bounded).
 *
 * Lives in `shared/` (not `features/products/`) because `features/cash`'s
 * "Vender" screen (pos-cashbox T5b) needs the same active-only list and
 * features never import from one another — same move as
 * `shared/api/cashSession.api.ts`'s `getCurrentCashSession`.
 * `features/products`'s own `productsApi.list` delegates here so there
 * stays one call site.
 */
export function listProducts(
  complexId: string,
  params?: { active?: boolean | undefined },
  signal?: AbortSignal,
): Promise<ProductsListResponse> {
  const searchParams: Record<string, string> = {};
  if (params?.active !== undefined) searchParams.active = String(params.active);
  return api
    .get(`complexes/${complexId}/products`, { searchParams, ...withSignal(signal) })
    .json()
    .then(parseWith(productsListResponseSchema, 'productsApi.list'));
}
