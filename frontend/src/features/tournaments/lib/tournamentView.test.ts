import { describe, it, expect } from 'vitest';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeMatch, makePair } from '../testing/fixtures';
import {
  bracketRounds,
  hasResults,
  pairName,
  pairPaymentKey,
  paidPairCount,
  resultText,
  roundName,
  scoreText,
  slotLabel,
} from './tournamentView';

const t = ES_AR.tournaments;

describe('pairName', () => {
  it('joins the last names of both players', () => {
    expect(pairName(makePair({ id: 'p1', players: ['Ana Pérez', 'Bea Gómez'] }))).toBe('Pérez / Gómez');
  });
});

describe('roundName', () => {
  it('names the last rounds from the final back and numbers the earlier ones', () => {
    expect(roundName(3, 3)).toBe(t.bracket.final);
    expect(roundName(2, 3)).toBe(t.bracket.semifinal);
    expect(roundName(1, 3)).toBe(t.bracket.quarterfinal);
    expect(roundName(1, 4)).toBe(t.bracket.roundOf16);
    expect(roundName(1, 5)).toBe(t.bracket.round(1));
  });
});

describe('bracketRounds', () => {
  it('groups the bracket matches by round, each top to bottom, and leaves out the group matches', () => {
    const rounds = bracketRounds([
      makeMatch({ id: 'f', stage: 'bracket', round: 2, position: 1 }),
      makeMatch({ id: 'b', stage: 'bracket', round: 1, position: 2 }),
      makeMatch({ id: 'a', stage: 'bracket', round: 1, position: 1 }),
      makeMatch({ id: 'g', stage: 'group', round: 1, position: 1 }),
    ]);
    expect(rounds.map((r) => r.map((m) => m.id))).toEqual([['a', 'b'], ['f']]);
  });
});

describe('hasResults', () => {
  it('is false when only byes are completed and true once a played match is', () => {
    const bye = makeMatch({ id: 'b', bye: true, status: 'completed' });
    expect(hasResults([bye, makeMatch({ id: 'x' })])).toBe(false);
    expect(hasResults([bye, makeMatch({ id: 'y', status: 'completed' })])).toBe(true);
  });
});

describe('pairPaymentKey and paidPairCount', () => {
  it('reads the state from the spot and the payment behind it', () => {
    expect(pairPaymentKey(makePair({ id: 'a', registration: 'held' }))).toBe('held');
    expect(pairPaymentKey(makePair({ id: 'b', registration: 'confirmed' }))).toBe('confirmed');
    expect(pairPaymentKey(makePair({ id: 'c', registration: 'cancelled', payment: 'refunded' }))).toBe('refunded');
    expect(pairPaymentKey(makePair({ id: 'd', registration: 'cancelled', payment: 'refund_pending' }))).toBe(
      'refundPending',
    );
    expect(pairPaymentKey(makePair({ id: 'e', registration: 'expired' }))).toBe('cancelledUnpaid');
  });

  it('counts the pairs that paid', () => {
    const pairs = [makePair({ id: 'a', registration: 'confirmed' }), makePair({ id: 'b', registration: 'held' })];
    expect(paidPairCount(pairs)).toBe(1);
  });
});

describe('slotLabel', () => {
  const pairs = new Map([['p1', makePair({ id: 'p1', players: ['Ana Pérez', 'Bea Gómez'] })]]);
  const groups = new Map([['g1', 'A']]);

  it('names the pair, or where it comes from, or says it is to be defined', () => {
    expect(slotLabel(makeMatch({ id: 'm', pair1Id: 'p1' }), 1, pairs, groups)).toBe('Pérez / Gómez');
    expect(slotLabel(makeMatch({ id: 'm', source2: { group_id: 'g1', rank: 2 } }), 2, pairs, groups)).toBe(
      t.bracket.sourceOf(2, 'A'),
    );
    expect(slotLabel(makeMatch({ id: 'm' }), 1, pairs, groups)).toBe(t.bracket.toBeDefined);
  });

  it('says a bye is free', () => {
    expect(slotLabel(makeMatch({ id: 'm', bye: true, pair1Id: 'p1' }), 2, pairs, groups)).toBe(t.bracket.bye);
  });
});

describe('scoreText', () => {
  it('writes the sets, the match tiebreak among them, or W.O.', () => {
    const sets = [
      { a: 6, b: 4, super: false },
      { a: 3, b: 6, super: false },
      { a: 10, b: 7, super: true },
    ];
    expect(scoreText(makeMatch({ id: 'm', sets }))).toBe('6-4 3-6 10-7');
    expect(scoreText(makeMatch({ id: 'm', walkoverLoser: 1 }))).toBe(t.bracket.walkover);
    expect(scoreText(makeMatch({ id: 'm' }))).toBe('');
  });
});

describe('resultText', () => {
  const pairs = new Map([
    ['p1', makePair({ id: 'p1', players: ['Ana Pérez', 'Bea Gómez'] })],
    ['p2', makePair({ id: 'p2', players: ['Carla Ruiz', 'Dana Soto'] })],
  ]);
  const played = { id: 'm', pair1Id: 'p1', pair2Id: 'p2', status: 'completed', winnerPairId: 'p2' } as const;

  it('says who won when the match has no score', () => {
    expect(resultText(makeMatch(played), pairs)).toBe(t.bracket.wonBy('Ruiz / Soto'));
  });

  it('keeps the sets and the walkover as they were', () => {
    const sets = [{ a: 6, b: 4, super: false }];
    expect(resultText(makeMatch({ ...played, sets }), pairs)).toBe('6-4');
    expect(resultText(makeMatch({ ...played, walkoverLoser: 1 }), pairs)).toBe(t.bracket.walkover);
  });

  it('says nothing for a match that is not played yet or a bye', () => {
    expect(resultText(makeMatch({ id: 'm', pair1Id: 'p1', pair2Id: 'p2' }), pairs)).toBe('');
    expect(resultText(makeMatch({ ...played, bye: true }), pairs)).toBe('');
  });
});
