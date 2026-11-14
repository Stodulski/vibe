import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { EventListSkeleton } from '../components/EventListSkeleton';
import { EventsTabsSkeleton } from '../components/EventsTabsSkeleton';
import { EVENTS_PAGE_CLASS } from '../lib/eventLayout';

/** What `EventsPage` draws while it loads: the header with its create button, the tabs, then the list. */
export function EventsPageSkeleton() {
  return (
    <div>
      <PageHeaderSkeleton actions={<Skeleton className="h-10 w-full rounded-xl sm:w-36 md:ml-auto" />} />
      <div className={EVENTS_PAGE_CLASS}>
        <EventsTabsSkeleton />
        <EventListSkeleton />
      </div>
    </div>
  );
}
