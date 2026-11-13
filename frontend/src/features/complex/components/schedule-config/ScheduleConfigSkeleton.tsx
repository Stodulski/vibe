import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * One row, with the same classes and wrapping as `ScheduleDayRow`: below `sm`
 * the day and its toggle share the first line and the times take the second;
 * from `sm` up it is one line. The times control is `h-10` (`TimeSelect`), and
 * it is what sets the row height, so the bars below copy it rather than guess.
 */
function ScheduleRowSkeleton() {
  return (
    <div className="border-border-subtle flex flex-wrap items-center gap-x-4 gap-y-2 border-b py-3 last:border-0">
      <div className="order-1 flex h-5 min-w-0 flex-1 items-center sm:w-28 sm:flex-none">
        <Skeleton className="h-4 w-20 rounded-lg" />
      </div>
      <div className="order-3 flex w-full items-center gap-2 sm:order-2 sm:flex-1">
        <Skeleton className="h-10 min-w-0 flex-1 rounded-xl sm:w-[6.75rem] sm:flex-none" />
        <Skeleton className="h-2.5 w-2 shrink-0 rounded-full" />
        <Skeleton className="h-10 min-w-0 flex-1 rounded-xl sm:w-[6.75rem] sm:flex-none" />
      </div>
      <div className="order-2 flex shrink-0 items-center gap-2 sm:order-3">
        <Skeleton className="h-2.5 w-10 rounded-lg" />
        <Skeleton className="size-[18px] rounded-[5px]" />
      </div>
    </div>
  );
}

/**
 * Same shape as the form it stands in for: seven rows, then the footer with its
 * top rule and a full-width button below `sm` (`SectionFooter`, `align="start"`),
 * all at the form's `space-y-5`. `useSchedules` loading swaps this in, so the
 * panel keeps the form's height instead of collapsing to a spinner (UI-06).
 */
export function ScheduleConfigSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading} className="space-y-5">
      <div>
        {Array.from({ length: 7 }, (_, i) => (
          <ScheduleRowSkeleton key={i} />
        ))}
      </div>
      <div className="border-border-subtle border-t pt-4">
        <Skeleton className="h-9 w-full rounded-full sm:w-24" />
      </div>
    </div>
  );
}
