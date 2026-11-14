import { format } from 'date-fns/format';
import { parseISO } from 'date-fns/parseISO';
import { formatEventDay } from '@/shared/lib/eventDates';

/** A `YYYY-MM-DD` date, or a timestamp starting with one, as "14/12". */
export function formatDayMonth(date: string): string {
  return format(parseISO(date.slice(0, 10)), 'dd/MM');
}

/** A `YYYY-MM-DD` date as "Lunes 7 de diciembre de 2026": the one way a series date reads, year always there. */
export function formatSeriesDay(date: string): string {
  const day = formatEventDay(date);
  return `${day.charAt(0).toUpperCase()}${day.slice(1)} de ${date.slice(0, 4)}`;
}

/** The weekday of a `YYYY-MM-DD` date, 0 = Sunday, as the series API numbers them. */
export function weekdayOf(date: string): number {
  return parseISO(date.slice(0, 10)).getDay();
}
