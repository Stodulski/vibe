import { BOOKING_MESSAGES } from '../../messages';
import type { WaitlistRequest } from '../waitlist/WaitlistDialog';
import type { RulerHour } from './rulerHours';

const w = BOOKING_MESSAGES.waitlist;

/** The waitlist a taken hour opens: one option per court taken at that hour. */
export function waitlistOf(hour: RulerHour, date: string): WaitlistRequest {
  return {
    title: w.title,
    description: w.slotDescription,
    submitLabel: w.sendSlot,
    slots: hour.taken.map(({ court, slot }) => ({
      value: court.court_id,
      label: `${slot.start_time} · ${court.court_name}`,
      target: {
        kind: 'slot',
        court_id: court.court_id,
        date,
        start_time: slot.start_time,
        duration_minutes: slot.duration_minutes,
      },
    })),
  };
}
