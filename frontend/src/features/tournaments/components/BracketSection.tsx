import { keepTogether } from '@/shared/lib/keepTogether';
import { formatEventDay } from '@/shared/lib/eventDates';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn, formatTime } from '@/shared/lib/utils';
import { bracketGrid, bracketRounds, resultText, roundName, slotLabel } from '../lib/tournamentView';
import type { TournamentMatch, TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.bracket;

interface Lookups {
  pairs: Map<string, TournamentOwnerPair>;
  groupNames: Map<string, string>;
}

function BracketSlot({ match, side, lookups }: { match: TournamentMatch; side: 1 | 2; lookups: Lookups }) {
  const winnerId = match.winner_pair_id;
  const pairId = side === 1 ? match.pair1_id : match.pair2_id;
  const won = !!winnerId && winnerId === pairId;
  const known = !!pairId;
  return (
    <p
      className={cn(
        'min-w-0 px-2.5 py-1.5 text-sm break-words',
        won ? 'text-text-primary font-semibold' : known ? 'text-text-secondary' : 'text-text-tertiary italic',
      )}
    >
      {slotLabel(match, side, lookups.pairs, lookups.groupNames)}
    </p>
  );
}

/** Which connector a match draws: one to the next round, one from the previous, both only when the rounds halve. */
interface Connectors {
  /** The match is the first or the second of the pair feeding one match of the next round. */
  out: 'first' | 'second' | null;
  in: boolean;
}

const CONNECTOR = 'border-border-default pointer-events-none absolute';

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
  const score = resultText(match, lookups.pairs);
  const when = match.schedule
    ? ES_AR.tournaments.matches.scheduledAt(
        keepTogether(match.schedule.court_name),
        formatEventDay(match.schedule.date),
        formatTime(match.schedule.start_time),
      )
    : null;
  return (
    <li
      className="relative flex items-center py-1.5"
      style={{ gridRow: `${String(rowStart)} / span ${String(rowSpan)}` }}
    >
      {connectors.in && (
        <span
          aria-hidden="true"
          data-bracket-connector="in"
          className={cn(CONNECTOR, 'top-1/2 -left-3 w-3 border-t')}
        />
      )}
      <div className="border-border-subtle bg-bg-subtle divide-border-subtle w-full divide-y rounded-xl border">
        <BracketSlot match={match} side={1} lookups={lookups} />
        <BracketSlot match={match} side={2} lookups={lookups} />
        {(score || when) && (
          <p className="text-text-tertiary px-2.5 py-1 text-xs break-words">
            {score && <span className="tabular-nums">{score}</span>}
            {score && when && ' · '}
            {when && <span className="first-letter:uppercase">{when}</span>}
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
 * The bracket as one column per round, first round to the final. It scrolls
 * sideways inside its own region (never the page) when the columns do not fit,
 * 320 px included.
 */
export function BracketSection({ matches, pairs, groupNames }: BracketSectionProps) {
  const rounds = bracketRounds(matches);
  if (rounds.length === 0) return <p className="text-text-tertiary text-sm">{t.none}</p>;
  const lookups = { pairs, groupNames };
  const grid = bracketGrid(rounds.map((round) => round.length));

  return (
    <div className="space-y-1.5">
      <p className="text-text-tertiary text-xs sm:hidden">{t.scrollHint}</p>
      <div
        role="region"
        aria-label={t.label}
        className="focus-self border-border-subtle max-w-full overflow-x-auto rounded-xl border p-3"
      >
        <div
          className="grid min-w-max gap-x-6"
          style={{
            gridTemplateColumns: `repeat(${String(rounds.length)}, 14rem)`,
            gridTemplateRows: `auto repeat(${String(grid.rows)}, 1fr)`,
          }}
        >
          {rounds.map((round, index) => {
            const span = grid.spans[index] ?? 1;
            return (
              <section
                key={index}
                className="grid"
                style={{ gridRow: `1 / span ${String(grid.rows + 1)}`, gridTemplateRows: 'subgrid' }}
              >
                <h3 className="text-text-secondary pb-2 text-xs font-semibold">
                  {roundName(index + 1, rounds.length, round.length)}
                </h3>
                <ul className="grid" style={{ gridRow: `2 / span ${String(grid.rows)}`, gridTemplateRows: 'subgrid' }}>
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
      </div>
    </div>
  );
}
