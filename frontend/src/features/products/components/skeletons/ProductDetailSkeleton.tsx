import { Panel } from '@/shared/components/common/Panel';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/** Mirrors `ProductDetailHeader`: name and category, the price / stock pair, then the badges line. */
function HeaderSkeleton() {
  return (
    <Panel>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <Skeleton className="h-7 w-48 rounded-lg" />
          <Skeleton className="h-5 w-24 rounded-lg" />
        </div>
        <Skeleton className="size-9 shrink-0 rounded-lg" />
      </div>
      <div className="mt-4 grid grid-cols-2 gap-3">
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

/** Mirrors `StockMovementList`: the section heading and a flat Panel of rows. */
function StockMovementsSkeleton({ rows }: { rows: number }) {
  return (
    <div>
      <Skeleton className="mb-2 h-5 w-40 rounded-lg" />
      <Panel size="sm" className="p-0">
        {Array.from({ length: rows }, (_, i) => (
          <div
            key={i}
            className="border-border-subtle flex items-start justify-between gap-4 border-b py-4 last:border-b-0 sm:px-4"
          >
            <div className="min-w-0 flex-1 space-y-1">
              <Skeleton className="h-5 w-32 rounded-lg" />
              <Skeleton className="h-4 w-24 rounded-lg" />
            </div>
            <Skeleton className="h-5 w-12 shrink-0 rounded-lg" />
          </div>
        ))}
      </Panel>
    </div>
  );
}

/** Loading stand-in for the product detail page. */
export function ProductDetailSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading} className="space-y-4">
      <HeaderSkeleton />
      <StockMovementsSkeleton rows={3} />
    </div>
  );
}
