import { ProductActionsMenu } from './ProductActionsMenu';
import type { Product } from '@/shared/types/api.types';

interface ProductActionsMenuForProps {
  product: Product;
  onEdit: (product: Product) => void;
  onRestock: (product: Product) => void;
  onAdjust: (product: Product) => void;
  onToggleActive: (product: Product) => void;
}

/**
 * Binds `ProductActionsMenu`'s no-arg callbacks to one product — the
 * `() => onX(product)` wrapping `ProductCard`, `ProductTableRow` and
 * `ProductDetailHeader` all repeated identically (one helper, no duplicated
 * logic, per the cash review lesson: "one helper per label rule").
 */
export function ProductActionsMenuFor({
  product,
  onEdit,
  onRestock,
  onAdjust,
  onToggleActive,
}: ProductActionsMenuForProps) {
  return (
    <ProductActionsMenu
      product={product}
      onEdit={() => {
        onEdit(product);
      }}
      onRestock={() => {
        onRestock(product);
      }}
      onAdjust={() => {
        onAdjust(product);
      }}
      onToggleActive={() => {
        onToggleActive(product);
      }}
    />
  );
}
