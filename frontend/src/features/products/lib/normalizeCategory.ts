/**
 * A typed category equal to an existing catalog category, after trimming and
 * ignoring case, is saved with the EXISTING spelling — otherwise "Bebidas",
 * "bebidas" and "Bebida " would become separate categories in Vender
 * (`odd/tasks/product-category-chips.md`). A category with no match in the
 * catalog is returned trimmed, unchanged otherwise.
 */
export function normalizeCategory(value: string, existingCategories: readonly string[]): string {
  const trimmed = value.trim();
  if (!trimmed) return trimmed;
  const match = existingCategories.find((category) => category.toLowerCase() === trimmed.toLowerCase());
  return match ?? trimmed;
}
