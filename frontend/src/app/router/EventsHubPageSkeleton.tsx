import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { Skeleton } from '@/shared/components/ui/skeleton';
// Deep imports on purpose: routeSkeletons loads eagerly, and the events barrel would
// drag the real create and detail components into the main bundle.
import { EventListSkeleton } from '@/features/events/components/EventListSkeleton';
import { EVENTS_PAGE_CLASS } from '@/features/events/lib/eventLayout';
import { EventsHubFiltersSkeleton } from './EventsHubFiltersSkeleton';

/** What `EventsHubPage` draws while it loads: the header with its create button, the filters with the window, then the list. */
export function EventsHubPageSkeleton() {
  return (
    <div>
      <PageHeaderSkeleton actions={<Skeleton className="h-10 w-full rounded-xl sm:w-36 md:ml-auto" />} />
      <div className={EVENTS_PAGE_CLASS}>
        <EventsHubFiltersSkeleton />
        <EventListSkeleton />
      </div>
    </div>
  );
}
