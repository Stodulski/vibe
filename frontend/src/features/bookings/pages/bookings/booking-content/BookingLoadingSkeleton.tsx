import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * `BookingCalendar`/`CourtTimeGrid` render no card at any width — it is the
 * page's own timeline, not a boxed section — so this stays fully flat too
 * (odd/tasks/app-dark-contrast.md T2 follow-up).
 */
export function BookingLoadingSkeleton() {
  return (
    <div className="space-y-2" role="status" aria-label={t.common.loading}>
      {Array.from({ length: 5 }, (_, i) => (
        <Skeleton key={i} className="h-16 w-full rounded-xl" />
      ))}
    </div>
  );
}
