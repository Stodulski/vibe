import { Skeleton } from '@/shared/components/ui/skeleton';
import { PRODUCT_CARD_CLASS } from './productLayout';

/** Loading shape of `ProductCard`: name and category, the price / stock pair, then the badges line. */
export function ProductCardSkeleton() {
  return (
    <div className={PRODUCT_CARD_CLASS} aria-hidden="true">
      <div className="space-y-1 pr-10">
        <Skeleton className="h-5 w-40 rounded-lg" />
        <Skeleton className="h-4 w-24 rounded-lg" />
      </div>
      <div className="mt-3 flex items-end justify-between gap-2">
        <div className="space-y-1">
          <Skeleton className="h-4 w-10 rounded-lg" />
          <Skeleton className="h-7 w-20 rounded-lg" />
        </div>
        <div className="flex flex-col items-end space-y-1">
          <Skeleton className="h-4 w-10 rounded-lg" />
          <Skeleton className="h-5 w-12 rounded-lg" />
        </div>
      </div>
      <Skeleton className="mt-3 h-6 w-24 rounded-full" />
    </div>
  );
}
