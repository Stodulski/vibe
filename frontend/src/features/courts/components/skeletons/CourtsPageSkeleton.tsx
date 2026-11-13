import { Skeleton } from '@/shared/components/ui/skeleton';
import { CourtListSkeleton } from './CourtListSkeleton';

/**
 * What `CourtsPage` draws while it loads, chrome included: the header row
 * (mobile title, create button at every width) and the list.
 */
export function CourtsPageSkeleton() {
  return (
    <div>
      <div className="mb-3 flex flex-col gap-2.5 sm:mb-5 sm:flex-row sm:items-center sm:justify-between">
        <Skeleton className="h-7 w-32 rounded-lg md:hidden" />
        <Skeleton className="h-9 w-full rounded-xl sm:w-40 md:ml-auto" />
      </div>
      <CourtListSkeleton />
    </div>
  );
}
