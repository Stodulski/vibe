import { useParams, Link } from 'react-router-dom';
import { ArrowLeft, AlertTriangle } from 'lucide-react';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useProduct } from '../hooks/useProduct';
import { ProductDetailHeader } from '../components/ProductDetailHeader';
import { StockMovementList } from '../components/StockMovementList';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';
import { ProductDetailDialogs } from '../components/ProductDetailDialogs';
import { useProductDetailDialogs } from './products/useProductDetailDialogs';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

function ProductDetailLoadError({ onRetry }: { onRetry: () => void }) {
  return (
    <EmptyState
      icon={AlertTriangle}
      title={t.products.productLoadError}
      description={t.common.loadErrorDescription}
      actionLabel={t.layout.retry}
      onAction={onRetry}
    />
  );
}

function ProductDetailBody({
  product,
  complexId,
  isStale,
  onRetry,
  dialogs,
}: {
  product: Product;
  complexId: string;
  isStale: boolean;
  onRetry: () => void;
  dialogs: ReturnType<typeof useProductDetailDialogs>;
}) {
  return (
    <>
      {isStale && <StaleDataNotice onRetry={onRetry} />}
      <ProductDetailHeader
        product={product}
        onEdit={() => {
          dialogs.setEditOpen(true);
        }}
        onRestock={() => {
          dialogs.setRestockOpen(true);
        }}
        onAdjust={() => {
          dialogs.setAdjustOpen(true);
        }}
        onToggleActive={() => {
          dialogs.setToggleOpen(true);
        }}
      />
      <StockMovementList complexId={complexId} productId={product.id} />
      <ProductDetailDialogs product={product} complexId={complexId} dialogs={dialogs} />
    </>
  );
}

export default function ProductDetailPage() {
  usePageTitle(t.products.detailTitle);
  const { productId } = useParams<{ productId: string }>();
  const { selectedComplexId } = useSelectedComplex();
  const query = useProduct(selectedComplexId, productId ?? null);
  const dialogs = useProductDetailDialogs(productId);

  if (!selectedComplexId) return null;

  return (
    <div className="animate-fade-in space-y-4">
      <Link
        to="/cash/products"
        className="text-text-tertiary hover:text-text-secondary flex items-center gap-1.5 text-sm transition-colors"
      >
        <ArrowLeft className="size-4" aria-hidden="true" />
        {t.products.backToProducts}
      </Link>

      <PageHeader title={t.products.detailTitle} />

      {query.isLoading ? (
        <SkeletonTable rows={4} mobile="flat" />
      ) : !query.data ? (
        // Only the full-screen error when there is nothing cached — a
        // background refetch failure with `data` still around keeps
        // rendering it, with a non-blocking notice instead (same "don't wipe
        // the view" lesson as the cash feature's own detail page).
        <ProductDetailLoadError
          onRetry={() => {
            void query.refetch();
          }}
        />
      ) : (
        <ProductDetailBody
          product={query.data.product}
          complexId={selectedComplexId}
          isStale={query.isError}
          onRetry={() => {
            void query.refetch();
          }}
          dialogs={dialogs}
        />
      )}
    </div>
  );
}
