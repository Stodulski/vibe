import { Panel } from '@/shared/components/common/Panel';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { PRODUCT_DETAIL_FIGURES_CLASS, PRODUCT_DETAIL_TOP_ROW_CLASS } from './productLayout';

/** Loading shape of `ProductDetailHeader`: name and category, the price / stock pair, then the badges line. */
export function ProductDetailHeaderSkeleton() {
  return (
    <Panel>
      <div className={PRODUCT_DETAIL_TOP_ROW_CLASS}>
        <div className="min-w-0 space-y-1">
          <Skeleton className="h-7 w-48 rounded-lg" />
          <Skeleton className="h-5 w-24 rounded-lg" />
        </div>
        <Skeleton className="size-9 shrink-0 rounded-lg" />
      </div>
      <div className={PRODUCT_DETAIL_FIGURES_CLASS}>
        {[0, 1].map((i) => (
          <div key={i} className="space-y-1">
            <Skeleton className="h-4 w-12 rounded-lg" />
            <Skeleton className="h-7 w-20 rounded-lg" />
          </div>
        ))}
      </div>
      <Skeleton className="mt-3 h-6 w-24 rounded-full" />
    </Panel>
  );
}
