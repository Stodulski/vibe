import { Skeleton } from '@/shared/components/ui/skeleton';
import { ExpectedCashCardShell } from './ExpectedCashCardShell';

/** Loading shape of `ExpectedCashCard`: the opened-at line, the big figure and the terms list. */
export function ExpectedCashCardSkeleton() {
  return (
    <ExpectedCashCardShell>
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
    </ExpectedCashCardShell>
  );
}
