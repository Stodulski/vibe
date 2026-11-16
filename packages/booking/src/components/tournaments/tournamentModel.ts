import { formatDateFull } from '@vibe/ui';
import { keepTogether } from '../../lib/keepTogether';
import { BOOKING_MESSAGES } from '../../messages';
import type {
  PublicTournament,
  PublicTournamentCategory,
  PublicTournamentMatch,
  PublicTournamentPair,
  PublicTournamentSchedule,
  PublicTournamentSummary,
} from '../../api/types';

const t = BOOKING_MESSAGES.tournaments;
const SPORT_LABELS: Record<string, string> = BOOKING_MESSAGES.courts.sportTypes;
const FORMAT_LABELS: Record<string, string> = {
  groups: t.formatGroups,
  elimination: t.formatElimination,
};

export function sportLabel(sport: string): string {
  return SPORT_LABELS[sport] ?? sport;
}

/** The label of a category's format, or the raw value for one this build predates. */
export function formatLabel(format: string): string {
  return FORMAT_LABELS[format] ?? format;
}

const STANDARD_GAMES_PER_SET = 6;

/**
 * How a category's matches are played, e.g. "Al mejor de 3 sets · super
 * tie-break" or "A 1 set de 4 games". The games are named only when they are
 * not the standard 6 of a best of three.
 */
export function matchFormatLabel(
  category: Pick<PublicTournamentCategory, 'sets_to_win' | 'games_per_set' | 'super_tiebreak_decider'>,
): string {
  const games = `${t.matchGamesPrefix} ${String(category.games_per_set)} ${t.matchGamesSuffix}`;
  if (category.sets_to_win === 1) return `${t.matchSingleSet} ${games}`;
  const sets = category.games_per_set === STANDARD_GAMES_PER_SET ? t.matchBestOfThree : `${t.matchBestOfThree} ${games}`;
  return category.super_tiebreak_decider ? `${sets} \u00b7 ${t.matchSuperTiebreak}` : sets;
}

export function spotsLeftLabel(spotsLeft: number): string {
  if (spotsLeft === 1) return t.spotsLeftOne;
  return `${t.spotsLeftPrefix} ${String(spotsLeft)} ${t.spotsLeftSuffix}`;
}

/** Whether a category still takes pairs. The server decides for real; this only shapes the page. */
export function isOpenForSignUp(category: Pick<PublicTournamentCategory, 'status' | 'spots_left'>): boolean {
  return category.status === 'open' && category.spots_left > 0;
}

/** Whether a category's sign-up is closed: the state a card names "Inscripción cerrada". */
export function isSignUpClosed(category: Pick<PublicTournamentCategory, 'status'>): boolean {
  return category.status === 'closed';
}

/**
 * The tournaments the public page lists, each with only the categories whose
 * sign-up is not closed: a full or cancelled category still says so. A
 * tournament left with no such category is not listed at all.
 */
export function listedTournaments(tournaments: PublicTournamentSummary[]): PublicTournamentSummary[] {
  return tournaments.flatMap((tournament) => {
    const categories = tournament.categories.filter((category) => !isSignUpClosed(category));
    return categories.length > 0 ? [{ ...tournament, categories }] : [];
  });
}

export type PairLookup = ReadonlyMap<string, PublicTournamentPair>;

export function pairLookup(pairs: readonly PublicTournamentPair[]): PairLookup {
  return new Map(pairs.map((p) => [p.id, p]));
}

/** One line per player, first and last name. Never a phone or an email: the page has none. */
export function pairLines(pair: PublicTournamentPair | undefined): string[] {
  if (!pair) return [];
  return pair.players.map((p) => `${p.first_name} ${p.last_name}`.trim());
}

/** The name of the round, counted back from the final. */
export function roundLabel(round: number, totalRounds: number): string {
  switch (totalRounds - round) {
    case 0:
      return t.roundFinal;
    case 1:
      return t.roundSemifinal;
    case 2:
      return t.roundQuarterfinal;
    case 3:
      return t.roundOf16;
    default:
      return `${t.roundPrefix} ${String(round)}`;
  }
}

