import { Skeleton } from '@/shared/components/ui/skeleton';

const ROW_COUNT = 3;
const COLUMNS = ['w-24', 'w-16', 'w-40', 'w-20', 'w-10', 'w-16'] as const;

/**
 * Loading shape of `CourtTable`: same bordered panel, a 36px header row and
 * 64px body rows, so the page does not shift when the courts arrive.
 */
export function CourtTableSkeleton() {
  return (
    <div className="border-border-default bg-bg-subtle w-full overflow-hidden rounded-2xl border" aria-hidden="true">
      <div className="bg-bg-elevated border-border-subtle flex h-9 items-center justify-between gap-4 border-b px-6">
        {COLUMNS.map((width, i) => (
          <Skeleton key={i} className={`h-3 rounded-lg ${width}`} />
        ))}
      </div>
      {Array.from({ length: ROW_COUNT }, (_, row) => (
        <div
          key={row}
          className="border-border-subtle flex h-16 items-center justify-between gap-4 border-b px-6 last:border-b-0"
        >
          {COLUMNS.map((width, i) => (
            <Skeleton key={i} className={`h-4 rounded-lg ${width}`} />
          ))}
        </div>
      ))}
    </div>
  );
}
