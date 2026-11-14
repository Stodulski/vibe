import { ES_AR } from '@/shared/i18n/es_AR';
import { TOURNAMENT_LIST_CLASS } from '../lib/tournamentLayout';
import { TournamentCardSkeleton } from './TournamentCardSkeleton';

/** Loading shape of `TournamentList`. The one `role="status"` of the page body. */
export function TournamentListSkeleton() {
  return (
    <div role="status" aria-label={ES_AR.common.loading} className={TOURNAMENT_LIST_CLASS}>
      {Array.from({ length: 4 }, (_, i) => (
        <TournamentCardSkeleton key={i} />
      ))}
    </div>
  );
}
