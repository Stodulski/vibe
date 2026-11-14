import { describe, it, expect } from 'vitest';
import { ES_AR } from '@/shared/i18n/es_AR';
import { parseScore, scoreRowCount, type ScoreFormat } from './score';

const t = ES_AR.tournaments.result.errors;
const STANDARD: ScoreFormat = { setsToWin: 2, gamesPerSet: 6, superTiebreak: true };

function rows(...scores: string[]) {
  return scores.map((score) => {
    const [a = '', b = ''] = score.split('-');
    return { a, b };
  });
}

describe('scoreRowCount', () => {
  it('is one row for a single set and three for best of three', () => {
    expect(scoreRowCount({ ...STANDARD, setsToWin: 1 })).toBe(1);
    expect(scoreRowCount(STANDARD)).toBe(3);
  });
});

describe('parseScore: best of three with a match tiebreak', () => {
  it('takes a straight-sets win and sends only the sets played', () => {
    expect(parseScore(rows('6-4', '6-3', '-'), STANDARD)).toEqual({
      ok: true,
      sets: [
        { a: 6, b: 4, super: false },
        { a: 6, b: 3, super: false },
      ],
    });
  });

  it('marks the deciding set as the match tiebreak', () => {
    expect(parseScore(rows('6-4', '3-6', '10-7'), STANDARD)).toEqual({
      ok: true,
      sets: [
        { a: 6, b: 4, super: false },
        { a: 3, b: 6, super: false },
        { a: 10, b: 7, super: true },
      ],
    });
  });

  it('takes 7-5 and 7-6 as finished sets', () => {
    expect(parseScore(rows('7-5', '6-7', '12-10'), STANDARD).ok).toBe(true);
  });

  it('refuses a set that is not finished', () => {
    expect(parseScore(rows('6-5', '6-3', '-'), STANDARD)).toEqual({ ok: false, error: t.invalidSet(1, 6, 5, 6) });
    expect(parseScore(rows('8-6', '6-3', '-'), STANDARD)).toEqual({ ok: false, error: t.invalidSet(1, 8, 6, 6) });
  });

  it('refuses a match tiebreak that is not to 10 with two points', () => {
    expect(parseScore(rows('6-4', '3-6', '10-9'), STANDARD)).toEqual({ ok: false, error: t.invalidSuper });
    expect(parseScore(rows('6-4', '3-6', '6-3'), STANDARD)).toEqual({ ok: false, error: t.invalidSuper });
  });

  it('asks for the third set when each side took one', () => {
    expect(parseScore(rows('6-4', '3-6', '-'), STANDARD)).toEqual({ ok: false, error: t.decisiveSuper });
  });

  it('refuses a set played after the match was decided', () => {
    expect(parseScore(rows('6-0', '6-0', '10-5'), STANDARD)).toEqual({ ok: false, error: t.decided(3) });
  });

  it('asks for the first sets before the next one', () => {
    expect(parseScore(rows('-', '6-3', '-'), STANDARD)).toEqual({ ok: false, error: t.missing(1) });
    expect(parseScore(rows('6-4', '-', '10-7'), STANDARD)).toEqual({ ok: false, error: t.gap(2) });
  });

  it('refuses text that is not a whole number', () => {
    expect(parseScore(rows('6-x', '6-3', '-'), STANDARD)).toEqual({ ok: false, error: t.notNumber });
  });
});

describe('parseScore: the category format', () => {
  it('plays a single set to the category games', () => {
    const single: ScoreFormat = { setsToWin: 1, gamesPerSet: 4, superTiebreak: false };
    expect(parseScore(rows('4-2'), single)).toEqual({ ok: true, sets: [{ a: 4, b: 2, super: false }] });
    expect(parseScore(rows('5-3'), single).ok).toBe(true);
    expect(parseScore(rows('5-4'), single).ok).toBe(true);
    expect(parseScore(rows('4-3'), single)).toEqual({ ok: false, error: t.invalidSet(1, 4, 3, 4) });
    expect(parseScore(rows('-'), single)).toEqual({ ok: false, error: t.missing(1) });
  });

  it('takes a regular third set when the match tiebreak is off', () => {
    const noSuper: ScoreFormat = { ...STANDARD, superTiebreak: false };
    expect(parseScore(rows('6-4', '3-6', '6-3'), noSuper)).toEqual({
      ok: true,
      sets: [
        { a: 6, b: 4, super: false },
        { a: 3, b: 6, super: false },
        { a: 6, b: 3, super: false },
      ],
    });
    expect(parseScore(rows('6-4', '3-6', '-'), noSuper)).toEqual({ ok: false, error: t.undecided });
  });
});
