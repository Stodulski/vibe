import { useCallback, useEffect, useRef, useState } from 'react';
import { BOOKING_MESSAGES } from '../../messages';
import type { PublicTournamentMatch, PublicTournamentPair } from '../../api/types';
import { bracketRounds, pairLookup, scheduleLabel, scoreCells, slotLabel, type PairLookup, type Slot } from './tournamentModel';

const t = BOOKING_MESSAGES.tournaments;

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

/** Which connector a match draws: one to the next round and one from the previous, only where the rounds halve. */
interface Connectors {
  /** The match is the first or the second of the pair that feeds one match of the next round. */
  out: 'first' | 'second' | null;
  in: boolean;
}

const CONNECTOR = 'border-border-default pointer-events-none absolute';

function connectorsOf(rounds: readonly { matches: readonly unknown[] }[], roundIndex: number, position: number): Connectors {
  const size = rounds[roundIndex]?.matches.length ?? 0;
  const next = rounds[roundIndex + 1]?.matches.length ?? 0;
  const previous = rounds[roundIndex - 1]?.matches.length ?? 0;
  return {
    out: next > 0 && size === next * 2 ? (position % 2 === 0 ? 'first' : 'second') : null,
    in: previous > 0 && previous === size * 2,
  };
}

interface BracketMatchProps {
  connectors: Connectors;
  match: PublicTournamentMatch;
  label: string;
  pairs: PairLookup;
  groupNames: ReadonlyMap<string, string>;
}

function BracketMatch({ match, label, pairs, groupNames, connectors }: BracketMatchProps) {
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
    <li aria-label={`${label}, ${t.matchLabel} ${String(match.position)}`} className="relative flex flex-1 items-center py-1.5">
      {connectors.in && (
        <span aria-hidden="true" data-bracket-connector="in" className={`${CONNECTOR} top-1/2 -left-2 w-2 border-t`} />
      )}
      <div className="border-border-subtle bg-bg-subtle w-full rounded-xl border">
        {side(1)}
        <div className="border-border-subtle border-t" />
        {side(2)}
        {match.schedule && (
          <p className="border-border-subtle text-text-tertiary border-t px-3 py-1.5 text-xs">{scheduleLabel(match.schedule)}</p>
        )}
      </div>
      {connectors.out && (
        <span
          aria-hidden="true"
          data-bracket-connector="out"
          className={`${CONNECTOR} -right-2 h-1/2 w-2 border-r ${connectors.out === 'first' ? 'top-1/2 border-t' : 'bottom-1/2 border-b'}`}
        />
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

/** A column never gets narrower than this; below it the bracket scrolls inside its card. */
const COLUMN_MIN_REM = 9.5;
const COLUMN_GAP_REM = 1;

interface Overflow {
  start: boolean;
  end: boolean;
}

/** Which edges of the scroller have more bracket hidden behind them. */
function useOverflowEdges() {
  const ref = useRef<HTMLDivElement>(null);
  const [edges, setEdges] = useState<Overflow>({ start: false, end: false });

  const measure = useCallback(() => {
    const el = ref.current;
    if (!el) return;
    const max = el.scrollWidth - el.clientWidth;
    const next = { start: max > 1 && el.scrollLeft > 1, end: max > 1 && el.scrollLeft < max - 1 };
    setEdges((prev) => (prev.start === next.start && prev.end === next.end ? prev : next));
  }, []);

  useEffect(() => {
    measure();
    const el = ref.current;
    if (!el || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => {
      observer.disconnect();
    };
  }, [measure]);

  return { ref, edges, measure };
}

const FADE = 'from-bg-base pointer-events-none absolute inset-y-px z-10 w-8 to-transparent';

/**
 * The knockout bracket, one column per round. The columns share the card's
 * width when the bracket fits; when it truly does not, it scrolls sideways
 * inside its own card (so a wide bracket never widens the page, 320 px
 * included) and a fade on the cut edge says there is more.
 */
export function BracketView({ matches, pairs, groupNames }: BracketViewProps) {
  const { ref, edges, measure } = useOverflowEdges();
  const rounds = bracketRounds(matches);
  if (rounds.length === 0) return null;
  const lookup = pairLookup(pairs);
  const minWidth = `${String(rounds.length * COLUMN_MIN_REM + (rounds.length - 1) * COLUMN_GAP_REM)}rem`;

  return (
    <div className="relative max-w-full min-w-0">
      <div
        ref={ref}
        onScroll={measure}
        role="region"
        aria-label={t.bracketTitle}
        // A scrollable region must be reachable by keyboard to scroll it.
        // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex
        tabIndex={0}
        className="border-border-subtle max-w-full min-w-0 overflow-x-auto overscroll-x-contain rounded-xl border p-3"
      >
        <div className="flex gap-4" style={{ minWidth }}>
          {rounds.map((round, roundIndex) => (
            <section key={round.round} className="flex min-w-0 flex-1 basis-0 flex-col gap-2">
              <h4 className="text-text-secondary text-sm font-semibold">{round.label}</h4>
              <ol className="flex flex-1 flex-col">
                {round.matches.map((match, position) => (
                  <BracketMatch
                    key={match.id}
                    match={match}
                    label={round.label}
                    pairs={lookup}
                    groupNames={groupNames}
                    connectors={connectorsOf(rounds, roundIndex, position)}
                  />
                ))}
              </ol>
            </section>
          ))}
        </div>
      </div>
      {edges.start && <span aria-hidden="true" data-bracket-fade="start" className={`${FADE} left-px rounded-l-xl bg-gradient-to-r`} />}
      {edges.end && <span aria-hidden="true" data-bracket-fade="end" className={`${FADE} right-px rounded-r-xl bg-gradient-to-l`} />}
    </div>
  );
}
