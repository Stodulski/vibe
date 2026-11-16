import { Skeleton } from '@/shared/components/ui/skeleton';
import { TOURNAMENT_CARD_CLASS, TOURNAMENT_CARD_TOP_ROW_CLASS } from '../lib/tournamentLayout';

/** Loading shape of `TournamentCard`: name, days, categories, then pairs. */
export function TournamentCardSkeleton() {
  return (
    <div className={TOURNAMENT_CARD_CLASS} aria-hidden="true">
      <div className={TOURNAMENT_CARD_TOP_ROW_CLASS}>
        <Skeleton className="h-6 w-44 rounded-lg" />
      </div>
      <Skeleton className="h-5 w-36 rounded-lg" />
      <Skeleton className="h-4 w-40 rounded-lg" />
      <Skeleton className="mt-1.5 h-5 w-24 rounded-lg" />
    </div>
  );
}
