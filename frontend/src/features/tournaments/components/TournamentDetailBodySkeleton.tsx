import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { TOURNAMENT_DETAIL_CLASS } from '../lib/tournamentLayout';
import { CategoryTabsSkeleton } from './CategoryTabsSkeleton';
import { SegmentedTabsSkeleton } from './SegmentedTabsSkeleton';

/**
 * Loading shape of the detail body, line for line: the header (name on a phone,
 * days under it), the category tabs, the category's summary line, the section
 * switch, then the section's rows flat on the page, as the loaded body draws them.
 */
export function TournamentDetailBodySkeleton() {
  return (
    <div role="status" aria-label={ES_AR.common.loading} className={TOURNAMENT_DETAIL_CLASS}>
      <div className="space-y-2" aria-hidden="true">
        <Skeleton className="h-7 w-48 rounded-lg md:hidden" />
        <Skeleton className="h-4 w-56 rounded-lg" />
      </div>
      <CategoryTabsSkeleton count={3} />
      <div className="space-y-2" aria-hidden="true">
        <Skeleton className="h-4 w-72 max-w-full rounded-lg" />
        <Skeleton className="h-4 w-24 rounded-lg" />
      </div>
      <SegmentedTabsSkeleton count={4} />
      <div className="space-y-3" aria-hidden="true">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-14 w-full rounded-lg" />
        ))}
      </div>
    </div>
  );
}
