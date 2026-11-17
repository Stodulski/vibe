import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import {
  TOURNAMENT_DETAIL_CLASS,
  TOURNAMENT_SWITCH_FIELD_CLASS,
  TOURNAMENT_SWITCH_ROW_CLASS,
} from '../lib/tournamentLayout';

/** The loading shape of one select of the switch row: its label, then the field. */
function SwitchFieldSkeleton() {
  return (
    <div className={TOURNAMENT_SWITCH_FIELD_CLASS}>
      <Skeleton className="h-4 w-16 rounded-md" />
      <Skeleton className="h-10 w-full rounded-lg" />
    </div>
  );
}

/**
 * Loading shape of the detail body, line for line: the header (name on a phone,
 * days under it), the category and section selects side by side, then the
 * section's rows flat on the page, as the loaded body draws them.
 */
export function TournamentDetailBodySkeleton() {
  return (
    <div role="status" aria-label={ES_AR.common.loading} className={TOURNAMENT_DETAIL_CLASS}>
      <div className="space-y-2" aria-hidden="true">
        <Skeleton className="h-7 w-48 rounded-lg md:hidden" />
        <Skeleton className="h-4 w-56 rounded-lg" />
      </div>
      <div className={TOURNAMENT_SWITCH_ROW_CLASS} aria-hidden="true">
        <SwitchFieldSkeleton />
        <SwitchFieldSkeleton />
      </div>
      <div className="space-y-3" aria-hidden="true">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-14 w-full rounded-lg" />
        ))}
      </div>
    </div>
  );
}
