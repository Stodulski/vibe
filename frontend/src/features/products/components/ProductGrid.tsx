import { useMediaQuery } from '@/shared/hooks/useMediaQuery';
import { ProductCard } from './ProductCard';
import { ProductTable } from './ProductTable';
import type { Product } from '@/shared/types/api.types';

interface ProductGridProps {
  products: Product[];
  onSelect: (product: Product) => void;
  onEdit: (product: Product) => void;
  onRestock: (product: Product) => void;
  onAdjust: (product: Product) => void;
  onToggleActive: (product: Product) => void;
}

/** The catalog, as cards below `lg` and as a dense table from `lg` up. Same "render a different component, don't mount both" shape as `ClientGrid`. */
export function ProductGrid({ products, onSelect, onEdit, onRestock, onAdjust, onToggleActive }: ProductGridProps) {
  const isTableLayout = useMediaQuery('(min-width: 1024px)');

  if (isTableLayout) {
    return (
      <ProductTable
        products={products}
        onSelect={onSelect}
        onEdit={onEdit}
        onRestock={onRestock}
        onAdjust={onAdjust}
        onToggleActive={onToggleActive}
      />
    );
  }

  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      {products.map((product) => (
        <ProductCard
          key={product.id}
          product={product}
          onSelect={onSelect}
          onEdit={onEdit}
          onRestock={onRestock}
          onAdjust={onAdjust}
          onToggleActive={onToggleActive}
        />
      ))}
    </div>
  );
}
