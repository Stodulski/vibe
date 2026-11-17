import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import type { MatchSide } from '../lib/tournamentView';

const t = ES_AR.tournaments.matches;

function MatchPair({ side, won }: { side: MatchSide; won: boolean }) {
  return (
    <span
      className={cn(
        'min-w-0 truncate',
        won ? 'text-text-primary font-semibold' : side.known ? 'text-text-secondary' : 'text-text-tertiary italic',
      )}
    >
      {side.label}
    </span>
  );
}

/**
 * The two pairs of a match: stacked on phones with a green "vs" between them, one
 * line from `sm`. The winner is bold; a side no pair holds yet reads as a placeholder.
 */
export function MatchNames({
  first,
  second,
  winnerSide,
}: {
  first: MatchSide;
  second: MatchSide;
  winnerSide: 1 | 2 | null;
}) {
  return (
    <p
      className="flex w-fit max-w-full min-w-0 flex-col text-sm sm:flex-row"
      title={`${first.label} ${t.versus} ${second.label}`}
    >
      <MatchPair side={first} won={winnerSide === 1} />
      <span className="text-primary-400 shrink-0 text-base font-bold sm:px-1.5 sm:text-sm sm:font-semibold">
        {t.versus}
      </span>
      <MatchPair side={second} won={winnerSide === 2} />
    </p>
  );
}
