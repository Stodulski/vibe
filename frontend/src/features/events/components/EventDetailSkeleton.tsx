import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { EVENT_DETAIL_BODY_CLASS } from '../lib/eventLayout';

/** Loading shape of the sheet's body: the four facts, then the participants list. */
export function EventDetailSkeleton() {
  return (
    <div role="status" aria-label={ES_AR.common.loading} className={EVENT_DETAIL_BODY_CLASS}>
      {Array.from({ length: 4 }, (_, i) => (
        <div key={i} className="space-y-1.5" aria-hidden="true">
          <Skeleton className="h-3.5 w-20 rounded-lg" />
          <Skeleton className="h-5 w-44 rounded-lg" />
        </div>
      ))}
      <div className="space-y-3 pt-2" aria-hidden="true">
        <Skeleton className="h-4 w-28 rounded-lg" />
        {Array.from({ length: 3 }, (_, i) => (
          <Skeleton key={i} className="h-12 w-full rounded-lg" />
        ))}
      </div>
    </div>
  );
}
