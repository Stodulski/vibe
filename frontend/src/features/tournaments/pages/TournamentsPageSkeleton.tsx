import { EventsSectionTabsSkeleton } from '@/shared/components/common/EventsSectionTabsSkeleton';
import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { UpcomingPastSwitchSkeleton } from '@/shared/components/common/UpcomingPastSwitch';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { TournamentListSkeleton } from '../components/TournamentListSkeleton';
import { TOURNAMENTS_PAGE_CLASS } from '../lib/tournamentLayout';

/** What `TournamentsPage` draws while it loads: the header with its create button, the Eventos tabs, the upcoming/past toggle, then the list. */
export function TournamentsPageSkeleton() {
  return (
    <div>
      <PageHeaderSkeleton actions={<Skeleton className="h-10 w-full rounded-xl sm:w-36 md:ml-auto" />} />
      <EventsSectionTabsSkeleton />
      <div className={TOURNAMENTS_PAGE_CLASS}>
        <UpcomingPastSwitchSkeleton />
        <TournamentListSkeleton />
      </div>
    </div>
  );
}
