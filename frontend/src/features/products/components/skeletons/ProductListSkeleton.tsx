import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { useMediaQuery } from '@/shared/hooks/useMediaQuery';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

const CARD_COUNT = 6;

/** Mirrors `ProductCard`: name and category, the price / stock pair, then the badges line. */
function ProductCardSkeleton() {
  return (
    <div className="border-border-subtle bg-bg-subtle flex flex-col rounded-2xl border p-4">
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

/** Loading stand-in for the catalog: the dense table from `lg` (as `ProductGrid`), stacked cards below it. */
export function ProductListSkeleton() {
  const isTableLayout = useMediaQuery('(min-width: 1024px)');

  if (isTableLayout) return <SkeletonTable />;

  return (
    <div role="status" aria-label={t.common.loading} className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      {Array.from({ length: CARD_COUNT }, (_, i) => (
        <ProductCardSkeleton key={i} />
      ))}
    </div>
  );
}
