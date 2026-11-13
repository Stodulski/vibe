import { Panel } from '@/shared/components/common/Panel';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface CashSummarySkeletonProps {
  /** Ingreso / Egreso / Cerrar caja row — the open shift has it, a closed session's detail page does not. */
  showActions?: boolean;
}

/** Mirrors `ExpectedCashCard`: same Panel recipe, then the opened-at line, the big figure and the terms list. */
function SummaryCardSkeleton() {
  return (
    <Panel size="sm" mobile="card" className="w-full space-y-4">
      <Skeleton className="h-5 w-48 rounded-lg" />
      <div className="space-y-1">
        <Skeleton className="h-5 w-32 rounded-lg" />
        <Skeleton className="h-9 w-48 rounded-lg sm:h-10" />
      </div>
      <div className="border-border-subtle space-y-2 border-t pt-4">
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex items-center justify-between gap-4">
            <Skeleton className="h-5 w-32 rounded-lg" />
            <Skeleton className="h-5 w-20 rounded-lg" />
          </div>
        ))}
      </div>
    </Panel>
  );
}

/** Mirrors `CashActionButtons`: two columns below `sm` with the third button spanning, one row from `sm`. */
function ActionButtonsSkeleton() {
  return (
    <div className="grid grid-cols-2 gap-2 sm:flex sm:items-center">
      <Skeleton className="h-12 rounded-xl sm:w-32" />
      <Skeleton className="h-12 rounded-xl sm:w-32" />
      <Skeleton className="col-span-2 h-12 rounded-xl sm:ml-auto sm:w-32" />
    </div>
  );
}

/** Mirrors `MovementList`/`MovementRow`: a heading, then rows of three stacked lines beside the amount. */
function MovementRowsSkeleton({ rows }: { rows: number }) {
  return (
    <div>
      <Skeleton className="mb-1 h-6 w-32 rounded-lg" />
      {Array.from({ length: rows }, (_, i) => (
        <div
          key={i}
          className="border-border-subtle flex items-start justify-between gap-4 border-b py-4 last:border-b-0"
        >
          <div className="min-w-0 flex-1 space-y-1">
            <Skeleton className="h-5 w-32 rounded-lg" />
            <Skeleton className="h-4 w-40 rounded-lg" />
            <Skeleton className="h-4 w-24 rounded-lg" />
          </div>
          <Skeleton className="h-5 w-20 shrink-0 rounded-lg" />
        </div>
      ))}
    </div>
  );
}

/** Loading stand-in for the open shift and for a closed session's detail: summary card, optional actions, movements. */
export function CashSummarySkeleton({ showActions = true }: CashSummarySkeletonProps) {
  return (
    <div role="status" aria-label={t.common.loading} className="max-w-3xl space-y-6">
      <SummaryCardSkeleton />
      {showActions && <ActionButtonsSkeleton />}
      <MovementRowsSkeleton rows={4} />
    </div>
  );
}