export interface BracketRound {
  round: number;
  label: string;
  matches: PublicTournamentMatch[];
}

/** The bracket as columns: the first round first, each column's matches top to bottom. */
export function bracketRounds(matches: readonly PublicTournamentMatch[]): BracketRound[] {
  const bracket = matches.filter((m) => m.stage === 'bracket');
  if (bracket.length === 0) return [];
  const totalRounds = Math.max(...bracket.map((m) => m.round));
  const rounds = [...new Set(bracket.map((m) => m.round))].sort((a, b) => a - b);
  return rounds.map((round) => ({
    round,
    label: roundLabel(round, totalRounds),
    matches: bracket.filter((m) => m.round === round).sort((a, b) => a.position - b.position),
  }));
}

function gcd(a: number, b: number): number {
  return b === 0 ? a : gcd(b, a % b);
}

/**
 * The shared row grid of a bracket: every round column spans the same `rows`,
 * and each match of a round takes `spans[round]` of them. A match then sits on
 * the midpoint of the two matches that feed it, because their rows are exactly
 * its rows split in half.
 */
export interface BracketGrid {
  rows: number;
  spans: number[];
}

export function bracketGrid(counts: readonly number[]): BracketGrid {
  const rows = counts.reduce((acc, count) => (count > 0 ? (acc * count) / gcd(acc, count) : acc), 1);
  return { rows, spans: counts.map((count) => (count > 0 ? rows / count : rows)) };
}

export type Slot =
  | { kind: 'pair'; lines: string[] }
  | { kind: 'source' | 'bye' | 'tbd'; text: string };

/** What a bracket slot shows: its pair, the group place it waits for, a bye, or nothing yet. */
export function slotLabel(
  match: PublicTournamentMatch,
  side: 1 | 2,
  pairs: PairLookup,
  groupNames: ReadonlyMap<string, string>,
): Slot {
  const pairId = side === 1 ? match.pair1_id : match.pair2_id;
  const pair = pairId ? pairs.get(pairId) : undefined;
  if (pair) return { kind: 'pair', lines: pairLines(pair) };
  if (match.bye) return { kind: 'bye', text: t.bye };
  const source = side === 1 ? match.source1 : match.source2;
  if (source) {
    const name = groupNames.get(source.group_id);
    if (name) return { kind: 'source', text: `${String(source.rank)}${t.sourcePosition} ${name}` };
  }
  return { kind: 'tbd', text: t.toBeDefined };
}

/** The side that won a match loaded without a score (no sets, no walkover), or null for any other match. */
export function winnerOnlySide(match: PublicTournamentMatch): 1 | 2 | null {
  if (match.status !== 'completed' || match.bye || match.walkover_loser !== 0 || match.sets.length > 0) return null;
  if (match.winner_pair_id === null) return null;
  if (match.winner_pair_id === match.pair1_id) return 1;
  return match.winner_pair_id === match.pair2_id ? 2 : null;
}

/** One side's score cells: its games per set (a match tiebreak in brackets), W.O. for the side that forfeited, or "Ganó" for the winner of a match with no score. */
export function scoreCells(match: PublicTournamentMatch, side: 1 | 2): string[] {
  if (match.walkover_loser !== 0) return match.walkover_loser === side ? [t.walkover] : [];
  if (winnerOnlySide(match) !== null) return winnerOnlySide(match) === side ? [t.won] : [];
  return match.sets.map((set) => {
    const games = String(side === 1 ? set.a : set.b);
    return set.super ? `[${games}]` : games;
  });
}

/** "6-4 6-3", a match tiebreak as "[8-10]", or W.O.; empty before a result. */
export function scoreText(match: PublicTournamentMatch): string {
  if (match.walkover_loser !== 0) return t.walkover;
  return match.sets.map((set) => (set.super ? `[${String(set.a)}-${String(set.b)}]` : `${String(set.a)}-${String(set.b)}`)).join(' ');
}

