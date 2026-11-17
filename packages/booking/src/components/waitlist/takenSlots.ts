import type { CourtAvailability } from '../../api/types';
import type { WaitlistSlotOption } from './WaitlistDialog';

/** Argentina has no daylight saving time, so the venue's offset is fixed. */
const VENUE_UTC_OFFSET = '-03:00';

/** When a slot starts: the day's midnight in the venue's zone plus its minutes, which run past 1440 overnight. */
function slotStart(date: string, startMin: number): number {
  return new Date(`${date}T00:00:00${VENUE_UTC_OFFSET}`).getTime() + startMin * 60_000;
}

/**
 * The day's taken slots that are still ahead, as options for "¿No está el horario que
 * buscás?": one per court and start time, soonest first, "19:00 · Pádel 1". A slot that
 * already started is left out: it cannot free for anyone.
 */
export function takenSlotOptions(
  courts: readonly CourtAvailability[],
  date: string,
  now: Date = new Date(),
): WaitlistSlotOption[] {
  const options = courts.flatMap((court) =>
    court.slots
      .filter((slot) => !slot.available && slotStart(date, slot.start_min) > now.getTime())
      .map((slot) => ({
        order: slot.start_min,
        option: {
          value: `${court.court_id}|${slot.start_time}|${String(slot.duration_minutes)}`,
          label: `${slot.start_time} · ${court.court_name}`,
          target: {
            kind: 'slot' as const,
            court_id: court.court_id,
            date,
            start_time: slot.start_time,
            duration_minutes: slot.duration_minutes,
          },
        },
      })),
  );
  return options
    .sort((a, b) => a.order - b.order || a.option.label.localeCompare(b.option.label))
    .map((entry) => entry.option);
}
