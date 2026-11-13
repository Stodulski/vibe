import { Skeleton } from '@/shared/components/ui/skeleton';
import { CLIENTS_SEARCH_BAR_CLASS } from './clientsSearchBarLayout';

/** Holds the count pill's place, so the input does not narrow when the pill appears. */
export function ClientCountSkeleton() {
  return <Skeleton className="h-7 w-24 shrink-0 rounded-full" aria-hidden="true" />;
}

/** Loading stand-in for `ClientsSearchBar`: the input (40px) and the count pill's place. */
export function ClientsSearchBarSkeleton() {
  return (
    <div className={CLIENTS_SEARCH_BAR_CLASS}>
      <Skeleton className="h-10 flex-1 rounded-xl" />
      <ClientCountSkeleton />
    </div>
  );
}
