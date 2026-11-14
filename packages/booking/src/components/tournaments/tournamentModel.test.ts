// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { DETAIL, MATCHES, PAIRS, matchById } from '../../test/tournamentFixtures';
import {
  bracketRounds,
  groupMatchdays,
  pairLines,
  pairLookup,
  roundLabel,
  scheduleByDay,
  scoreCells,
  slotLabel,
} from './tournamentModel';

const pairs = pairLookup(PAIRS);

describe('pairLines', () => {
  it('names both players and nothing else', () => {
    expect(pairLines(pairs.get('p1'))).toEqual(['Ana Pérez', 'Luis Gómez']);
  });

  it('is empty for a pair that is not known yet', () => {
    expect(pairLines(undefined)).toEqual([]);
  });
});

describe('roundLabel', () => {
  it('names the last rounds of a bracket and numbers the early ones', () => {
    expect(roundLabel(3, 3)).toBe('Final');
    expect(roundLabel(2, 3)).toBe('Semifinales');
    expect(roundLabel(1, 3)).toBe('Cuartos de final');
    expect(roundLabel(1, 4)).toBe('Octavos de final');
    expect(roundLabel(1, 5)).toBe('Ronda 1');
    expect(roundLabel(1, 1)).toBe('Final');
  });
});

describe('bracketRounds', () => {
  it('builds one column per round, first round first, matches top to bottom', () => {
    const rounds = bracketRounds(MATCHES.filter((m) => m.category_id === 'c-groups'));
    expect(rounds.map((r) => [r.round, r.label, r.matches.map((m) => m.id)])).toEqual([
      [1, 'Semifinales', ['b-s1', 'b-s2']],
      [2, 'Final', ['b-f']],
    ]);
  });

  it('ignores group matches and is empty without a bracket', () => {
    expect(bracketRounds(MATCHES.filter((m) => m.stage === 'group'))).toEqual([]);
  });
});

describe('slotLabel', () => {
  const groupNames = new Map(DETAIL.groups.map((g) => [g.id, g.name]));
  const semi = matchById('b-s1');
  const bye = matchById('e-s1');

  it('says where a waiting slot comes from', () => {
    expect(slotLabel(semi, 1, pairs, groupNames)).toEqual({ kind: 'source', text: '1º de la zona A' });
    expect(slotLabel(semi, 2, pairs, groupNames)).toEqual({ kind: 'source', text: '2º de la zona B' });
  });

  it('marks the empty side of a bye and a slot nobody feeds yet', () => {
    expect(slotLabel(bye, 2, pairs, groupNames)).toEqual({ kind: 'bye', text: 'Libre' });
    expect(slotLabel(matchById('b-f'), 1, pairs, groupNames)).toEqual({ kind: 'tbd', text: 'Por definir' });
  });

  it('gives the players of a known pair', () => {
    expect(slotLabel(matchById('e-f'), 1, pairs, groupNames)).toEqual({
      kind: 'pair',
      lines: ['Lola Nieto', 'Mati Ocampo'],
    });
  });
});

describe('scoreCells', () => {
  it('gives each side its games per set, a match tiebreak in brackets', () => {
    const m = matchById('m-b1');
    expect(scoreCells(m, 1)).toEqual(['6', '3', '[8]']);
    expect(scoreCells(m, 2)).toEqual(['4', '6', '[10]']);
  });

  it('marks the loser of a walkover instead of a score', () => {
    const m = matchById('e-s2');
    expect(scoreCells(m, 2)).toEqual(['W.O.']);
    expect(scoreCells(m, 1)).toEqual([]);
  });

  it('is empty before a result', () => {
    expect(scoreCells(matchById('m-a2'), 1)).toEqual([]);
  });
});

describe('groupMatchdays', () => {
  it('lists a group matches by matchday', () => {
    const days = groupMatchdays(MATCHES, 'gA');
    expect(days.map((d) => [d.round, d.matches.map((m) => m.id)])).toEqual([
      [1, ['m-a1']],
      [2, ['m-a2']],
    ]);
  });
});

describe('scheduleByDay', () => {
  it('orders the scheduled matches by day, then time, then court', () => {
    const days = scheduleByDay(MATCHES.filter((m) => m.category_id === 'c-groups'));
    expect(days).toHaveLength(1);
    expect(days[0]?.date).toBe('2099-12-05');
    expect(days[0]?.matches.map((m) => m.id)).toEqual(['m-a1', 'm-a2']);
  });
});
