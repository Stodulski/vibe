import { BOOKING_MESSAGES } from '../../messages';
import type { PublicTournamentMatch } from '../../api/types';
import { scoreText, slotLabel, type PairLookup, type Slot } from './tournamentModel';

const t = BOOKING_MESSAGES.tournaments;

function slotText(slot: Slot): string {
  return slot.kind === 'pair' ? slot.lines.join(' / ') : slot.text;
}

interface MatchLineProps {
  match: PublicTournamentMatch;
  pairs: PairLookup;
  groupNames: ReadonlyMap<string, string>;
  /** What sits above the pairs: the stage, the court and the time. */
  caption?: string;
}

/** One match as a card: its caption, both pairs and the score. */
export function MatchLine({ match, pairs, groupNames, caption }: MatchLineProps) {
  const first = slotLabel(match, 1, pairs, groupNames);
  const second = slotLabel(match, 2, pairs, groupNames);
  const score = scoreText(match);
  const name = (n: 1 | 2, slot: Slot) => {
    const id = n === 1 ? match.pair1_id : match.pair2_id;
    const winner = id !== null && match.winner_pair_id === id;
    return (
      <p className={winner ? 'text-text-primary font-semibold' : 'text-text-secondary'}>{slotText(slot)}</p>
    );
  };

  return (
    <li className="border-border-subtle bg-bg-subtle flex flex-col gap-1 rounded-xl border p-3 text-sm">
      {caption && <p className="text-text-tertiary text-xs">{caption}</p>}
      {name(1, first)}
      <p className="text-text-tertiary text-xs" aria-label={t.versus}>
        vs
      </p>
      {name(2, second)}
      {score && <p className="text-text-primary pt-1 font-semibold tabular-nums">{score}</p>}
    </li>
  );
}
