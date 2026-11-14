// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { DETAIL, MATCHES, PAIRS, matchById } from '../../test/tournamentFixtures';
import {
  bracketRounds,
  groupMatchdays,
  matchFormatLabel,
  pairLines,
  pairLookup,
  roundLabel,
  scheduleByDay,
  scoreCells,
  scoreText,
  scheduleLabel,
  slotLabel,
  tournamentDates,
  winnerOnlySide,
} from './tournamentModel';

const pairs = pairLookup(PAIRS);

describe('matchFormatLabel', () => {
  const base = { sets_to_win: 2, games_per_set: 6, super_tiebreak_decider: false } as const;

  it('says best of three, with the super tie-break when it decides', () => {
    expect(matchFormatLabel(base)).toBe('Al mejor de 3 sets');
    expect(matchFormatLabel({ ...base, super_tiebreak_decider: true })).toBe('Al mejor de 3 sets \u00b7 súper tie-break');
  });

  it('names the games only when they are not the standard 6 of a best of three', () => {
    expect(matchFormatLabel({ ...base, games_per_set: 4, super_tiebreak_decider: true })).toBe(
      'Al mejor de 3 sets de 4 games \u00b7 súper tie-break',
    );
  });

  it('says one set with its games', () => {
    expect(matchFormatLabel({ ...base, sets_to_win: 1 })).toBe('A 1 set de 6 games');
    expect(matchFormatLabel({ ...base, sets_to_win: 1, games_per_set: 4, super_tiebreak_decider: true })).toBe('A 1 set de 4 games');
  });
});

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
    expect(slotLabel(matchById('b-f'), 1, pairs, groupNames)).toEqual({ kind: 'tbd', text: 'A definir' });
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

describe('a winner-only result', () => {
  it('has no score cells or text, and names its winner side', () => {
    const m = matchById('m-b2');
    expect(winnerOnlySide(m)).toBe(2);
    expect(scoreCells(m, 2)).toEqual(['Ganó']);
    expect(scoreCells(m, 1)).toEqual([]);
    expect(scoreText(m)).toBe('');
  });

  it('is not claimed by played, walkover, bye or pending matches', () => {
    for (const id of ['m-a1', 'e-s2', 'e-s1', 'm-a2']) expect(winnerOnlySide(matchById(id))).toBeNull();
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

describe('tournamentDates', () => {
  it('names the month once when both days share it, capitalized', () => {
    expect(tournamentDates({ start_date: '2026-12-12', end_date: '2026-12-13' })).toBe('Sábado 12 al domingo 13 de diciembre');
  });

  it('names both months when the tournament crosses them', () => {
    expect(tournamentDates({ start_date: '2026-11-28', end_date: '2026-12-01' })).toBe(
      'Sábado 28 de noviembre al martes 1 de diciembre',
    );
  });

  it('is one capitalized day for a single-day tournament', () => {
    expect(tournamentDates({ start_date: '2026-12-12', end_date: '2026-12-12' })).toBe('Sábado 12 de diciembre');
  });
});

describe('scheduleLabel', () => {
  it('reads court, day and time, with the court name unbreakable and no "hs"', () => {
    expect(
      scheduleLabel({ court_name: 'Cancha 1', date: '2026-12-12', start_time: '10:00' }),
    ).toBe('Cancha\u00a01 \u00b7 sábado 12 de diciembre \u00b7 10:00');
  });
});
