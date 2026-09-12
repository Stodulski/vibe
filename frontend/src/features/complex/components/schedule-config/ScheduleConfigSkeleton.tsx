import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * Same seven-row shape as `ScheduleDayRow` (day name, opening hours, closed
 * toggle) so the panel keeps `ScheduleForm`'s height while `useSchedules`
 * loads, instead of collapsing to a centered spinner (UI-06).
 */
export function ScheduleConfigSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading}>
      {Array.from({ length: 7 }, (_, i) => (
        <div key={i} className="border-border-subtle flex items-center gap-x-4 gap-y-2 border-b py-3 last:border-0">
          <Skeleton className="h-4 w-20 shrink-0 rounded-lg sm:w-28" />
          <Skeleton className="h-8 flex-1 rounded-lg sm:w-40 sm:flex-none" />
          <Skeleton className="ml-auto h-4 w-10 shrink-0 rounded-full" />
        </div>
      ))}
    </div>
  );
}
