import type { TournamentOwnerMatch } from '@/shared/types/api.types';

type Match = Pick<
  TournamentOwnerMatch,
  'stage' | 'group_id' | 'round' | 'position' | 'bye' | 'status' | 'source1' | 'source2'
>;

/** The bracket match the winner of (round, position) goes to: the next round, position halved. */
function nextOf<T extends Match>(match: T, matches: T[]): T | undefined {
  return matches.find(
    (m) => m.stage === 'bracket' && m.round === match.round + 1 && m.position === Math.floor(match.position / 2),
  );
}

/**
 * Whether a played match can no longer be corrected, as the server decides it:
 * a bracket match once the match it feeds has a result; a group match once a
 * bracket match its group feeds has one (looking through a bye to the match
 * after it).
 */
export function isCorrectionLocked<T extends Match>(match: T, matches: T[]): boolean {
  if (match.stage === 'bracket') return nextOf(match, matches)?.status === 'completed';
  return matches.some((m) => {
    if (m.stage !== 'bracket') return false;
    const fed = [m.source1, m.source2].some((source) => source !== null && source.group_id === match.group_id);
    if (!fed) return false;
    const target = m.bye ? nextOf(m, matches) : m;
    return target?.status === 'completed';
  });
}
