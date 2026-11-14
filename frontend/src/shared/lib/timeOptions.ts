import { generateTimeSlots, timeToMinutes } from './time';

interface OpeningHours {
  open_time: string;
  close_time: string;
  is_closed: boolean;
}

const DAY = 24 * 60;

function pad(minutes: number): string {
  const m = ((minutes % DAY) + DAY) % DAY;
  return `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`;
}

/** Half-hour marks from `open` to `close`, both included; a close not after the open means the next day. */
function marksBetween(open: string, close: string): string[] {
  const from = timeToMinutes(open);
  const to = timeToMinutes(close);
  const end = to > from ? to : to + DAY;
  const marks: string[] = [];
  for (let m = from; m <= end; m += 30) marks.push(pad(m));
  return marks;
}

/**
 * The half-hour marks a picker offers for something that happens inside a
 * complex: from its earliest opening to its latest closing across the open
 * days, so an event is never scheduled at an hour the complex is shut.
 *
 * A closing after midnight keeps going past it: the list starts after the
 * longest stretch with no mark (the hours the complex is shut) and wraps, so it
 * reads in the order of the clock. No schedule, or every day closed, offers the
 * whole day rather than an empty list. `keep` adds the values the field already
 * holds even when they fall outside, so editing an old record never blanks its
 * select.
 */
export function timeOptionsForOpeningHours(
  schedules: readonly OpeningHours[] | undefined,
  keep: readonly string[] = [],
): string[] {
  const open = (schedules ?? []).filter((s) => !s.is_closed);
  const marks = new Set<string>(
    open.length === 0 ? generateTimeSlots() : open.flatMap((s) => marksBetween(s.open_time, s.close_time)),
  );
  for (const value of keep) if (value) marks.add(value);

  const sorted = [...marks].sort((a, b) => timeToMinutes(a) - timeToMinutes(b));
  // Start right after the widest gap between neighbouring marks (the last one wraps to the first).
  let start = 0;
  let widest = -1;
  sorted.forEach((mark, i) => {
    const next = sorted[(i + 1) % sorted.length] ?? mark;
    const gap = (timeToMinutes(next) - timeToMinutes(mark) + DAY) % DAY || DAY;
    if (gap > widest) {
      widest = gap;
      start = (i + 1) % sorted.length;
    }
  });
  return [...sorted.slice(start), ...sorted.slice(0, start)];
}
