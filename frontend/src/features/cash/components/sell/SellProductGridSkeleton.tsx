import { Skeleton } from '@/shared/components/ui/skeleton';
import { SELL_TILE_GRID_CLASS } from './sellLayout';

const TILE_COUNT = 8;

/** Loading shape of `SellProductGrid`: search box, category chips, then list rows below `sm` and a 3 / 4 column tile grid above. */
export function SellProductGridSkeleton() {
  return (
    <div className="min-w-0 space-y-3">
      <Skeleton className="h-11 w-full rounded-xl" />
      <div className="flex gap-2 overflow-hidden">
        {[0, 1, 2, 3, 4].map((i) => (
          <Skeleton key={i} className="h-9 w-24 shrink-0 rounded-full" />
        ))}
      </div>
      <div className={`${SELL_TILE_GRID_CLASS} pt-3`}>
        {Array.from({ length: TILE_COUNT }, (_, i) => (
          <Skeleton key={i} className="h-[58px] rounded-2xl sm:h-24" />
        ))}
      </div>
    </div>
  );
}
