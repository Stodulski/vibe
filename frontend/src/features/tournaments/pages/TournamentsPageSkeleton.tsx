import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { SegmentedTabsSkeleton } from '../components/SegmentedTabsSkeleton';
import { TournamentListSkeleton } from '../components/TournamentListSkeleton';
import { TOURNAMENTS_PAGE_CLASS } from '../lib/tournamentLayout';

/** What `TournamentsPage` draws while it loads: the header with its create button, the tabs, then the list. */
export function TournamentsPageSkeleton() {
  return (
    <div>
      <PageHeaderSkeleton actions={<Skeleton className="h-10 w-full rounded-xl sm:w-36 md:ml-auto" />} />
      <div className={TOURNAMENTS_PAGE_CLASS}>
        <SegmentedTabsSkeleton />
        <TournamentListSkeleton />
      </div>
    </div>
  );
}
