import { hasResults } from './tournamentView';
import type {
  TournamentDetailResponse,
  TournamentGroup,
  TournamentMatch,
  TournamentOwnerPair,
} from '@/shared/types/api.types';

export interface CategoryView {
  pairs: TournamentOwnerPair[];
  groups: TournamentGroup[];
  matches: TournamentMatch[];
  pairsById: Map<string, TournamentOwnerPair>;
  groupNames: Map<string, string>;
  /** A played match exists: the draw can no longer be repeated and pairs can no longer be moved. */
  locked: boolean;
  /** The pairs that paid: the ones the draw takes and a cancel refunds. */
  paidPairs: TournamentOwnerPair[];
}

/** One category's slice of the tournament detail, with the lookups every section needs. */
export function categoryView(detail: TournamentDetailResponse, categoryId: string): CategoryView {
  const pairs = detail.pairs.filter((pair) => pair.category_id === categoryId);
  const groups = detail.groups
    .filter((group) => group.category_id === categoryId)
    .sort((a, b) => a.position - b.position);
  const matches = detail.matches.filter((match) => match.category_id === categoryId);
  return {
    pairs,
    groups,
    matches,
    pairsById: new Map(pairs.map((pair) => [pair.id, pair])),
    groupNames: new Map(groups.map((group) => [group.id, group.name])),
    locked: hasResults(matches),
    paidPairs: pairs.filter((pair) => pair.registration.status === 'confirmed' && pair.status === 'active'),
  };
}
