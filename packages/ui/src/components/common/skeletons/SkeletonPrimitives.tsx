import { Panel } from '../Panel';
import { Skeleton } from '../../ui/skeleton';
import { messages } from '../../../messages';

export function SkeletonCourtCard() {
  return (
    <div
      className="border-border-subtle bg-bg-subtle flex h-full flex-col overflow-hidden rounded-2xl border"
      aria-hidden="true"
    >
      {/* Mirrors `CourtCard`: header (name + switch, sport line, description
          line), two price rows, then an action row divided by a top border. */}
      <div className="flex-1 p-4 sm:p-5">
        <div className="flex items-center justify-between gap-2">
          <Skeleton className="h-5 w-2/5 rounded-lg" />
          <Skeleton className="h-5 w-9 shrink-0 rounded-full" />
        </div>
        <Skeleton className="mt-1.5 h-3.5 w-1/3 rounded-lg" />
        <Skeleton className="mt-2 h-3.5 w-3/5 rounded-lg" />
        <div className="mt-4 space-y-1">
          {[0, 1].map((i) => (
            <div key={i} className="flex h-5 items-center gap-3">
              <Skeleton className="h-3 w-16 shrink-0 rounded-lg" />
              <Skeleton className="h-4 w-28 rounded-lg" />
            </div>
          ))}
        </div>
      </div>
      <div className="border-border-subtle flex items-center gap-2 border-t px-4 py-3 sm:px-5">
        <Skeleton className="h-8 flex-1 rounded-lg sm:w-20 sm:flex-none" />
        <Skeleton className="h-8 flex-1 rounded-lg sm:w-24 sm:flex-none" />
        <Skeleton className="ml-auto hidden h-8 w-24 rounded-lg sm:block" />
        <Skeleton className="h-8 w-8 shrink-0 rounded-lg sm:hidden" />
      </div>
    </div>
  );
}

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
