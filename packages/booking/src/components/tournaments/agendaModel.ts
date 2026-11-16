import type { Modality, PublicTournamentCategory, PublicTournamentSummary } from '../../api/types';
import { BOOKING_MESSAGES } from '../../messages';
import { categoryTitle, tournamentLabel } from '../../lib/traitLabels';
import { formatLabel, isOpenForSignUp, SHORT_MONTHS, spotsLeftLabel } from './tournamentModel';

const t = BOOKING_MESSAGES.tournaments;

const WEEKDAYS_SHORT = ['Dom', 'Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb'];
const WEEKDAYS_LONG = ['domingo', 'lunes', 'martes', 'miércoles', 'jueves', 'viernes', 'sábado'];

/** Up to this many places left, the agenda says the last places instead of counting them. */
const LAST_PLACES = 2;

/** One category on the agenda: when it starts, what it is called and which tournament it belongs to. */
export interface AgendaRow {
  category: PublicTournamentCategory;
  tournamentId: string;
  /** The small label above the title: "Torneo de pádel". */
  label: string;
  /** The day the category starts (`YYYY-MM-DD`): its own, or its tournament's first day when it has none. */
  date: string;
  /** `HH:MM`, or `null` for a category created before it had a time. */
  time: string | null;
  title: string;
}

export type ModalityFilter = 'all' | Modality;

export interface AgendaFilter {
  modality: ModalityFilter;
  /** A day (`YYYY-MM-DD`), or `null` for every day. */
  day: string | null;
}

export interface DayChip {
  date: string;
  /** "Sáb 5": the weekday and the day. */
  label: string;
}

export interface DayParts {
  /** "Sáb" */
  weekday: string;
  /** "sábado" */
  weekdayLong: string;
  /** "5", without a leading zero. */
  day: string;
  /** "dic" */
  month: string;
}

export interface SpotsNotice {
  text: string;
  /** True when the places are the last ones, which the agenda highlights. */
  last: boolean;
}

function compareTimes(a: string | null, b: string | null): number {
  if (a === b) return 0;
  // Within one day, a category without a time comes after the ones with one.
  if (a === null) return 1;
  if (b === null) return -1;
  return a.localeCompare(b);
}

function byWhen(a: AgendaRow, b: AgendaRow): number {
  return (
    a.date.localeCompare(b.date) ||
    compareTimes(a.time, b.time) ||
    a.title.localeCompare(b.title, 'es-AR') ||
    a.category.id.localeCompare(b.category.id)
  );
}

/**
 * The agenda: one row per category that is open for sign-up and has a place
 * left, of a tournament that is active, sorted by day and time. A category that
 * predates its start takes its tournament's first day and no time.
 */
export function agendaRows(tournaments: readonly PublicTournamentSummary[]): AgendaRow[] {
  const rows: AgendaRow[] = [];
  for (const tournament of tournaments) {
    if (tournament.status !== 'active') continue;
    for (const category of tournament.categories) {
      if (!isOpenForSignUp(category)) continue;
      rows.push({
        category,
        tournamentId: tournament.id,
        label: tournamentLabel(tournament.sport),
        date: category.starts_on ?? tournament.start_date,
        time: category.starts_at,
        title: categoryTitle(category),
      });
    }
  }
  return rows.sort(byWhen);
}

/** The rows that match a modality and a day. A category without a modality is only in "all". */
export function filterAgenda(rows: readonly AgendaRow[], filter: AgendaFilter): AgendaRow[] {
  return rows.filter((row) => {
    if (filter.modality !== 'all' && row.category.modality !== filter.modality) return false;
    return filter.day === null || row.date === filter.day;
  });
}

/** One chip per day the listed rows fall on, in order. */
export function dayChips(rows: readonly AgendaRow[]): DayChip[] {
  const dates = [...new Set(rows.map((row) => row.date))].sort();
  return dates.map((date) => {
    const parts = dayParts(date);
    return { date, label: `${parts.weekday} ${parts.day}` };
  });
}

/** The weekday, day and month of a `YYYY-MM-DD`, read from the string so no time zone can move it. */
export function dayParts(date: string): DayParts {
  const [year = 0, month = 1, day = 1] = date.split('-').map(Number);
  const weekdayIndex = new Date(Date.UTC(year, month - 1, day)).getUTCDay();
  return {
    weekday: WEEKDAYS_SHORT[weekdayIndex] ?? '',
    weekdayLong: WEEKDAYS_LONG[weekdayIndex] ?? '',
    day: String(day),
    month: SHORT_MONTHS[month - 1] ?? '',
  };
}

/** What a row says about its places: how many are left, or that they are the last ones. */
export function spotsNotice(spotsLeft: number): SpotsNotice {
  if (spotsLeft <= LAST_PLACES) return { text: t.lastSpots, last: true };
  return { text: spotsLeftLabel(spotsLeft), last: false };
}

/** The format in words, with a group stage's size and how many pass: "Zonas y cuadro · Zonas de 4 parejas, pasan 2". */
export function formatDetailLabel(
  category: Pick<PublicTournamentCategory, 'format' | 'group_size' | 'qualifiers_per_group'>,
): string {
  const label = formatLabel(category.format);
  if (category.format !== 'groups') return label;
  const zones = `${t.zonesOf} ${String(category.group_size)} ${t.pairsPassed} ${String(category.qualifiers_per_group)}`;
  return `${label} · ${zones}`;
}

/** "Sábado 05/12 · 18:00 hs": the day and, when it has one, the time. */
export function categoryWhen(row: Pick<AgendaRow, 'date' | 'time'>): string {
  const parts = dayParts(row.date);
  const [, month = '', day = ''] = row.date.split('-');
  const dayText = `${parts.weekdayLong.charAt(0).toUpperCase()}${parts.weekdayLong.slice(1)} ${day}/${month}`;
  return row.time === null ? dayText : `${dayText} · ${row.time} ${t.hoursSuffix}`;
}
