import { Skeleton } from '@/shared/components/ui/skeleton';

/** Loading shape of `EventsTabs`: two buttons of the same height and gap. */
export function EventsTabsSkeleton() {
  return (
    <div className="flex gap-2" aria-hidden="true">
      <Skeleton className="h-8 flex-1 rounded-xl sm:w-24 sm:flex-none" />
      <Skeleton className="h-8 flex-1 rounded-xl sm:w-20 sm:flex-none" />
    </div>
  );
}
