import { ES_AR } from '@/shared/i18n/es_AR';
import { formatDateFull, formatTime } from '@/shared/lib/utils';

/**
 * How an event's day and hours read wherever one is listed: "viernes 20 de
 * noviembre · 19:00 a 21:00". Canchas abiertas and torneos share it so the same
 * kind of thing is never dated two ways; a screen styles the day and the hours
 * apart by using the two parts, or takes the whole line from `formatEventWhen`.
 */
export function formatEventDay(date: string): string {
  return formatDateFull(date);
}

export function formatEventTimeRange(start: string, end: string): string {
  return `${formatTime(start)} a ${formatTime(end)}`;
}

export function formatEventWhen(date: string, start: string, end: string): string {
  return `${formatEventDay(date)} · ${formatEventTimeRange(start, end)}`;
}

/**
 * The day in figures for a phone line where the long date would wrap: "Dom 22/11".
 * Read from the `YYYY-MM-DD` string, like `eventDayParts`, so no time zone can move it.
 */
export function formatEventDayShort(date: string): string {
  const [, month = '01', day = '01'] = date.split('-');
  return `${eventDayParts(date).weekday} ${day}/${month}`;
}

export interface EventDayParts {
  /** "Mié" */
  weekday: string;
  /** "18", without a leading zero. */
  day: string;
  /** "nov" */
  month: string;
}

/** The weekday, day and month of a `YYYY-MM-DD`, read from the string so no time zone can move it. */
export function eventDayParts(date: string): EventDayParts {
  const [year = 0, month = 1, day = 1] = date.split('-').map(Number);
  const weekdayIndex = new Date(Date.UTC(year, month - 1, day)).getUTCDay();
  return {
    weekday: ES_AR.common.weekdaysShort[weekdayIndex] ?? '',
    day: String(day),
    month: ES_AR.common.monthsShort[month - 1] ?? '',
  };
}
