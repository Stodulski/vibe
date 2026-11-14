import { addDays } from 'date-fns/addDays';
import { format } from 'date-fns/format';
import { parseISO } from 'date-fns/parseISO';

interface DateRange {
  from: string;
  to: string;
}

const iso = (date: Date) => format(date, 'yyyy-MM-dd');

/**
 * Events from today on. The API takes at most 366 days per request and lets an
 * event be created up to 365 days ahead, so one request covers every upcoming
 * event there can be.
 */
export function upcomingRange(today: string): DateRange {
  return { from: today, to: iso(addDays(parseISO(today), 365)) };
}

/** The year before today, ending yesterday: what "Pasados" lists. */
export function pastRange(today: string): DateRange {
  const base = parseISO(today);
  return { from: iso(addDays(base, -365)), to: iso(addDays(base, -1)) };
}
