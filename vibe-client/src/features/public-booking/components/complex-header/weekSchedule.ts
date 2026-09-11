import { ES_AR } from '@/shared/i18n/es_AR';
import type { Schedule } from '@/shared/types/api.types';
import { DAY_NAMES, getTodayName } from './dayNames';

const t = ES_AR;

/**
 * The week starts on Monday here, not on Sunday.
 *
 * `WEEKDAYS` is indexed by `Date.getDay()`, so it has to start on Sunday to
 * answer "what day is it". Nobody reads a venue's opening hours that way —
 * a list that opens on Sunday and closes on Saturday splits the weekend
 * across both ends of the block.
 */
const WEEK_ORDER = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'] as const;

export interface ScheduleRow {
  day: string;
  /** "Lunes". */
  label: string;
  /** "08:00 - 23:00", or the closed label. */
  hours: string;
  isToday: boolean;
}

function hoursOf(schedule: Schedule | undefined): string {
  if (!schedule || schedule.is_closed) return t.publicBooking.closed;
  return `${schedule.open_time} - ${schedule.close_time}`;
}

/**
 * The seven days, each on its own line, Monday first.
 *
 * Runs of identical days used to be merged into "Lunes a Miércoles". It reads
 * well and it is how opening hours are usually written, but it makes the
 * reader do the expansion: someone checking Wednesday has to notice they are
 * inside a range rather than find their own day. Seven labelled rows in two
 * columns cost no more space than five and are scanned, not parsed.
 */
/**
 * @param highlightDay Which row reads as "today" — pass the day name for the
 * date the person actually has open (`WeekScheduleList` reads it from the
 * URL's `?date=`) so the bolded row matches what they are looking at rather
 * than the calendar date. Defaults to the real today when omitted.
 */
export function buildWeekSchedule(schedules: Schedule[], highlightDay?: string): ScheduleRow[] {
  const target = highlightDay ?? getTodayName();

  return WEEK_ORDER.map((day) => ({
    day,
    label: DAY_NAMES[day] ?? day,
    hours: hoursOf(schedules.find((s) => s.day === day)),
    isToday: day === target,
  }));
}
