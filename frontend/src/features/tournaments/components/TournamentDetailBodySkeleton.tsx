import { Panel } from '@/shared/components/common/Panel';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { TOURNAMENT_DETAIL_CLASS } from '../lib/tournamentLayout';
import { CategoryTabsSkeleton } from './CategoryTabsSkeleton';
import { SegmentedTabsSkeleton } from './SegmentedTabsSkeleton';

/**
 * Loading shape of the detail body, card for card: the header card, the category
 * tabs, the category card, the section switch, then the list card.
 */
export function TournamentDetailBodySkeleton() {
  return (
    <div role="status" aria-label={ES_AR.common.loading} className={TOURNAMENT_DETAIL_CLASS}>
      <Panel size="md" mobile="card" className="space-y-3" aria-hidden="true">
        <Skeleton className="h-5 w-48 rounded-lg" />
        <Skeleton className="h-9 w-full rounded-xl sm:w-40" />
      </Panel>
      <CategoryTabsSkeleton count={3} />
      <Panel size="md" mobile="card" className="space-y-3" aria-hidden="true">
        <Skeleton className="h-5 w-56 rounded-lg" />
        <Skeleton className="h-5 w-44 rounded-lg" />
      </Panel>
      <SegmentedTabsSkeleton count={4} />
      <Panel size="md" mobile="card" className="space-y-3" aria-hidden="true">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-14 w-full rounded-lg" />
        ))}
      </Panel>
    </div>
  );
}
