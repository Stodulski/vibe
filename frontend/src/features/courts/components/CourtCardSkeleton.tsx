import { Skeleton } from '@/shared/components/ui/skeleton';
import { cn } from '@/shared/lib/utils';
import { COURT_CARD_ACTIONS_CLASS, COURT_CARD_BODY_CLASS, COURT_CARD_CLASS } from './courtLayout';

/**
 * Loading shape of `CourtCard`: header (name + switch, sport line, description
 * line), two price rows, then an action row divided by a top border.
 */
export function CourtCardSkeleton() {
  return (
    <div className={cn(COURT_CARD_CLASS, 'border-border-subtle')} aria-hidden="true">
      <div className={COURT_CARD_BODY_CLASS}>
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
      <div className={COURT_CARD_ACTIONS_CLASS}>
        <Skeleton className="h-8 flex-1 rounded-lg sm:w-20 sm:flex-none" />
        <Skeleton className="h-8 flex-1 rounded-lg sm:w-24 sm:flex-none" />
        <Skeleton className="ml-auto hidden h-8 w-24 rounded-lg sm:block" />
        <Skeleton className="h-8 w-8 shrink-0 rounded-lg sm:hidden" />
      </div>
    </div>
  );
}
