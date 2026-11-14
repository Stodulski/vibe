import { addMinutes } from '@/shared/lib/time';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatDayMonth, formatDayMonthYear, weekdayOf } from './seriesDates';

const t = ES_AR;

interface SeriesSummaryInput {
  /** First occurrence, `YYYY-MM-DD`. */
  date: string;
  startTime: string;
  durationMinutes: number;
  courtName: string;
  /** How the series ends, as the API takes it; empty for no end. */
  end: { ends_on?: string; weeks?: number };
}

/** The date of the last occurrence of a series counted in `weeks`, `YYYY-MM-DD`. */
function lastOfWeeks(date: string, weeks: number): string {
  const last = new Date(`${date}T00:00:00Z`);
  last.setUTCDate(last.getUTCDate() + (weeks - 1) * 7);
  return last.toISOString().slice(0, 10);
}

/**
 * The one line that says what a weekly booking will do, for the step where the
 * owner confirms it: "Todos los lunes de 18:00 a 19:30 en Cancha 1, desde el
 * 16/11", plus "hasta el …" once it has an end. Each "el" and its date are one
 * unbreakable unit, and both dates carry the year when the series crosses one.
 */
export function seriesSummary({ date, startTime, durationMinutes, courtName, end }: SeriesSummaryInput): string {
  const until = end.ends_on ?? (end.weeks ? lastOfWeeks(date, end.weeks) : undefined);
  // The year only when the series reaches into another one, and then on both ends.
  const crossesYear = !!until && until.slice(0, 4) !== date.slice(0, 4);
  const day = crossesYear ? formatDayMonthYear : formatDayMonth;
  return t.bookings.series.summary(
    t.bookings.series.weekdaysPlural[weekdayOf(date)] ?? '',
    startTime,
    addMinutes(startTime, durationMinutes),
    courtName,
    day(date),
    until ? day(until) : undefined,
  );
}
