import { Skeleton } from '@/shared/components/ui/skeleton';
import { EVENTS_FILTER_BAR_CLASS, EVENTS_FILTER_FIELD_CLASS } from './eventsHubLayout';

/** One filter of `EventsHubFilters`: a label line over a 40px select. */
function FilterSkeleton() {
  return (
    <div className={EVENTS_FILTER_FIELD_CLASS}>
      <Skeleton className="h-3.5 w-12 rounded-lg" />
      <Skeleton className="h-10 w-full rounded-md" />
    </div>
  );
}

/** Loading stand-in for `EventsHubFilters`: the four selects (window, kind, modality, day), so the list does not move when they arrive. */
export function EventsHubFiltersSkeleton() {
  return (
    <div className={EVENTS_FILTER_BAR_CLASS} aria-hidden="true">
      <FilterSkeleton />
      <FilterSkeleton />
      <FilterSkeleton />
      <FilterSkeleton />
    </div>
  );
}
