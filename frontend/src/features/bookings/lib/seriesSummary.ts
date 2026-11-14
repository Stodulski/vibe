import { addMinutes } from '@/shared/lib/time';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatDayMonthYear, weekdayOf } from './seriesDates';
import { keepTogether } from '@/shared/lib/keepTogether';

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

/** One fact of the summary, label and value. */
interface SummaryRow {
  label: string;
  value: string;
}

/**
 * What a weekly booking will do, one fact per row for the step where the owner
 * confirms it: the weekday it repeats on, its hours, its court, its first date
 * and its last one (or that it has none). Dates carry the year so a series that
 * crosses into the next one reads unambiguously.
 */
export function seriesSummaryRows({
  date,
  startTime,
  durationMinutes,
  courtName,
  end,
}: SeriesSummaryInput): SummaryRow[] {
  const s = t.bookings.series;
  const until = end.ends_on ?? (end.weeks ? lastOfWeeks(date, end.weeks) : undefined);
  return [
    { label: s.summaryRepeats, value: s.everyWeekday(s.weekdaysPlural[weekdayOf(date)] ?? '') },
    { label: s.summaryHours, value: s.hoursRange(startTime, addMinutes(startTime, durationMinutes)) },
    { label: s.summaryCourt, value: keepTogether(courtName) },
    { label: s.summarySince, value: formatDayMonthYear(date) },
    { label: s.summaryUntil, value: until ? formatDayMonthYear(until) : s.endNever },
  ];
}
