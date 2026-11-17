import type { Modality } from '../../api/types';
import { MODALITIES, modalityLabel } from '../../lib/traitLabels';

/**
 * What every agenda of the public complex page shares: the day of a row, the
 * places it says, and the filters above the list. Tournaments and open play
 * each add their own rows to it.
 */

const WEEKDAYS_SHORT = ['Dom', 'Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb'];
const WEEKDAYS_LONG = ['domingo', 'lunes', 'martes', 'miércoles', 'jueves', 'viernes', 'sábado'];
export const SHORT_MONTHS = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic'];

/** Up to this many places left, an agenda says the last places instead of counting them. */
const LAST_PLACES = 2;

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

export interface DayChip {
  date: string;
  /** "Sáb 5 dic": the weekday, the day and the month. */
  label: string;
}

export interface SpotsCopy {
  lastSpots: string;
  spotsLeft: (count: number) => string;
}

export interface SpotsNotice {
  text: string;
  /** True when the places are the last ones, which the agenda highlights. */
  last: boolean;
}

/** The filters above an agenda: one modality (or all) and one day (or every day). */
export interface AgendaFilter {
  modality: 'all' | Modality;
  /** A day (`YYYY-MM-DD`), or `null` for every day. */
  day: string | null;
}

export interface ModalityOption {
  value: Modality;
  /** The modality in the words of the rows' sports: "Caballeros", "Masculino". */
  label: string;
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

/** "Sábado 05/12": the long weekday, then the day and the month. */
export function longDayLabel(date: string): string {
  const parts = dayParts(date);
  const [, month = '', day = ''] = date.split('-');
  return `${parts.weekdayLong.charAt(0).toUpperCase()}${parts.weekdayLong.slice(1)} ${day}/${month}`;
}

/** One chip per day the listed rows fall on, in order. */
export function dayChips(rows: readonly { date: string }[]): DayChip[] {
  const dates = [...new Set(rows.map((row) => row.date))].sort();
  return dates.map((date) => {
    const parts = dayParts(date);
    return { date, label: `${parts.weekday} ${parts.day} ${parts.month}` };
  });
}

/** What a row says about its places: how many are left, or that they are the last ones. */
export function spotsNotice(spotsLeft: number, copy: SpotsCopy): SpotsNotice {
  if (spotsLeft <= LAST_PLACES) return { text: copy.lastSpots, last: true };
  return { text: copy.spotsLeft(spotsLeft), last: false };
}

/** The rows that match a modality and a day. A row without a modality is only in "all". */
export function matchesAgendaFilter(
  modality: string | null | undefined,
  date: string,
  filter: AgendaFilter,
): boolean {
  if (filter.modality !== 'all' && modality !== filter.modality) return false;
  return filter.day === null || date === filter.day;
}

/**
 * One chip per modality the rows have, in the order of the modalities. Each is
 * named in the words of the sports that have it: "Caballeros" for pádel,
 * "Masculino" for fútbol, and both when two sports name it differently.
 */
export function modalityOptions(
  rows: readonly { modality?: string | null | undefined; sport: string }[],
): ModalityOption[] {
  return MODALITIES.flatMap((value) => {
    const words = new Set(rows.filter((row) => row.modality === value).map((row) => modalityLabel(value, row.sport)));
    return words.size === 0 ? [] : [{ value, label: [...words].join(' / ') }];
  });
}
