import { formatEventDayShort } from '@/shared/lib/eventDates';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn, formatTime } from '@/shared/lib/utils';
import { bracketGrid, bracketRounds, matchSide, roundName, winnerSide } from '../lib/tournamentView';
import { MatchNames } from './MatchNames';
import type { TournamentMatch, TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.bracket;

interface Lookups {
  pairs: Map<string, TournamentOwnerPair>;
  groupNames: Map<string, string>;
}

/** Which connector a match draws: one to the next round, one from the previous, both only when the rounds halve. */
interface Connectors {
  /** The match is the first or the second of the pair feeding one match of the next round. */
  out: 'first' | 'second' | null;
  in: boolean;
}

/** Connectors only exist side by side, from `lg`: stacked rounds have no columns to join. */
const CONNECTOR = 'border-border-default pointer-events-none absolute hidden lg:block';

/** The result and the schedule of a bracket match, joined on one line. */
/**
 * What a bracket card says under the pairs: only when it is played, "Dom 22/11 14:00".
 * The winner is already the bold pair; the score and the court live in Partidos.
 */
function bracketMeta(match: TournamentMatch): string {
  return match.schedule ? `${formatEventDayShort(match.schedule.date)} ${formatTime(match.schedule.start_time)}` : '';
}

function BracketMatchCard({
  match,
  lookups,
  connectors,
  rowStart,
  rowSpan,
}: {
  match: TournamentMatch;
  lookups: Lookups;
  connectors: Connectors;
  /** First grid row of the match and how many it spans: the share that centres it between its feeders. */
  rowStart: number;
  rowSpan: number;
}) {
  const meta = bracketMeta(match);
  return (
    <li
      className="border-border-subtle relative flex min-w-0 flex-col justify-center border-b py-2.5 last:border-b-0 lg:border-0 lg:py-1.5"
      style={{ gridRow: `${String(rowStart)} / span ${String(rowSpan)}` }}
    >
      {connectors.in && (
        <span
          aria-hidden="true"
          data-bracket-connector="in"
          className={cn(CONNECTOR, 'top-1/2 -left-3 w-3 border-t')}
        />
      )}
      <div className="divide-border-subtle lg:bg-bg-subtle lg:border-border-subtle w-full min-w-0 lg:divide-y lg:rounded-xl lg:border">
        <div className="min-w-0 py-0.5 lg:px-2.5 lg:py-1.5">
          <MatchNames
            first={matchSide(match, 1, lookups.pairs, lookups.groupNames)}
            second={matchSide(match, 2, lookups.pairs, lookups.groupNames)}
            winnerSide={winnerSide(match)}
          />
        </div>
        {meta && (
          <p className="text-text-tertiary truncate py-0.5 text-xs lg:px-2.5 lg:py-1" title={meta}>
            {meta}
          </p>
        )}
      </div>
      {connectors.out && (
        <span
          aria-hidden="true"
          data-bracket-connector="out"
          className={cn(
            CONNECTOR,
            '-right-3 h-1/2 w-3',
            connectors.out === 'first' ? 'top-1/2 border-t border-r' : 'bottom-1/2 border-r border-b',
          )}
        />
      )}
    </li>
  );
}

/**
 * The connectors of one match. They only exist where the rounds halve (two
 * matches feed one of the next round, as a bracket sized to a power of two
 * does); otherwise the columns are left unjoined rather than joined wrongly.
 */
function connectorsOf(rounds: TournamentMatch[][], roundIndex: number, position: number): Connectors {
  const size = rounds[roundIndex]?.length ?? 0;
  const next = rounds[roundIndex + 1]?.length ?? 0;
  const previous = rounds[roundIndex - 1]?.length ?? 0;
  const feeds = next > 0 && size === next * 2;
  return {
    out: feeds ? (position % 2 === 0 ? 'first' : 'second') : null,
    in: previous > 0 && previous === size * 2,
  };
}

interface BracketSectionProps extends Lookups {
  matches: TournamentMatch[];
}

/**
 * The bracket, first round to the final. Below `lg` each round stacks under its
 * heading, one match per row, flat on the page. From `lg` the rounds sit side by
 * side in equal columns, with the match boxes and their connectors. Nothing
 * scrolls sideways at any width.
 */
export function BracketSection({ matches, pairs, groupNames }: BracketSectionProps) {
  const rounds = bracketRounds(matches);
  if (rounds.length === 0) return <p className="text-text-tertiary text-xs">{t.none}</p>;
  const lookups = { pairs, groupNames };
  const grid = bracketGrid(rounds.map((round) => round.length));

  return (
    <div
      role="region"
      aria-label={t.label}
      className="flex min-w-0 flex-col gap-6 lg:grid lg:gap-x-6 lg:gap-y-0"
      style={{
        gridTemplateColumns: `repeat(${String(rounds.length)}, minmax(0, 1fr))`,
        gridTemplateRows: `auto repeat(${String(grid.rows)}, 1fr)`,
      }}
    >
      {rounds.map((round, index) => {
        const span = grid.spans[index] ?? 1;
        return (
          <section
            key={index}
            className="flex min-w-0 flex-col lg:grid"
            style={{ gridRow: `1 / span ${String(grid.rows + 1)}`, gridTemplateRows: 'subgrid' }}
          >
            <h3 className="text-primary-400 pb-1.5 text-base font-bold lg:pb-2">
              {roundName(index + 1, rounds.length, round.length)}
            </h3>
            <ul
              className="flex min-w-0 flex-col lg:grid"
              style={{ gridRow: `2 / span ${String(grid.rows)}`, gridTemplateRows: 'subgrid' }}
            >
              {round.map((match, position) => (
                <BracketMatchCard
                  key={match.id}
                  match={match}
                  lookups={lookups}
                  connectors={connectorsOf(rounds, index, position)}
                  rowStart={position * span + 1}
                  rowSpan={span}
                />
              ))}
            </ul>
          </section>
        );
      })}
    </div>
  );
}
