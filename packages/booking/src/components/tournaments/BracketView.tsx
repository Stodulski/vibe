import { BOOKING_MESSAGES } from '../../messages';
import type { PublicTournamentMatch, PublicTournamentPair, PublicTournamentSchedule } from '../../api/types';
import { bracketRounds, pairLookup, scoreCells, slotLabel, type PairLookup, type Slot } from './tournamentModel';

const t = BOOKING_MESSAGES.tournaments;

/** "05/12 10:00 hs, Cancha 1": the short form a bracket card has room for. */
function shortSchedule(schedule: PublicTournamentSchedule): string {
  const [, month = '', day = ''] = schedule.date.split('-');
  return `${day}/${month} ${schedule.start_time} ${t.hoursSuffix}, ${schedule.court_name}`;
}

interface SlotRowProps {
  slot: Slot;
  withdrawn: boolean;
  winner: boolean;
  cells: string[];
}

function SlotRow({ slot, withdrawn, winner, cells }: SlotRowProps) {
  return (
    <div className="flex items-start justify-between gap-2 px-3 py-2">
      <div className={winner ? 'text-text-primary min-w-0 font-semibold' : 'text-text-secondary min-w-0'}>
        {slot.kind === 'pair' ? (
          slot.lines.map((line) => (
            <span key={line} className="block truncate text-sm">
              {line}
            </span>
          ))
        ) : (
          <span className="text-text-tertiary block text-sm italic">{slot.text}</span>
        )}
        {withdrawn && <span className="text-text-tertiary block text-xs">{t.withdrawn}</span>}
      </div>
      <div className="text-text-primary flex shrink-0 gap-1.5 text-sm font-semibold tabular-nums">
        {cells.map((cell, index) => (
          <span key={index}>{cell}</span>
        ))}
      </div>
    </div>
  );
}

interface BracketMatchProps {
  match: PublicTournamentMatch;
  label: string;
  pairs: PairLookup;
  groupNames: ReadonlyMap<string, string>;
}

function BracketMatch({ match, label, pairs, groupNames }: BracketMatchProps) {
  const side = (n: 1 | 2) => {
    const id = n === 1 ? match.pair1_id : match.pair2_id;
    const pair = id ? pairs.get(id) : undefined;
    return (
      <SlotRow
        slot={slotLabel(match, n, pairs, groupNames)}
        withdrawn={pair?.status === 'withdrawn'}
        winner={id !== null && match.winner_pair_id === id && !match.bye}
        cells={scoreCells(match, n)}
      />
    );
  };
  return (
    <li aria-label={`${label}, ${t.matchLabel} ${String(match.position)}`} className="border-border-subtle bg-bg-subtle rounded-xl border">
      {side(1)}
      <div className="border-border-subtle border-t" />
      {side(2)}
      {match.schedule && (
        <p className="border-border-subtle text-text-tertiary border-t px-3 py-1.5 text-xs">{shortSchedule(match.schedule)}</p>
      )}
    </li>
  );
}

interface BracketViewProps {
  /** The category's matches: only the bracket ones are drawn. */
  matches: readonly PublicTournamentMatch[];
  pairs: readonly PublicTournamentPair[];
  groupNames: ReadonlyMap<string, string>;
}

/**
 * The knockout bracket, one column per round. It scrolls sideways inside its
 * own container so a wide bracket never widens the page (320 px included).
 */
export function BracketView({ matches, pairs, groupNames }: BracketViewProps) {
  const rounds = bracketRounds(matches);
  if (rounds.length === 0) return null;
  const lookup = pairLookup(pairs);

  return (
    <div
      role="region"
      aria-label={t.bracketTitle}
      // A scrollable region must be reachable by keyboard to scroll it.
      // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex
      tabIndex={0}
      className="border-border-subtle max-w-full min-w-0 overflow-x-auto overscroll-x-contain rounded-xl border p-3"
    >
      <div className="flex min-w-max gap-6">
        {rounds.map((round) => (
          <section key={round.round} className="flex w-60 shrink-0 flex-col gap-3">
            <h4 className="text-text-secondary text-sm font-semibold">{round.label}</h4>
            <ol className="flex flex-1 flex-col justify-around gap-3">
              {round.matches.map((match) => (
                <BracketMatch key={match.id} match={match} label={round.label} pairs={lookup} groupNames={groupNames} />
              ))}
            </ol>
          </section>
        ))}
      </div>
    </div>
  );
}