export interface Matchday {
  round: number;
  matches: PublicTournamentMatch[];
}

/** A group's matches by matchday. */
export function groupMatchdays(matches: readonly PublicTournamentMatch[], groupId: string): Matchday[] {
  const own = matches.filter((m) => m.stage === 'group' && m.group_id === groupId);
  const rounds = [...new Set(own.map((m) => m.round))].sort((a, b) => a - b);
  return rounds.map((round) => ({
    round,
    matches: own.filter((m) => m.round === round).sort((a, b) => a.position - b.position),
  }));
}

export interface ScheduleDay {
  date: string;
  matches: PublicTournamentMatch[];
}

/** The scheduled matches by day, then time, then court. */
export function scheduleByDay(matches: readonly PublicTournamentMatch[]): ScheduleDay[] {
  const scheduled = matches.filter((m) => m.schedule !== null && !m.bye);
  scheduled.sort((a, b) => {
    const x = a.schedule;
    const y = b.schedule;
    if (!x || !y) return 0;
    return (
      x.date.localeCompare(y.date) || x.start_time.localeCompare(y.start_time) || x.court_name.localeCompare(y.court_name)
    );
  });
  const days: ScheduleDay[] = [];
  for (const match of scheduled) {
    const date = match.schedule?.date ?? '';
    const last = days.at(-1);
    if (last?.date === date) last.matches.push(match);
    else days.push({ date, matches: [match] });
  }
  return days;
}

/** "Zona A, fecha 1" or the round of the bracket. */
export function matchStage(
  match: PublicTournamentMatch,
  groupNames: ReadonlyMap<string, string>,
  bracketRoundsCount: number,
): string {
  if (match.stage === 'group') {
    const name = match.group_id ? groupNames.get(match.group_id) : undefined;
    const matchday = `${t.matchdayPrefix} ${String(match.round)}`;
    return name ? `${t.groupTitlePrefix} ${name}, ${matchday.toLowerCase()}` : matchday;
  }
  return roundLabel(match.round, bracketRoundsCount);
}

export function qualifiersNote(count: number): string {
  if (count === 1) return t.qualifiersNoteOne;
  return `${t.qualifiersNotePrefix} ${String(count)} ${t.qualifiersNoteSuffix}`;
}

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

const SHORT_MONTHS = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic'];

/** The day number and short month of a `YYYY-MM-DD`, read from the string so no time zone can move it. */
function shortDay(day: string): { number: string; month: string } {
  const [, month = '', date = ''] = day.split('-');
  return { number: String(Number(date)), month: SHORT_MONTHS[Number(month) - 1] ?? '' };
}

/**
 * "Sábado 12 de diciembre" for one day. A range is written short so it fits a
 * line at phone width: "12 al 13 dic" within one month, "28 nov al 1 dic"
 * across two.
 */
export function tournamentDates(tournament: Pick<PublicTournament, 'start_date' | 'end_date'>): string {
  if (tournament.end_date === tournament.start_date) return capitalize(formatDateFull(tournament.start_date));
  const first = shortDay(tournament.start_date);
  const last = shortDay(tournament.end_date);
  if (first.month === last.month) return `${first.number} ${t.datesSeparator} ${last.number} ${last.month}`;
  return `${first.number} ${first.month} ${t.datesSeparator} ${last.number} ${last.month}`;
}

/** "Cancha 1 · sábado 12 de diciembre · 10:00": one way to write when and where a match is played. The court never breaks. */
export function scheduleLabel(schedule: Pick<PublicTournamentSchedule, 'court_name' | 'date' | 'start_time'>): string {
  const court = keepTogether(schedule.court_name);
  return `${court} \u00b7 ${formatDateFull(schedule.date)} \u00b7 ${schedule.start_time}`;
}

