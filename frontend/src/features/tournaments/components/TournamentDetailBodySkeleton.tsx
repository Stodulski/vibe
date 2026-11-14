import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { TOURNAMENT_DETAIL_CLASS } from '../lib/tournamentLayout';
import { SegmentedTabsSkeleton } from './SegmentedTabsSkeleton';

/** Loading shape of the detail body: the facts, the category and section tabs, then a list of pair rows. */
export function TournamentDetailBodySkeleton() {
  return (
    <div role="status" aria-label={ES_AR.common.loading} className={TOURNAMENT_DETAIL_CLASS}>
      <div className="space-y-3" aria-hidden="true">
        <Skeleton className="h-5 w-48 rounded-lg" />
        <Skeleton className="h-10 w-full rounded-xl sm:w-40" />
      </div>
      <SegmentedTabsSkeleton count={3} />
      <div className="space-y-3" aria-hidden="true">
        <Skeleton className="h-5 w-56 rounded-lg" />
        <Skeleton className="h-5 w-44 rounded-lg" />
        <SegmentedTabsSkeleton count={4} />
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-14 w-full rounded-lg" />
        ))}
      </div>
    </div>
  );
}
