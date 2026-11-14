import { keepTogether } from '@/shared/lib/keepTogether';
import { formatEventDay } from '@/shared/lib/eventDates';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn, formatTime } from '@/shared/lib/utils';
import { bracketRounds, resultText, roundName, slotLabel } from '../lib/tournamentView';
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
}: {
  match: TournamentMatch;
  lookups: Lookups;
  connectors: Connectors;
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
    <li className="relative flex flex-1 items-center py-1.5">
      {connectors.in && (
        <span
          aria-hidden="true"
          data-bracket-connector="in"
          className={cn(CONNECTOR, 'top-1/2 -left-2 w-2 border-t')}
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
            '-right-2 h-1/2 w-2 border-r',
            connectors.out === 'first' ? 'top-1/2 border-t' : 'bottom-1/2 border-b',
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

  return (
    <div className="space-y-1.5">
      <p className="text-text-tertiary text-xs sm:hidden">{t.scrollHint}</p>
      <div
        role="region"
        aria-label={t.label}
        className="focus-self border-border-subtle max-w-full overflow-x-auto rounded-xl border p-3"
      >
        <div className="flex min-w-max gap-4">
          {rounds.map((round, index) => (
            <section key={index} className="flex w-56 shrink-0 flex-col gap-2">
              <h3 className="text-text-secondary text-xs font-semibold">
                {roundName(index + 1, rounds.length, round.length)}
              </h3>
              <ul className="flex flex-1 flex-col">
                {round.map((match, position) => (
                  <BracketMatchCard
                    key={match.id}
                    match={match}
                    lookups={lookups}
                    connectors={connectorsOf(rounds, index, position)}
                  />
                ))}
              </ul>
            </section>
          ))}
        </div>
      </div>
    </div>
  );
}
