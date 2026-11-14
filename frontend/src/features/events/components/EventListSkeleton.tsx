import { ES_AR } from '@/shared/i18n/es_AR';
import { EVENT_LIST_CLASS } from '../lib/eventLayout';
import { EventCardSkeleton } from './EventCardSkeleton';

/** Loading shape of `EventList`. The one `role="status"` of the page body. */
export function EventListSkeleton() {
  return (
    <div role="status" aria-label={ES_AR.common.loading} className={EVENT_LIST_CLASS}>
      {Array.from({ length: 4 }, (_, i) => (
        <EventCardSkeleton key={i} />
      ))}
    </div>
  );
}
