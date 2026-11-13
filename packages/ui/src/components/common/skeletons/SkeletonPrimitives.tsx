import { Panel } from '../Panel';
import { Skeleton } from '../../ui/skeleton';
import { messages } from '../../../messages';

interface SkeletonTableProps {
  rows?: number;
  /** Set to false when nested inside a parent that already owns the loading announcement. */
  live?: boolean;
  /**
   * `'card'` (the default) matches call sites whose loaded rows are tappable
   * tiles boxed at every width (`ProductCard`, `ClientCard`: `ProductsContent`,
   * `ClientsContent`). `'flat'` matches call sites whose loaded state is a
   * `Panel` section — flat below `sm`, boxed from `sm:` up — such as
   * `StockMovementList`, `MovementList`, `SalesSection` and
   * `CashSessionHistoryList` (odd/tasks/app-dark-contrast.md T2 follow-up:
   * these used to show a card while loading and go flat once the real
   * `Panel` mounted).
   */
  mobile?: 'flat' | 'card';
}

const SKELETON_TABLE_ROW = {
  flat: 'border-border-subtle flex gap-4 border-b py-3 last:border-b-0 sm:px-4',
  card: 'border-border-subtle flex gap-4 border-b p-4 last:border-b-0',
};

export function SkeletonTable({ rows = 5, live = true, mobile = 'card' }: SkeletonTableProps) {
  const rowClassName = SKELETON_TABLE_ROW[mobile];
  return (
    <Panel
      size="sm"
      mobile={mobile}
      className="p-0"
      role={live ? 'status' : undefined}
      aria-label={live ? messages.loading : undefined}
      aria-hidden={live ? undefined : true}
    >
      <div className={rowClassName}>
        <Skeleton className="h-4 w-1/4 rounded-lg" />
        <Skeleton className="h-4 w-1/4 rounded-lg" />
        <Skeleton className="h-4 w-1/4 rounded-lg" />
        <Skeleton className="h-4 w-1/4 rounded-lg" />
      </div>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className={rowClassName}>
          <Skeleton className="h-4 w-1/4 rounded-lg" />
          <Skeleton className="h-4 w-1/4 rounded-lg" />
          <Skeleton className="h-4 w-1/4 rounded-lg" />
          <Skeleton className="h-4 w-1/4 rounded-lg" />
        </div>
      ))}
    </Panel>
  );
}

/** Matches `StatTile`'s own `Panel as="article" mobile="card"` recipe exactly (same size, same padding), so a stat tile never resizes once real data replaces it. */
export function SkeletonStat() {
  return (
    <Panel as="article" mobile="card" aria-hidden="true">
      <div className="flex items-center justify-between">
        <Skeleton className="h-3 w-20 rounded-lg" />
        <Skeleton className="size-9 rounded-lg" />
      </div>
      <Skeleton className="mt-4 h-8 w-16 rounded-lg" />
    </Panel>
  );
}
