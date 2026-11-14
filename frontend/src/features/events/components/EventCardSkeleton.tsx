import { Skeleton } from '@/shared/components/ui/skeleton';
import { EVENT_CARD_BOTTOM_ROW_CLASS, EVENT_CARD_CLASS, EVENT_CARD_TOP_ROW_CLASS } from '../lib/eventLayout';

/** Loading shape of `EventCard`: name, date and time, courts, then spots and price. */
export function EventCardSkeleton() {
  return (
    <div className={EVENT_CARD_CLASS} aria-hidden="true">
      <div className={EVENT_CARD_TOP_ROW_CLASS}>
        <Skeleton className="h-6 w-44 rounded-lg" />
      </div>
      <Skeleton className="h-5 w-52 rounded-lg" />
      <Skeleton className="h-4 w-40 rounded-lg" />
      <div className={EVENT_CARD_BOTTOM_ROW_CLASS}>
        <Skeleton className="h-5 w-28 rounded-lg" />
        <Skeleton className="h-5 w-16 rounded-lg" />
      </div>
    </div>
  );
}
