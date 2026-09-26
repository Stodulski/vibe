import type { Product } from '@/shared/types/api.types';

/**
 * Every distinct category across a product list, deduplicated and
 * alphabetically sorted — the catalog's own suggestion set for
 * `ProductCategoryField`. Extracted from `useProductsPage` so
 * `ProductDetailDialogs` can build the same set from the whole catalog
 * instead of just the product being edited (`odd/tasks/product-category-chips.md` T2).
 */
export function buildCategorySet(products: readonly Product[]): string[] {
  const set = new Set<string>();
  for (const product of products) if (product.category) set.add(product.category);
  return [...set].sort((a, b) => a.localeCompare(b));
}
