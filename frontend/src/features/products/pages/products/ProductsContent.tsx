import { AlertTriangle, Package } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { ProductGrid } from '../../components/ProductGrid';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

interface ProductsContentProps {
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  products: Product[];
  hasAnyProducts: boolean;
  onCreate: () => void;
  onSelect: (product: Product) => void;
  onEdit: (product: Product) => void;
  onRestock: (product: Product) => void;
  onAdjust: (product: Product) => void;
  onToggleActive: (product: Product) => void;
}

export function ProductsContent({
  isLoading,
  isError,
  onRetry,
  products,
  hasAnyProducts,
  onCreate,
  onSelect,
  onEdit,
  onRestock,
  onAdjust,
  onToggleActive,
}: ProductsContentProps) {
  if (isLoading) return <SkeletonTable />;

  if (isError) {
    return (
      <EmptyState
        icon={AlertTriangle}
        title={t.products.loadError}
        description={t.common.loadErrorDescription}
        actionLabel={t.layout.retry}
        onAction={onRetry}
      />
    );
  }

  if (products.length === 0) {
    return hasAnyProducts ? (
      <EmptyState icon={Package} title={t.products.noSearchResults} description="" />
    ) : (
      <EmptyState
        icon={Package}
        title={t.products.emptyTitle}
        description={t.products.emptyDescription}
        actionLabel={t.products.newProduct}
        actionVariant="outline"
        onAction={onCreate}
      />
    );
  }

  return (
    <ProductGrid
      products={products}
      onSelect={onSelect}
      onEdit={onEdit}
      onRestock={onRestock}
      onAdjust={onAdjust}
      onToggleActive={onToggleActive}
    />
  );
}
