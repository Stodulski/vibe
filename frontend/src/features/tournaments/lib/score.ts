import { ES_AR } from '@/shared/i18n/es_AR';
import type { TournamentCategory, TournamentSet } from '@/shared/types/api.types';

const t = ES_AR.tournaments.result.errors;

/** How a category plays its matches: what the result form shows and what it accepts. */
export interface ScoreFormat {
  /** 1 is a single set, 2 is best of three. */
  setsToWin: 1 | 2;
  gamesPerSet: number;
  /** The deciding third set is a match tiebreak to 10; only ever on for best of three. */
  superTiebreak: boolean;
}

export interface ScoreRow {
  a: string;
  b: string;
}

export type ParsedScore = { ok: true; sets: TournamentSet[] } | { ok: false; error: string };

/** The format a category's matches follow, as the backend reads it (a match tiebreak never applies to a single set). */
export function scoreFormatOf(category: TournamentCategory): ScoreFormat {
  return {
    setsToWin: category.sets_to_win,
    gamesPerSet: category.games_per_set,
    superTiebreak: category.super_tiebreak_decider && category.sets_to_win === 2,
  };
}

/** The most sets a match can take, which is how many rows the form shows. */
export function scoreRowCount(format: ScoreFormat): number {
  return 2 * format.setsToWin - 1;
}

/** Whether the row at `index` is the match tiebreak (the last possible set, when the category plays one). */
export function isSuperRow(format: ScoreFormat, index: number): boolean {
  return format.superTiebreak && index === scoreRowCount(format) - 1;
}

// Same rules as the backend's `draw.ParseResult`, so a bad score is told here
// in Spanish before it is sent; the server stays the one that decides.
function regularSet(a: number, b: number, games: number): boolean {
  const hi = Math.max(a, b);
  const lo = Math.min(a, b);
  if (hi === games) return lo <= games - 2;
  if (hi === games + 1) return lo === games - 1 || lo === games;
  return false;
}

function superTiebreak(a: number, b: number): boolean {
  const hi = Math.max(a, b);
  const lo = Math.min(a, b);
  if (hi < 10) return false;
  return hi === 10 ? lo <= 8 : hi - lo === 2;
}

function isBlank(row: ScoreRow): boolean {
  return row.a.trim() === '' && row.b.trim() === '';
}

function readNumber(raw: string): number | null {
  return /^\d{1,2}$/.test(raw.trim()) ? Number(raw.trim()) : null;
}

/** The sets played: the rows up to the first blank one; a filled row after a blank one is a gap. */
function playedRows(rows: ScoreRow[]): { played: ScoreRow[]; gapAt: number | null } {
  const firstBlank = rows.findIndex(isBlank);
  if (firstBlank === -1) return { played: rows, gapAt: null };
  const afterGap = rows.findIndex((row, i) => i > firstBlank && !isBlank(row));
  return { played: rows.slice(0, firstBlank), gapAt: afterGap === -1 ? null : firstBlank + 1 };
}

function checkSet(set: TournamentSet, index: number, format: ScoreFormat): string | null {
  if (isSuperRow(format, index)) return superTiebreak(set.a, set.b) ? null : t.invalidSuper;
  return regularSet(set.a, set.b, format.gamesPerSet)
    ? null
    : t.invalidSet(index + 1, set.a, set.b, format.gamesPerSet);
}

function readSets(played: ScoreRow[], format: ScoreFormat): TournamentSet[] | string {
  const sets: TournamentSet[] = [];
  for (const [i, row] of played.entries()) {
    const a = readNumber(row.a);
    const b = readNumber(row.b);
    if (a === null || b === null) return t.notNumber;
    sets.push({ a, b, super: isSuperRow(format, i) });
  }
  return sets;
}

/** Sets each side won in `sets`, once the match is over a side has `setsToWin`. */
function checkOrder(sets: TournamentSet[], format: ScoreFormat): string | null {
  let wonA = 0;
  let wonB = 0;
  for (const [i, set] of sets.entries()) {
    if (wonA === format.setsToWin || wonB === format.setsToWin) return t.decided(i + 1);
    const error = checkSet(set, i, format);
    if (error) return error;
    if (set.a > set.b) wonA++;
    else wonB++;
  }
  if (wonA === format.setsToWin || wonB === format.setsToWin) return null;
  return format.superTiebreak ? t.decisiveSuper : t.undecided;
}

/**
 * Reads the result form's rows (a blank row is a set not played) into the sets
 * to send, or says in Spanish what is wrong. The row that can be the match
 * tiebreak is sent with `super: true`.
 */
export function parseScore(rows: ScoreRow[], format: ScoreFormat): ParsedScore {
  const { played, gapAt } = playedRows(rows);
  if (played.length === 0) return { ok: false, error: t.missing(1) };
  if (gapAt !== null) return { ok: false, error: t.gap(gapAt) };
  const sets = readSets(played, format);
  if (typeof sets === 'string') return { ok: false, error: sets };
  const error = checkOrder(sets, format);
  return error ? { ok: false, error } : { ok: true, sets };
}
