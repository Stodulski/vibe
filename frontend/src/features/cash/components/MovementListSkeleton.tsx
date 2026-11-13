import { Skeleton } from '@/shared/components/ui/skeleton';
import { MOVEMENT_ROW_CLASS } from './cashLayout';

/** Loading shape of `MovementList`/`MovementRow`: a heading, then rows of three stacked lines beside the amount. */
export function MovementListSkeleton({ rows }: { rows: number }) {
  return (
    <div>
      <Skeleton className="mb-1 h-6 w-32 rounded-lg" />
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className={MOVEMENT_ROW_CLASS}>
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
