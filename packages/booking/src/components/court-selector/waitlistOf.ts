import { BOOKING_MESSAGES } from '../../messages';
import type { WaitlistRequest } from '../waitlist/WaitlistDialog';
import type { RulerHour } from './rulerHours';

const w = BOOKING_MESSAGES.waitlist;

/**
 * The waitlist a taken hour opens. The visitor already chose the hour on the
 * ruler, so nothing is asked again: the wait is for every court taken then, and
 * whichever frees first sends the notice.
 */
export function waitlistOf(hour: RulerHour, date: string): WaitlistRequest {
  const [first] = hour.taken;
  return {
    title: w.title,
    description: `${w.slotAnyCourtPrefix} ${hour.startTime}.`,
    submitLabel: w.sendSlot,
    target: {
      kind: 'slot',
      court_ids: hour.taken.map(({ court }) => court.court_id),
      date,
      start_time: hour.startTime,
      duration_minutes: first?.slot.duration_minutes ?? 0,
    },
  };
}
