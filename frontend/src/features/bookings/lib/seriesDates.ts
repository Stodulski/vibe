import { format } from 'date-fns/format';
import { parseISO } from 'date-fns/parseISO';
import { es } from 'date-fns/locale/es';

/** A `YYYY-MM-DD` date as "mar 18 nov". `parseISO` reads a date-only string as local midnight, so no day shifts. */
export function formatDateOnly(date: string): string {
  return format(parseISO(date), 'EEE d MMM', { locale: es });
}

/** A `YYYY-MM-DD` date, or a timestamp starting with one, as "14/12". */
export function formatDayMonth(date: string): string {
  return format(parseISO(date.slice(0, 10)), 'dd/MM');
}

/** The weekday of a `YYYY-MM-DD` date, 0 = Sunday, as the series API numbers them. */
export function weekdayOf(date: string): number {
  return parseISO(date.slice(0, 10)).getDay();
}
