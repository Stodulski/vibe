import { timeToMinutes } from '@/shared/lib/time';
import { toDisplayDate } from '@/shared/lib/utils';
import type { DayType, Schedule } from '@/shared/types/api.types';

const DAY_NAMES: readonly DayType[] = ['sunday', 'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday'];

/** Minutes in a day. A window running past midnight is measured beyond it. */
const MINUTES_PER_DAY = 24 * 60;

/**
 * The opening window an hour was sold out of, and where in that window it sits.
 *
 * Both halves price it. `day` selects the rate card and `startMin` is minutes
 * from that window's own day's midnight, exceeding 1440 for an hour past the
 * rollover — a 00:30 slot from a Thursday 08:00–01:30 window is Thursday
 * minute 1470, and Thursday's bands are laid out across the same span.
 */
export interface PriceWindow {
  day: DayType;
  startMin: number;
}

/**
 * Resolves which day's rate card prices an hour.
 *
 * This mirrors `priceWindow` in the server's `internal/bookings` package,
 * deliberately and exactly. It is the one piece of pricing logic that exists
 * twice, because the preview has to answer before the booking is submitted and
 * the charge has to answer after — and a preview that disagrees with the charge
 * is worse than no preview. Change one and change the other.
 *
 * The night still open wins. If Thursday trades 08:00 to 01:30 and Friday opens
 * at 00:00, a Friday 00:30 booking falls inside both windows and is Thursday's:
 * a session does not become the next day's at midnight just because the venue
 * also opened then.
 *
 * It always resolves. An hour no window claims still has a card — its own
 * weekday's — because staff book days the venue is shut and a card is per
 * weekday, not per opening.
 */
export function resolvePriceWindow(schedules: Schedule[], date: string, startTime: string): PriceWindow {
  const startMin = timeToMinutes(startTime);
  const shown = toDisplayDate(date);
  const ownDay = DAY_NAMES[shown.getDay()] ?? 'monday';
  const previousDay = DAY_NAMES[(shown.getDay() + 6) % 7] ?? 'monday';

  // Last night first, asked as "are you still open at this hour tomorrow".
  const night = windowFor(schedules, previousDay);
  if (night && covers(night, startMin + MINUTES_PER_DAY)) {
    return { day: previousDay, startMin: startMin + MINUTES_PER_DAY };
  }

  // Then the date's own window, asked about its own hours.
  const own = windowFor(schedules, ownDay);
  if (own && covers(own, startMin)) {
    return { day: ownDay, startMin };
  }

  return { day: ownDay, startMin };
}

interface OpeningWindow {
  openMin: number;
  closeMin: number;
}

/**
 * One weekday's opening window in minutes, with a close at or before the open
 * read as closing after midnight — the same reading `complex_schedules` has
 * always used for a venue trading past midnight.
 *
 * A closed day has no window, and a weekday with no row at all is the same
 * fact: nothing says the venue opens.
 */
function windowFor(schedules: Schedule[], day: DayType): OpeningWindow | null {
  const row = schedules.find((s) => s.day === day);
  if (!row || row.is_closed) return null;

  const openMin = timeToMinutes(row.open_time);
  let closeMin = timeToMinutes(row.close_time);
  if (closeMin <= openMin) closeMin += MINUTES_PER_DAY;
  return { openMin, closeMin };
}

function covers(w: OpeningWindow, minute: number): boolean {
  return minute >= w.openMin && minute < w.closeMin;
}
