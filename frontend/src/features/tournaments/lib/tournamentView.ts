import { ES_AR } from '@/shared/i18n/es_AR';
import type { TournamentMatch, TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

/** "Pérez / Gómez": the pair as the bracket and the tables name it. */
export function pairName(pair: Pick<TournamentOwnerPair, 'players'>): string {
  return pair.players.map((player) => player.last_name).join(' / ');
}

/** "Ana Pérez / Bea Gómez": the pair with first names, for lists where the owner needs to tell people apart. */
export function pairFullName(pair: Pick<TournamentOwnerPair, 'players'>): string {
  return pair.players.map((player) => `${player.first_name} ${player.last_name}`).join(' / ');
}

/** "Final", "Semifinal"…: a round is named by how far it is from the final. */
export function roundName(round: number, rounds: number): string {
  switch (rounds - round) {
    case 0:
      return t.bracket.final;
    case 1:
      return t.bracket.semifinal;
    case 2:
      return t.bracket.quarterfinal;
    case 3:
      return t.bracket.roundOf16;
    default:
      return t.bracket.round(round);
  }
}

/** The bracket matches by round, first round first, each top to bottom. Group matches are left out. */
export function bracketRounds<M extends TournamentMatch>(matches: M[]): M[][] {
  const rounds = new Map<number, M[]>();
  for (const match of matches) {
    if (match.stage !== 'bracket') continue;
    rounds.set(match.round, [...(rounds.get(match.round) ?? []), match]);
  }
  return [...rounds.entries()]
    .sort(([a], [b]) => a - b)
    .map(([, list]) => list.sort((a, b) => a.position - b.position));
}

/** Whether a played match exists: the draw can be repeated and pairs moved only while it does not. A bye is "completed" from the start. */
export function hasResults(matches: TournamentMatch[]): boolean {
  return matches.some((match) => match.status === 'completed' && !match.bye);
}

export type PairPaymentKey = keyof typeof t.pairs.state;

/** The one payment state a pair row shows, from the payer's spot and the payment behind it. */
export function pairPaymentKey(pair: TournamentOwnerPair): PairPaymentKey {
  const { status } = pair.registration;
  if (status === 'held') return 'held';
  if (status === 'confirmed') return 'confirmed';
  switch (pair.payment?.status) {
    case 'refunded':
      return 'refunded';
    case 'refund_pending':
      return 'refundPending';
    case 'partial_refund':
      return 'partialRefund';
    default:
      return 'cancelledUnpaid';
  }
}

/** How many pairs paid: the ones cancelling refunds. */
export function paidPairCount(pairs: TournamentOwnerPair[]): number {
  return pairs.filter((pair) => pair.registration.status === 'confirmed').length;
}

/** The pair a slot holds, or where it comes from ("1º de la zona A"), or that it is free (a bye) or to be defined. */
export function slotLabel(
  match: TournamentMatch,
  side: 1 | 2,
  pairs: Map<string, TournamentOwnerPair>,
  groupNames: Map<string, string>,
): string {
  const pairId = side === 1 ? match.pair1_id : match.pair2_id;
  const pair = pairId ? pairs.get(pairId) : undefined;
  if (pair) return pairName(pair);
  if (match.bye) return t.bracket.bye;
  const source = side === 1 ? match.source1 : match.source2;
  const group = source ? groupNames.get(source.group_id) : undefined;
  return source && group ? t.bracket.sourceOf(source.rank, group) : t.bracket.toBeDefined;
}

/** "6-4 3-6 10-7", "W.O." or nothing for a match with no result. */
export function scoreText(match: TournamentMatch): string {
  if (match.walkover_loser !== 0) return t.bracket.walkover;
  return match.sets.map((set) => `${String(set.a)}-${String(set.b)}`).join(' ');
}
