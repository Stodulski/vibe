import { useMediaQuery } from '@/shared/hooks/useMediaQuery';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ProductCardSkeleton } from './ProductCardSkeleton';
import { ProductTableSkeleton } from './ProductTableSkeleton';
import { PRODUCT_GRID_CLASS } from './productLayout';

const t = ES_AR;

const CARD_COUNT = 6;

/** Loading shape of `ProductGrid`: the dense table from `lg` (same switch), stacked cards below it. */
export function ProductGridSkeleton() {
  const isTableLayout = useMediaQuery('(min-width: 1024px)');

  return (
    <div role="status" aria-label={t.common.loading} className={isTableLayout ? undefined : PRODUCT_GRID_CLASS}>
      {isTableLayout ? (
        <ProductTableSkeleton />
      ) : (
        Array.from({ length: CARD_COUNT }, (_, i) => <ProductCardSkeleton key={i} />)
      )}
    </div>
  );
}
