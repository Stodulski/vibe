import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * The chip options `ProductCategoryField` offers: the catalog's own
 * categories first, then the starter categories (`ES_AR.products.starterCategories`)
 * that are not already present, deduplicated case-insensitively — a
 * brand-new catalog still has something to tap
 * (`odd/tasks/product-category-chips.md` T1).
 */
export function buildCategoryChipOptions(existingCategories: readonly string[]): string[] {
  const seen = new Set(existingCategories.map((category) => category.toLowerCase()));
  const starters = t.products.starterCategories.filter((category) => !seen.has(category.toLowerCase()));
  return [...existingCategories, ...starters];
}
