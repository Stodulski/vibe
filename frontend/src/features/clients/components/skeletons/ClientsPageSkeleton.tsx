import { Skeleton } from '@/shared/components/ui/skeleton';
import { ClientsListSkeleton } from './ClientsListSkeleton';

/**
 * What `ClientsPage` draws while its list loads, chrome included: the mobile
 * title, the search input (40px) with the count pill's place, then the list.
 */
export function ClientsPageSkeleton() {
  return (
    <div className="animate-fade-in">
      <Skeleton className="mb-3 h-7 w-32 rounded-lg sm:mb-5 md:hidden" />
      <div className="space-y-2.5 sm:space-y-4">
        <div className="flex items-center gap-3">
          <Skeleton className="h-10 flex-1 rounded-xl" />
          <Skeleton className="h-7 w-24 shrink-0 rounded-full" />
        </div>
        <ClientsListSkeleton />
      </div>
    </div>
  );
}
