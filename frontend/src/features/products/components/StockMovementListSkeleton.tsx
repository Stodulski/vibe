import { Panel } from '@/shared/components/common/Panel';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { STOCK_MOVEMENT_ROW_CLASS } from './productLayout';

/** Loading shape of the rows `StockMovementList` shows under its heading: a flat Panel of `StockMovementRow`s. */
export function StockMovementListSkeleton({ rows = 3 }: { rows?: number }) {
  return (
    <Panel size="sm" className="p-0" aria-hidden="true">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className={STOCK_MOVEMENT_ROW_CLASS}>
          <div className="min-w-0 space-y-1">
            <Skeleton className="h-5 w-32 rounded-lg" />
            <Skeleton className="h-4 w-24 rounded-lg" />
          </div>
          <Skeleton className="h-5 w-12 shrink-0 rounded-lg" />
        </div>
      ))}
    </Panel>
  );
}
