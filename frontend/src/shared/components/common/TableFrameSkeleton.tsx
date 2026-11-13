import { Skeleton } from '@/shared/components/ui/skeleton';
import { TABLE_FRAME_CLASS } from './tableFrameLayout';

interface TableFrameSkeletonProps {
  /** Width class of each column's bar, in column order. */
  columns: readonly string[];
  rows?: number;
}

/**
 * Loading shape of the dense owner tables: same bordered panel, a 36px header
 * row and 64px body rows, so the page does not shift when the data arrives.
 */
export function TableFrameSkeleton({ columns, rows = 3 }: TableFrameSkeletonProps) {
  return (
    <div className={TABLE_FRAME_CLASS} aria-hidden="true">
      <div className="bg-bg-elevated border-border-subtle flex h-9 items-center justify-between gap-4 border-b px-6">
        {columns.map((width, i) => (
          <Skeleton key={i} className={`h-3 rounded-lg ${width}`} />
        ))}
      </div>
      {Array.from({ length: rows }, (_, row) => (
        <div
          key={row}
          className="border-border-subtle flex h-16 items-center justify-between gap-4 border-b px-6 last:border-b-0"
        >
          {columns.map((width, i) => (
            <Skeleton key={i} className={`h-4 rounded-lg ${width}`} />
          ))}
        </div>
      ))}
    </div>
  );
}
