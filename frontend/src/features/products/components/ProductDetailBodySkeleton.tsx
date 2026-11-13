import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ProductDetailHeaderSkeleton } from './ProductDetailHeaderSkeleton';
import { StockMovementListSkeleton } from './StockMovementListSkeleton';
import { PRODUCT_DETAIL_BODY_CLASS } from './productLayout';

const t = ES_AR;

/** Loading stand-in for the product detail body: the header card, then the stock history under its heading. */
export function ProductDetailBodySkeleton() {
  return (
    <div role="status" aria-label={t.common.loading} className={PRODUCT_DETAIL_BODY_CLASS}>
      <ProductDetailHeaderSkeleton />
      <div>
        <Skeleton className="mb-2 h-5 w-40 rounded-lg" />
        <StockMovementListSkeleton />
      </div>
    </div>
  );
}
