import { ES_AR } from '@/shared/i18n/es_AR';
import { cn, formatDateShort, formatTime } from '@/shared/lib/utils';
import { bracketRounds, roundName, scoreText, slotLabel } from '../lib/tournamentView';
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

function BracketMatchCard({ match, lookups }: { match: TournamentMatch; lookups: Lookups }) {
  const score = scoreText(match);
  const when = match.schedule
    ? `${match.schedule.court_name} · ${formatDateShort(match.schedule.date)} · ${formatTime(match.schedule.start_time)}`
    : null;
  return (
    <li className="border-border-subtle bg-bg-subtle divide-border-subtle divide-y rounded-xl border">
      <BracketSlot match={match} side={1} lookups={lookups} />
      <BracketSlot match={match} side={2} lookups={lookups} />
      {(score || when) && (
        <p className="text-text-tertiary score-text px-2.5 py-1 text-xs break-words">
          {[score, when].filter(Boolean).join(' · ')}
        </p>
      )}
    </li>
  );
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
              <h3 className="text-text-secondary text-xs font-semibold">{roundName(index + 1, rounds.length)}</h3>
              <ul className="flex flex-1 flex-col justify-around gap-3">
                {round.map((match) => (
                  <BracketMatchCard key={match.id} match={match} lookups={lookups} />
                ))}
              </ul>
            </section>
          ))}
        </div>
      </div>
    </div>
  );
}
